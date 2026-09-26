package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studygo/internal/domain/lei"
	"studygo/internal/domain/usuario"
	"studygo/internal/service"

	"github.com/google/uuid"
)

// Snapshot do contrato HTTP.
//
// Guarda a FORMA do payload — quais chaves existem e de que tipo — e não os
// valores, que mudam a cada execução. É isto que o frontend consome, então uma
// mudança aqui é uma mudança de contrato: o teste falha e obriga quem mexeu a
// dizer, no commit, o que mudou e por quê.
//
// Regravar: ATUALIZAR_CONTRATO=1 go test ./internal/adapter/httpapi

var atualizarContrato = os.Getenv("ATUALIZAR_CONTRATO") != ""

func TestContratoHTTP_Plano(t *testing.T) {
	t.Parallel()

	// Um plano preenchido de propósito: campo que só aparece quando há dado
	// (revisão, registro, alerta) precisa estar no snapshot, ou o contrato só
	// descreve o caso vazio.
	horas := 1.5
	questoes, acertos, erros := 20, 15, 5
	hoje := 0
	data := "2026-09-01"

	p := service.PlanoMontado{
		Concurso: service.ConcursoDoPlano{
			Slug: "x", Nome: "X", Banca: "B", Cargo: "C", Emoji: "📚", Resumo: "r",
			Disciplinas: []service.DisciplinaDoPlano{{
				Codigo: "LINPO", Nome: "Língua Portuguesa", Bloco: "ger",
				Peso: 1, Cor: 0, CadernoURL: "https://exemplo",
				Temas:  []string{"Crase"},
				Fontes: []service.FonteDoPlano{{Titulo: "Lei", URL: "u", Tipo: "lei"}},
			}},
			Conteudo: []service.ItemDeConteudo{{Tipo: "p", Texto: "t"}},
		},
		Config: service.ConfigDoPlano{
			Inicio: data, Prova: "2026-12-15", HorasDia: 2,
			DiasEstudo: []int{1}, DiaRevisao: 5, RetaFinalDias: 30,
			TemaUI: "dark", Questoes: map[string]int{"LINPO": 15},
			BlocosPorDia: 2, MinutosBloco: 60, MinutosRevisao: 20,
			Reforcos:  map[string]float64{"LINPO": 1},
			Simulados: "semanal", Discursiva: true,
			Modos: map[string]string{"LINPO": "completo"}, PctQuestoes: 0.5, LimiarFraco: 70,
		},
		Dias: []service.DiaDoPlano{{
			N: 1, Data: data, Semana: 1, Fase: "base", Tipo: "est",
			Tema: "", Meta: 20, Concluido: true,
			Horas: &horas, Questoes: &questoes, Acertos: &acertos, Nota: "n",
			Itens: []service.AtividadeDoDia{{
				ID: uuid.New(), Disciplina: "LINPO", Tema: "Crase", Passada: 1,
				Movida: true, Horas: &horas, Questoes: &questoes,
				Acertos: &acertos, Erros: &erros, Nota: "n", Concluido: true,
			}},
			Blocos: []service.BlocoDoDia{{Minutos: 60, Titulo: "1º bloco", Detalhe: "d"}},
			Revisao: &service.RevisaoDoDia{
				Disciplina: "LINPO",
				Temas: []service.TemaDaRevisao{
					{Tema: "Crase", Aproveitamento: &hoje},
					{Tema: "Regência"},
				},
				Questoes: &questoes, Acertos: &acertos, Observacao: "o",
			},
		}},
		Marcos: []service.MarcoDoPlano{{
			ID: uuid.New(), Rotulo: 1, DataInicio: data, DataFim: &data,
			Titulo: "Inscrições", ExigeAcao: true, EProva: false, Cumprido: true,
		}},
		Balanceamento: []service.LinhaBalanceamento{{
			Codigo: "LINPO", Nome: "Língua Portuguesa", Bloco: "ger", Cor: 0,
			Questoes: 15, QuestoesEdital: 15, Delta: 0, Modo: "completo",
			Peso: 1, Pontos: 15, PctIdeal: 100, BlocosConteudo: 10, BlocosReta: 2,
			Temas: 4, Passadas: 2.5, Visitas: 10, RevisoesGerais: 1,
			IntervaloDias: 3, HorasPrevisto: 12, HorasLancado: 1.5,
			Desvio: 0, AcertoPct: &hoje,
		}},
		Props: service.ResumoDoPlano{
			FaltamDias: 100, Progresso: 1, HorasTotal: 1.5, HorasAlvo: 100,
			AcertoPct: &hoje, TotalDias: 75, DiasConcluidos: 1, VoltasRevisao: 3.4,
		},
		Alertas:   []service.Alerta{{Nivel: "warn", Titulo: "t", Texto: "x"}},
		HojeIndex: &hoje,
		GeradoEm:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}

	compararComGolden(t, "plano.json", forma(t, planoParaDTO(p)))
}

