import { describe, expect, it } from 'vitest';
import { marcarComoCodigo, partesDoTexto, separador, trechosEmLinha } from './texto';

describe('separador', () => {
	it('destaque no meio da frase continua na mesma linha', () => {
		expect(separador('não lhes sobra tempo ', 'para examinar o passado')).toBe('');
		expect(separador('para examinar o passado', '.')).toBe('');
		expect(separador('dispor de ', 'X')).toBe('');
	});

	it('frase fechada seguida de outra vira parágrafo', () => {
		expect(separador('.', 'Em relação à oração que a antecede')).toBe('\n');
		expect(separador('o analista deve indicar:', 'I. o modo de configuração')).toBe('\n');
		expect(separador('em outra VLAN.', 'Os itens I, II e III são')).toBe('\n');
	});

	// O título do texto de apoio não termina em ponto, mas é linha própria.
	it('título em negrito não fica colado no texto', () => {
		expect(separador('Uma vela para Dario', 'Dario vem apressado', true)).toBe('\n');
		// Destaque no meio da frase continua na mesma linha.
		expect(separador('não lhes sobra', 'tempo para examinar', true)).toBe('');
		expect(separador('o termo', 'Sublinhado', false)).toBe('');
	});

	it('quebra ou espaço já presentes não dobram', () => {
		expect(separador('.\n', 'Em relação')).toBe('');
		expect(separador('.', '\nEm relação')).toBe('');
	});
});


describe('markdown de código', () => {
	it('crases marcam código em linha', () => {
		expect(trechosEmLinha('rode `chmod 755 x` e depois `ls`.')).toEqual([
			{ codigo: false, texto: 'rode ' },
			{ codigo: true, texto: 'chmod 755 x' },
			{ codigo: false, texto: ' e depois ' },
			{ codigo: true, texto: 'ls' },
			{ codigo: false, texto: '.' }
		]);
		expect(trechosEmLinha('crase `sem par')).toEqual([{ codigo: false, texto: 'crase `sem par' }]);
	});

	it('três crases abrem um bloco, com a linguagem opcional', () => {
		expect(partesDoTexto('Considere o comando:\n```bash\nrobocopy D:\\Processos /MIR\n```\nEle faz o quê?')).toEqual([
			{ codigo: false, texto: 'Considere o comando:' },
			{ codigo: true, texto: 'robocopy D:\\Processos /MIR', linguagem: 'bash' },
			{ codigo: false, texto: 'Ele faz o quê?' }
		]);
		expect(partesDoTexto('sem código')).toEqual([{ codigo: false, texto: 'sem código' }]);
	});

	it('marcar como código escolhe crase ou bloco pela seleção', () => {
		expect(marcarComoCodigo('use ls -la aqui', 4, 10).texto).toBe('use `ls -la` aqui');
		expect(marcarComoCodigo('a\nb', 0, 3).texto).toBe('```\na\nb\n```');
	});
});

