// Contrato do catálogo de provas e da curadoria. Espelha
// backend/internal/adapter/httpapi/dto_prova.go; o snapshot em
// testdata/prova*.json falha quando um dos dois muda sozinho.

export type TipoBloco = 'texto' | 'codigo' | 'imagem';
export type Formato = '' | 'negrito' | 'italico' | 'sublinhado';

/** Retângulo do PDF original, em pontos da página física. */
export interface Origem {
	pagina: number;
	retangulo: number[];
	regiao: string;
}

export interface Bloco {
	tipo: TipoBloco;
	texto: string;
	formato: Formato;
	/** Id do recorte PNG; só em bloco de imagem. */
	arquivo: string;
	descricao: string;
	origem: Origem | null;
	revisado: boolean;
	/** Tamanho da figura na tela, em % da coluna; 0 é o tamanho natural. */
	largura: number;
}

export interface Alternativa {
	letra: string;
	blocos: Bloco[];
}

export interface Questao {
	numero: number;
	disciplina: string;
	/** O tema dentro da matéria, que o curador dá; a extração não preenche. */
	assunto: string;
	blocos: Bloco[];
	alternativas: Alternativa[];
	/** Ids dos materiais de apoio que a questão usa. */
	apoios: string[];
	origens: Origem[];
	resposta: string;
	situacao: string;
	revisada: boolean;
	completa: boolean;
	/** De qual prova publicada o conteúdo foi reaproveitado ("TJCE 2026 · E05, questão 3"). */
	igualA: string;
}

export interface Apoio {
	id: string;
	blocos: Bloco[];
	questoes: number[];
	/** A frase do caderno que diz quais questões usam o texto. */
	aviso: string;
	/** Onde o texto está no caderno original. */
	origens: Origem[];
	revisado: boolean;
	igualA: string;
}

export type TipoGabarito = '' | 'preliminar' | 'definitivo' | 'nao_informado';

export interface Gabarito {
	cargo: string;
	caderno: string;
	tipo: TipoGabarito;
	respostas: Record<string, string>;
	situacoes: Record<string, string>;
}

export interface Extracao {
	modelo: string;
	tokensEntrada: number;
	tokensSaida: number;
	regiao: string;
	prompt: string;
	versao: string;
}

export interface Rascunho {
	banca: string;
	orgao: string;
	ano: number;
	/** Código que o caderno e o gabarito citam, como "F06": confere o gabarito. */
	cargo: string;
	/** Nome do cargo por extenso, como na capa: é o que o aluno lê. */
	cargoNome: string;
	caderno: string;
	total: number;
	questoes: Questao[];
	apoios: Apoio[];
	gabarito: Gabarito;
	alertas: string[];
	/** Só leitura: o servidor ignora o que vier aqui. */
	extracoes: Extracao[];
	/** Questões que a banca anulou e o curador tirou da prova; o total segue o da capa. */
	anuladasExcluidas: number[];
}

export type EstadoImportacao =
	| 'na_fila'
	| 'processando'
	| 'em_revisao'
	| 'falhou'
	| 'publicada'
	| 'cancelada';

export interface Importacao {
	id: string;
	estado: EstadoImportacao;
	versao: number;
	etapa: number;
	totalEtapas: number;
	erro: string;
	provaId: string;
	/** Os nomes com que o curador enviou os PDFs; vazios nas importações anteriores a eles. */
	nomeDocumento: string;
	nomeGabarito: string;
	regioes: Origem[];
	rascunho: Rascunho;
	pendencias: string[];
	/** Se a conferência do curador entra nas pendências neste ambiente. */
	conferenciaObrigatoria: boolean;
	criadoEm: string;
	atualizadoEm: string;
}

export interface ImportacaoResumo {
	id: string;
	estado: EstadoImportacao;
	etapa: number;
	totalEtapas: number;
	erro: string;
	provaId: string;
	nomeDocumento: string;
	nomeGabarito: string;
	orgao: string;
	ano: number;
	cargo: string;
	cargoNome: string;
	caderno: string;
	criadoEm: string;
	atualizadoEm: string;
}

export interface ProvaResumo {
	id: string;
	revisao: number;
	banca: string;
	orgao: string;
	ano: number;
	cargo: string;
	cargoNome: string;
	caderno: string;
	total: number;
	gabaritoTipo: TipoGabarito;
	publicadoEm: string;
}

export interface Prova extends ProvaResumo {
	questoes: Questao[];
	apoios: Apoio[];
}

/** Questão do catálogo sem o conteúdo, para o treino por matéria. A mesma
 *  questão em dois cargos vem uma vez só; o conteúdo vem da prova dela. */
export interface QuestaoAvulsa {
	provaId: string;
	numero: number;
	disciplina: string;
	/** O tema dentro da matéria; vazio quando a questão não foi classificada. */
	assunto: string;
	resposta: string;
	orgao: string;
	ano: number;
	cargo: string;
	cargoNome: string;
}

export interface MateriaSugerida {
	numero: number;
	materia: string;
}

/** O que o estudante anotou numa questão, em markdown. Só ele lê. */
export interface Anotacao {
	numero: number;
	texto: string;
	atualizadaEm: string;
}
