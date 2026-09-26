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

/**
 * O que login, cadastro e renovação devolvem — os três iguais.
 *
 * Não há `refreshToken` aqui de propósito: ele vem em cookie HttpOnly, que o
 * JavaScript não lê. Se este campo reaparecer, alguém desfez isso.
 */
export interface AuthResponse {
	usuario: Usuario;
	accessToken: string;
	accessExpiresAt: string;
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
	/**
	 * Identidade da matéria, devolvida ao servidor nas edições. É o que mantém
	 * cronograma e histórico ligados quando ela é renomeada — uma matéria que
	 * volta sem id é tratada como nova. Ausente nas que o formulário acabou de
	 * criar.
	 */
	id?: string;
	/**
	 * Tag exibida no chip do cronograma ("RLM"). Escolhida pelo usuário; vazia,
	 * o servidor mantém a que a matéria já tem ou deriva uma do nome.
	 */
	codigo?: string;
	nome: string;
	bloco: 'esp' | 'ger';
	questoes: number;
	/** 0 = use the block default (1 for ger, 2 for esp); a positive value overrides it. */
	peso: number;
	/** Link opcional para o caderno de erros externo da matéria (TEC, Qconcursos, um doc). */
	cadernoUrl: string;
	/** Link opcional para o notebook do NotebookLM desta matéria. */
	notebookUrl: string;
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

/** Uma linha da planilha que encontrou sua atividade no cronograma. */
export interface LinhaImportada {
	linha: number;
	data: string;
	disciplina: string;
	tema: string;
	minutos: number | null;
	questoes: number | null;
	acertos: number | null;
	concluido: boolean;
	/** A atividade não existia naquele dia e vai ser reconstruída pela planilha. */
	criada: boolean;
}

/** Uma linha que não entrou, com o motivo em português. */
export interface LinhaRecusada {
	linha: number;
	data: string;
	disciplina: string;
	motivo: string;
}

/** O que a importação faria (prévia) ou fez (confirmada). */
export interface ImportacaoCSV {
	aplicadas: LinhaImportada[];
	recusadas: LinhaRecusada[];
	/** Quantas linhas reconstroem a atividade que faltava no dia. */
	criadas: number;
	/** Quantas anotações de caderno a planilha traz e que ainda não existem aqui. */
	anotacoes: number;
	/** Quantas matérias recuperam a personalização: tag, link do caderno e ajustes. */
	materias: number;
	/** Dias vencidos que ficaram sem estudo e foram redistribuídos à frente. */
	diasVagos: number;
	/** 0 na prévia; quantas linhas foram gravadas na importação confirmada. */
	gravadas: number;
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
	/** Link opcional para o caderno de erros externo; o bloco de revisão leva até ele. */
	cadernoUrl: string;
	/** Link opcional para o notebook do NotebookLM desta matéria. */
	notebookUrl: string;
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

/** Um assunto que a fila traz de volta hoje. */
export interface TemaRevisao {
	tema: string;
	/** Aproveitamento no caderno de erros; null quando o tema nunca deu problema. */
	aproveitamento: number | null;
}

/** A cauda de revisão do dia — presente a partir do segundo dia de estudo, que
 *  é quando a fila já tem o que nomear. */
export interface Revisao {
	disciplina: string;
	/** O que revisar hoje, na ordem em que a fila os traz de volta. */
	temas: TemaRevisao[];
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

// ---- legislação ----

export type TipoDispositivo =
	| 'parte'
	| 'livro'
	| 'titulo'
	| 'capitulo'
	| 'secao'
	| 'subsecao'
	| 'artigo'
	| 'paragrafo'
	| 'inciso'
	| 'alinea'
	| 'item'
	| 'nome'
	| 'preambulo'
	| 'fecho'
	| 'solto';

export interface Dispositivo {
	/** Endereço jurídico e âncora do link direto: "art71.inc2". */
	ref: string;
	pai: string | null;
	tipo: TipoDispositivo;
	rotulo: string;
	nome: string;
	texto: string;
	notas: string[];
	anteriores: string[];
	revogado: boolean;
}

export interface UnidadeDeLei {
	ref: string;
	titulo: string;
	dispositivos: string[];
	hash: string;
}

export interface CorrecaoDeQuestao {
	escolhida: string;
	acertou: boolean;
	gabarito: string;
	comentario: string;
	trecho: string;
	respondidaEm: string;
}

export interface QuestaoDeLei {
	id: string;
	unidade: string;
	dispositivos: string[];
	enunciado: string;
	alternativas: string[];
	/** Só existe depois de respondida: o gabarito não vem antes. */
	resposta: CorrecaoDeQuestao | null;
}

export interface LeiIdentidade {
	slug: string;
	nome: string;
	curto: string;
	reconhecer: string[];
	fonte: string;
}

/** Uma raiz do recorte do edital: "Seção IX — DA FISCALIZAÇÃO… (arts. 70 a 75)". */
export interface TrechoDoRecorte {
	ref: string;
	rotulo: string;
	nome: string;
	artigos: string;
}

/** O que o concurso cobra da lei. `refs` vazias: a lei inteira. */
export interface RecorteNoConcurso {
	refs: string[];
	trechos: TrechoDoRecorte[];
	materias: { disciplinaId: string; nome: string }[];
}

export interface LeituraDeLei {
	lei: LeiIdentidade;
	versao: string;
	dispositivos: Dispositivo[];
	unidades: UnidadeDeLei[];
	questoes: QuestaoDeLei[];
	/** null: nenhuma matéria do concurso ativo cobra esta lei. */
	recorte: RecorteNoConcurso | null;
	/** O que a versão guarda quando é só parte da lei; vazio, a lei inteira. */
	guardado: TrechoDoRecorte[];
}

export interface LeiResumo {
	slug: string;
	nome: string;
	curto: string;
	fonte: string;
	versao: string;
	questoes: number;
	importadaEm: string;
}

/** A lei vista da matéria: o recorte que ela cobra, ou que o tópico pede. Vazio: a lei inteira. */
export interface LeiNaMateria extends LeiResumo {
	recorte: TrechoDoRecorte[];
}

export interface LeisDaMateria {
	disciplinaId: string;
	codigo: string;
	nome: string;
	vinculadas: LeiNaMateria[];
	sugeridas: LeiNaMateria[];
	/** Os tópicos da matéria e os slugs das leis vinculadas que cada um cita. */
	temas: { texto: string; leis: string[] }[];
}

/** Um pedaço do tópico ("Administração Pública") e o que ele pede da lei. */
export interface AssuntoDoTema {
	texto: string;
	refs: string[];
	trechos: TrechoDoRecorte[];
}

export interface DivisaoDaLei {
	ref: string;
	pai: string | null;
	tipo: TipoDispositivo;
	rotulo: string;
	nome: string;
	/** O começo do artigo; vazio nas divisões. */
	texto: string;
}

/** O que a pesquisa pelo tópico achou, antes de importar. */
export interface PesquisaDoTema {
	fonte: string;
	link: string;
	epigrafe: string;
	nome: string;
	curto: string;
	/** importar: lei nova; vincular: a guardada já cobre; ampliar: falta parte. */
	acao: 'importar' | 'vincular' | 'ampliar';
	existente: { slug: string; curto: string } | null;
	assuntos: AssuntoDoTema[];
	/** Vazio: o tópico pede a lei inteira. */
	recorte: string[];
	trechos: TrechoDoRecorte[];
	estrutura: DivisaoDaLei[];
}

export interface PedidoDeCaptura {
	link: string;
	recorte?: string[];
	artigos?: string;
	slug?: string;
	inteira?: boolean;
}

export interface ResumoDaExclusao {
	curto: string;
	questoes: number;
	respostas: number;
}

/** Uma coisa que a captura resolveu sozinha e alguém precisa conferir. */
export interface AvisoDaCaptura {
	id: string;
	texto: string;
	trecho: string;
}

export interface ResultadoDaCaptura {
	fonte: string;
	gemini: boolean;
	versao: string;
	publicavel: boolean;
	bloqueios: string[];
	avisos: AvisoDaCaptura[];
	resumo: {
		vigentes: number;
		anteriores: number;
		notas: number;
		revogados: number;
		tipos: Record<string, number>;
		descartados: string[];
		riscados: string[];
		juncoes: string[];
	};
	sumario: { ref: string; tipo: string; rotulo: string; nome: string }[];
	artigos: number;
	dispositivos: number;
	/** As raízes guardadas; vazio, a lei inteira. */
	recorte: string[];
}

/** Uma matéria do concurso cujo tópico cita a lei capturada, e o que ele pede. */
export interface SugestaoDoEdital {
	disciplinaId: string;
	materia: string;
	temas: string[];
	/** Vazio: o tópico pede a lei inteira. */
	recorte: string[];
	trechos: TrechoDoRecorte[];
}

export interface CapturaDeLei {
	id: string;
	/** A primeira linha da lei ("LEI Nº 13.709, DE…"), para sugerir o nome. */
	epigrafe: string;
	edital: SugestaoDoEdital[];
	estado: 'rodando' | 'pronta' | 'falhou';
	etapa: string;
	progresso: { feitos: number; total: number };
	erro?: string;
	resultado: ResultadoDaCaptura | null;
}

export interface PedidoDePublicacao {
	/** Vazio: lei nova. Preenchido: atualiza o texto daquela lei. */
	slug?: string;
	nome: string;
	curto: string;
	reconhecer: string[];
	aceitos: string[];
}

export interface PublicacaoDeLei {
	slug: string;
	curto: string;
	versao: string;
	novaVersao: boolean;
	unidadesDesatualizadas: number;
}

export interface ImportacaoDeQuestoes {
	curto: string;
	novas: number;
	atualizadas: number;
	desativadas: number;
	mantidas: number;
}
