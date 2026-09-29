import type { ItemDoMapa, QuestaoDoMapa } from '$lib/types';

export interface Placar {
	total: number;
	respondidas: number;
	certas: number;
	erradas: number;
}

export function placar(questoes: QuestaoDoMapa[]): Placar {
	const respondidas = questoes.filter((q) => q.resposta !== null);
	const certas = respondidas.filter((q) => q.resposta?.acertou).length;
	return { total: questoes.length, respondidas: respondidas.length, certas, erradas: respondidas.length - certas };
}

/** "2 questões · 1 respondida · 1 certa": o placar num texto só. */
export function descreverPlacar(p: Placar): string {
	const questoes = p.total === 1 ? '1 questão' : `${p.total} questões`;
	const respondidas = p.respondidas === 1 ? '1 respondida' : `${p.respondidas} respondidas`;
	const certas = p.certas === 1 ? '1 certa' : `${p.certas} certas`;
	return `${questoes} · ${respondidas} · ${certas}`;
}

export interface GrupoDeQuestoes {
	ramo: string;
	questoes: QuestaoDoMapa[];
}

/**
 * As questões por ramo, na ordem dos ramos do mapa. O ramo que o mapa não tem
 * mais (renomeado numa reimportação) vai para o fim, com o nome que a questão
 * guardou: a questão não some por causa do mapa.
 */
export function porRamo(arvore: ItemDoMapa[], questoes: QuestaoDoMapa[]): GrupoDeQuestoes[] {
	const ordem = arvore.map((r) => r.texto.replaceAll('**', ''));
	const grupos = new Map<string, QuestaoDoMapa[]>();
	for (const q of questoes) {
		const lista = grupos.get(q.ramo) ?? [];
		lista.push(q);
		grupos.set(q.ramo, lista);
	}
	const posicao = (ramo: string) => {
		const i = ordem.indexOf(ramo);
		return i === -1 ? ordem.length : i;
	};
	return [...grupos.entries()]
		.map(([ramo, qs]) => ({ ramo, questoes: qs }))
		.sort((a, b) => posicao(a.ramo) - posicao(b.ramo));
}

/** Como a resposta aparece: a letra, ou "Certo"/"Errado". */
export function rotuloDaResposta(r: string): string {
	if (r === 'CERTO') return 'Certo';
	if (r === 'ERRADO') return 'Errado';
	return r;
}
