// O editor da revisão: cada campo (enunciado, alternativa, texto de apoio) é
// um texto só, com marcação curta, em vez de uma pilha de blocos com seletor de
// tipo e de destaque. É o que o curador lê e digita; os blocos continuam sendo
// o que se grava.
//
//   **negrito**   *itálico*   __sublinhado__   `código em linha`
//   ```            abre e fecha um bloco de código, em linhas próprias
//   [figura 1]     a primeira figura do campo, numa linha própria

import { novoBloco } from './revisao';
import { separador } from './texto';
import type { Bloco, Formato } from './types';

const MARCA: Record<string, string> = { negrito: '**', italico: '*', sublinhado: '__' };

const CERCA = /^```[\w+-]*\s*$/;
const FIGURA = /^\s*\[figura (\d+)\]\s*$/i;
// O código em linha vem primeiro: o que está entre crases não é marcação. O
// "??" fecha no primeiro par: "**a** e **b**" são dois negritos, não um.
const EM_LINHA = /(`[^`\n]+`)|\*\*(\S(?:[^\n]*?\S)??)\*\*|__(\S(?:[^\n]*?\S)??)__|\*(\S(?:[^*\n]*?\S)??)\*/g;

/** Os blocos de um campo, como o curador os edita. */
export function blocosParaTexto(blocos: Bloco[]): string {
	let out = '';
	let figuras = 0;
	let anterior: Bloco | undefined;
	for (const b of blocos) {
		const propria = b.tipo !== 'texto';
		const parte =
			b.tipo === 'imagem' ? `[figura ${++figuras}]` : b.tipo === 'codigo' ? '```\n' + b.texto + '\n```' : marcar(b);
		if (anterior) {
			if (propria || anterior.tipo !== 'texto') {
				if (!out.endsWith('\n') && !parte.startsWith('\n')) out += '\n';
			} else out += separador(anterior.texto, b.texto);
		}
		out += parte;
		anterior = b;
	}
	return out;
}

/** O destaque em volta do texto, com os espaços das pontas do lado de fora. */
function marcar(b: Bloco): string {
	const m = MARCA[b.formato];
	if (!m) return b.texto;
	return b.texto
		.split('\n')
		.map((linha) => {
			const [, antes, meio, depois] = linha.match(/^(\s*)(.*?)(\s*)$/)!;
			return meio ? antes + m + meio + m + depois : linha;
		})
		.join('\n');
}

/**
 * O texto do campo de volta a blocos. As figuras são as que o campo já tinha,
 * pela ordem: "[figura 2]" é a segunda, com recorte, tamanho e conferência
 * intactos. Apagar a linha tira a figura; número que não existe fica como
 * texto, à vista na prévia.
 */
export function textoParaBlocos(texto: string, anteriores: Bloco[]): Bloco[] {
	const figuras = anteriores.filter((b) => b.tipo === 'imagem');
	const out: Bloco[] = [];
	let prosa: string[] = [];
	let codigo: string[] | null = null;

	const fecharProsa = () => {
		const t = prosa.join('\n').replace(/^\n+|\n+$/g, '');
		prosa = [];
		if (t.trim()) out.push(...emLinha(t));
	};

	for (const linha of texto.split('\n')) {
		if (codigo) {
			if (CERCA.test(linha)) {
				out.push({ ...novoBloco('codigo'), texto: codigo.join('\n') });
				codigo = null;
			} else codigo.push(linha);
			continue;
		}
		if (CERCA.test(linha)) {
			fecharProsa();
			codigo = [];
			continue;
		}
		const numero = linha.match(FIGURA)?.[1];
		const figura = numero ? figuras[Number(numero) - 1] : undefined;
		if (figura) {
			fecharProsa();
			out.push(figura);
			continue;
		}
		prosa.push(linha);
	}

	// Bloco de código sem o fecho vai até o fim, como a prévia mostra.
	if (codigo) out.push({ ...novoBloco('codigo'), texto: codigo.join('\n') });
	else fecharProsa();
	return out;
}

function emLinha(texto: string): Bloco[] {
	const out: Bloco[] = [];
	const juntar = (t: string, formato: Formato = '') => {
		if (!t) return;
		const ultimo = out.at(-1);
		if (ultimo && ultimo.formato === formato) ultimo.texto += t;
		else out.push({ ...novoBloco(), texto: t, formato });
	};
	let desde = 0;
	for (const m of texto.matchAll(EM_LINHA)) {
		if (m[1]) continue;
		juntar(texto.slice(desde, m.index));
		if (m[2]) juntar(m[2], 'negrito');
		else if (m[3]) juntar(m[3], 'sublinhado');
		else juntar(m[4], 'italico');
		desde = m.index + m[0].length;
	}
	juntar(texto.slice(desde));
	return out;
}

/**
 * Põe a marca em volta da seleção; sem seleção, um par vazio com o cursor no
 * meio. Devolve o texto novo e onde fica o cursor.
 */
export function envolver(
	texto: string,
	inicio: number,
	fim: number,
	marca: string
): { texto: string; cursor: number } {
	const selecao = texto.slice(inicio, fim);
	const novo = texto.slice(0, inicio) + marca + selecao + marca + texto.slice(fim);
	return { texto: novo, cursor: inicio + marca.length + selecao.length + (selecao ? marca.length : 0) };
}
