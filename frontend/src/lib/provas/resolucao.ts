// Resolver uma prova publicada. As respostas ficam só no navegador: o catálogo
// não registra o que o estudante marcou (é outra entrega, com banco próprio).

import type { Questao } from './types';

export interface Resposta {
	/** Letra que o estudante marcou. */
	marcada: string;
	/** Já respondeu: a alternativa trava e o gabarito aparece. */
	conferida: boolean;
}

export type Respostas = Record<number, Resposta>;

export type EstadoAlternativa = 'livre' | 'marcada' | 'correta' | 'errada' | 'neutra';

/**
 * Como pintar uma alternativa. Antes de responder, só a marcada se destaca.
 * Depois, a do gabarito fica certa e a marcada, se for outra, errada. Sem
 * gabarito não há certo nem errado: a marcada fica neutra.
 */
export function estadoDaAlternativa(q: Questao, letra: string, r: Resposta | undefined): EstadoAlternativa {
	if (!r?.conferida) return r?.marcada === letra ? 'marcada' : 'livre';
	if (!q.resposta) return r.marcada === letra ? 'neutra' : 'livre';
	if (letra === q.resposta) return 'correta';
	return r.marcada === letra ? 'errada' : 'livre';
}

export interface Placar {
	respondidas: number;
	/** Respondidas que têm resposta no gabarito — o denominador dos acertos. */
	corrigiveis: number;
	acertos: number;
}

/** Só o número e o gabarito contam: serve à questão inteira e à avulsa. */
type Corrigivel = Pick<Questao, 'numero' | 'resposta'>;

export function placar(questoes: Corrigivel[], respostas: Respostas): Placar {
	const p: Placar = { respondidas: 0, corrigiveis: 0, acertos: 0 };
	for (const q of questoes) {
		const r = respostas[q.numero];
		if (!r?.conferida) continue;
		p.respondidas++;
		if (!q.resposta) continue;
		p.corrigiveis++;
		if (r.marcada === q.resposta) p.acertos++;
	}
	return p;
}

export type Situacao = 'aberta' | 'certa' | 'errada' | 'respondida';

/** Como a questão está para o estudante: é o que o mapa e o marcador mostram. */
export function situacaoDaQuestao(q: Pick<Questao, 'resposta'>, r: Resposta | undefined): Situacao {
	if (!r?.conferida) return 'aberta';
	if (!q.resposta) return 'respondida';
	return r.marcada === q.resposta ? 'certa' : 'errada';
}

/** Uma das 13 cores de categoria, sempre a mesma para o mesmo nome de matéria. */
export function corDaMateria(nome: string): number {
	let h = 0;
	for (const c of nome.normalize('NFC')) h = (h * 31 + c.codePointAt(0)!) % 1_000_003;
	return h % 13;
}
