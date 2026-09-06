/**
 * Study-domain helpers shared by the syllabus, schedule and logging screens.
 *
 * These encode three rules the UI kept re-deriving (and disagreeing about):
 *
 *  1. Group order — "conhecimentos gerais" comes before "conhecimentos
 *     específicos". Driven by the `bloco` field on the discipline, never by
 *     matching the label text.
 *  2. Hierarchical numbering — "1.", "1.1" — computed from position, so the
 *     number is never baked into the stored topic text.
 *  3. Topic splitting — some editais write an ementa as flowing prose in a
 *     single stored topic. Splitting it is a *suggestion* offered to the user,
 *     never applied silently to their data.
 */

/** Knowledge group. `ger` = conhecimentos gerais, `esp` = conhecimentos específicos. */
export type Bloco = 'esp' | 'ger';

/** Display order: gerais first, then específicas, then anything unknown. */
const ORDEM_BLOCO: Record<string, number> = { ger: 0, esp: 1 };

export const ROTULO_BLOCO: Record<Bloco, string> = {
	ger: 'Conhecimentos gerais',
	esp: 'Conhecimentos específicos'
};

export function ordemBloco(bloco: string): number {
	return ORDEM_BLOCO[bloco] ?? 2;
}

/**
 * Sort disciplines into the canonical study order: gerais first, específicas
 * next, each group keeping the order the concurso defines. Returns a new array.
 */
export function ordenarDisciplinas<T extends { bloco: string }>(disciplinas: readonly T[]): T[] {
	return [...disciplinas].sort((a, b) => ordemBloco(a.bloco) - ordemBloco(b.bloco));
}

export interface GrupoDisciplinas<T> {
	bloco: Bloco;
	rotulo: string;
	itens: T[];
}

/**
 * Group disciplines by `bloco` in display order, dropping empty groups. The
 * single place any screen should go to render "the syllabus by group".
 */
export function agruparPorBloco<T extends { bloco: string }>(
	disciplinas: readonly T[]
): GrupoDisciplinas<T>[] {
	return (['ger', 'esp'] as const)
		.map((bloco) => ({
			bloco,
			rotulo: ROTULO_BLOCO[bloco],
			itens: disciplinas.filter((d) => d.bloco === bloco)
		}))
		.filter((g) => g.itens.length > 0);
}

/**
 * Hierarchical number for a position: `numeroHierarquico(0)` -> "1",
 * `numeroHierarquico(0, 2)` -> "1.3". Levels are 0-based on input and
 * 1-based on output, so callers pass array indices directly.
 */
export function numeroHierarquico(...indices: number[]): string {
	return indices.map((i) => i + 1).join('.');
}

/**
 * A topic already carrying its own leading number ("1.2 Ortografia") would read
 * as "1.2 1.2 Ortografia" once the UI numbers it. Strip a leading numeric
 * prefix so the displayed number always comes from position.
 *
 * Deliberately conservative: only a run of digits/dots followed by a separator,
 * and never when what follows is empty.
 */
export function semNumeroInicial(texto: string): string {
	const limpo = texto.replace(/^\s*\d+(\.\d+)*\s*[.)\-–—]?\s+/, '');
	return limpo.trim() || texto.trim();
}

/** Abaixo disto um pedaço não é um assunto: é abreviação, número de lei, sobra. */
const MIN_TOPICO = 24;

/** Uma frase nova começa com maiúscula. */
const COMECA_FRASE = /^[A-ZÀ-Þ]/;

/**
 * Um item que abre com o nome de um instrumento normativo.
 *
 * É o que autoriza dividir num ponto e vírgula. Deliberadamente estreito: uma
 * lista de normativos são vários assuntos de estudo, mas o ponto e vírgula
 * genérico do edital quase sempre enumera as partes de UM assunto
 * ("fiscalização contábil, financeira; controle interno e controle externo").
 * Dividir em todos eles multiplicaria os tópicos e deixaria o cronograma
 * impossível de percorrer.
 */
const COMECA_NORMATIVO =
	/^(lei|leis|decreto|resolu[çc][ãa]o|resolu[çc][õo]es|portaria|instru[çc][ãa]o normativa|medida provis[óo]ria|emenda constitucional|s[úu]mula|ordem de servi[çc]o|plano diretor|ato normativo|regimento|estatuto|constitui[çc][ãa]o)\b/i;

/**
 * Junta os pedaços de volta em assuntos.
 *
 * Um pedaço só começa assunto NOVO quando `comecaItem` o reconhece como item e
 * o que veio antes já dá um assunto sozinho. Os dois testes existem pelo mesmo
 * motivo: pedaço curto ou que não abre item é continuação, não assunto novo.
 *
 * `juntar` é o que volta entre dois pedaços remendados, para que o texto
 * continue sendo o do edital.
 */
