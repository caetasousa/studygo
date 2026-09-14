// O markdown das anotações do estudante. É lido para uma árvore e desenhado
// por componentes — nada vira HTML cru, então o que o estudante cola não
// executa. Cobre o que se usa numa nota de estudo, como no Notion: títulos,
// listas, tarefas, citação, código, divisor, negrito, itálico, riscado, código
// em linha e link.

export type Trecho =
	| { tipo: 'texto'; texto: string }
	| { tipo: 'codigo'; texto: string }
	| { tipo: 'negrito' | 'italico' | 'riscado'; filhos: Trecho[] }
	| { tipo: 'link'; href: string; filhos: Trecho[] };

export interface ItemDeLista {
	trechos: Trecho[];
	/** Linha do item no texto: é por ela que a tarefa é marcada. */
	linha: number;
	tarefa?: { feita: boolean };
}

export type BlocoMd =
	| { tipo: 'paragrafo'; linhas: Trecho[][] }
	| { tipo: 'titulo'; nivel: 1 | 2 | 3; trechos: Trecho[] }
	| { tipo: 'lista'; ordenada: boolean; itens: ItemDeLista[] }
	| { tipo: 'citacao'; linhas: Trecho[][] }
	| { tipo: 'codigo'; linguagem: string; texto: string }
	| { tipo: 'divisor' };

