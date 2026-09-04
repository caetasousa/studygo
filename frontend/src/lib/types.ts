// Tipos do contrato HTTP — espelho dos DTOs do backend
// (backend/internal/adapter/httpapi/dto_*.go).
//
// O snapshot em backend/internal/adapter/httpapi/testdata guarda a forma desses
// payloads: quando ele muda, este arquivo muda junto.

export interface Usuario {
	id: string;
	email: string;
	nome: string;
	temaUi: 'light' | 'dark' | 'system';
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
	/** Optional link to this subject's external error notebook (TEC, Qconcursos, a doc). */
	cadernoUrl: string;
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
	/** Mnemônico exibido nos chips do cronograma ("DIRAD"). É o que o servidor
	 *  gravou: a tela não deriva sigla própria, ou discordaria do banco. */
	codigo: string;
	nome: string;
	bloco: 'esp' | 'ger';
	peso: number;
	cor: number;
	/** Optional link to this subject's external error notebook; the review block links to it. */
	cadernoUrl: string;
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

/** Uma atividade agendada, com o que foi lançado nela.
 *
 *  O cronograma é materializado no servidor, então `id` é sempre um uuid real
 *  desde o primeiro carregamento. */
export interface Atividade {
	id: string;
	disciplina: string;
	tema: string;
	passada: number;
	/** Verdadeiro quando foi o estudante que colocou a atividade aqui. */
	movida: boolean;
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	erros: number | null;
	nota: string;
	concluido: boolean;
}

export interface Bloco {
	minutos: number;
	titulo: string;
	detalhe: string;
}

/** A cauda de revisão do dia — presente a partir do segundo dia de estudo, que
 *  é quando a fila já tem o que nomear. */
export interface Revisao {
	disciplina: string;
	questoes: number | null;
	acertos: number | null;
	observacao: string;
}

export interface Dia {
	n: number;
	data: string;
	semana: number;
	fase: Fase;
	tipo: Tipo;
	itens: Atividade[];
	tema: string;
	meta: number;
	blocos: Bloco[];
	/** Nome do dia na tela ("SIMULADO", "VÉSPERA"); vazio nos dias de conteúdo.
	 *  Vem do servidor para que exista uma única tabela desses nomes. */
	rotulo: string;
	/** DERIVADO das atividades do dia — o cliente nunca o envia. */
	concluido: boolean;
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	nota: string;
	revisao: Revisao | null;
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
	/** Days of the learning phase that study this subject — times you come back. */
	visitas: number;
	/** Complete passes over the whole subject in the reta final. */
	revisoesGerais: number;
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
	/** Complete laps over everything studied, before the reta final. */
	voltasRevisao: number;
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
	/** Habilita "restaurar ordem automática": o estudante rearranjou algo. */
	temMovimentacaoManual: boolean;
	geradoEm: string;
}

// ConfigInput altera a configuração do plano. Todo campo de método é opcional:
// um campo ausente deixa aquela escolha intacta, para que salvar um controle não
// redefina os outros.
export interface ConfigInput {
	inicio?: string;
	prova?: string;
	horasDia?: number;
	diasEstudo?: number[];
	diaRevisao?: number;
	retaFinalDias?: number;
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

/** O lançamento de UMA atividade. A conclusão do dia não vai aqui: ela é
 *  derivada no servidor a partir das atividades daquele dia. */
export interface RegistroInput {
	atividadeId: string;
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	nota: string;
	concluido: boolean;
}

/** O que pertence ao dia e não a uma atividade: a anotação livre e o resultado
 *  da cauda de revisão. */
export interface RegistroDiaInput {
	nota: string;
	questoes: number | null;
	acertos: number | null;
	observacao: string;
}

export interface PontoSerie {
	data: string;
	horas: number;
	questoes: number;
	acertos: number;
}

export interface ResumoSemana {
	semana: number;
	horasPrevisto: number;
	horas: number;
	questoes: number;
	acertos: number;
}

export interface Estatisticas {
	serie: PontoSerie[];
	porSemana: ResumoSemana[];
	porDisciplina: LinhaBalanceamento[];
	streak: number;
	horasTotal: number;
	questoesTotal: number;
	acertoPct: number | null;
}

export type OrigemAnotacao = 'manual' | 'revisao' | 'tec' | 'simulado';

export interface AnotacaoView {
	id: string;
	data: string | null;
	/** Código da disciplina; vazio quando a anotação não é de nenhuma. */
	disciplina: string;
	tema: string;
	texto: string;
	origem: OrigemAnotacao;
	url: string;
	resolvido: boolean;
}

export interface DiaComNota {
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
	aprov: number;
}

export interface ItemCaderno {
	tema: string;
	questoes: number;
	acertos: number;
	/** Quantas vezes o tema foi errado — o que o torna candidato a revisão. */
	erros: number;
	aprov: number;
	ultimaData: string;
}

/** O caderno de erros de uma disciplina — o que a revisão diária vai drilar. */
export interface CadernoDisciplina {
	codigo: string;
	nome: string;
	cor: number;
	itens: ItemCaderno[];
}

export interface Caderno {
	porDisciplina: CadernoDisciplina[];
	anotacoes: AnotacaoView[];
	diasComNota: DiaComNota[];
	diasFracos: DiaFraco[];
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
	disciplina: string;
	tema: string;
	questoes: number;
	acertos: number;
	erros: number;
	pct: number;
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
