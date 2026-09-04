package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"studygo/internal/domain/tec"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// maxPlanilhaTEC limita o CSV aceito: a exportação do TEC é pequena, e um
// arquivo muito maior que isso é engano ou abuso.
const maxPlanilhaTEC = 5 << 20

// PlanoHandler serve tudo sob /api/concursos/{slug}/plano.
//
// Os handlers são finos de propósito: autenticam, decodificam, chamam o caso de
// uso e serializam. Regra de negócio nenhuma mora aqui.
type PlanoHandler struct {
	planos      *service.PlanoService
	cronograma  *service.CronogramaService
	registros   *service.RegistroService
	estatistica *service.EstatisticaService
	caderno     *service.CadernoService
	dossie      *service.DossieService
	exportacao  *service.ExportacaoService
	tec         *service.ImportacaoTECService
	logger      *slog.Logger
}

func NewPlanoHandler(
	planos *service.PlanoService,
	cronograma *service.CronogramaService,
	registros *service.RegistroService,
	estatistica *service.EstatisticaService,
	caderno *service.CadernoService,
	dossie *service.DossieService,
	exportacao *service.ExportacaoService,
	tec *service.ImportacaoTECService,
	logger *slog.Logger,
) *PlanoHandler {
	return &PlanoHandler{
		planos:      planos,
		cronograma:  cronograma,
		registros:   registros,
		estatistica: estatistica,
		caderno:     caderno,
		dossie:      dossie,
		exportacao:  exportacao,
		tec:         tec,
		logger:      logger,
	}
}

// contexto extrai o usuário autenticado e o slug da rota.
func (h *PlanoHandler) contexto(r *http.Request) (uuid.UUID, string, bool) {
	id, ok := usuarioID(r.Context())
	if !ok {
		return uuid.Nil, "", false
	}

	return id, r.PathValue("slug"), true
}

// responderPlano é o fim de quase toda rota do plano: elas devolvem o plano
// remontado, para que a tela não precise de uma segunda requisição.
func (h *PlanoHandler) responderPlano(
	w http.ResponseWriter,
	r *http.Request,
	p service.PlanoMontado,
	err error,
) {
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, planoParaDTO(p))
}

func (h *PlanoHandler) Obter(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	p, err := h.planos.Obter(r.Context(), id, slug)
	h.responderPlano(w, r, p, err)
}

type configRequest struct {
	Inicio        string         `json:"inicio"`
	Prova         string         `json:"prova"`
	HorasDia      *float64       `json:"horasDia"`
	DiasEstudo    []int          `json:"diasEstudo"`
	DiaRevisao    *int           `json:"diaRevisao"`
	RetaFinalDias *int           `json:"retaFinalDias"`
	Questoes      map[string]int `json:"questoes"`

	BlocosPorDia   *int               `json:"blocosPorDia"`
	MinutosBloco   *int               `json:"minutosBloco"`
	MinutosRevisao *int               `json:"minutosRevisao"`
	Reforcos       map[string]float64 `json:"reforcos"`
	CicloRevisao   *[]itemCicloDTO    `json:"cicloRevisao"`
	RevisaoSemanal *bool              `json:"revisaoSemanal"`
	Simulados      *string            `json:"simulados"`
	Discursiva     *bool              `json:"discursiva"`
	Modos          map[string]string  `json:"modos"`
	PctQuestoes    *float64           `json:"pctQuestoes"`
	LimiarFraco    *int               `json:"limiarFraco"`
}

func (h *PlanoHandler) Salvar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req configRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	cmd := service.ConfigCommand{
		Inicio:         req.Inicio,
		Prova:          req.Prova,
		HorasDia:       req.HorasDia,
		DiasEstudo:     req.DiasEstudo,
		DiaRevisao:     req.DiaRevisao,
		RetaFinalDias:  req.RetaFinalDias,
		Questoes:       req.Questoes,
		BlocosPorDia:   req.BlocosPorDia,
		MinutosBloco:   req.MinutosBloco,
		MinutosRevisao: req.MinutosRevisao,
		Reforcos:       req.Reforcos,
		RevisaoSemanal: req.RevisaoSemanal,
		Simulados:      req.Simulados,
		Discursiva:     req.Discursiva,
		Modos:          req.Modos,
		PctQuestoes:    req.PctQuestoes,
		LimiarFraco:    req.LimiarFraco,
	}

	if req.CicloRevisao != nil {
		ciclo := make([]service.ItemCicloCommand, 0, len(*req.CicloRevisao))
		for _, it := range *req.CicloRevisao {
			ciclo = append(ciclo, service.ItemCicloCommand{
				Titulo: it.Titulo, Questoes: it.Questoes,
			})
		}

		cmd.CicloRevisao = &ciclo
	}

	p, err := h.planos.Salvar(r.Context(), id, slug, cmd)
	h.responderPlano(w, r, p, err)
}

