import type { ItemDoMapa, MarcaDoItem } from '$lib/types';

/** O nome de cada marca, como a tela a mostra. */
export const ROTULO_MARCA: Record<Exclude<MarcaDoItem, ''>, string> = {
	def: 'Definição',
	pegadinha: 'Pegadinha',
	cai: 'Cai em prova',
	ex: 'Exemplo',
	questao: 'Questão'
};

/** Quantas cores o tema oferece (--c0 … --c12): cada ramo principal pega uma. */
const CORES = 13;

/**
 * Um item com o que o mapa precisa saber dele além do texto.
 *
 * O `id` é o caminho de índices desde o ramo ("3.1.4"): é o que mantém de pé o
 * estado de aberto/fechado enquanto a tela se redesenha, e não depende do
 * texto, que se repete ("Propósito" aparece dezenas de vezes).
 */
export interface NoDoMapa {
	id: string;
	/** 0 nos ramos principais. */
	nivel: number;
	/** O índice da cor do ramo de cima: todo o ramo usa a mesma. */
	cor: number;
	item: ItemDoMapa;
	filhos: NoDoMapa[];
	/** Quantos itens há abaixo deste. */
	total: number;
	/** O texto como a busca o compara (ver `paraBusca`). */
	busca: string;
}

/**
 * O texto como a busca o compara: sem o `**` do negrito, sem acento e em
 * minúsculas. No celular quase ninguém digita acento, e "condensacao" tem de
 * achar "Condensação".
 */
export function paraBusca(texto: string): string {
	return texto.replaceAll('**', '').normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

export function indexar(ramos: ItemDoMapa[]): NoDoMapa[] {
	const descer = (itens: ItemDoMapa[], prefixo: string, nivel: number, cor: number | null): NoDoMapa[] =>
		itens.map((item, i) => {
			const id = prefixo === '' ? String(i) : `${prefixo}.${i}`;
			const corDoNo = cor ?? i % CORES;
			const filhos = descer(item.filhos, id, nivel + 1, corDoNo);

			return {
				id,
				nivel,
				cor: corDoNo,
				item,
				filhos,
				total: filhos.reduce((n, f) => n + 1 + f.total, 0),
				busca: paraBusca(item.texto)
			};
		});

	return descer(ramos, '', 0, null);
}

/** Os ids de todo item que tem filhos: o que "abrir tudo" abre. */
export function idsComFilhos(nos: NoDoMapa[]): string[] {
	return nos.flatMap((n) => (n.filhos.length > 0 ? [n.id, ...idsComFilhos(n.filhos)] : []));
}

/** O item ou algum descendente dele casa com o filtro (já passado por `paraBusca`). */
export function casa(no: NoDoMapa, filtro: string): boolean {
	return no.busca.includes(filtro) || no.filhos.some((f) => casa(f, filtro));
}

/** Os itens que têm, abaixo deles, algum que casa com o filtro: o caminho que a busca abre. */
export function caminhosAte(nos: NoDoMapa[], filtro: string): string[] {
	return nos.flatMap((n) => {
		const abaixo = caminhosAte(n.filhos, filtro);

		return n.filhos.some((f) => casa(f, filtro)) ? [n.id, ...abaixo] : abaixo;
	});
}

/** Quantos itens casam com o filtro, em qualquer nível. */
export function contarAchados(nos: NoDoMapa[], filtro: string): number {
	return nos.reduce((n, no) => n + (no.busca.includes(filtro) ? 1 : 0) + contarAchados(no.filhos, filtro), 0);
}

export interface Pedaco {
	texto: string;
	negrito: boolean;
}

/**
 * Separa o texto em pedaços comuns e em negrito (`**termo**`). É a única
 * marcação que o mapa tem, e é interpretada aqui, em pedaços de texto — nunca
 * como HTML.
 */
export function partesDoTexto(texto: string): Pedaco[] {
	const pedacos = texto.split('**');

	// Um `**` sem par não abre negrito: o que vem depois dele é texto comum,
	// com os asteriscos.
	if (pedacos.length % 2 === 0) {
		const resto = pedacos.pop() ?? '';
		pedacos[pedacos.length - 1] += `**${resto}`;
	}

	return pedacos.flatMap((p, i) => (p === '' ? [] : [{ texto: p, negrito: i % 2 === 1 }]));
}
