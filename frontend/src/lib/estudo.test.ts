import { afterEach, describe, expect, it, vi } from 'vitest';
import { alvoNoPonto } from '$lib/arrastarToque';
import {
	PESO_PADRAO,
	agruparPorBloco,
	VAZIO,
	aplicarMovimento,
	blocosComAtividade,
	diaConcluido,
	numeroHierarquico,
	sigla,
	siglas,
	valoresIniciais,
	valoresInvalidos,
	ordenarDisciplinas,
	iniciaisPlano,
	pareceEmentaCorrida,
	pesoPadrao,
	planoCorresponde,
	rotuloPlano,
	semNumeroInicial,
	sugerirTopicos
} from './estudo';

// The prose ementa that ships as ONE stored topic for this concurso's
// "Conhecimentos Gerais" disciplines — the case that made the schedule show a
// whole subject in a single block.
const EMENTA_PORTUGUES =
	'Redação Oficial. Ortografia e acentuação. Emprego do sinal indicativo de crase. ' +
	'Compreensão e interpretação de textos de gêneros variados. Denotação e conotação. ' +
	'Intertextualidade. Figuras de linguagem. Morfossintaxe.';

describe('ordenarDisciplinas', () => {
	it('coloca conhecimentos gerais antes dos específicos', () => {
		const ordenado = ordenarDisciplinas([
			{ bloco: 'esp', nome: 'Banco de Dados' },
			{ bloco: 'ger', nome: 'Língua Portuguesa' },
			{ bloco: 'esp', nome: 'Segurança da Informação' },
			{ bloco: 'ger', nome: 'Matemática' }
		]);
		expect(ordenado.map((d) => d.nome)).toEqual([
			'Língua Portuguesa',
			'Matemática',
			'Banco de Dados',
			'Segurança da Informação'
		]);
	});

	it('preserva a ordem original dentro de cada grupo', () => {
		const ordenado = ordenarDisciplinas([
			{ bloco: 'esp', nome: 'Z' },
			{ bloco: 'esp', nome: 'A' }
		]);
		expect(ordenado.map((d) => d.nome)).toEqual(['Z', 'A']);
	});

	it('não altera o array recebido', () => {
		const entrada = [
			{ bloco: 'esp', nome: 'B' },
			{ bloco: 'ger', nome: 'A' }
		];
		ordenarDisciplinas(entrada);
		expect(entrada[0].nome).toBe('B');
	});
});

describe('agruparPorBloco', () => {
	it('devolve gerais primeiro e omite grupos vazios', () => {
		const grupos = agruparPorBloco([
			{ bloco: 'esp', nome: 'Banco de Dados' },
			{ bloco: 'ger', nome: 'Língua Portuguesa' }
		]);
		expect(grupos.map((g) => g.bloco)).toEqual(['ger', 'esp']);
	});

	it('omite o grupo que não tem disciplinas', () => {
		const grupos = agruparPorBloco([{ bloco: 'esp', nome: 'Só específica' }]);
		expect(grupos).toHaveLength(1);
		expect(grupos[0].bloco).toBe('esp');
	});
});

describe('numeroHierarquico', () => {
	it('numera matéria e tópico a partir da posição', () => {
		expect(numeroHierarquico(0)).toBe('1');
		expect(numeroHierarquico(1)).toBe('2');
		expect(numeroHierarquico(0, 0)).toBe('1.1');
		expect(numeroHierarquico(0, 2)).toBe('1.3');
		expect(numeroHierarquico(1, 1, 0)).toBe('2.2.1');
	});
});

describe('semNumeroInicial', () => {
	it('remove uma numeração já embutida no texto', () => {
		expect(semNumeroInicial('1.2 Ortografia')).toBe('Ortografia');
		expect(semNumeroInicial('3. Pontuação')).toBe('Pontuação');
		expect(semNumeroInicial('2) Crase')).toBe('Crase');
	});

	it('preserva texto que apenas começa com número relevante', () => {
		// A law number is content, not a list marker.
		expect(semNumeroInicial('Lei 8.666/93 e alterações')).toBe('Lei 8.666/93 e alterações');
	});

	it('nunca devolve string vazia', () => {
		expect(semNumeroInicial('1.2')).toBe('1.2');
	});
});

