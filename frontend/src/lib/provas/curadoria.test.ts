import { expect, it } from 'vitest';
import { comTitulo, semAProva, tituloDigitado } from './curadoria';
import type { ImportacaoResumo } from './types';

function importacao(id: string, provaId: string, estado: ImportacaoResumo['estado']): ImportacaoResumo {
	return {
		id,
		estado,
		provaId,
		etapa: 0,
		totalEtapas: 0,
		erro: '',
		nomeDocumento: '',
		nomeGabarito: '',
		orgao: 'TRT 18',
		ano: 2023,
		cargo: 'L12',
		cargoNome: 'Analista Judiciário',
		caderno: '001',
		criadoEm: '',
		atualizadoEm: ''
	};
}

const lista = [
	importacao('a', 'prova-1', 'publicada'),
	importacao('b', 'prova-1', 'em_revisao'),
	importacao('c', 'prova-2', 'publicada'),
	importacao('d', '', 'cancelada')
];

it('excluir a prova tira da lista todas as importações dela', () => {
	expect(semAProva(lista, 'prova-1').map((i) => i.id)).toEqual(['c', 'd']);
});

it('renomear muda só as importações publicadas daquela prova', () => {
	const nova = comTitulo(lista, 'prova-1', 'Técnico Judiciário');
	expect(nova.map((i) => i.cargoNome)).toEqual([
		'Técnico Judiciário',
		'Analista Judiciário',
		'Analista Judiciário',
		'Analista Judiciário'
	]);
	expect(lista[0].cargoNome).toBe('Analista Judiciário');
});

it('o título digitado perde os espaços repetidos', () => {
	expect(tituloDigitado('  Técnico   Judiciário \n - TI ')).toBe('Técnico Judiciário - TI');
});
