// O treino por matéria: escolher, entre as questões de todas as provas, as que
// o estudante quer resolver agora. Aqui só se decide e se conta — a tela guarda
// o filtro e lê as respostas do navegador.

import { situacaoDaQuestao, type Resposta, type Respostas, type Situacao } from './resolucao';
import type { QuestaoAvulsa } from './types';

/** O que o estudante já fez com a questão, como critério do filtro. */
export type SituacaoDoFiltro = 'todas' | 'abertas' | 'erradas';

export interface FiltroDoTreino {
	/** Vazio é todas as matérias. */
	materias: string[];
	/**
	 * Os assuntos escolhidos, cada um com a sua matéria (chaveDoAssunto). Só
	 * restringem a matéria deles: escolher Crase em Português não tira as
	 * questões de Redes do treino.
	 */
	assuntos: string[];
	/** Zero é qualquer ano. */
	ano: number;
	situacao: SituacaoDoFiltro;
}

export const SEM_FILTRO: FiltroDoTreino = { materias: [], assuntos: [], ano: 0, situacao: 'todas' };

const SEPARADOR = ' › ';

/** "Língua Portuguesa › Crase": o mesmo assunto pode existir em duas matérias. */
export function chaveDoAssunto(materia: string, assunto: string): string {
	return `${materia}${SEPARADOR}${assunto}`;
}

/** O nome do assunto, sem a matéria. */
export function nomeDoAssunto(chave: string): string {
	const i = chave.indexOf(SEPARADOR);
	return i < 0 ? chave : chave.slice(i + SEPARADOR.length);
}

function doAssunto(chave: string, materia: string): boolean {
	return chave.startsWith(materia + SEPARADOR);
}

/** As respostas guardadas de várias provas, pelo id da prova. */
export type RespostasPorProva = Record<string, Respostas>;

type Ref = Pick<QuestaoAvulsa, 'provaId' | 'numero'>;

/** Chave de uma questão avulsa: o número só não basta, cada prova tem a sua 7. */
export function chaveDaAvulsa(q: Ref): string {
	return `${q.provaId}.${q.numero}`;
}

export function respostaDaAvulsa(q: Ref, respostas: RespostasPorProva): Resposta | undefined {
	return respostas[q.provaId]?.[q.numero];
}

export function situacaoDaAvulsa(q: QuestaoAvulsa, respostas: RespostasPorProva): Situacao {
	return situacaoDaQuestao(q, respostaDaAvulsa(q, respostas));
}

function atendeAssuntos(q: QuestaoAvulsa, assuntos: string[]): boolean {
	const daMateria = assuntos.filter((k) => doAssunto(k, q.disciplina));
	return daMateria.length === 0 || daMateria.includes(chaveDoAssunto(q.disciplina, q.assunto));
}

function atende(s: Situacao, filtro: SituacaoDoFiltro): boolean {
	if (filtro === 'abertas') return s === 'aberta';
	if (filtro === 'erradas') return s === 'errada';
	return true;
}

type Criterio = keyof FiltroDoTreino;

/**
 * As questões que o filtro escolhe. `menos` deixa um critério de fora: é assim
 * que cada grupo de opções conta quantas questões teria se o estudante trocasse
 * só ali.
 */
export function filtrarTreino(
	qs: QuestaoAvulsa[],
	f: FiltroDoTreino,
	respostas: RespostasPorProva,
	menos?: Criterio
): QuestaoAvulsa[] {
	return qs.filter(
		(q) =>
			(menos === 'materias' || f.materias.length === 0 || f.materias.includes(q.disciplina)) &&
			(menos === 'assuntos' || atendeAssuntos(q, f.assuntos)) &&
			(menos === 'ano' || !f.ano || q.ano === f.ano) &&
			(menos === 'situacao' || atende(situacaoDaAvulsa(q, respostas), f.situacao))
	);
}

/**
 * Os grupos de matéria, na ordem da tela, com o que cada um é numa linha. O
 * que o servidor não souber agrupar fica nas específicas.
 */
export const GRUPOS = [
	['basicas', 'Básicas', 'Português, matemática e informática'],
	['legislacao', 'Legislação e administração pública', 'Cai em concurso de qualquer órgão'],
	['orgao', 'Do órgão', 'Regimento, ética e leis do estado: só deste concurso'],
	['especificas', 'Específicas de TI', 'Inclui as leis e normas de TI']
] as const;

export interface GrupoDeMaterias {
	grupo: string;
	rotulo: string;
	/** O que o grupo junta, numa linha, para o cartão dele. */
	explicacao: string;
	/** As matérias do grupo, em ordem alfabética, com a contagem. */
	materias: [string, number][];
}

export interface OpcoesDoTreino {
	/** Todas as matérias do catálogo, em ordem alfabética, com a contagem. */
	materias: [string, number][];
	/** As mesmas matérias, por grupo; grupo sem matéria não aparece. */
	grupos: GrupoDeMaterias[];
	/** Os assuntos de cada matéria escolhida, em ordem alfabética, com a contagem. */
	assuntos: { materia: string; assuntos: [string, number][] }[];
	/** Anos das provas, do mais recente, com a contagem. */
	anos: [number, number][];
	situacoes: Record<SituacaoDoFiltro, number>;
}