describe('sugerirTopicos', () => {
	it('divide uma ementa corrida em tópicos legíveis', () => {
		const topicos = sugerirTopicos(EMENTA_PORTUGUES);
		expect(topicos.length).toBeGreaterThan(3);
		// "Redação Oficial." alone is too short to stand as a topic, so it is kept
		// with the sentence that follows rather than split off on its own.
		expect(topicos[0]).toBe('Redação Oficial. Ortografia e acentuação.');
		expect(topicos.some((t) => t.includes('Figuras de linguagem'))).toBe(true);
	});

	it('não corta dentro de número de lei nem de abreviação', () => {
		const texto =
			'Lei nº 8.666/93 e suas alterações posteriores aplicáveis ao caso. ' +
			'Princípios da Administração Pública e seus desdobramentos práticos.';
		const topicos = sugerirTopicos(texto);
		expect(topicos.every((t) => !t.startsWith('666'))).toBe(true);
		expect(topicos[0]).toContain('8.666/93');
	});

	it('mantém um tópico curto como está', () => {
		expect(sugerirTopicos('Ortografia')).toEqual(['Ortografia']);
	});

	it('devolve lista vazia para texto vazio', () => {
		expect(sugerirTopicos('   ')).toEqual([]);
	});

	it('não perde conteúdo ao dividir', () => {
		const juntos = sugerirTopicos(EMENTA_PORTUGUES).join(' ');
		// Every word of the original survives the split.
		for (const palavra of ['Redação', 'crase', 'Morfossintaxe']) {
			expect(juntos).toContain(palavra);
		}
	});
});

describe('pareceEmentaCorrida', () => {
	it('reconhece a ementa longa que veio como tópico único', () => {
		expect(pareceEmentaCorrida(EMENTA_PORTUGUES.repeat(3))).toBe(true);
	});

	it('não sinaliza um tópico já granular', () => {
		expect(pareceEmentaCorrida('Ortografia e acentuação')).toBe(false);
	});
});

describe('pesoPadrao', () => {
	it('dá peso 2 às específicas e 1 às gerais', () => {
		expect(pesoPadrao('esp')).toBe(2);
		expect(pesoPadrao('ger')).toBe(1);
		expect(PESO_PADRAO.esp).toBe(2);
		expect(PESO_PADRAO.ger).toBe(1);
	});

	it('cai no peso 1 para um grupo desconhecido', () => {
		expect(pesoPadrao('outro')).toBe(1);
	});
});

// The two records actually stored for this user: same concurso (TCE-GO), same
// cargo, different especialidade — which is why the picker switches the study
// plan, not the concurso.
const PLANO_TI = {
	nome: 'TCE-GO - Técnico de Controle Externo - Tecnologia da Informação',
	cargo: 'Técnico de Controle Externo — Especialidade: Tecnologia da Informação',
	banca: 'Fundação Carlos Chagas'
};
const PLANO_ADM = {
	nome: 'TCE-GO - Técnico de Controle Externo - Técnico Administrativo',
	cargo: 'Técnico de Controle Externo — Especialidade: Técnico Administrativo',
	banca: 'Fundação Carlos Chagas'
};

describe('rotuloPlano', () => {
	it('separa órgão, cargo e especialidade em vez de concatenar', () => {
		expect(rotuloPlano(PLANO_TI)).toEqual({
			orgao: 'TCE-GO',
			cargo: 'Técnico de Controle Externo',
			especialidade: 'Tecnologia da Informação',
			banca: 'Fundação Carlos Chagas'
		});
	});

	it('distingue dois planos que só diferem na especialidade', () => {
		const a = rotuloPlano(PLANO_TI);
		const b = rotuloPlano(PLANO_ADM);
		expect(a.orgao).toBe(b.orgao);
		expect(a.cargo).toBe(b.cargo);
		expect(a.especialidade).not.toBe(b.especialidade);
	});

	it('funciona sem especialidade', () => {
		const r = rotuloPlano({ nome: 'INSS - Analista', cargo: 'Analista do Seguro Social' });
		expect(r.cargo).toBe('Analista do Seguro Social');
		expect(r.especialidade).toBe('');
	});

	it('não inventa campos quando só há o nome', () => {
		const r = rotuloPlano({ nome: 'Concurso da Prefeitura' });
		expect(r.orgao).toBe('Concurso da Prefeitura');
		expect(r.cargo).toBe('');
		expect(r.banca).toBe('');
	});
});

