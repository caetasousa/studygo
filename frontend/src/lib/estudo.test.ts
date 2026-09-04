import { afterEach, describe, expect, it, vi } from 'vitest';
import { chave, migrar } from '$lib/storageKey';
import {
	agruparPorBloco,
	VAZIO,
	numeroHierarquico,
	valoresIniciais,
	valoresInvalidos,
	ordenarDisciplinas,
	iniciaisPlano,
	pareceEmentaCorrida,
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

afterEach(() => vi.unstubAllGlobals());

describe('migração das chaves de armazenamento', () => {
	// Renaming the project must not sign anyone out: the value stored under the
	// old annygo.* key is adopted once, then the stale key is dropped.
	function fakeStorage(inicial: Record<string, string> = {}) {
		const dados = { ...inicial };
		return {
			dados,
			getItem: (k: string) => (k in dados ? dados[k] : null),
			setItem: (k: string, v: string) => {
				dados[k] = v;
			},
			removeItem: (k: string) => {
				delete dados[k];
			}
		};
	}

	it('adota o valor da chave antiga e remove a antiga', () => {
		const st = fakeStorage({ 'annygo.auth.v1': 'token-antigo' });

		expect(migrar(st, '.auth.v1')).toBe('token-antigo');
		expect(st.dados['studygo.auth.v1']).toBe('token-antigo');
		expect('annygo.auth.v1' in st.dados).toBe(false);
	});

	it('prefere a chave nova quando as duas existem', () => {
		const st = fakeStorage({ 'annygo.auth.v1': 'antigo', 'studygo.auth.v1': 'novo' });

		expect(migrar(st, '.auth.v1')).toBe('novo');
	});

	it('devolve null quando não há nada guardado', () => {
		expect(migrar(fakeStorage(), '.auth.v1')).toBeNull();
	});

	it('não quebra quando o armazenamento lança', () => {
		const quebrado = {
			getItem: () => {
				throw new Error('bloqueado');
			},
			setItem: () => {},
			removeItem: () => {}
		};

		expect(migrar(quebrado, '.auth.v1')).toBeNull();
	});

	it('monta a chave com o prefixo atual', () => {
		expect(chave('.plano.tce.v1')).toBe('studygo.plano.tce.v1');
		expect(chave(':rail')).toBe('studygo:rail');
	});
});
