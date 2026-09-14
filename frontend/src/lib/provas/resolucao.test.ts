import { describe, expect, it } from 'vitest';
import { corDaMateria, estadoDaAlternativa, placar, situacaoDaQuestao } from './resolucao';
import { novaQuestao } from './revisao';
import type { Questao } from './types';

function questao(numero: number, resposta: string): Questao {
	return { ...novaQuestao(numero), resposta, completa: true };
}

describe('estadoDaAlternativa', () => {
	const q = questao(1, 'C');

	it('antes de responder, só a marcada se destaca', () => {
		expect(estadoDaAlternativa(q, 'B', { marcada: 'B', conferida: false })).toBe('marcada');
		expect(estadoDaAlternativa(q, 'C', { marcada: 'B', conferida: false })).toBe('livre');
	});

	it('depois de responder, mostra a certa e a errada marcada', () => {
		const r = { marcada: 'B', conferida: true };
		expect(estadoDaAlternativa(q, 'C', r)).toBe('correta');
		expect(estadoDaAlternativa(q, 'B', r)).toBe('errada');
		expect(estadoDaAlternativa(q, 'A', r)).toBe('livre');
	});

	it('sem gabarito não há certo nem errado', () => {
		const semGabarito = questao(2, '');
		expect(estadoDaAlternativa(semGabarito, 'B', { marcada: 'B', conferida: true })).toBe('neutra');
	});
});

it('placar conta acertos só entre as que o gabarito corrige', () => {
	const qs = [questao(1, 'A'), questao(2, 'B'), questao(3, ''), questao(4, 'D')];
	const p = placar(qs, {
		1: { marcada: 'A', conferida: true },
		2: { marcada: 'C', conferida: true },
		3: { marcada: 'E', conferida: true },
		4: { marcada: 'D', conferida: false }
	});
	expect(p).toEqual({ respondidas: 3, corrigiveis: 2, acertos: 1 });
});

describe('situação e cor', () => {
	const q = { resposta: 'C' } as Questao;

	it('aberta até responder; depois certa, errada ou só respondida', () => {
		expect(situacaoDaQuestao(q, undefined)).toBe('aberta');
		expect(situacaoDaQuestao(q, { marcada: 'C', conferida: false })).toBe('aberta');
		expect(situacaoDaQuestao(q, { marcada: 'C', conferida: true })).toBe('certa');
		expect(situacaoDaQuestao(q, { marcada: 'A', conferida: true })).toBe('errada');
		expect(situacaoDaQuestao({ resposta: '' } as Questao, { marcada: 'A', conferida: true })).toBe('respondida');
	});

	it('a mesma matéria tem sempre a mesma cor', () => {
		expect(corDaMateria('Língua Portuguesa')).toBe(corDaMateria('Língua Portuguesa'));
		expect(corDaMateria('Redes')).toBeGreaterThanOrEqual(0);
		expect(corDaMateria('Redes')).toBeLessThan(13);
	});
});
