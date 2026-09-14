import { describe, expect, it } from 'vitest';
import {
	ajustarAoCatalogo,
	enderecoDoTreino,
	filtrarTreino,
	filtroDoEndereco,
	lerFiltro,
	opcoesDoTreino,
	SEM_FILTRO,
	type RespostasPorProva
} from './treino';
import type { QuestaoAvulsa } from './types';

function avulsa(provaId: string, numero: number, disciplina: string, ano: number, resposta = 'C'): QuestaoAvulsa {
	return { provaId, numero, disciplina, ano, resposta, orgao: 'TJCE', cargo: 'E05', cargoNome: '' };
}

const qs = [
	avulsa('tjce', 1, 'Língua Portuguesa', 2026),
	avulsa('tjce', 2, 'Língua Portuguesa', 2026),
	avulsa('tjce', 3, 'Redes', 2026),
	avulsa('trt', 1, 'Língua Portuguesa', 2025),
	avulsa('trt', 2, 'Banco de Dados', 2025, '')
];

// tjce 1 certa, tjce 2 errada, trt 2 respondida sem gabarito, tjce 3 só marcada.
const respostas: RespostasPorProva = {
	tjce: {
		1: { marcada: 'C', conferida: true },
		2: { marcada: 'A', conferida: true },
		3: { marcada: 'B', conferida: false }
	},
	trt: { 2: { marcada: 'D', conferida: true } }
};

function chaves(lista: QuestaoAvulsa[]) {
	return lista.map((q) => `${q.provaId}.${q.numero}`);
}

describe('filtrarTreino', () => {
	it('sem filtro, tudo', () => {
		expect(filtrarTreino(qs, SEM_FILTRO, respostas)).toHaveLength(5);
	});

	it('matérias, ano e situação se somam', () => {
		const f = { materias: ['Língua Portuguesa'], ano: 2026, situacao: 'todas' as const };
		expect(chaves(filtrarTreino(qs, f, respostas))).toEqual(['tjce.1', 'tjce.2']);
		expect(chaves(filtrarTreino(qs, { ...f, situacao: 'erradas' }, respostas))).toEqual(['tjce.2']);
	});

	it('não resolvida é a que não foi respondida, mesmo marcada', () => {
		const f = { ...SEM_FILTRO, situacao: 'abertas' as const };
		expect(chaves(filtrarTreino(qs, f, respostas))).toEqual(['tjce.3', 'trt.1']);
	});

	it('sem gabarito não conta como errada', () => {
		const f = { ...SEM_FILTRO, situacao: 'erradas' as const };
		expect(chaves(filtrarTreino(qs, f, respostas))).toEqual(['tjce.2']);
	});
});

describe('opcoesDoTreino', () => {
	it('cada grupo conta mantendo os outros critérios', () => {
		const o = opcoesDoTreino(qs, { materias: ['Língua Portuguesa'], ano: 2025, situacao: 'todas' }, respostas);
		// As matérias contam no ano escolhido, e todas aparecem, mesmo com zero.
		expect(o.materias).toEqual([
			['Banco de Dados', 1],
			['Língua Portuguesa', 1],
			['Redes', 0]
		]);
		// Os anos contam na matéria escolhida.
		expect(o.anos).toEqual([
			[2026, 2],
			[2025, 1]
		]);
		expect(o.situacoes).toEqual({ todas: 1, abertas: 1, erradas: 0 });
	});

	it('matérias em ordem alfabética do português', () => {
		const o = opcoesDoTreino([avulsa('a', 1, 'Ética', 2026), avulsa('a', 2, 'Direito', 2026)], SEM_FILTRO, {});
		expect(o.materias.map(([m]) => m)).toEqual(['Direito', 'Ética']);
	});
});

describe('filtro salvo', () => {
	it('o que não se reconhece volta ao padrão', () => {
		expect(lerFiltro(null)).toEqual(SEM_FILTRO);
		expect(lerFiltro('{')).toEqual(SEM_FILTRO);
		expect(lerFiltro('{"materias":["Redes",3],"ano":"2026","situacao":"x"}')).toEqual({
			materias: ['Redes'],
			ano: 0,
			situacao: 'todas'
		});
	});

	it('perde a matéria e o ano que o catálogo não tem mais', () => {
		const f = { materias: ['Redes', 'Contabilidade'], ano: 2019, situacao: 'erradas' as const };
		expect(ajustarAoCatalogo(f, qs)).toEqual({ materias: ['Redes'], ano: 0, situacao: 'erradas' });
	});
});

describe('endereço do treino', () => {
	it('ida e volta', () => {
		const f = { materias: ['Língua Portuguesa', 'Redes & Cia'], ano: 2026, situacao: 'abertas' as const };
		const url = new URL(enderecoDoTreino(f), 'http://x');
		expect(url.pathname).toBe('/questoes/resolver');
		expect(filtroDoEndereco(url.searchParams)).toEqual(f);
	});

	it('sem filtro, sem busca', () => {
		expect(enderecoDoTreino(SEM_FILTRO)).toBe('/questoes/resolver');
		expect(filtroDoEndereco(new URLSearchParams())).toEqual(SEM_FILTRO);
	});
});
