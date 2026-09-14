import { describe, expect, it } from 'vitest';
import {
	citaTexto,
	incompleta,
	problemasDaQuestao,
	rotuloDaRegiao,
	novoApoio,
	numerosDoAviso,
	comecoDoApoio,
	rotuloDoApoio,
	aplicarMaterias,
	aplicarRecorte,
	definirResposta,
	destinosDoRecorte,
	novaQuestao,
	novoBloco,
	numerosFaltando,
	proximaPendente,
	escreverNumeros,
	lerNumeros,
	regiaoDaQuestao,
	removerApoio,
	retanguloInicial,
	vincularApoio,
	temFigura
} from './revisao';
import type { Origem, Questao, Rascunho } from './types';

const regiao: Origem = { pagina: 1, retangulo: [0, 100, 600, 900], regiao: '3' };

function conferida(numero: number): Questao {
	return { ...novaQuestao(numero, regiao), revisada: true, completa: true };
}

function rascunho(questoes: Questao[], total = questoes.length): Rascunho {
	return {
		banca: 'FCC',
		orgao: 'TJCE',
		ano: 2026,
		cargo: 'E05',
		cargoNome: '',
		caderno: '004',
		total,
		questoes,
		apoios: [],
		gabarito: { cargo: 'E05', caderno: '4', tipo: 'preliminar', respostas: {}, situacoes: {} },
		alertas: [],
		extracoes: []
	};
}

describe('proximaPendente', () => {
	it('pula as conferidas e dá a volta', () => {
		const qs = [novaQuestao(1), conferida(2), conferida(3)];
		expect(proximaPendente(qs, 1)).toBe(0);
	});

	it('figura sem conferência é pendência mesmo com a questão marcada', () => {
		const q = conferida(2);
		q.blocos.push({ ...novoBloco('imagem'), arquivo: 'x', origem: regiao });
		expect(proximaPendente([conferida(1), q], 0)).toBe(1);
	});

	it('sem pendência devolve -1', () => {
		expect(proximaPendente([conferida(1), conferida(2)], 0)).toBe(-1);
	});
});

it('numerosFaltando aponta a questão que a extração perdeu', () => {
	expect(numerosFaltando(rascunho([conferida(1), conferida(3)], 4))).toEqual([2, 4]);
});

it('definirResposta leva a letra ao gabarito e desfaz a conferência', () => {
	const q = conferida(7);
	const r = rascunho([q]);
	definirResposta(r, q, 'C');
	expect(r.gabarito.respostas['7']).toBe('C');
	expect(q.revisada).toBe(false);
});

it('regiaoDaQuestao abre a prévia onde a questão foi lida', () => {
	const regioes: Origem[] = [
		{ ...regiao, regiao: '0' },
		{ ...regiao, regiao: '3' }
	];
	expect(regiaoDaQuestao(regioes, conferida(1))).toBe(1);
	expect(regiaoDaQuestao(regioes, undefined)).toBe(0);
});

describe('recorte', () => {
	it('substitui a figura existente e desfaz as conferências', () => {
		const q = conferida(44);
		q.blocos.push({ ...novoBloco('imagem'), arquivo: 'velho', origem: regiao, revisado: true });
		const r = rascunho([q]);
		const origem: Origem = { pagina: 1, retangulo: [10, 200, 300, 400], regiao: '3' };

		aplicarRecorte(r, q, 'q:1', 'novo', origem);

		expect(q.blocos[1]).toMatchObject({ arquivo: 'novo', origem, revisado: false });
		expect(q.revisada).toBe(false);
	});

	it('figura nova vai para a alternativa escolhida', () => {
		const q = conferida(44);
		const r = rascunho([q]);
		aplicarRecorte(r, q, 'novo:a:2', 'fig', regiao);
		expect(q.alternativas[2].blocos.at(-1)).toMatchObject({ tipo: 'imagem', arquivo: 'fig' });
	});

	it('só oferece os apoios que a questão usa', () => {
		const q = conferida(1);
		q.apoios = ['r0-t1'];
		const apoios = [
			{ ...novoApoio([]), id: 'r0-t1', questoes: [1], revisado: true },
			{ ...novoApoio([]), id: 'r5-t2', questoes: [30], revisado: true }
		];
		const chaves = destinosDoRecorte(q, apoios).map((d) => d.chave);
		expect(chaves).toContain('novo:p:r0-t1');
		expect(chaves).not.toContain('novo:p:r5-t2');
	});

	it('retângulo inicial usa a figura quando ela está na região', () => {
		const alvo = { ...novoBloco('imagem'), origem: { ...regiao, retangulo: [50, 300, 200, 400] } };
		expect(retanguloInicial(regiao, alvo)).toEqual([50, 300, 200, 400]);
		expect(retanguloInicial(regiao, undefined)).toEqual([150, 380, 450, 620]);
	});
});

describe('apoios', () => {
	it('vincularApoio liga e solta as questões, desfazendo a conferência delas', () => {
		const [q1, q2, q3] = [conferida(1), conferida(2), conferida(3)];
		q3.apoios = ['t1'];
		const r = rascunho([q1, q2, q3]);
		const apoio = { ...novoApoio([]), id: 't1', questoes: [3], revisado: true };
		r.apoios = [apoio];

		vincularApoio(r, apoio, [1, 2]);

		expect(q1.apoios).toEqual(['t1']);
		expect(q3.apoios).toEqual([]);
		expect([q1.revisada, q2.revisada, q3.revisada]).toEqual([false, false, false]);
	});

	it('removerApoio não deixa questão apontando para material que sumiu', () => {
		const q = conferida(1);
		q.apoios = ['t1'];
		const r = rascunho([q]);
		r.apoios = [{ ...novoApoio([]), id: 't1', questoes: [1], revisado: true }];

		removerApoio(r, 't1');

		expect(r.apoios).toEqual([]);
		expect(q.apoios).toEqual([]);
	});

	it('lê e escreve faixas de números', () => {
		expect(lerNumeros('1-10, 12')).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12]);
		expect(lerNumeros('5 a 3, x')).toEqual([]);
		expect(escreverNumeros([12, 1, 2, 3])).toBe('1-3, 12');
	});
});