describe('iniciaisPlano', () => {
	it('usa a sigla do órgão', () => {
		expect(iniciaisPlano(PLANO_TI)).toBe('TC');
	});

	it('nunca devolve string vazia', () => {
		expect(iniciaisPlano({ nome: '' })).toBe('?');
	});
});

describe('planoCorresponde', () => {
	it('encontra por órgão, cargo ou especialidade', () => {
		expect(planoCorresponde(PLANO_TI, 'tce')).toBe(true);
		expect(planoCorresponde(PLANO_TI, 'tecnologia')).toBe(true);
		expect(planoCorresponde(PLANO_TI, 'carlos chagas')).toBe(true);
	});

	it('ignora acentos', () => {
		expect(planoCorresponde(PLANO_TI, 'tecnico')).toBe(true);
	});

	it('separa os dois planos do mesmo concurso', () => {
		expect(planoCorresponde(PLANO_TI, 'administrativo')).toBe(false);
		expect(planoCorresponde(PLANO_ADM, 'administrativo')).toBe(true);
	});

	it('termo vazio casa com tudo', () => {
		expect(planoCorresponde(PLANO_TI, '  ')).toBe(true);
	});
});

describe('sigla', () => {
	// The exact examples the spec calls for.
	it.each([
		['Raciocínio Lógico', 'RL'],
		['Língua Portuguesa', 'LP'],
		['Direito Constitucional', 'DC'],
		['Banco de Dados', 'BD'],
		['Informática', 'INF']
	])('%s -> %s', (nome, esperado) => {
		expect(sigla(nome)).toBe(esperado);
	});

	it('ignora conectivos e aberturas genéricas', () => {
		expect(sigla('Noções de Direito Administrativo')).toBe('DA');
		expect(sigla('Contabilidade aplicada ao Setor Público')).toBe('CSP');
	});

	it('trata acentos sem deixar diacrítico na sigla', () => {
		expect(sigla('Estatística')).toBe('EST');
		expect(sigla('Órgãos Públicos')).toBe('OP');
	});

	it('é determinística', () => {
		expect(sigla('Raciocínio Lógico')).toBe(sigla('Raciocínio Lógico'));
	});

	it('devolve vazio quando não há nada em que se apoiar', () => {
		expect(sigla('')).toBe('');
		expect(sigla('   ---   ')).toBe('');
	});

	it('limita a 4 letras para não virar texto', () => {
		expect(sigla('Administração Financeira e Orçamentária Pública').length).toBeLessThanOrEqual(4);
	});
});

describe('siglas', () => {
	it('resolve colisões sem hardcode por disciplina', () => {
		const mapa = siglas([
			{ codigo: 'D01', nome: 'Direito Constitucional' },
			{ codigo: 'D02', nome: 'Direito Civil' },
			{ codigo: 'D03', nome: 'Direito Comercial' }
		]);

		const valores = Object.values(mapa);
		expect(new Set(valores).size).toBe(valores.length);
		expect(mapa['D01']).toBe('DC');
	});

	it('mantém o codigo técnico como chave, nunca o substitui', () => {
		const mapa = siglas([{ codigo: 'D07', nome: 'Raciocínio Lógico' }]);
		expect(mapa['D07']).toBe('RL');
	});

	it('cai no codigo quando o nome não produz sigla', () => {
		const mapa = siglas([{ codigo: 'D09', nome: '---' }]);
		expect(mapa['D09']).toBe('D09');
	});
});

