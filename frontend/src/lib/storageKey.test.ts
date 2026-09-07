import { describe, expect, it } from 'vitest';
import { chave, esquecerEm, type ArmazenamentoIteravel } from '$lib/storageKey';

/**
 * A limpeza por prefixo é o que o logout usa para não deixar o histórico de
 * estudo de quem saiu no navegador — num computador compartilhado, era o
 * cronograma inteiro parado no disco esperando o próximo a sentar ali.
 */

/** Um localStorage de mentira, com a iteração por índice que o real tem. */
function memoria(inicial: Record<string, string> = {}): ArmazenamentoIteravel {
	const dados = new Map(Object.entries(inicial));

	return {
		get length() {
			return dados.size;
		},
		key: (i) => [...dados.keys()][i] ?? null,
		getItem: (k) => dados.get(k) ?? null,
		setItem: (k, v) => void dados.set(k, v),
		removeItem: (k) => void dados.delete(k)
	};
}

function chaves(st: ArmazenamentoIteravel): string[] {
	return Array.from({ length: st.length }, (_, i) => st.key(i) ?? '');
}

describe('esquecerEm', () => {
	it('apaga as chaves do prefixo pedido e preserva o resto', () => {
		const st = memoria({
			[chave('.plano.tce-go-a1.v1')]: '{}',
			[chave('.plano.trf-1-b2.v1')]: '{}',
			[chave('.auth.v1')]: '{}',
			[chave('.concurso.ativo.v1')]: 'tce-go-a1',
			'outro-app.plano.x': '{}'
		});

		const removidas = esquecerEm(st, '.plano.');

		expect(removidas).toHaveLength(2);
		expect(chaves(st).sort()).toEqual(
			[chave('.auth.v1'), chave('.concurso.ativo.v1'), 'outro-app.plano.x'].sort()
		);
	});

	it('varre também o prefixo antigo do projeto', () => {
		const st = memoria({
			'annygo.plano.tce-go.v1': '{}',
			'studygo.plano.tce-go.v1': '{}'
		});

		expect(esquecerEm(st, '.plano.')).toHaveLength(2);
		expect(st.length).toBe(0);
	});

	// Apagar durante a iteração renumera os índices e faz a varredura pular
	// chaves — o motivo de a remoção acontecer só depois de listar.
	it('não pula chaves quando todas casam', () => {
		const st = memoria({
			[chave('.plano.a.v1')]: '1',
			[chave('.plano.b.v1')]: '2',
			[chave('.plano.c.v1')]: '3',
			[chave('.plano.d.v1')]: '4'
		});

		expect(esquecerEm(st, '.plano.')).toHaveLength(4);
		expect(st.length).toBe(0);
	});

	it('apaga o cache de um concurso só quando o prefixo é o dele', () => {
		const st = memoria({
			[chave('.plano.tce-go-a1.v1')]: '{}',
			[chave('.plano.trf-1-b2.v1')]: '{}'
		});

		esquecerEm(st, '.plano.tce-go-a1.v1');

		expect(chaves(st)).toEqual([chave('.plano.trf-1-b2.v1')]);
	});

	it('não faz nada quando não há o que esquecer', () => {
		const st = memoria({ [chave('.auth.v1')]: '{}' });

		expect(esquecerEm(st, '.plano.')).toEqual([]);
		expect(st.length).toBe(1);
	});
});
