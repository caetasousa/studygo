import { atividadeFeita } from '$lib/estudo';
import type { Dia } from '$lib/types';

/**
 * Onde uma atividade cai quando o estudante a sobe ou desce um degrau.
 *
 * Isto vivia dentro da página do cronograma. A tela Hoje passou a oferecer o
 * mesmo remanejamento — e uma segunda cópia da regra acabaria discordando da
 * primeira no primeiro caso de borda (dia todo concluído, primeira linha do
 * dia, fim do plano). Decidir aqui é o que mantém as duas telas com o mesmo
 * comportamento; quem chama só grava o resultado.
 */

/** Dias que recebem atividade: estudo, revisão semanal e revisão dirigida. */
export function ehDiaUtil(d: Dia): boolean {
	return d.tipo === 'est' || d.tipo === 'rev' || d.tipo === 'revd';
}

/** Onde a atividade está agora, ou null quando ela não está no plano. */
export function posicaoAtual(dias: Dia[], id: string): { dia: Dia; indice: number } | null {
	for (const d of dias) {
		const i = d.itens.findIndex((x) => x.id === id);
		if (i >= 0) return { dia: d, indice: i };
	}

	return null;
}

/**
 * O dia útil mais próximo daquele lado que ainda tem onde receber — um dia
 * inteiramente concluído é história, não destino.
 */
export function diaVizinho(dias: Dia[], d: Dia, passo: -1 | 1): Dia | null {
	const i = dias.findIndex((x) => x.data === d.data);
	if (i < 0) return null;

	for (let j = i + passo; j >= 0 && j < dias.length; j += passo) {
		const cand = dias[j];
		if (!ehDiaUtil(cand)) continue;

		// Um dia só com linhas concluídas está fechado para movimentação.
		if (cand.itens.length > 0 && cand.itens.every(atividadeFeita)) continue;

		return cand;
	}

	return null;
}

/**
 * A próxima vaga acima ou abaixo NO MESMO DIA que ainda não foi concluída.
 * Pular por cima das linhas prontas as deixa onde foram lançadas e põe a linha
 * que se move onde uma troca de verdade faz sentido.
 */
export function proximoAlvoNoDia(d: Dia, indice: number, passo: -1 | 1): number {
	for (let j = indice + passo; j >= 0 && j < d.itens.length; j += passo) {
		if (!atividadeFeita(d.itens[j])) return j;
	}

	return -1;
}

/** O destino de um degrau, no formato que a API de mover espera. */
export interface Destino {
	data: string;
	posicao: number;
	/** Troca com quem está na vaga, em vez de inserir ao lado. */
	trocar: boolean;
}

/**
 * Calcula UM degrau.
 *
 * Dentro do dia é troca com o vizinho NÃO concluído (TrocarAtividades no
 * servidor). Na borda do dia atravessa para o dia útil mais próximo — subindo
 * cai no FIM do dia anterior, descendo no TOPO do seguinte — com trocar=false,
 * para o dia de origem perder uma e o de destino ganhar uma.
 *
 * Devolve null quando não há para onde ir: o botão daquele lado nem aparece.
 */
export function passoDeMovimento(dias: Dia[], id: string, passo: -1 | 1): Destino | null {
	const p = posicaoAtual(dias, id);
	if (!p) return null;

	const alvo = proximoAlvoNoDia(p.dia, p.indice, passo);
	if (alvo >= 0) return { data: p.dia.data, posicao: alvo, trocar: true };

	const vizinho = diaVizinho(dias, p.dia, passo);
	if (!vizinho) return null;

	return {
		data: vizinho.data,
		posicao: passo === -1 ? vizinho.itens.length : 0,
		trocar: false
	};
}
