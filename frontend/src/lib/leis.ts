import type { Dispositivo, QuestaoDeLei, TipoDispositivo } from '$lib/types';

/** Divisões da lei: viram cabeçalho no leitor e entrada no sumário. */
export const AGRUPAMENTOS: TipoDispositivo[] = ['parte', 'livro', 'titulo', 'capitulo', 'secao', 'subsecao'];

/** O recuo de cada tipo no texto corrido, em degraus. */
const RECUO: Partial<Record<TipoDispositivo, number>> = {
	paragrafo: 1,
	inciso: 2,
	alinea: 3,
	item: 4
};

export function recuo(d: Dispositivo, porRef: Map<string, Dispositivo>): number {
	if (d.tipo !== 'solto') return RECUO[d.tipo] ?? 0;
	// Texto solto (citação, continuação) fica no recuo de quem o contém.
	const pai = d.pai ? porRef.get(d.pai) : undefined;
	return pai ? recuo(pai, porRef) + 1 : 0;
}

/**
 * O artigo a que um dispositivo pertence. As questões citam o dispositivo
 * exato ("art71.inc2"), mas é no artigo que o estudante clica.
 */
export function artigoDe(ref: string, porRef: Map<string, Dispositivo>): string | null {
	let atual = porRef.get(ref);
	for (let passos = 0; atual && passos < 64; passos++) {
		if (atual.tipo === 'artigo') return atual.ref;
		atual = atual.pai ? porRef.get(atual.pai) : undefined;
	}
	return null;
}

/** As questões que citam o artigo ou qualquer dispositivo dentro dele. */
export function questoesPorArtigo(
	questoes: QuestaoDeLei[],
	porRef: Map<string, Dispositivo>
): Map<string, QuestaoDeLei[]> {
	const m = new Map<string, QuestaoDeLei[]>();
	for (const q of questoes) {
		const artigos = new Set(q.dispositivos.map((r) => artigoDe(r, porRef)).filter((a) => a !== null));
		for (const a of artigos) {
			const lista = m.get(a) ?? [];
			lista.push(q);
			m.set(a, lista);
		}
	}
	return m;
}

export interface Placar {
	total: number;
	respondidas: number;
	certas: number;
	erradas: number;
}

/** Conta pela ÚLTIMA resposta de cada questão, que é a que o servidor manda. */
export function placar(questoes: QuestaoDeLei[]): Placar {
	const respondidas = questoes.filter((q) => q.resposta !== null);
	const certas = respondidas.filter((q) => q.resposta?.acertou).length;
	return {
		total: questoes.length,
		respondidas: respondidas.length,
		certas,
		erradas: respondidas.length - certas
	};
}

/**
 * Separa o rótulo do resto para destacá-lo, sem mexer no texto: juntos, os
 * dois pedaços são exatamente o texto da lei.
 */
export function comRotulo(d: Dispositivo): [string, string] {
	if (d.rotulo && d.texto.startsWith(d.rotulo)) {
		return [d.rotulo, d.texto.slice(d.rotulo.length)];
	}
	return ['', d.texto];
}

/** O trecho da questão dentro do texto do dispositivo, para grifá-lo. */
export function partirNoTrecho(texto: string, trecho: string): [string, string, string] | null {
	const i = texto.indexOf(trecho);
	if (i < 0) return null;
	return [texto.slice(0, i), trecho, texto.slice(i + trecho.length)];
}