function agrupar(pedacos: string[], juntar: string, comecaItem: RegExp): string[] {
	const partes: string[] = [];
	let atual = '';

	for (const pedaco of pedacos) {
		if (!atual || !comecaItem.test(pedaco) || atual.length < MIN_TOPICO) {
			atual = atual ? `${atual}${juntar}${pedaco}` : pedaco;
			continue;
		}

		partes.push(atual);
		atual = pedaco;
	}

	if (atual) {
		// Uma sobra pequena demais para se sustentar pertence ao assunto anterior.
		if (partes.length > 0 && atual.length < MIN_TOPICO) {
			partes[partes.length - 1] = `${partes[partes.length - 1]}${juntar}${atual}`;
		} else {
			partes.push(atual);
		}
	}

	return partes;
}

function pedacosDe(texto: string, separador: RegExp): string[] {
	return texto
		.split(separador)
		.map((p) => p.trim())
		.filter(Boolean);
}

/**
 * Split a prose ementa into topics.
 *
 * Some editais write "Conhecimentos Gerais" as flowing prose — a single stored
 * topic of 800+ characters — while the específicas arrive already itemised.
 *
 * Dois separadores, nesta ordem:
 *
 *  1. O PONTO E VÍRGULA, e SÓ quando ele separa instrumentos normativos
 *     ("Resolução nº 13/2016, que institui o CETI; Resolução nº 14/2024, que
 *     dispõe sobre …"): cada lei é um assunto de estudo inteiro, e mantê-las
 *     juntas vira um bloco só que o cronograma não tem como distribuir. Vem
 *     antes do ponto final porque uma lista dessas é cheia de pontos que não
 *     separam nada — número de lei, data.
 *
 *     O ponto e vírgula genérico NÃO divide. No edital ele quase sempre enumera
 *     as partes de um mesmo assunto ("Constituição de 1988: Administração
 *     Pública; fiscalização contábil, financeira; controle interno e externo"),
 *     e dividir ali multiplicaria os tópicos até o cronograma ficar impossível
 *     de percorrer.
 *  2. O PONTO FINAL seguido de maiúscula, dentro de cada trecho: o único limite
 *     confiável na prosa corrida, já que o ponto também é de abreviação.
 *
 * This is a *suggestion*: callers must show the result and let the user accept,
 * edit or reject it. Never rewrite stored data with it automatically.
 */
export function sugerirTopicos(texto: string): string[] {
	const base = texto.trim();
	if (!base) return [];

	const porFrase = (t: string) => agrupar(pedacosDe(t, /(?<=\.)\s+/), ' ', COMECA_FRASE);

	const porPontoEVirgula = agrupar(pedacosDe(base, /;\s*/), '; ', COMECA_NORMATIVO);
	const partes =
		porPontoEVirgula.length > 1 ? porPontoEVirgula.flatMap(porFrase) : porFrase(base);

	const limpos = partes.map((p) => p.replace(/\s+/g, ' ').trim()).filter(Boolean);
	// If the split produced nothing useful, keep the original as one topic.
	return limpos.length > 1 ? limpos : [base];
}

/**
 * Default weight for a knowledge group: específicas count double, gerais single.
 * This is what the plan engine assumes when a discipline states no weight of its
 * own, so the form pre-fills the same number instead of showing an empty field.
 */
/**
 * Peso sugerido por bloco, usado só para PREENCHER o formulário de concurso.
 *
 * Não é a regra: quem decide o peso efetivo é o servidor, e mandar 0 significa
 * "use o padrão do bloco". Aqui isto existe para o campo já abrir com o número
 * que o usuário quase sempre quer.
 */
export const PESO_PADRAO: Record<Bloco, number> = { esp: 2, ger: 1 };

export function pareceEmentaCorrida(texto: string, minimo = 220): boolean {
	return texto.trim().length >= minimo && sugerirTopicos(texto).length > 1;
}

// ---------------------------------------------------------------------------
// Naming a study plan
//
// A record here is one *cargo* of a concurso: the two TCE-GO rows differ only by
// especialidade (Tecnologia da Informação vs Técnico Administrativo). So the
// picker switches the study plan / cargo, not the concurso — and the label has
// to separate órgão, cargo and especialidade instead of concatenating them into
// one long string that can only be truncated mid-word.
// ---------------------------------------------------------------------------

export interface RotuloPlano {
	/** Short identifier: the órgão sigla when the name carries one ("TCE-GO"). */
	orgao: string;
	/** The cargo without its especialidade suffix. */
	cargo: string;
	/** Especialidade, when the cargo states one. */
	especialidade: string;
	/** Organising banca, secondary information. */
	banca: string;
}

// Editais write the cargo as "Cargo — Especialidade: X" (or with a hyphen).
const RE_ESPECIALIDADE = /\s*[—–-]\s*especialidade\s*:\s*/i;
// A name like "TCE-GO - Técnico ..." leads with the órgão sigla.
const RE_SIGLA = /^([A-Z][A-Z0-9]{1,9}(?:-[A-Z]{2,3})?)\s*[—–-]\s+/;

