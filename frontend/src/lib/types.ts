// Wire types — mirror of the Go service view models (internal/service/plano_view.go
// and the httpapi handlers). Keep in sync with the backend.

export interface Usuario {
	id: string;
	email: string;
	nome: string;
}

export interface AuthResponse {
	usuario: Usuario;
	accessToken: string;
	accessExpiresAt: string;
	refreshToken: string;
}

export interface ConcursoResumo {
	slug: string;
	nome: string;
	banca: string;
	cargo: string;
	emoji: string;
	prova: string;
}

export interface ConcursoLista {
	concursos: ConcursoResumo[];
	importacaoEdital: boolean;
}

export interface FonteInput {
	titulo: string;
	url: string;
	tipo: string;
}

export interface Fonte {
	titulo: string;
	url: string;
	tipo: string;
}

export interface DisciplinaInput {
	nome: string;
	bloco: 'esp' | 'ger';
	questoes: number;
	/** 0 = use the block default (1 for ger, 2 for esp); a positive value overrides it. */
	peso: number;
	temas: string[];
	fontes: FonteInput[];
}

export interface MarcoInput {
	data: string;
	dataFim: string;
	titulo: string;
	exigeAcao: boolean;
}

export interface ConteudoInputItem {
	tipo: string;
	texto: string;
}

export interface ConcursoInput {
	nome: string;
	banca: string;
	cargo: string;
	emoji: string;
	prova: string;
	retaFinalDias: number;
	disciplinas: DisciplinaInput[];
	marcos: MarcoInput[];
	conteudo: ConteudoInputItem[];
}

export interface ConcursoDetalhe {
	slug: string;
	dados: ConcursoInput;
}

// ---- edital import wizard ----

export interface AlertaResposta {
	codigo: string;
	gravidade: 'info' | 'warning' | 'blocker';
	mensagem: string;
	campo?: string;
}

export interface CargoOpcao {
	codigo: string;
	nome: string;
	especialidade?: string;
	escolaridade?: string;
	/** null when the edital did not state a number. */
	vagas: number | null;
}

export interface AnaliseResposta {
	documentoId: string;
	banca: string;
	totalPaginas: number;
	paginasOcr: number;
	cargos: CargoOpcao[];
	alertas: AlertaResposta[];
}

export interface DisciplinaExtraida {
	nome: string;
	/** null unless the edital broke the group's total down by discipline. */
	questoes: number | null;
	/** null unless a weight was stated for this discipline specifically. */
	peso: number | null;
}

export interface GrupoResposta {
	kind: 'ger' | 'esp' | 'outro';
	rotulo: string;
	total: number | null;
	peso: number | null;
	pesoEscopo?: 'group' | 'discipline';
	disciplinas: DisciplinaExtraida[];
}

export interface DiscursivaResposta {
	modalidade: 'redacao' | 'estudo_de_caso' | 'outro';
	rotulo: string;
	questoes: number | null;
}

export interface DuracaoResposta {
	minutos: number;
	escopo: 'exam_set' | 'single_prova' | 'unknown';
}

export interface EstruturaResposta {
	nome: string;
	prova: string;
	gerais: GrupoResposta[];
	especificas: GrupoResposta[];
	discursivas: DiscursivaResposta[];
	duracao: DuracaoResposta | null;
	marcos: MarcoInput[];
	alertas: AlertaResposta[];
}

export interface ConteudoEditalResposta {
	itens: { nome: string; temas: string[] }[];
	alertas: AlertaResposta[];
}

export interface Disciplina {
	codigo: string;
	/** Short mnemonic for display (RL, LP, BD); codigo remains the technical id. */
	sigla: string;
	nome: string;
	bloco: 'esp' | 'ger';
	peso: number;
	cor: number;
	temas: string[];
	fontes: Fonte[];
}

export interface ConteudoItem {
	tipo: 'ficha' | 'rot' | 'h' | 'p';
	texto: string;
}