func TestContratoHTTP_Estatisticas(t *testing.T) {
	t.Parallel()

	pct := 75

	e := service.Estatisticas{
		Serie:         []service.PontoDaSerie{{Data: "2026-09-01", Horas: 2, Questoes: 20, Acertos: 15}},
		PorSemana:     []service.ResumoDaSemana{{Semana: 1, Horas: 8, Questoes: 60, Acertos: 45}},
		PorDisciplina: []service.LinhaBalanceamento{{Codigo: "LINPO", Nome: "Língua Portuguesa"}},
		Streak:        3, HorasTotal: 40, QuestoesTotal: 300, AcertoPct: &pct,
	}

	compararComGolden(t, "estatisticas.json", forma(t, estatisticasParaDTO(e)))
}

func TestContratoHTTP_Caderno(t *testing.T) {
	t.Parallel()

	data := "2026-09-01"

	c := service.Caderno{
		PorDisciplina: []service.CadernoDaDisciplina{{
			Codigo: "LINPO", Nome: "Língua Portuguesa", Cor: 0,
			Itens: []service.ItemDoCaderno{{
				Tema: "Crase", Questoes: 10, Acertos: 4, Erros: 6, Aprov: 40, UltimaData: data,
			}},
		}},
		Anotacoes: []service.AnotacaoDoCaderno{{
			ID: uuid.New(), Data: &data, Disciplina: "LINPO", Tema: "Crase",
			Texto: "t", Origem: "manual", URL: "u", Resolvido: false,
		}},
		DiasComNota: []service.DiaComNota{{
			Data: data, N: 1, Disciplinas: []string{"LINPO"}, Nota: "errei por pressa",
		}},
		DiasFracos: []service.DiaFraco{{Data: data, N: 1, Questoes: 10, Acertos: 4, Aprov: 40}},
	}

	compararComGolden(t, "caderno.json", forma(t, cadernoParaDTO(c)))
}

// A sessão é o contrato mais sensível do frontend: é a mesma resposta para
// cadastro, login e renovação, e o refresh token NÃO está nela — ele sai em
// cookie HttpOnly. Um campo `refreshToken` reaparecendo aqui é a regressão que
// este golden existe para pegar.
func TestContratoHTTP_Sessao(t *testing.T) {
	t.Parallel()

	u := usuario.Usuario{
		ID: uuid.New(), Email: "a@b.c", Nome: "Ana", TemaUI: usuario.TemaPadrao,
	}
	par := service.ParDeTokens{
		AccessToken:    "jwt",
		AccessExpiraEm: time.Now(),
		RefreshToken:   "nao-pode-sair-no-corpo",
	}

	compararComGolden(t, "sessao.json", forma(t, toAuthResponse(u, par)))
}

// forma serializa v e troca cada escalar pelo nome do tipo, reduzindo listas ao
// formato do primeiro elemento: o contrato é o conjunto de chaves e tipos, não
// os valores.
func forma(t *testing.T, v any) string {
	t.Helper()

	bruto, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var qualquer any
	if err := json.Unmarshal(bruto, &qualquer); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	saida, err := json.MarshalIndent(tipos(qualquer), "", "  ")
	if err != nil {
		t.Fatalf("marshal da forma: %v", err)
	}

	return string(saida) + "\n"
}

func tipos(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = tipos(val)
		}

		return out

	case []any:
		if len(x) == 0 {
			return []any{}
		}

		return []any{tipos(x[0])}

	case string:
		return "string"

	case float64:
		return "number"

	case bool:
		return "bool"

	default:
		return nil
	}
}

func compararComGolden(t *testing.T, nome, atual string) {
	t.Helper()

	caminho := filepath.Join("testdata", nome)

	if atualizarContrato {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("criando testdata: %v", err)
		}

		if err := os.WriteFile(caminho, []byte(atual), 0o600); err != nil {
			t.Fatalf("gravando golden: %v", err)
		}

		t.Logf("golden %s regravado", nome)

		return
	}

	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf(
			"lendo golden %s: %v\nrode ATUALIZAR_CONTRATO=1 go test ./internal/adapter/httpapi",
			caminho, err,
		)
	}

	if string(esperado) != atual {
		t.Errorf(
			"o contrato de %s mudou.\n\n--- esperado ---\n%s\n--- atual ---\n%s\n"+
				"Se a mudança é intencional, regrave com ATUALIZAR_CONTRATO=1 e "+
				"diga no commit qual campo mudou.",
			nome, esperado, atual,
		)
	}
}

