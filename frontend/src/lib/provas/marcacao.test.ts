import { describe, expect, it } from 'vitest';
import { blocosParaTexto, envolver, textoParaBlocos } from './marcacao';
import { novoBloco } from './revisao';
import type { Bloco, Formato } from './types';

const texto = (t: string, formato: Formato = ''): Bloco => ({ ...novoBloco(), texto: t, formato });
const codigo = (t: string): Bloco => ({ ...novoBloco('codigo'), texto: t });
const figura = (arquivo: string): Bloco => ({ ...novoBloco('imagem'), arquivo, largura: 60, revisado: true });
const resumo = (bs: Bloco[]) => bs.map((b) => [b.tipo, b.formato, b.texto, b.arquivo]);

// A questão 1 do TJCE como a extração devolve: o destaque partido em blocos.
const Q1 = [
	texto('não lhes sobra tempo '),
	texto('para examinar o passado', 'sublinhado'),
	texto('.'),
	texto('Em relação à oração que a antecede, a oração sublinhada expressa ideia de')
];

describe('blocosParaTexto', () => {
	it('o destaque vira marcação no meio da frase, e a frase nova vira linha', () => {
		expect(blocosParaTexto(Q1)).toBe(
			'não lhes sobra tempo __para examinar o passado__.\nEm relação à oração que a antecede, a oração sublinhada expressa ideia de'
		);
	});

	it('código e figura ficam em linhas próprias', () => {
		const bs = [texto('Considere:'), codigo('SELECT *\nFROM t\nWHERE x = ___I___'), texto('\nA lacuna I'), figura('f1')];
		expect(blocosParaTexto(bs)).toBe('Considere:\n```\nSELECT *\nFROM t\nWHERE x = ___I___\n```\nA lacuna I\n[figura 1]');
	});

	it('espaço nas pontas do destaque fica fora da marca', () => {
		expect(blocosParaTexto([texto('a'), texto(' negrito ', 'negrito'), texto('b')])).toBe('a **negrito** b');
	});
});

describe('textoParaBlocos', () => {
	it('lê a marcação de volta, com o mesmo destaque', () => {
		expect(resumo(textoParaBlocos(blocosParaTexto(Q1), Q1))).toEqual([
			['texto', '', 'não lhes sobra tempo ', ''],
			['texto', 'sublinhado', 'para examinar o passado', ''],
			['texto', '', '.\nEm relação à oração que a antecede, a oração sublinhada expressa ideia de', '']
		]);
	});

	it('negrito, itálico e sublinhado; crase guarda o que tem dentro', () => {
		expect(resumo(textoParaBlocos('**a** *b* __c__ `x**y**` d', []))).toEqual([
			['texto', 'negrito', 'a', ''],
			['texto', '', ' ', ''],
			['texto', 'italico', 'b', ''],
			['texto', '', ' ', ''],
			['texto', 'sublinhado', 'c', ''],
			['texto', '', ' `x**y**` d', '']
		]);
	});

	it('dois destaques na mesma linha são dois, não um', () => {
		expect(resumo(textoParaBlocos('**a** e **bc**', []))).toEqual([
			['texto', 'negrito', 'a', ''],
			['texto', '', ' e ', ''],
			['texto', 'negrito', 'bc', '']
		]);
	});

	it('bloco de código não é marcação: a lacuna sublinhada fica como está', () => {
		const bs = textoParaBlocos('Considere:\n```\nWITH ___I___\n```\nA lacuna I', []);
		expect(resumo(bs)).toEqual([
			['texto', '', 'Considere:', ''],
			['codigo', '', 'WITH ___I___', ''],
			['texto', '', 'A lacuna I', '']
		]);
	});

	it('a figura volta com recorte, tamanho e conferência; apagar a linha tira a figura', () => {
		const f1 = figura('f1');
		const f2 = figura('f2');
		const bs = textoParaBlocos('Veja:\n[figura 2]\ne também', [texto('x'), f1, f2]);
		expect(bs[1]).toBe(f2);
		expect(bs.includes(f1)).toBe(false);
	});

	it('figura que não existe e marca sem par ficam como texto', () => {
		expect(resumo(textoParaBlocos('[figura 3]\n2 * 3 e **aberto', []))).toEqual([
			['texto', '', '[figura 3]\n2 * 3 e **aberto', '']
		]);
	});

	it('código sem o fecho vai até o fim', () => {
		expect(resumo(textoParaBlocos('Rode:\n```\nls -l', []))).toEqual([
			['texto', '', 'Rode:', ''],
			['codigo', '', 'ls -l', '']
		]);
	});
});

describe('envolver', () => {
	it('marca a seleção e põe o cursor depois dela', () => {
		expect(envolver('abc def', 4, 7, '__')).toEqual({ texto: 'abc __def__', cursor: 11 });
	});

	it('sem seleção, abre o par com o cursor no meio', () => {
		expect(envolver('abc ', 4, 4, '**')).toEqual({ texto: 'abc ****', cursor: 6 });
	});
});
