import { expect, it } from 'vitest';
import { apagarDe, destinoDaCopia, guardarEm, lerDe } from './rascunhoLocal';
import type { Armazenamento } from '$lib/storageKey';
import type { Rascunho } from './types';

function memoria(): Armazenamento & { dados: Map<string, string> } {
	const dados = new Map<string, string>();
	return {
		dados,
		getItem: (k) => dados.get(k) ?? null,
		setItem: (k, v) => void dados.set(k, v),
		removeItem: (k) => void dados.delete(k)
	};
}

function rascunho(conferidas: boolean): Rascunho {
	return {
		banca: 'FCC',
		orgao: 'TJCE',
		ano: 2026,
		cargo: 'E05',
		cargoNome: '',
		caderno: '004',
		total: 1,
		questoes: [{ numero: 1, revisada: conferidas } as Rascunho['questoes'][number]],
		apoios: [],
		gabarito: {
			cargo: '',
			caderno: '',
			tipo: '',
			respostas: {},
			situacoes: {}
		},
		alertas: [],
		extracoes: [],
		anuladasExcluidas: []
	};
}

it('guarda, lê e apaga a cópia da revisão pelo id', () => {
	const st = memoria();
	guardarEm(st, 'imp-1', { versao: 3, rascunho: rascunho(true) });
	expect(lerDe(st, 'imp-1')?.rascunho.questoes[0].revisada).toBe(true);
	expect(lerDe(st, 'outra')).toBeNull();
	apagarDe(st, 'imp-1');
	expect(lerDe(st, 'imp-1')).toBeNull();
});

it('cópia corrompida ou armazenamento que falha não quebram a tela', () => {
	const st = memoria();
	st.dados.set('studygo.provas.revisao.imp-1', '{"versao":"x"');
	expect(lerDe(st, 'imp-1')).toBeNull();
	const quebrado: Armazenamento = {
		getItem: () => {
			throw new Error('bloqueado');
		},
		setItem: () => {
			throw new Error('cheio');
		},
		removeItem: () => {
			throw new Error('bloqueado');
		}
	};
	expect(() => guardarEm(quebrado, 'imp-1', { versao: 1, rascunho: rascunho(true) })).not.toThrow();
	expect(lerDe(quebrado, 'imp-1')).toBeNull();
});

it('só recupera sobre a mesma versão, e só se houver diferença', () => {
	const servidor = { versao: 4, rascunho: rascunho(false) };
	expect(destinoDaCopia(null, servidor)).toBe('nenhuma');
	expect(destinoDaCopia({ versao: 4, rascunho: rascunho(true) }, servidor)).toBe('recuperar');
	expect(destinoDaCopia({ versao: 4, rascunho: rascunho(false) }, servidor)).toBe('nenhuma');
	// Salva depois em outra aba: aplicar a cópia desfaria o que foi salvo.
	expect(destinoDaCopia({ versao: 3, rascunho: rascunho(true) }, servidor)).toBe('descartar');
});