function contar<K>(todas: K[], escolhidas: K[]): Map<K, number> {
	const m = new Map<K, number>(todas.map((k) => [k, 0]));
	for (const k of escolhidas) m.set(k, (m.get(k) ?? 0) + 1);
	return m;
}

/** As opções de cada critério e quantas questões cada uma daria, mantidos os outros. */
export function opcoesDoTreino(
	qs: QuestaoAvulsa[],
	f: FiltroDoTreino,
	respostas: RespostasPorProva
): OpcoesDoTreino {
	const materias = contar(
		qs.map((q) => q.disciplina),
		filtrarTreino(qs, f, respostas, 'materias').map((q) => q.disciplina)
	);
	const semAssunto = filtrarTreino(qs, f, respostas, 'assuntos');
	const assuntos = [...f.materias]
		.sort((a, b) => a.localeCompare(b, 'pt-BR'))
		.map((materia) => {
			const deAssunto = (lista: QuestaoAvulsa[]) =>
				lista.filter((q) => q.disciplina === materia && q.assunto).map((q) => q.assunto);
			const contagem = contar(deAssunto(qs), deAssunto(semAssunto));
			return { materia, assuntos: [...contagem].sort(([a], [b]) => a.localeCompare(b, 'pt-BR')) };
		})
		.filter((g) => g.assuntos.length > 0);
	const anos = contar(
		qs.map((q) => q.ano),
		filtrarTreino(qs, f, respostas, 'ano').map((q) => q.ano)
	);
	const situacoes: Record<SituacaoDoFiltro, number> = { todas: 0, abertas: 0, erradas: 0 };
	for (const q of filtrarTreino(qs, f, respostas, 'situacao')) {
		const s = situacaoDaAvulsa(q, respostas);
		situacoes.todas++;
		if (s === 'aberta') situacoes.abertas++;
		if (s === 'errada') situacoes.erradas++;
	}

	const emOrdem = [...materias].sort(([a], [b]) => a.localeCompare(b, 'pt-BR'));
	const grupoDe = new Map(qs.map((q) => [q.disciplina, q.grupo]));
	const conhecido = (g: string | undefined) => GRUPOS.some(([id]) => id === g);
	const grupos = GRUPOS.map(([grupo, rotulo, explicacao]) => ({
		grupo,
		rotulo,
		explicacao,
		materias: emOrdem.filter(([m]) => {
			const g = grupoDe.get(m);
			return conhecido(g) ? g === grupo : grupo === 'especificas';
		})
	})).filter((g) => g.materias.length > 0);

	return {
		materias: emOrdem,
		grupos,
		assuntos,
		anos: [...anos].sort(([a], [b]) => b - a),
		situacoes
	};
}

/**
 * Tira do filtro salvo o que o catálogo não tem mais — matéria ou assunto
 * renomeado na curadoria, ano de uma prova retirada. Sem isso, o filtro escolheria nada e
 * não haveria chip para desmarcar.
 */
export function ajustarAoCatalogo(f: FiltroDoTreino, qs: QuestaoAvulsa[]): FiltroDoTreino {
	const materias = new Set(qs.map((q) => q.disciplina));
	const assuntos = new Set(qs.map((q) => chaveDoAssunto(q.disciplina, q.assunto)));
	const escolhidas = f.materias.filter((m) => materias.has(m));
	return {
		materias: escolhidas,
		// O assunto sem a matéria escolhida não teria chip para desmarcar.
		assuntos: f.assuntos.filter((k) => assuntos.has(k) && escolhidas.some((m) => doAssunto(k, m))),
		ano: qs.some((q) => q.ano === f.ano) ? f.ano : 0,
		situacao: f.situacao
	};
}

function situacaoValida(s: unknown): SituacaoDoFiltro {
	return s === 'abertas' || s === 'erradas' ? s : 'todas';
}

function textos(v: unknown): string[] {
	return Array.isArray(v) ? v.filter((m): m is string => typeof m === 'string') : [];
}

/** O filtro guardado no navegador. O que não se reconhece volta ao padrão. */
export function lerFiltro(texto: string | null): FiltroDoTreino {
	try {
		const f = JSON.parse(texto ?? '{}') as Partial<Record<Criterio, unknown>>;
		return {
			materias: textos(f.materias),
			assuntos: textos(f.assuntos),
			ano: typeof f.ano === 'number' && Number.isInteger(f.ano) ? f.ano : 0,
			situacao: situacaoValida(f.situacao)
		};
	} catch {
		return { ...SEM_FILTRO };
	}
}

/** O endereço da sessão de treino: o filtro vai nele, e recarregar não o perde. */
export function enderecoDoTreino(f: FiltroDoTreino): string {
	const q = new URLSearchParams();
	for (const m of f.materias) q.append('disciplina', m);
	for (const a of f.assuntos) q.append('assunto', a);
	if (f.ano) q.set('ano', String(f.ano));
	if (f.situacao !== 'todas') q.set('situacao', f.situacao);
	const busca = q.toString();
	return `/questoes/resolver${busca ? `?${busca}` : ''}`;
}

export function filtroDoEndereco(p: URLSearchParams): FiltroDoTreino {
	return {
		materias: p.getAll('disciplina').filter(Boolean),
		assuntos: p.getAll('assunto').filter(Boolean),
		ano: Number(p.get('ano')) || 0,
		situacao: situacaoValida(p.get('situacao'))
	};
}