type registroRequest struct {
	AtividadeID string   `json:"atividadeId"`
	Horas       *float64 `json:"horas"`
	Questoes    *int     `json:"questoes"`
	Acertos     *int     `json:"acertos"`
	Nota        string   `json:"nota"`
	Concluido   bool     `json:"concluido"`
}

// Registrar lança o resultado de UMA atividade. A conclusão do dia não é
// enviada pelo cliente: ela é derivada das atividades daquele dia.
func (h *PlanoHandler) Registrar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req registroRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	atividadeID, err := uuid.Parse(req.AtividadeID)
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	p, err := h.registros.Registrar(r.Context(), id, slug, service.RegistroCommand{
		AtividadeID: atividadeID,
		Horas:       req.Horas,
		Questoes:    req.Questoes,
		Acertos:     req.Acertos,
		Nota:        req.Nota,
		Concluido:   req.Concluido,
	})
	h.responderPlano(w, r, p, err)
}

type registroDiaRequest struct {
	Nota       string `json:"nota"`
	Questoes   *int   `json:"questoes"`
	Acertos    *int   `json:"acertos"`
	Observacao string `json:"observacao"`
}

// RegistrarDia grava o que pertence ao dia: a anotação livre e o resultado da
// cauda de revisão.
func (h *PlanoHandler) RegistrarDia(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req registroDiaRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	p, err := h.registros.RegistrarDia(r.Context(), id, slug, service.RegistroDiaCommand{
		Data:       r.PathValue("data"),
		Nota:       req.Nota,
		Questoes:   req.Questoes,
		Acertos:    req.Acertos,
		Observacao: req.Observacao,
	})
	h.responderPlano(w, r, p, err)
}

func (h *PlanoHandler) LimparRegistros(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	p, err := h.registros.LimparRegistros(r.Context(), id, slug)
	h.responderPlano(w, r, p, err)
}

type marcarMarcoRequest struct {
	Cumprido bool `json:"cumprido"`
}

func (h *PlanoHandler) MarcarMarco(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	marcoID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	var req marcarMarcoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	p, err := h.planos.MarcarMarco(r.Context(), id, slug, marcoID, req.Cumprido)
	h.responderPlano(w, r, p, err)
}

type cadernoDisciplinaRequest struct {
	CadernoURL string `json:"cadernoUrl"`
}

func (h *PlanoHandler) AtualizarCadernoDisciplina(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req cadernoDisciplinaRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	p, err := h.planos.AtualizarCadernoDisciplina(
		r.Context(), id, slug, r.PathValue("codigo"), req.CadernoURL,
	)
	h.responderPlano(w, r, p, err)
}

type moverRequest struct {
	ID      string `json:"id"`
	Data    string `json:"data"`
	Posicao int    `json:"posicao"`
	Trocar  bool   `json:"trocar"`
}

func (h *PlanoHandler) Mover(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req moverRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	atividadeID, err := uuid.Parse(req.ID)
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	p, err := h.cronograma.Mover(r.Context(), id, slug, service.MoverCommand{
		ID: atividadeID, Data: req.Data, Posicao: req.Posicao, Trocar: req.Trocar,
	})
	h.responderPlano(w, r, p, err)
}

type anteciparRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

// Antecipar salta uma atividade de um dia futuro direto para hoje. Mover anda
// uma vaga por vez; isto é a operação que faz o salto inteiro.
func (h *PlanoHandler) Antecipar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req anteciparRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	atividadeID, err := uuid.Parse(req.ID)
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	p, err := h.cronograma.Antecipar(r.Context(), id, slug, atividadeID, req.Data)
	h.responderPlano(w, r, p, err)
}

func (h *PlanoHandler) AdiarDia(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	p, err := h.cronograma.AdiarDia(r.Context(), id, slug, r.PathValue("data"))
	h.responderPlano(w, r, p, err)
}

func (h *PlanoHandler) RestaurarOrdem(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	p, err := h.cronograma.RestaurarOrdem(r.Context(), id, slug)
	h.responderPlano(w, r, p, err)
}

func (h *PlanoHandler) Compactar(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	p, err := h.cronograma.Compactar(r.Context(), id, slug)
	h.responderPlano(w, r, p, err)
}

func (h *PlanoHandler) Estatisticas(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	e, err := h.estatistica.Estatisticas(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, estatisticasParaDTO(e))
}

func (h *PlanoHandler) Caderno(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	c, err := h.caderno.Caderno(r.Context(), id, slug)
	h.responderCaderno(w, r, c, err)
}

func (h *PlanoHandler) responderCaderno(
	w http.ResponseWriter,
	r *http.Request,
	c service.Caderno,
	err error,
) {
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, cadernoParaDTO(c))
}

type anotacaoRequest struct {
	Data       string `json:"data"`
	Disciplina string `json:"disciplina"`
	Tema       string `json:"tema"`
	Texto      string `json:"texto"`
	URL        string `json:"url"`
	Resolvido  bool   `json:"resolvido"`
}

func (r anotacaoRequest) comando() service.AnotacaoCommand {
	return service.AnotacaoCommand{
		Data:       r.Data,
		Disciplina: r.Disciplina,
		Tema:       r.Tema,
		Texto:      r.Texto,
		URL:        r.URL,
		Resolvido:  r.Resolvido,
	}
}