const CERCA = /^\s*```\s*([\w+-]*)\s*$/;
const TITULO = /^(#{1,3})\s+(.*)$/;
const DIVISOR = /^\s*(?:-{3,}|\*{3,}|_{3,})\s*$/;
const CITACAO = /^\s*>\s?(.*)$/;
const MARCADOR = /^\s*[-*+]\s+(.*)$/;
const NUMERO = /^\s*\d+[.)]\s+(.*)$/;
// A tarefa pode estar vazia: "- [ ]" sozinho é uma caixa sem texto, como no Notion.
const TAREFA = /^\[([ xX])\](?:\s+(.*))?$/;

function comecaBloco(linha: string): boolean {
	return [CERCA, TITULO, DIVISOR, CITACAO, MARCADOR, NUMERO].some((r) => r.test(linha));
}

export function lerMarkdown(texto: string): BlocoMd[] {
	const linhas = texto.replace(/\r\n?/g, '\n').split('\n');
	const out: BlocoMd[] = [];
	let i = 0;
	while (i < linhas.length) {
		const linha = linhas[i];
		const cerca = linha.match(CERCA);
		if (cerca) {
			const conteudo: string[] = [];
			i++;
			while (i < linhas.length && !CERCA.test(linhas[i])) conteudo.push(linhas[i++]);
			i++;
			out.push({ tipo: 'codigo', linguagem: cerca[1], texto: conteudo.join('\n') });
			continue;
		}
		if (!linha.trim()) {
			i++;
			continue;
		}
		const titulo = linha.match(TITULO);
		if (titulo) {
			out.push({ tipo: 'titulo', nivel: titulo[1].length as 1 | 2 | 3, trechos: trechos(titulo[2]) });
			i++;
			continue;
		}
		if (DIVISOR.test(linha)) {
			out.push({ tipo: 'divisor' });
			i++;
			continue;
		}
		if (CITACAO.test(linha)) {
			const citadas: Trecho[][] = [];
			while (i < linhas.length && CITACAO.test(linhas[i])) citadas.push(trechos(linhas[i++].match(CITACAO)![1]));
			out.push({ tipo: 'citacao', linhas: citadas });
			continue;
		}
		const ordenada = NUMERO.test(linha);
		if (ordenada || MARCADOR.test(linha)) {
			const padrao = ordenada ? NUMERO : MARCADOR;
			const itens: ItemDeLista[] = [];
			while (i < linhas.length && padrao.test(linhas[i])) {
				const corpo = linhas[i].match(padrao)![1];
				const tarefa = ordenada ? null : corpo.match(TAREFA);
				itens.push(
					tarefa
						? { trechos: trechos(tarefa[2] ?? ''), linha: i, tarefa: { feita: tarefa[1] !== ' ' } }
						: { trechos: trechos(corpo), linha: i }
				);
				i++;
			}
			out.push({ tipo: 'lista', ordenada, itens });
			continue;
		}
		const paragrafo: Trecho[][] = [];
		while (i < linhas.length && linhas[i].trim() && !comecaBloco(linhas[i])) paragrafo.push(trechos(linhas[i++]));
		out.push({ tipo: 'paragrafo', linhas: paragrafo });
	}
	return out;
}

// Código em linha primeiro: o que está entre crases não é marcação.
const EM_LINHA =
	/(`+)([^`]+?)\1|\*\*(?=\S)(.+?)\*\*|~~(?=\S)(.+?)~~|\*(?=\S)([^*]*?\S)\*|(?<![\w])_(?=\S)([^_]*?\S)_(?![\w])|\[([^\]]+)\]\(([^)\s]+)\)|(https?:\/\/[^\s<>]+[^\s<>().,;:!?'"])/g;

/** Só estes esquemas viram link; `javascript:` e companhia ficam como texto. */
function seguro(href: string): boolean {
	return /^(?:https?:|mailto:)/i.test(href);
}

export function trechos(texto: string): Trecho[] {
	const out: Trecho[] = [];
	const juntar = (t: string) => {
		if (!t) return;
		const ultimo = out.at(-1);
		if (ultimo?.tipo === 'texto') ultimo.texto += t;
		else out.push({ tipo: 'texto', texto: t });
	};
	let desde = 0;
	for (const m of texto.matchAll(EM_LINHA)) {
		juntar(texto.slice(desde, m.index));
		desde = m.index + m[0].length;
		if (m[2] !== undefined) out.push({ tipo: 'codigo', texto: m[2] });
		else if (m[3] !== undefined) out.push({ tipo: 'negrito', filhos: trechos(m[3]) });
		else if (m[4] !== undefined) out.push({ tipo: 'riscado', filhos: trechos(m[4]) });
		else if (m[5] !== undefined) out.push({ tipo: 'italico', filhos: trechos(m[5]) });
		else if (m[6] !== undefined) out.push({ tipo: 'italico', filhos: trechos(m[6]) });
		else if (m[7] !== undefined) {
			if (seguro(m[8])) out.push({ tipo: 'link', href: m[8], filhos: trechos(m[7]) });
			else juntar(m[0]);
		} else out.push({ tipo: 'link', href: m[9], filhos: [{ tipo: 'texto', texto: m[9] }] });
	}
	juntar(texto.slice(desde));
	return out;
}

/** Marca ou desmarca a tarefa da linha `linha`: "- [ ]" ↔ "- [x]". */
export function alternarTarefa(texto: string, linha: number): string {
	const linhas = texto.split('\n');
	linhas[linha] = linhas[linha]?.replace(/^(\s*[-*+]\s+)\[([ xX])\]/, (_, antes, marca) =>
		`${antes}[${marca === ' ' ? 'x' : ' '}]`
	);
	return linhas.join('\n');
}

const PREFIXO_DE_LISTA = /^(\s*)([-*+]\s+\[[ xX]\](?:\s+|$)|[-*+]\s+|(\d+)([.)])\s+)/;

/**
 * Enter numa linha de lista continua a lista, como no Notion: o próximo item
 * já vem com o marcador (a tarefa, desmarcada; o número, o seguinte). Enter
 * num item vazio encerra a lista. Fora de lista, devolve null — o Enter segue
 * normal.
 */
export function continuarLista(texto: string, cursor: number): { texto: string; cursor: number } | null {
	const inicio = texto.lastIndexOf('\n', cursor - 1) + 1;
	const linha = texto.slice(inicio, cursor);
	const m = linha.match(PREFIXO_DE_LISTA);
	if (!m) return null;
	if (!linha.slice(m[0].length).trim()) {
		// Item vazio: tira o marcador e sai da lista.
		const novo = texto.slice(0, inicio) + texto.slice(cursor);
		return { texto: novo, cursor: inicio };
	}
	const [, recuo, marcador, numero, fecho] = m;
	const proximo = numero
		? `${recuo}${Number(numero) + 1}${fecho} `
		: `${recuo}${marcador.replace(/\[[xX]\]/, '[ ]')}`;
	const novo = texto.slice(0, cursor) + '\n' + proximo + texto.slice(cursor);
	return { texto: novo, cursor: cursor + 1 + proximo.length };
}

/**
 * Põe (ou tira, se todas já têm) o prefixo nas linhas da seleção: "## " para
 * título, "- " para lista, "> " para citação, "- [ ] " para tarefa.
 */
export function prefixarLinhas(
	texto: string,
	inicio: number,
	fim: number,
	prefixo: string
): { texto: string; inicio: number; fim: number } {
	const de = texto.lastIndexOf('\n', inicio - 1) + 1;
	const quebra = texto.indexOf('\n', fim);
	const ate = quebra < 0 ? texto.length : quebra;
	const linhas = texto.slice(de, ate).split('\n');
	const todas = linhas.every((l) => l.startsWith(prefixo));
	const novas = linhas.map((l) => (todas ? l.slice(prefixo.length) : prefixo + l.replace(/^(#{1,3}\s+|[-*+]\s+(\[[ xX]\]\s+)?|>\s?)/, '')));
	const trecho = novas.join('\n');
	return { texto: texto.slice(0, de) + trecho + texto.slice(ate), inicio: de, fim: de + trecho.length };
}