export interface ConcursoInfo {
	slug: string;
	nome: string;
	banca: string;
	cargo: string;
	emoji: string;
	resumo: string;
	disciplinas: Disciplina[];
	conteudo: ConteudoItem[];
}

export type Simulados = 'nunca' | 'quinzenal' | 'semanal';
export type Modo = 'completo' | 'questoes' | 'teoria';

export interface CicloItem {
	titulo: string;
	questoes: number;
}

// Config is the whole plan configuration — dates, rhythm and the study method,
// flat (the old nested `perfil` object is gone).
export interface Config {
	inicio: string;
	prova: string;
	horasDia: number; // derivado de minutosBloco × blocosPorDia + cauda de revisão
	diasEstudo: number[];
	diaRevisao: number;
	retaFinalDias: number;
	temaUi: 'light' | 'dark' | 'system';
	questoes: Record<string, number>;

	blocosPorDia: number;
	minutosBloco: number; // duração de um bloco normal; define o dia
	/** Length of the day's review block, in minutes. 0 = no review block. */
	minutosRevisao: number;
	reforcos: Record<string, number>;
	cicloRevisao: CicloItem[];
	/** Reserve a whole day of the week for review. Off by default: review is a
	 *  daily slice fed by the error notebook. */
	revisaoSemanal: boolean;
	simulados: Simulados;
	discursiva: boolean;
	modos: Record<string, Modo>;
	pctQuestoes: number;
	limiarFraco: number;
}

export type Tipo = 'est' | 'revd' | 'sim' | 'disc' | 'vespera' | 'rev';
export type Fase = 'base' | 'reta';

export interface ItemDia {
	/** Addresses this scheduled activity. Generated plans receive a stable
	 *  synthetic id; the backend materialises it transparently on the first write. */
	id: string;
	disciplina: string;
	tema: string;
	passada: number;
	/** True when the user placed this activity here, rather than the engine. */
	movida: boolean;
}

export interface Bloco {
	minutos: number;
	titulo: string;
	detalhe: string;
}

export interface RegistroBloco {
	disciplina: string;
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	erros: number | null;
	nota: string;
	/** This discipline finished, independently of the day's own flag. */
	concluido: boolean;
	/** The scheduled activity this record belongs to. Empty on legacy rows. */
	atividadeId: string;
}

export interface Registro {
	horas: number | null;
	concluido: boolean;
	questoes: number | null;
	acertos: number | null;
	erros: number | null;
	nota: string;
	blocos: RegistroBloco[];
}


export interface Dia {
	n: number;
	data: string;
	semana: number;
	fase: Fase;
	tipo: Tipo;
	itens: ItemDia[];
	tema: string;
	meta: number;
	blocos: Bloco[];
	registro: Registro | null;
	reordenado: boolean;
}

export interface Marco {
	id: string;
	rotulo: number;
	dataInicio: string;
	dataFim: string | null;
	titulo: string;
	exigeAcao: boolean;
	eProva: boolean;
	cumprido: boolean;
}

export interface LinhaBalanceamento {
	codigo: string;
	nome: string;
	bloco: 'esp' | 'ger';
	cor: number;
	questoes: number;
	questoesEdital: number;
	delta: number;
	modo: Modo;
	peso: number;
	pontos: number;
	pctIdeal: number;
	blocosConteudo: number;
	blocosReta: number;
	/** Topics the discipline has. */
	temas: number;
	/** Complete passes over the whole subject in the content phase. */
	passadas: number;
	/** Revisits during the learning phase, after the first read. */
	reforcos: number;
	/** Complete passes over the whole subject in the reta final. */
	revisoesGerais: number;
	/** The two added up: every complete pass before the exam. */
	totalPassadas: number;
	/** Average days between two days that study this discipline. */
	intervaloDias: number;
	horasPrevisto: number;
	horasLancado: number;
	desvio: number;
	acertoPct: number | null;
}

export interface Props {
	faltamDias: number;
	progresso: number;
	horasTotal: number;
	horasAlvo: number;
	acertoPct: number | null;
	totalDias: number;
	diasConcluidos: number;
}