func (h *PlanoHandler) CriarAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req anotacaoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	c, err := h.caderno.Criar(r.Context(), id, slug, req.comando())
	h.responderCaderno(w, r, c, err)
}

func (h *PlanoHandler) AtualizarAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	anotacaoID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	var req anotacaoRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	c, err := h.caderno.Atualizar(r.Context(), id, slug, anotacaoID, req.comando())
	h.responderCaderno(w, r, c, err)
}

func (h *PlanoHandler) RemoverAnotacao(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	anotacaoID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	c, err := h.caderno.Remover(r.Context(), id, slug, anotacaoID)
	h.responderCaderno(w, r, c, err)
}

// --- importação do TEC ---

type previewTECDTO struct {
	Casados      []casamentoTECDTO `json:"casados"`
	SemCorrespon []casamentoTECDTO `json:"semCorrespondencia"`
	Questoes     int               `json:"questoes"`
	Acertos      int               `json:"acertos"`
}

type casamentoTECDTO struct {
	Assunto    string `json:"assunto"`
	Disciplina string `json:"disciplina"`
	Tema       string `json:"tema"`
	Questoes   int    `json:"questoes"`
	Acertos    int    `json:"acertos"`
	Erros      int    `json:"erros"`
	Pct        int    `json:"pct"`
}

func previewTECParaDTO(p tec.Preview) previewTECDTO {
	casados := make([]casamentoTECDTO, 0, len(p.Casados))

	for _, c := range p.Casados {
		casados = append(casados, casamentoTECDTO{
			Assunto:    c.Assunto,
			Disciplina: c.Disciplina,
			Tema:       c.Tema,
			Questoes:   c.Questoes,
			Acertos:    c.Acertos,
			Erros:      c.Erros,
			Pct:        c.Pct,
		})
	}

	sem := make([]casamentoTECDTO, 0, len(p.SemCorrespon))

	for _, c := range p.SemCorrespon {
		sem = append(sem, casamentoTECDTO{
			Assunto:  c.Assunto,
			Questoes: c.Questoes,
			Acertos:  c.Acertos,
			Erros:    c.Erros,
			Pct:      c.Pct,
		})
	}

	return previewTECDTO{
		Casados:      casados,
		SemCorrespon: sem,
		Questoes:     p.Questoes,
		Acertos:      p.Acertos,
	}
}

// PreviewTEC lê a planilha enviada e mostra o que seria gravado, sem escrever
// nada.
func (h *PlanoHandler) PreviewTEC(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	if err := r.ParseMultipartForm(maxPlanilhaTEC); err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}
	defer file.Close()

	prev, err := h.tec.Preview(r.Context(), id, slug, io.LimitReader(file, maxPlanilhaTEC))
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, previewTECParaDTO(prev))
}

type importarTECRequest struct {
	CSV  string `json:"csv"`
	Data string `json:"data"`
}

// ImportarTEC aplica a planilha ao dia escolhido.
func (h *PlanoHandler) ImportarTEC(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	var req importarTECRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	prev, err := h.tec.Importar(r.Context(), id, slug, service.ImportarCommand{
		CSV: req.CSV, Data: req.Data,
	})
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	writeJSON(w, h.logger, http.StatusOK, previewTECParaDTO(prev))
}

type dossieDTO struct {
	Disciplina string           `json:"disciplina"`
	Markdown   string           `json:"markdown"`
	Fontes     []fonteDossieDTO `json:"fontes"`
}

type fonteDossieDTO struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
}

// Dossie devolve o documento de estudo de uma disciplina, pronto para virar
// fonte no NotebookLM.
func (h *PlanoHandler) Dossie(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	codigo := r.URL.Query().Get("disciplina")
	if codigo == "" {
		writeError(w, r, h.logger, errRequisicaoInvalida)

		return
	}

	d, err := h.dossie.Dossie(r.Context(), id, slug, codigo)
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	fontes := make([]fonteDossieDTO, 0, len(d.Fontes))
	for _, f := range d.Fontes {
		fontes = append(fontes, fonteDossieDTO{Titulo: f.Titulo, URL: f.URL})
	}

	writeJSON(w, h.logger, http.StatusOK, dossieDTO{
		Disciplina: d.Disciplina,
		Markdown:   d.Markdown,
		Fontes:     fontes,
	})
}

// ExportarCSV devolve o plano inteiro como CSV, para abrir numa planilha.
func (h *PlanoHandler) ExportarCSV(w http.ResponseWriter, r *http.Request) {
	id, slug, ok := h.contexto(r)
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)

		return
	}

	dados, err := h.exportacao.CSV(r.Context(), id, slug)
	if err != nil {
		writeError(w, r, h.logger, err)

		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="plano-`+slug+`.csv"`)
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(dados); err != nil {
		h.logger.ErrorContext(r.Context(), "escrevendo csv", slog.Any("error", err))
	}
}