/**
 * Split a concurso record into its parts for display. Purely presentational:
 * it never invents data — a field it cannot find comes back as ''.
 */
export function rotuloPlano(c: {
	nome: string;
	cargo?: string;
	banca?: string;
}): RotuloPlano {
	const nome = (c.nome ?? '').trim();
	const banca = (c.banca ?? '').trim();

	const sigla = RE_SIGLA.exec(nome);
	const orgao = sigla ? sigla[1] : nome;

	// Prefer the dedicated cargo field; fall back to what follows the sigla.
	const cargoBruto = (c.cargo ?? '').trim() || (sigla ? nome.slice(sigla[0].length) : '');

	const partes = cargoBruto.split(RE_ESPECIALIDADE);
	const cargo = (partes[0] ?? '').trim();
	const especialidade = partes.length > 1 ? partes.slice(1).join(' ').trim() : '';

	return { orgao, cargo, especialidade, banca };
}

/** One-line form, for a tooltip or a narrow context. */
export function rotuloPlanoTexto(c: { nome: string; cargo?: string; banca?: string }): string {
	const r = rotuloPlano(c);
	return [r.orgao, r.cargo, r.especialidade].filter(Boolean).join(' · ');
}

/** Initials for the compact (collapsed) badge: "TCE-GO" -> "TG". */
export function iniciaisPlano(c: { nome: string; cargo?: string; banca?: string }): string {
	const { orgao } = rotuloPlano(c);
	const letras = orgao.replace(/[^A-Za-zÀ-ÿ0-9]/g, '');
	return (letras.slice(0, 2) || '?').toUpperCase();
}

/** Case/accent-insensitive search across the fields a user would type. */
export function planoCorresponde(
	c: { nome: string; cargo?: string; banca?: string },
	termo: string
): boolean {
	const alvo = termo.trim().toLowerCase();
	if (!alvo) return true;
	const norm = (v: string) => v.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
	const r = rotuloPlano(c);
	return norm([c.nome, r.orgao, r.cargo, r.especialidade, r.banca].join(' ')).includes(norm(alvo));
}

/* ---------------------------------------------------------------------------
 * One activity's record
 *
 * The editing rules that the activity form and the store must agree on. Kept
 * pure so they can be tested without a DOM.
 * ------------------------------------------------------------------------- */

/* ---------------------------------------------------------------------------
 * Horas gravadas, minutos digitados
 *
 * O registro é gravado em HORAS: é assim que a API o transporta e que as
 * estatísticas somam a semana. Mas ninguém estuda "0,5 h" — o cronograma já
 * anuncia cada bloco em minutos, e digitar 30 é o gesto natural. Quem lança
 * converte na borda.
 *
 * O arredondamento em duas casas é o mesmo que a coluna `numeric(5,2)` do banco
 * aplica, e é o que faz o número voltar como foi digitado: 25 min viram 0,42 h,
 * que voltam a ser 25 min.
 * ------------------------------------------------------------------------- */

/** Horas gravadas -> minutos inteiros, para o campo de lançamento. */
export function horasEmMinutos(horas: number | null): number | null {
	return horas === null ? null : Math.round(horas * 60);
}

/** Minutos digitados -> horas, como a API e o banco as guardam. */
export function minutosEmHoras(minutos: number | null): number | null {
	return minutos === null ? null : Math.round((minutos / 60) * 100) / 100;
}

export interface ValoresAtividade {
	horas: number | null;
	questoes: number | null;
	acertos: number | null;
	concluido: boolean;
	nota: string;
}

export const VAZIO: ValoresAtividade = {
	horas: null,
	questoes: null,
	acertos: null,
	concluido: false,
	nota: ''
};

/** The pristine values a form opens with, for one activity's stored record. */
export function valoresIniciais(
	registro: Partial<ValoresAtividade> | null | undefined
): ValoresAtividade {
	return {
		horas: registro?.horas ?? null,
		questoes: registro?.questoes ?? null,
		acertos: registro?.acertos ?? null,
		concluido: registro?.concluido ?? false,
		nota: registro?.nota ?? ''
	};
}

/** Acertos above questões is the one combination that breaks the statistics. */
export function valoresInvalidos(v: ValoresAtividade): boolean {
	return v.questoes !== null && v.acertos !== null && v.acertos > v.questoes;
}

/**
 * Uma atividade está concluída quando o registro dela diz que sim.
 *
 * Não existe mais caminho alternativo: toda atividade tem id e todo registro
 * pertence a uma. Antes havia um fallback para a conclusão do DIA, e era ele que
 * fazia uma matéria adiantada aparecer riscada — o estudante a trouxe para o dia
 * justamente porque ela ainda estava por fazer.
 */
export function atividadeFeita(atividade: { concluido: boolean } | null | undefined): boolean {
	return atividade?.concluido ?? false;
}
