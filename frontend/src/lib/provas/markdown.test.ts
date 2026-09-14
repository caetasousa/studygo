import { describe, expect, it } from 'vitest';
import { alternarTarefa, continuarLista, lerMarkdown, prefixarLinhas, trechos } from './markdown';

describe('lerMarkdown', () => {
	it('lê os blocos de uma nota de estudo', () => {
		const nota = [
			'## Por que a C',
			'O art. 5º, inciso X, garante a intimidade.',
			'Continua na linha de baixo.',
			'',
			'- [x] ler a lei',
			'- [ ] ver a jurisprudência',
			'1. primeiro',
			'2. segundo',
			'> "Nenhuma pena passará da pessoa do condenado"',
			'---',
			'```sql',
			'SELECT * FROM t;',
			'```'
		].join('\n');

		expect(lerMarkdown(nota).map((b) => b.tipo)).toEqual([
			'titulo',
			'paragrafo',
			'lista',
			'lista',
			'citacao',
			'divisor',
			'codigo'
		]);
		const [titulo, paragrafo, tarefas, , , , codigo] = lerMarkdown(nota);
		expect(titulo).toMatchObject({ nivel: 2, trechos: [{ tipo: 'texto', texto: 'Por que a C' }] });
		expect(paragrafo).toMatchObject({ linhas: [[{ texto: 'O art. 5º, inciso X, garante a intimidade.' }], [{ texto: 'Continua na linha de baixo.' }]] });
		expect(tarefas).toMatchObject({
			ordenada: false,
			itens: [
				{ linha: 4, tarefa: { feita: true } },
				{ linha: 5, tarefa: { feita: false } }
			]
		});
		expect(codigo).toEqual({ tipo: 'codigo', linguagem: 'sql', texto: 'SELECT * FROM t;' });
	});

	it('texto vazio não tem blocos', () => {
		expect(lerMarkdown('  \n\n')).toEqual([]);
	});

	it('tarefa sem texto continua sendo tarefa', () => {
		// O servidor apara o fim: "- [ ] " chega como "- [ ]".
		expect(lerMarkdown('- [ ] ler\n- [ ]')).toEqual([
			{
				tipo: 'lista',
				ordenada: false,
				itens: [
					{ trechos: [{ tipo: 'texto', texto: 'ler' }], linha: 0, tarefa: { feita: false } },
					{ trechos: [], linha: 1, tarefa: { feita: false } }
				]
			}
		]);
	});
});

describe('trechos', () => {
	it('negrito, itálico, riscado e código em linha', () => {
		expect(trechos('**lei** e *norma* e _regra_ e ~~erro~~ e `art. 5º`')).toEqual([
			{ tipo: 'negrito', filhos: [{ tipo: 'texto', texto: 'lei' }] },
			{ tipo: 'texto', texto: ' e ' },
			{ tipo: 'italico', filhos: [{ tipo: 'texto', texto: 'norma' }] },
			{ tipo: 'texto', texto: ' e ' },
			{ tipo: 'italico', filhos: [{ tipo: 'texto', texto: 'regra' }] },
			{ tipo: 'texto', texto: ' e ' },
			{ tipo: 'riscado', filhos: [{ tipo: 'texto', texto: 'erro' }] },
			{ tipo: 'texto', texto: ' e ' },
			{ tipo: 'codigo', texto: 'art. 5º' }
		]);
	});

	it('link só com esquema seguro; javascript: fica como texto', () => {
		expect(trechos('[STF](https://portal.stf.jus.br)')).toEqual([
			{ tipo: 'link', href: 'https://portal.stf.jus.br', filhos: [{ tipo: 'texto', texto: 'STF' }] }
		]);
		expect(trechos('[clique](javascript:alert(1))')).toEqual([{ tipo: 'texto', texto: '[clique](javascript:alert(1))' }]);
	});

	it('endereço solto vira link, sem levar a pontuação do fim', () => {
		expect(trechos('veja https://www.planalto.gov.br/lei.htm.')).toEqual([
			{ tipo: 'texto', texto: 'veja ' },
			{ tipo: 'link', href: 'https://www.planalto.gov.br/lei.htm', filhos: [{ tipo: 'texto', texto: 'https://www.planalto.gov.br/lei.htm' }] },
			{ tipo: 'texto', texto: '.' }
		]);
	});

	it('sublinhado dentro de palavra não é itálico', () => {
		expect(trechos('snake_case_nome')).toEqual([{ tipo: 'texto', texto: 'snake_case_nome' }]);
	});
});

describe('edição', () => {
	it('Enter continua a lista, a tarefa desmarcada e o número seguinte', () => {
		expect(continuarLista('- item', 6)).toEqual({ texto: '- item\n- ', cursor: 9 });
		expect(continuarLista('- [x] feito', 11)).toEqual({ texto: '- [x] feito\n- [ ] ', cursor: 18 });
		expect(continuarLista('1. um', 5)).toEqual({ texto: '1. um\n2. ', cursor: 9 });
	});

	it('Enter num item vazio sai da lista; fora de lista não faz nada', () => {
		expect(continuarLista('- a\n- ', 6)).toEqual({ texto: '- a\n', cursor: 4 });
		expect(continuarLista('- a\n- [ ]', 9)).toEqual({ texto: '- a\n', cursor: 4 });
		expect(continuarLista('texto comum', 11)).toBeNull();
	});

	it('marca e desmarca a tarefa pela linha', () => {
		expect(alternarTarefa('x\n- [ ] ler', 1)).toBe('x\n- [x] ler');
		expect(alternarTarefa('- [x] ler', 0)).toBe('- [ ] ler');
	});

	it('põe e tira o prefixo das linhas da seleção', () => {
		expect(prefixarLinhas('um\ndois', 0, 7, '- ')).toEqual({ texto: '- um\n- dois', inicio: 0, fim: 11 });
		expect(prefixarLinhas('- um\n- dois', 0, 11, '- ').texto).toBe('um\ndois');
		expect(prefixarLinhas('## velho', 0, 8, '> ').texto).toBe('> velho');
	});
});