it('aplicarMaterias só mexe nas questões cuja matéria mudou', () => {
	const [q1, q21] = [conferida(1), conferida(21)];
	q1.disciplina = 'Língua Portuguesa';
	q21.disciplina = 'CONHECIMENTOS ESPECÍFICOS';
	const r = rascunho([q1, q21]);

	const n = aplicarMaterias(r, [
		{ numero: 1, materia: 'Língua Portuguesa' },
		{ numero: 21, materia: 'Redes de Computadores' }
	]);

	expect(n).toBe(1);
	expect(q21.disciplina).toBe('Redes de Computadores');
	expect([q1.revisada, q21.revisada]).toEqual([true, false]);
});

describe('texto de apoio', () => {
	const apoio = {
		...novoApoio([]),
		id: 'r0-t1',
		questoes: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		blocos: [{ ...novoBloco(), texto: 'A vida divide-se em três períodos: o que se foi, o que está sendo e o que há de vir. Desses, o que estamos atravessando é breve.' }]
	};

	it('tem nome pelas questões e mostra o começo do texto', () => {
		expect(rotuloDoApoio(apoio)).toBe('Texto das questões 1-10');
		expect(comecoDoApoio(apoio, 40)).toBe('A vida divide-se em três períodos: o que…');
	});

	it('o enunciado que fala do texto pede um material ligado; "editor de texto" não', () => {
		const q = (t: string) => ({ ...novaQuestao(1), blocos: [{ ...novoBloco(), texto: t }] });
		expect(citaTexto(q('De acordo com o texto, a fortuna'))).toBe(true);
		expect(citaTexto(q('No Texto II, o autor'))).toBe(true);
		expect(citaTexto(q('Um editor de texto salva o arquivo texto'))).toBe(false);
	});
});

describe('faixa do caderno', () => {
	it('lê as faixas que o aviso cita', () => {
		expect(numerosDoAviso('Considere o texto para responder às questões de 1 a 5.')).toEqual([1, 2, 3, 4, 5]);
		expect(numerosDoAviso('As questões de números 11 a 13 referem-se ao texto')).toEqual([11, 12, 13]);
		expect(numerosDoAviso('Considere os textos para as questões 21 e 22.')).toEqual([21, 22]);
	});

	it('faixa absurda não liga nada', () => {
		expect(numerosDoAviso('o caderno tem 60 questões de 1 a 60')).toEqual([]);
		expect(numerosDoAviso('')).toEqual([]);
	});

	it('texto novo recebe um id que ninguém usa', () => {
		const a = novoApoio([]);
		expect(novoApoio([a, { ...a, id: 'm2' }]).id).toBe('m3');
	});
});

describe('figura', () => {
	it('conta figura no enunciado e nas alternativas; código não é figura', () => {
		const q = conferida(12);
		expect(temFigura(q)).toBe(false);
		q.blocos = [...q.blocos, { ...novoBloco(), tipo: 'codigo', texto: 'SELECT 1' }];
		expect(temFigura(q)).toBe(false);
		q.alternativas[3].blocos = [{ ...novoBloco(), tipo: 'imagem', arquivo: 'fig' }];
		expect(temFigura(q)).toBe(true);
	});
});

describe('problemas da questão', () => {
	it('diz quais alternativas faltam e aponta a cortada', () => {
		const q = { ...conferida(22), completa: false };
		q.alternativas = q.alternativas.slice(0, 1);
		q.alternativas[0].blocos = [{ ...novoBloco(), texto: 'RECOVERY' }];
		q.blocos = [{ ...novoBloco(), texto: 'Considere o banco' }];
		expect(problemasDaQuestao(q, rascunho([q]))).toEqual([
			'Tem 1 de 5 alternativas; faltam B, C, D, E.',
			'A extração marcou a questão como cortada.'
		]);
		expect(incompleta(q)).toBe(true);
	});

	it('questão inteira e com a resposta do gabarito não tem problema', () => {
		const q = conferida(1);
		q.blocos = [{ ...novoBloco(), texto: 'Enunciado' }];
		q.alternativas.forEach((a) => (a.blocos = [{ ...novoBloco(), texto: a.letra }]));
		q.resposta = 'C';
		const r = rascunho([q]);
		r.gabarito.respostas = { '1': 'C' };
		expect(problemasDaQuestao(q, r)).toEqual([]);
		expect(incompleta(q)).toBe(false);

		q.resposta = 'A';
		expect(problemasDaQuestao(q, r)).toEqual(['A resposta (A) difere do gabarito (C).']);
	});

	it('a releitura diz de qual questão é', () => {
		expect(rotuloDaRegiao({ ...regiao, regiao: 'q22' }, 13, 16)).toBe('Página 1 · releitura da questão 22');
		expect(rotuloDaRegiao(regiao, 3, 13)).toBe('Página 1 · região 4 de 13');
	});
});
