package httpapi

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"studygo/internal/domain/mapa"
	"studygo/internal/service"
)

// A exportação dos mapas: um .zip com o mesmo arranjo que a importação aceita —
// <slug>.md, <slug>.questoes.json, as imagens em <slug>/ e o estudo da conta
// em <slug>.conta.json (vínculos com os tópicos e respostas). É a saída do
// banco para fora do app: cópia de segurança, ou editar fora e reimportar. Volta
// por ImportarPacote, um mapa por envio — a tela abre o .zip e manda um a um,
// porque o de todos passa do que o túnel aceita num envio só.

// ExportarMapa devolve um mapa da conta num .zip.
func (h *MapaHandler) ExportarMapa(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	e, err := h.mapas.Exportar(r.Context(), id, r.PathValue("slug"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	zw := iniciarZip(w, e.Slug+".zip")
	if err := escreverMapa(zw, e); err != nil {
		h.logger.ErrorContext(r.Context(), "exportando o mapa", "erro", err)
	}

	if err := zw.Close(); err != nil {
		h.logger.DebugContext(r.Context(), "fechando o zip do mapa", "erro", err)
	}
}

// ExportarTodos devolve todos os mapas da conta num .zip, escrito mapa a mapa
// direto na resposta: as imagens de todos juntos não cabem de uma vez na
// memória que o servidor divide com o resto.
func (h *MapaHandler) ExportarTodos(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	slugs, err := h.mapas.SlugsDaConta(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	zw := iniciarZip(w, "mapas-"+time.Now().Format("2006-01-02")+".zip")

	for _, slug := range slugs {
		e, err := h.mapas.Exportar(r.Context(), id, slug)
		if err == nil {
			err = escreverMapa(zw, e)
		}

		if err != nil {
			// Os cabeçalhos já foram: o que dá para fazer é parar e registrar.
			h.logger.ErrorContext(r.Context(), "exportando os mapas", "mapa", slug, "erro", err)

			return
		}
	}

	if err := zw.Close(); err != nil {
		h.logger.DebugContext(r.Context(), "fechando o zip dos mapas", "erro", err)
	}
}

func iniciarZip(w http.ResponseWriter, nome string) *zip.Writer {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, nome))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	return zip.NewWriter(w)
}

func escreverMapa(zw *zip.Writer, e service.ExportacaoDeMapa) error {
	if err := escreverNoZip(zw, e.Slug+".md", []byte(e.Texto)); err != nil {
		return err
	}

	if len(e.Questoes) > 0 {
		arquivo := arquivoDeQuestoesDTO{Mapa: e.Slug, Questoes: make([]questaoDoArquivoDoMapaDTO, 0, len(e.Questoes))}
		for _, q := range e.Questoes {
			arquivo.Questoes = append(arquivo.Questoes, questaoDoArquivoDoMapaDTO{
				ID: q.Chave, Ramo: q.Ramo, Origem: q.Origem, Enunciado: q.Enunciado,
				Alternativas: q.Alternativas, Gabarito: q.Gabarito, Comentario: q.Comentario,
			})
		}

		corpo, err := json.MarshalIndent(arquivo, "", " ")
		if err != nil {
			return err
		}

		if err := escreverNoZip(zw, e.Slug+".questoes.json", corpo); err != nil {
			return err
		}
	}

	if len(e.Vinculos) > 0 || len(e.Respostas) > 0 {
		corpo, err := json.MarshalIndent(estudoParaDTO(e), "", " ")
		if err != nil {
			return err
		}

		if err := escreverNoZip(zw, e.Slug+".conta.json", corpo); err != nil {
			return err
		}
	}

	for _, img := range e.Imagens {
		// A imagem já é comprimida: guardá-la sem compressão poupa CPU à toa.
		f, err := zw.CreateHeader(&zip.FileHeader{Name: e.Slug + "/" + img.Nome, Method: zip.Store})
		if err != nil {
			return err
		}

		if _, err := f.Write(img.Dados); err != nil {
			return err
		}
	}

	return nil
}

func escreverNoZip(zw *zip.Writer, nome string, dados []byte) error {
	f, err := zw.Create(nome)
	if err != nil {
		return err
	}

	_, err = f.Write(dados)

	return err
}

// --- o estudo da conta: vínculos e respostas -----------------------------------

// versaoDoEstudo muda quando o arquivo mudar de forma: quem importa sabe o que lê.
const versaoDoEstudo = 1

type estudoDoMapaDTO struct {
	Versao    int                    `json:"versao"`
	Vinculos  []vinculoExportadoDTO  `json:"vinculos"`
	Respostas []respostaExportadaDTO `json:"respostas"`
}

type vinculoExportadoDTO struct {
	Concurso     string   `json:"concurso"`
	ConcursoNome string   `json:"concursoNome"`
	Disciplina   string   `json:"disciplina"`
	Nome         string   `json:"nome"`
	Temas        []string `json:"temas"`
}

type respostaExportadaDTO struct {
	Questao  string    `json:"questao"`
	Resposta string    `json:"resposta"`
	Acertou  bool      `json:"acertou"`
	Em       time.Time `json:"em"`
}

func estudoParaDTO(e service.ExportacaoDeMapa) estudoDoMapaDTO {
	out := estudoDoMapaDTO{
		Versao:    versaoDoEstudo,
		Vinculos:  make([]vinculoExportadoDTO, 0, len(e.Vinculos)),
		Respostas: make([]respostaExportadaDTO, 0, len(e.Respostas)),
	}

	for _, v := range e.Vinculos {
		out.Vinculos = append(out.Vinculos, vinculoExportadoDTO{
			Concurso: v.ConcursoSlug, ConcursoNome: v.ConcursoNome, Disciplina: v.Codigo, Nome: v.Disciplina, Temas: naoNula(v.Temas),
		})
	}

	for _, r := range e.Respostas {
		out.Respostas = append(out.Respostas, respostaExportadaDTO{Questao: r.Chave, Resposta: r.Resposta, Acertou: r.Acertou, Em: r.Em})
	}

	return out
}

type pacoteImportadoDTO struct {
	Mapa       mapaResumoDTO      `json:"mapa"`
	Novo       bool               `json:"novo"`
	Questoes   int                `json:"questoes"`
	Imagens    int                `json:"imagens"`
	Respostas  int                `json:"respostas"`
	Vinculadas []materiaDoMapaDTO `json:"vinculadas"`
	Avisos     []string           `json:"avisos"`
}

func pacoteImportadoParaDTO(p service.PacoteImportado) pacoteImportadoDTO {
	vinculadas := make([]materiaDoMapaDTO, 0, len(p.Vinculadas))
	for _, v := range p.Vinculadas {
		vinculadas = append(vinculadas, materiaDoMapaDTO{Codigo: v.Codigo, Nome: v.Nome})
	}

	return pacoteImportadoDTO{
		Mapa: mapaResumoParaDTO(p.Mapa), Novo: p.Novo, Questoes: p.Questoes, Imagens: p.Imagens,
		Respostas: p.Respostas, Vinculadas: vinculadas, Avisos: naoNula(p.Avisos),
	}
}

// maxCorpoPacote é o que os dois nginx do caminho deixam passar (44 MiB na
// borda): o maior mapa de hoje, com as imagens, tem 24 MB. O de todos os
// mapas a tela manda um a um.
const maxCorpoPacote = 44 << 20

// ImportarPacote traz de volta um mapa exportado, num multipart: o texto em
// "mapa", e, se vieram no .zip, "questoes" (o <slug>.questoes.json), "conta"
// (o <slug>.conta.json) e os arquivos em "imagens". "concurso" é o concurso
// aberto na tela: é nele que cai o vínculo cujo concurso a conta não tem.
func (h *MapaHandler) ImportarPacote(w http.ResponseWriter, r *http.Request) {
	id, ok := usuarioID(r.Context())
	if !ok {
		writeError(w, r, h.logger, errNaoAutenticado)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoPacote)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, r, h.logger, erroDoMultipart(err))
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // limpeza de temporário

	p := service.PacoteDeMapa{Texto: r.FormValue("mapa")}

	if v := r.FormValue("questoes"); strings.TrimSpace(v) != "" {
		var q arquivoDeQuestoesDTO
		if err := json.Unmarshal([]byte(v), &q); err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		a := arquivoDeQuestoesDoDTO(q)
		p.Questoes = &a
	}

	if v := r.FormValue("conta"); strings.TrimSpace(v) != "" {
		var e estudoDoMapaDTO
		if err := json.Unmarshal([]byte(v), &e); err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		p.ComEstudo = true
		for _, v := range e.Vinculos {
			p.Vinculos = append(p.Vinculos, mapa.VinculoExportado{
				ConcursoSlug: v.Concurso, ConcursoNome: v.ConcursoNome, Codigo: v.Disciplina, Disciplina: v.Nome, Temas: v.Temas,
			})
		}

		for _, x := range e.Respostas {
			p.Respostas = append(p.Respostas, mapa.RespostaExportada{Chave: x.Questao, Resposta: x.Resposta, Acertou: x.Acertou, Em: x.Em})
		}
	}

	for _, cab := range r.MultipartForm.File["imagens"] {
		f, err := cab.Open()
		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		b, err := io.ReadAll(f)
		f.Close() //nolint:errcheck,gosec // leitura de arquivo já recebido

		if err != nil {
			writeError(w, r, h.logger, errRequisicaoInvalida)
			return
		}

		p.Imagens = append(p.Imagens, service.ArquivoDeImagem{Nome: cab.Filename, Dados: b})
	}

	res, err := h.mapas.ImportarPacote(r.Context(), id, r.FormValue("concurso"), p)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}

	writeJSON(w, h.logger, http.StatusOK, pacoteImportadoParaDTO(res))
}