func TestContratoHTTP_LeituraDaLei(t *testing.T) {
	t.Parallel()

	// Uma questão respondida e uma não: a resposta só existe na primeira, e o
	// contrato precisa descrever as duas formas.
	agora := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	l := service.LeituraDaLei{
		Lei: lei.Lei{Slug: "cf88", Nome: "Constituição", Curto: "CF", Fonte: "https://x", Reconhecer: []string{"CF"}},
		Texto: lei.Texto{
			Versao: "v1",
			Dispositivos: []lei.Dispositivo{
				{Ref: "art71", Tipo: "artigo", Rotulo: "Art. 71", Texto: "Art. 71. O controle…"},
				{Ref: "art71.inc2", Pai: "art71", Tipo: "inciso", Rotulo: "II", Texto: "II - julgar…",
					Notas: []string{"(Redação dada…)"}, Anteriores: []string{"II - antigo"}, Revogado: false},
			},
			Unidades: []lei.Unidade{{Ref: "u1", Titulo: "Arts. 70 a 75", Dispositivos: []string{"art71"}, Hash: "h"}},
		},
		Questoes: []service.QuestaoParaLeitor{
			{ID: uuid.New(), Unidade: "u1", Dispositivos: []string{"art71"}, Enunciado: "e",
				Alternativas: []string{"a", "b", "c", "d", "e"},
				Resposta:     &service.Correcao{Escolhida: "A", Acertou: true, Gabarito: "A", Comentario: "c", Trecho: "t", Em: agora}},
			{ID: uuid.New(), Unidade: "u1", Dispositivos: []string{"art71"}, Enunciado: "e2",
				Alternativas: []string{"a", "b", "c", "d", "e"}},
		},
		Recorte: &service.RecorteNoConcurso{
			Refs:     []string{"art71"},
			Trechos:  []lei.TrechoDoRecorte{{Ref: "art71", Rotulo: "Art. 71", Artigos: "art. 71"}},
			Materias: []service.MateriaDoRecorte{{DisciplinaID: uuid.New(), Nome: "Legislação"}},
		},
	}

	compararComGolden(t, "leitura_lei.json", forma(t, leituraParaDTO(l)))
}

// A prévia da captura é o que a tela de Legislação lê para decidir se a lei
// pode ser publicada e o que a pessoa precisa revisar.
func TestContratoHTTP_CapturaDaLei(t *testing.T) {
	c := lei.Captura{
		ID: "c1", Estado: lei.CapturaPronta, Etapa: "verificando", Feitos: 2, Total: 2,
		Resultado: &lei.ResultadoDaCaptura{
			Fonte: "https://www.planalto.gov.br/l1.htm", Gemini: true, Versao: "v1",
			Dispositivos: []lei.Dispositivo{
				{Ref: "cap1", Tipo: "capitulo", Rotulo: "CAPÍTULO I", Nome: "DO CONTROLE", Texto: "CAPÍTULO I"},
				{Ref: "art1", Pai: "cap1", Tipo: "artigo", Rotulo: "Art. 1º", Texto: "Art. 1º Texto."},
			},
			Avisos: []lei.Aviso{{ID: "p0003: a regra diz artigo", Texto: "…", Trecho: "Art. 1º Texto."}},
			Resumo: lei.ResumoDaCaptura{Vigentes: 3, Tipos: map[string]int{"artigo": 1}},
		},
	}

	ce := service.CapturaComEdital{
		Captura: c, Epigrafe: "LEI Nº 1, DE 2026",
		Edital: []service.SugestaoDoEdital{{
			DisciplinaID: uuid.New(), Materia: "Legislação", Temas: []string{"Lei nº 1: controle"},
			Recorte: []string{"cap1"}, Trechos: []lei.TrechoDoRecorte{{Ref: "cap1", Rotulo: "CAPÍTULO I", Nome: "DO CONTROLE", Artigos: "art. 1º"}},
		}},
	}

	compararComGolden(t, "captura_lei.json", forma(t, capturaParaDTO(ce)))
}