describe('aplicarMovimento', () => {
	const base = () => [
		{ data: '2026-08-31', itens: [{ id: 'a' }, { id: 'b' }] },
		{ data: '2026-09-02', itens: [{ id: 'c' }] },
		{ data: '2026-09-07', itens: [] as { id: string }[] }
	];

	const ids = (dias: { data: string; itens: { id: string }[] }[], data: string) =>
		dias.find((d) => d.data === data)!.itens.map((i) => i.id);

	const todos = (dias: { itens: { id: string }[] }[]) =>
		dias.flatMap((d) => d.itens.map((i) => i.id)).sort();

	it('troca duas matérias entre dias ocupados', () => {
		const out = aplicarMovimento(base(), 'a', '2026-09-02', 0, true);

		expect(ids(out, '2026-09-02')).toEqual(['a']);
		expect(ids(out, '2026-08-31')).toEqual(['c', 'b']);
	});

	it('move para um dia vazio sem trocar', () => {
		const out = aplicarMovimento(base(), 'a', '2026-09-07', 0, false);

		expect(ids(out, '2026-09-07')).toEqual(['a']);
		expect(ids(out, '2026-08-31')).toEqual(['b']);
	});

	it('funciona entre meses diferentes, usando a data ISO completa', () => {
		const out = aplicarMovimento(base(), 'c', '2026-08-31', 0, true);

		expect(ids(out, '2026-08-31')).toEqual(['c', 'b']);
		expect(ids(out, '2026-09-02')).toEqual(['a']);
	});

	it('não duplica nem perde matérias', () => {
		const antes = todos(base());

		for (const [id, data, pos, troca] of [
			['a', '2026-09-02', 0, true],
			['b', '2026-09-07', 0, false],
			['c', '2026-08-31', 1, false]
		] as const) {
			const out = aplicarMovimento(base(), id, data, pos, troca);
			expect(todos(out)).toEqual(antes);
		}
	});

	it('reordena dentro do mesmo dia', () => {
		const out = aplicarMovimento(base(), 'b', '2026-08-31', 0, false);
		expect(ids(out, '2026-08-31')).toEqual(['b', 'a']);
	});

	it('esvazia o dia de origem quando leva a única matéria', () => {
		const out = aplicarMovimento(base(), 'c', '2026-09-07', 0, false);

		expect(ids(out, '2026-09-02')).toEqual([]);
		expect(ids(out, '2026-09-07')).toEqual(['c']);
	});

	it('ignora um id desconhecido em vez de corromper o plano', () => {
		const out = aplicarMovimento(base(), 'zzz', '2026-09-02', 0, true);
		expect(todos(out)).toEqual(todos(base()));
	});

	it('não altera os arrays originais', () => {
		const dias = base();
		aplicarMovimento(dias, 'a', '2026-09-02', 0, true);

		expect(ids(dias, '2026-08-31')).toEqual(['a', 'b']);
		expect(ids(dias, '2026-09-02')).toEqual(['c']);
	});
});