export interface Alerta {
	nivel: 'warn' | 'danger';
	titulo: string;
	texto: string;
}

export interface PlanoResposta {
	concurso: ConcursoInfo;
	config: Config;
	dias: Dia[];
	marcos: Marco[];
	balanceamento: LinhaBalanceamento[];
	props: Props;
	alertas: Alerta[];
	hojeIndex: number | null;
	reordenados: string[];
	geradoEm: string;
}

// ConfigInput patches the plan config. Every study-method field is optional — an
// absent field leaves that setting untouched, so a patch that touches one
// control never resets the rest.
export interface ConfigInput {
	inicio?: string;
	prova?: string;
	horasDia?: number;
	diasEstudo?: number[];
	diaRevisao?: number;
	retaFinalDias?: number;
	temaUi?: string;
	questoes?: Record<string, number>;

	blocosPorDia?: number;
	minutosBloco?: number;
	minutosRevisao?: number;
	reforcos?: Record<string, number>;
	cicloRevisao?: CicloItem[];
	revisaoSemanal?: boolean;
	simulados?: Simulados;
	discursiva?: boolean;
	modos?: Record<string, Modo>;
	pctQuestoes?: number;
	limiarFraco?: number;
}

export interface RegistroBlocoInput {
	disciplina: string;
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	nota: string;
	concluido: boolean;
	/** Addresses one activity; preferred over `disciplina`. */
	atividadeId: string;
}

export interface RegistroInput {
	horas: number | null;
	concluido: boolean;
	questoes: number | null;
	acertos: number | null;
	nota: string;
	blocos: RegistroBlocoInput[];
}

export interface PontoSerie {
	data: string;
	n: number;
	horas: number;
	questoes: number;
	acertos: number;
	concluido: boolean;
}

export interface ResumoSemana {
	semana: number;
	horasPrevisto: number;
	horasLancado: number;
	questoes: number;
}

export interface Estatisticas {
	serie: PontoSerie[];
	porSemana: ResumoSemana[];
	porDisciplina: LinhaBalanceamento[];
	streak: number;
	horasTotal: number;
	questoesTotal: number;
	acertosTotal: number;
}

export type OrigemAnotacao = 'manual' | 'revisao' | 'tec' | 'simulado';

export interface AnotacaoView {
	id: string;
	data: string | null;
	disciplina: string | null;
	tema: string;
	texto: string;
	origem: OrigemAnotacao;
	url: string;
	resolvido: boolean;
	criadoEm: string;
}

export interface DiaNota {
	data: string;
	n: number;
	disciplinas: string[];
	nota: string;
}

export interface DiaFraco {
	data: string;
	n: number;
	questoes: number;
	acertos: number;
	pct: number;
}

export interface ItemCaderno {
	tema: string;
	erros: number;
	questoes: number;
	acertos: number;
	aproveitamento: number;
	ultimaData: string;
}

/** One discipline's error notebook — what the daily review tail drills. */
export interface CadernoDisciplina {
	disciplina: string;
	nome: string;
	cor: number;
	temas: ItemCaderno[];
}

export interface Caderno {
	anotacoes: AnotacaoView[];
	diasComNota: DiaNota[];
	diasFracos: DiaFraco[];
	porDisciplina: CadernoDisciplina[];
}

export interface AnotacaoInput {
	data?: string | null;
	disciplina?: string | null;
	tema?: string;
	texto: string;
	url?: string;
	resolvido: boolean;
}

export interface CasamentoTEC {
	assunto: string;
	questoes: number;
	acertos: number;
	erros: number;
	pct: number;
	disciplina: string;
	nome: string;
	tema: string;
}

export interface PreviewTEC {
	casados: CasamentoTEC[];
	semCorrespondencia: CasamentoTEC[];
	questoes: number;
	acertos: number;
}

export interface DossieFonte {
	titulo: string;
	url: string;
}

export interface Dossie {
	disciplina: string;
	markdown: string;
	fontes: DossieFonte[];
}
