// Junção de blocos de texto seguidos. A extração separa em blocos tanto um
// destaque no meio da frase ("não lhes sobra tempo " + trecho sublinhado +
// ".") quanto um parágrafo novo — e nem sempre põe a quebra de linha no
// segundo caso. Sem ela, "passado." e "Em relação…" grudavam.

const FIM_DE_FRASE = /[.!?:;]$/;
const COMECO_DE_PARAGRAFO = /^[\p{Lu}\d("“'—–-]/u;

/**
 * O que vai entre dois blocos de texto seguidos: quebra de linha quando o
 * anterior fecha a frase e o próximo começa outra; nada quando é a mesma frase
 * continuando, ou quando o próprio texto já traz o espaço ou a quebra.
 *
 * O título do texto de apoio ("Uma vela para Dario") não termina em ponto, mas
 * é linha própria: a mudança de destaque — negrito do título para o texto
 * normal — também quebra. Sem isso, o título saía colado no primeiro parágrafo.
 */
export function separador(anterior: string, proximo: string, mudouODestaque = false): string {
	if (/\s$/.test(anterior) || /^\s/.test(proximo)) return '';
	if (!COMECO_DE_PARAGRAFO.test(proximo)) return '';
	return FIM_DE_FRASE.test(anterior) || mudouODestaque ? '\n' : '';
}

// Markdown de código, como no Notion: crases marcam `código em linha`, e três
// crases abrem um bloco. É o que o curador digita, e é o que a extração usa para
// comandos no meio da frase.

export interface Parte {
	codigo: boolean;
	texto: string;
	/** A linguagem escrita depois das três crases (```bash), se houver. */
	linguagem?: string;
}

const CERCA = /```([\w+-]*)[^\S\n]*\n?([\s\S]*?)\n?```/g;

/** Separa os blocos de código (```) do texto ao redor. */
export function partesDoTexto(texto: string): Parte[] {
	const out: Parte[] = [];
	let desde = 0;
	for (const m of texto.matchAll(CERCA)) {
		const antes = texto.slice(desde, m.index);
		if (antes.trim()) out.push({ codigo: false, texto: antes.replace(/\n$/, '') });
		out.push({ codigo: true, texto: m[2], linguagem: m[1] || undefined });
		desde = m.index + m[0].length;
	}
	const resto = texto.slice(desde);
	if (resto.trim() || out.length === 0) out.push({ codigo: false, texto: desde ? resto.replace(/^\n/, '') : resto });
	return out;
}

/** Separa o código em linha (`assim`) do texto comum. Crase sem par fica como está. */
export function trechosEmLinha(texto: string): Parte[] {
	const out: Parte[] = [];
	let desde = 0;
	for (const m of texto.matchAll(/`([^`\n]+)`/g)) {
		if (m.index > desde) out.push({ codigo: false, texto: texto.slice(desde, m.index) });
		out.push({ codigo: true, texto: m[1] });
		desde = m.index + m[0].length;
	}
	if (desde < texto.length || out.length === 0) out.push({ codigo: false, texto: texto.slice(desde) });
	return out;
}

/**
 * Marca a seleção como código: crases em volta de um trecho de uma linha, bloco
 * (```) em volta de várias. Devolve o texto novo e onde fica o cursor.
 */
export function marcarComoCodigo(texto: string, inicio: number, fim: number): { texto: string; cursor: number } {
	const selecao = texto.slice(inicio, fim);
	const marcado = selecao.includes('\n') ? '```\n' + selecao + '\n```' : '`' + (selecao || 'código') + '`';
	return { texto: texto.slice(0, inicio) + marcado + texto.slice(fim), cursor: inicio + marcado.length };
}

