import { describe, expect, it } from 'vitest';
import {
	nomesJaUsados,
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
	conferenciasQueCairam,
	anulada,
	excluirQuestao,
	excluirNumero,
	devolverQuestao,
	proximaPendente,
	escreverNumeros,
	lerNumeros,
	regiaoDaQuestao,
	regiaoDoCaderno,
	regiaoDoTrecho,
	regiaoDoTrechoDeApoio,
	trechoDeApoioInicial,
	removerApoio,
	retanguloInicial,
	retanguloParaEnviar,
	trechoInicial,
	eTrecho,
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
		extracoes: [],
		excluidas: []
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

it('anulada identifica a questão sem letra no gabarito', () => {
	const r = rascunho([conferida(1), conferida(2), conferida(3)], 3);
	r.gabarito.respostas = { '1': 'A', '2': '', '3': 'C' };
	expect([anulada(r, 1), anulada(r, 2), anulada(r, 9)]).toEqual([false, true, false]);
});

it('a questão excluída sai da prova sem virar questão que falta, e devolvida volta a faltar', () => {
	const r = rascunho([conferida(1), conferida(2), conferida(3)], 3);

	excluirQuestao(r, 1);
	expect(r.questoes.map((q) => q.numero)).toEqual([1, 3]);
	expect(r.excluidas).toEqual([2]);
	expect(numerosFaltando(r)).toEqual([]);

	devolverQuestao(r, 2);
	expect(r.excluidas).toEqual([]);
	expect(numerosFaltando(r)).toEqual([2]);

	// A que a extração não achou também pode ficar de fora.
	excluirNumero(r, 2);
	excluirNumero(r, 2);
	expect(r.excluidas).toEqual([2]);
});

it('a leitura repetida ou fora da numeração só sai do rascunho', () => {
	const r = rascunho([conferida(1), conferida(2), conferida(2), conferida(7)], 3);

	excluirQuestao(r, 2);
	excluirQuestao(r, 2);
	expect(r.questoes.map((q) => q.numero)).toEqual([1, 2]);
	expect(r.excluidas).toEqual([]);
});

it('conferenciasQueCairam aponta o que foi enviado conferido e voltou sem a marca', () => {
	const enviado = rascunho([conferida(1), conferida(2), conferida(3)]);
	enviado.questoes[2].revisada = false;
	const recebido = structuredClone(enviado);
	recebido.questoes[1].revisada = false;
	expect(conferenciasQueCairam(enviado, recebido)).toEqual({ questoes: [2], textos: 0 });
	expect(conferenciasQueCairam(enviado, enviado)).toEqual({ questoes: [], textos: 0 });
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

	it('a releitura e o trecho dizem de qual questão são', () => {
		expect(rotuloDaRegiao({ ...regiao, regiao: 'q22' }, 13, 16)).toBe('Página 1 · releitura da questão 22');
		expect(rotuloDaRegiao({ ...regiao, regiao: 't22' }, 14, 16)).toBe('Página 1 · trecho marcado da questão 22');
		expect(rotuloDaRegiao({ ...regiao, regiao: 'ta:r3-t1' }, 15, 16)).toBe(
			'Página 1 · trecho marcado de um texto de apoio'
		);
		expect(rotuloDaRegiao(regiao, 3, 13)).toBe('Página 1 · região 4 de 13');
	});
});

describe('trecho', () => {
	// Duas faixas na página 1, uma na 2, e a releitura e o trecho de questões.
	const regioes: Origem[] = [
		{ pagina: 1, retangulo: [0, 0, 600, 850], regiao: '0' },
		{ pagina: 1, retangulo: [0, 700, 600, 1550], regiao: '1' },
		{ pagina: 2, retangulo: [0, 0, 600, 850], regiao: '2' },
		{ pagina: 2, retangulo: [0, 300, 600, 500], regiao: 'q7' },
		{ pagina: 2, retangulo: [40, 310, 560, 480], regiao: 't7' }
	];
	const lidaEm = (o: Origem): Questao => ({ ...conferida(7), origens: [o] });

	it('começa na faixa do caderno em que a questão foi lida, nunca numa releitura ou trecho', () => {
		expect(regiaoDoTrecho(regioes, lidaEm(regioes[4]))).toBe(2);
		expect(regiaoDoTrecho(regioes, lidaEm({ pagina: 1, retangulo: [80, 1000, 500, 1200], regiao: '1' }))).toBe(1);
		expect(regiaoDoTrecho(regioes, undefined)).toBe(0);
		expect(regiaoDoCaderno(regioes[3]) || regiaoDoCaderno(regioes[4])).toBe(false);
		expect(eTrecho(regioes[4]) && !eTrecho(regioes[3])).toBe(true);
	});

	it('o trecho do texto de apoio começa na faixa e no começo dele, e vai até o pé', () => {
		const comTexto = [...regioes, { pagina: 2, retangulo: [40, 100, 560, 800], regiao: 'ta:r2-t1' }];
		const apoio = (o: Origem) => ({ ...novoApoio([]), origens: [o] });
		// O texto inteiro cabe só na primeira faixa da página 1.
		expect(regiaoDoTrechoDeApoio(comTexto, apoio({ pagina: 1, retangulo: [40, 100, 560, 600], regiao: '0' }))).toBe(0);
		expect(regiaoDoTrechoDeApoio(comTexto, apoio({ pagina: 1, retangulo: [40, 900, 560, 1400], regiao: '1' }))).toBe(1);
		expect(regiaoDoTrechoDeApoio(comTexto, undefined)).toBe(0);
		expect(regiaoDoCaderno(comTexto[5]) || !eTrecho(comTexto[5])).toBe(false);

		const faixa = regioes[0];
		expect(trechoDeApoioInicial(faixa, apoio({ pagina: 1, retangulo: [40, 400, 560, 450], regiao: '0' }))).toEqual([
			0,
			400 - 0.01 * 850,
			600,
			850
		]);
		// Sem origem na faixa, a faixa inteira.
		expect(trechoDeApoioInicial(faixa, apoio({ pagina: 2, retangulo: [40, 400, 560, 450], regiao: '2' }))).toEqual(
			faixa.retangulo
		);
	});

	// O pé da folha A4 é 841,9199…: arredondado, virava 841,92, um centésimo
	// fora da página, e o processador recusava o trecho da 60 do TRT-15.
	it('o retângulo arredondado não passa da borda da região', () => {
		const a4 = [0, 0, 595.4400024414062, 841.9199829101562];
		const r = retanguloParaEnviar([40, 704.123, 700, 900], a4)!;
		expect(r[1]).toBe(704.12);
		expect(r[2]).toBeLessThanOrEqual(a4[2]);
		expect(r[3]).toBeLessThanOrEqual(a4[3]);
		expect(r[3]).toBeGreaterThan(841.91);
		// Invertido vira ordenado; sem tamanho, não vai.
		expect(retanguloParaEnviar([300, 500, 100, 200], a4)).toEqual([100, 200, 300, 500]);
		expect(retanguloParaEnviar([10, 10, 12, 300], a4)).toBeNull();
	});

	// A 60 do TRT-15 foi lida só até o enunciado: o trecho que começava do
	// tamanho dela relia o mesmo pedaço, sem as alternativas.
	it('o retângulo vai do alto da questão até a seguinte, na largura da região', () => {
		const q = lidaEm({ pagina: 2, retangulo: [80, 320, 500, 470], regiao: '2' });
		const anterior = { ...conferida(6), origens: [{ pagina: 2, retangulo: [80, 100, 500, 300], regiao: '2' }] };
		const seguinte = { ...conferida(8), origens: [{ pagina: 2, retangulo: [80, 600, 500, 800], regiao: '2' }] };
		// Com folga: 1% da altura acima, 4% depois do começo da seguinte.
		expect(trechoInicial(regioes[2], q, [anterior, q, seguinte])).toEqual([0, 311.5, 600, 634]);
		// A última da página vai até o pé da região.
		expect(trechoInicial(regioes[2], q, [anterior, q])).toEqual([0, 311.5, 600, 850]);
		// Em outra página, ou sem origem, a região inteira.
		expect(trechoInicial(regioes[0], q, [q])).toEqual([0, 0, 600, 850]);
		expect(trechoInicial(regioes[0], undefined)).toEqual([0, 0, 600, 850]);
	});
});

describe('nomes já usados', () => {
	it('matérias de tudo, assuntos só da matéria, qualquer grafia', () => {
		const prova = [
			{ disciplina: 'Língua Portuguesa', assunto: 'Crase' },
			{ disciplina: 'Redes', assunto: 'DNS' }
		];
		const catalogo = [
			{ disciplina: 'língua  portuguesa', assunto: 'Pontuação ' },
			{ disciplina: 'Língua Portuguesa', assunto: 'crase' },
			{ disciplina: 'Língua Portuguesa', assunto: 'Crase' },
			{ disciplina: 'Língua Portuguesa', assunto: '' }
		];
		expect(nomesJaUsados(' Língua Portuguesa', prova, catalogo)).toEqual({
			materias: ['Língua Portuguesa', 'Redes'],
			assuntos: ['Crase', 'Pontuação']
		});
		expect(nomesJaUsados('', prova, catalogo).assuntos).toEqual([]);
	});
});

describe('questão do OCR', () => {
	it('é problema até o curador conferir', () => {
		const q = { ...novaQuestao(4), lidaPorOcr: true };
		const r = { questoes: [q], apoios: [], gabarito: { respostas: {} } } as unknown as Rascunho;
		expect(problemasDaQuestao(q, r).some((p) => p.startsWith('Veio do OCR'))).toBe(true);
		expect(problemasDaQuestao({ ...q, revisada: true }, r).some((p) => p.startsWith('Veio do OCR'))).toBe(false);
	});
});