describe('registro por atividade', () => {
	const itens = [
		{ id: 'atv-1', disciplina: 'D01' },
		{ id: 'atv-2', disciplina: 'D02' },
		// The same discipline twice in one day: each occurrence is independent.
		{ id: 'atv-3', disciplina: 'D01' }
	];

	const blocos = [
		{ atividadeId: 'atv-1', disciplina: 'D01', horas: 2, questoes: 20, acertos: 15, concluido: true, nota: 'ok' },
		{ atividadeId: 'atv-2', disciplina: 'D02', horas: 1, questoes: 10, acertos: 9, concluido: false, nota: '' }
	];

	it('carrega apenas os dados da atividade selecionada', () => {
		const so2 = blocos.find((b) => b.atividadeId === 'atv-2')!;
		expect(valoresIniciais(so2)).toEqual({
			horas: 1, questoes: 10, acertos: 9, concluido: false, nota: ''
		});
	});

	it('salvar altera somente a atividade escolhida', () => {
		const out = blocosComAtividade(itens, blocos, 'atv-2', {
			horas: 5, questoes: 50, acertos: 40, concluido: true, nota: 'nova'
		});

		expect(out.find((b) => b.atividadeId === 'atv-2')).toMatchObject({ horas: 5, nota: 'nova' });
		// the others are carried through untouched
		expect(out.find((b) => b.atividadeId === 'atv-1')).toMatchObject({
			horas: 2, questoes: 20, acertos: 15, concluido: true, nota: 'ok'
		});
	});

	it('mantém independentes duas ocorrências da mesma disciplina', () => {
		const out = blocosComAtividade(itens, blocos, 'atv-3', {
			horas: 4, questoes: null, acertos: null, concluido: true, nota: ''
		});

		expect(out.find((b) => b.atividadeId === 'atv-3')).toMatchObject({ horas: 4, concluido: true });
		// atv-1 is the same discipline (D01) and must NOT have changed
		expect(out.find((b) => b.atividadeId === 'atv-1')).toMatchObject({ horas: 2, concluido: true });
	});

	it('desmarcar preserva horas, questões, acertos e observação', () => {
		const atual = valoresIniciais(blocos[0]);
		const out = blocosComAtividade(itens, blocos, 'atv-1', { ...atual, concluido: false });
		const b = out.find((x) => x.atividadeId === 'atv-1')!;

		expect(b.concluido).toBe(false);
		expect(b).toMatchObject({ horas: 2, questoes: 20, acertos: 15, nota: 'ok' });
	});

	it('marcar não preenche horas automaticamente', () => {
		const out = blocosComAtividade(itens, blocos, 'atv-3', { ...VAZIO, concluido: true });
		expect(out.find((b) => b.atividadeId === 'atv-3')!.horas).toBeNull();
	});

	it('concluir uma matéria não conclui as outras', () => {
		const out = blocosComAtividade(itens, blocos, 'atv-2', {
			...valoresIniciais(blocos[1]), concluido: true
		});

		expect(out.find((b) => b.atividadeId === 'atv-2')!.concluido).toBe(true);
		expect(out.find((b) => b.atividadeId === 'atv-3')!.concluido).toBe(false);
		expect(diaConcluido(out)).toBe(false);
	});

	it('o dia conclui só quando todas as atividades concluem', () => {
		const todas = itens.map((it) => ({ ...VAZIO, concluido: true, disciplina: it.disciplina, atividadeId: it.id }));
		expect(diaConcluido(todas)).toBe(true);
		expect(diaConcluido([])).toBe(false);
	});

	it('cancelar não produz alteração: os valores originais seguem intactos', () => {
		const original = valoresIniciais(blocos[0]);
		// simulate editing a local copy and throwing it away
		const rascunho = { ...original, horas: 99, concluido: false };
		expect(rascunho).not.toEqual(original);
		expect(valoresIniciais(blocos[0])).toEqual(original);
	});

	it('rejeita acertos maiores que questões', () => {
		expect(valoresInvalidos({ ...VAZIO, questoes: 10, acertos: 11 })).toBe(true);
		expect(valoresInvalidos({ ...VAZIO, questoes: 10, acertos: 10 })).toBe(false);
		expect(valoresInvalidos({ ...VAZIO, questoes: null, acertos: 5 })).toBe(false);
	});

	it('adota registros antigos sem atividadeId pela disciplina', () => {
		const legado = [{ disciplina: 'D02', horas: 3, questoes: null, acertos: null, concluido: true, nota: '' }];
		const out = blocosComAtividade(itens, legado, 'atv-1', VAZIO);

		expect(out.find((b) => b.atividadeId === 'atv-2')).toMatchObject({ horas: 3, concluido: true });
	});
});

afterEach(() => vi.unstubAllGlobals());

describe('alvoNoPonto', () => {
	// The touch drop target is resolved from coordinates, since touch fires no
	// dragover. These cover the parsing contract, not the DOM hit-testing.
	function fakeElement(dia?: string, pos?: string) {
		return {
			closest: () => (dia === undefined ? null : { dataset: { atvDia: dia, atvPos: pos } })
		} as unknown as Element;
	}

	it('lê a data ISO e a posição do slot sob o ponto', () => {
		vi.stubGlobal('document', { elementFromPoint: () => fakeElement('2026-09-02', '1') });
		expect(alvoNoPonto(10, 10)).toEqual({ data: '2026-09-02', posicao: 1 });
	});

	it('devolve null quando não há atividade sob o ponto', () => {
		vi.stubGlobal('document', { elementFromPoint: () => fakeElement(undefined) });
		expect(alvoNoPonto(10, 10)).toBeNull();
	});

	it('rejeita uma posição não numérica em vez de mover para NaN', () => {
		vi.stubGlobal('document', { elementFromPoint: () => fakeElement('2026-09-02', 'abc') });
		expect(alvoNoPonto(10, 10)).toBeNull();
	});
});
