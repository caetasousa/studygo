import { describe, expect, it } from 'vitest';
import { diaVizinho, passoDeMovimento } from './mover';
import type { Atividade, Dia } from './types';

function atv(id: string, concluido = false): Atividade {
	return {
		id,
		disciplina: 'DES',
		tema: id,
		passada: 1,
		movida: false,
		horas: null,
		questoes: null,
		acertos: null,
		erros: null,
		nota: '',
		concluido
	};
}

function dia(data: string, itens: Atividade[], tipo: Dia['tipo'] = 'est'): Dia {
	return {
		n: 1,
		data,
		semana: 1,
		fase: 'base',
		tipo,
		itens,
		tema: '',
		meta: 0,
		blocos: [],
		rotulo: '',
		concluido: false,
		horas: null,
		questoes: null,
		acertos: null,
		nota: '',
		revisao: null
	};
}

describe('passoDeMovimento', () => {
	it('troca com o vizinho dentro do próprio dia', () => {
		const dias = [dia('2026-09-07', [atv('a'), atv('b'), atv('c')])];

		expect(passoDeMovimento(dias, 'b', -1)).toEqual({
			data: '2026-09-07',
			posicao: 0,
			trocar: true
		});
		expect(passoDeMovimento(dias, 'b', 1)).toEqual({
			data: '2026-09-07',
			posicao: 2,
			trocar: true
		});
	});

	it('pula por cima das linhas já concluídas', () => {
		const dias = [dia('2026-09-07', [atv('a'), atv('b', true), atv('c')])];

		// 'c' sobe para a vaga 0, não para a 1: a do meio é história.
		expect(passoDeMovimento(dias, 'c', -1)).toEqual({
			data: '2026-09-07',
			posicao: 0,
			trocar: true
		});
	});

	it('atravessa para o dia anterior caindo no fim dele', () => {
		const dias = [dia('2026-09-07', [atv('a'), atv('b')]), dia('2026-09-08', [atv('c')])];

		expect(passoDeMovimento(dias, 'c', -1)).toEqual({
			data: '2026-09-07',
			posicao: 2,
			trocar: false
		});
	});

	it('atravessa para o dia seguinte caindo no topo dele', () => {
		const dias = [dia('2026-09-07', [atv('a')]), dia('2026-09-08', [atv('b'), atv('c')])];

		expect(passoDeMovimento(dias, 'a', 1)).toEqual({
			data: '2026-09-08',
			posicao: 0,
			trocar: false
		});
	});

	it('não devolve destino quando não há para onde ir', () => {
		const dias = [dia('2026-09-07', [atv('a')])];

		expect(passoDeMovimento(dias, 'a', -1)).toBeNull();
		expect(passoDeMovimento(dias, 'a', 1)).toBeNull();
		expect(passoDeMovimento(dias, 'inexistente', 1)).toBeNull();
	});
});

describe('diaVizinho', () => {
	it('pula o dia que não recebe atividade', () => {
		const dias = [
			dia('2026-09-07', [atv('a')]),
			dia('2026-09-08', [], 'sim'),
			dia('2026-09-09', [atv('b')])
		];

		expect(diaVizinho(dias, dias[0], 1)?.data).toBe('2026-09-09');
	});

	it('pula o dia inteiramente concluído — ele é história, não destino', () => {
		const dias = [
			dia('2026-09-07', [atv('a')]),
			dia('2026-09-08', [atv('b', true), atv('c', true)]),
			dia('2026-09-09', [atv('d')])
		];

		expect(diaVizinho(dias, dias[0], 1)?.data).toBe('2026-09-09');
	});

	it('aceita o dia vazio que ainda pode receber', () => {
		const dias = [dia('2026-09-07', [atv('a')]), dia('2026-09-08', [])];

		expect(diaVizinho(dias, dias[0], 1)?.data).toBe('2026-09-08');
	});
});
