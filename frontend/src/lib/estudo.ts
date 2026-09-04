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

/**
 * Split a prose ementa into topics.
 *
 * Some editais write "Conhecimentos Gerais" as flowing prose — a single stored
 * topic of 800+ characters — while the específicas arrive already itemised.
 * Sentence-final periods are the only reliable boundary in that prose, but a
 * period is also part of abbreviations, law numbers and initials, so this
 * splits only on a period that is followed by whitespace and an uppercase
 * letter, and refuses fragments that are too short to be a real topic.
 *
 * This is a *suggestion*: callers must show the result and let the user accept,
 * edit or reject it. Never rewrite stored data with it automatically.
 */
export function sugerirTopicos(texto: string): string[] {
	const base = texto.trim();
	if (!base) return [];

	const partes: string[] = [];
	let atual = '';

	// Split on ". " only where the next piece starts like a new sentence. A piece
	// too short to be a topic on its own (an abbreviation, "Lei nº 8.666/93.")
	// stays glued to the topic it belongs to instead of becoming one.
	const MIN_TOPICO = 24;
	const tokens = base.split(/(?<=\.)\s+/);
	for (const token of tokens) {
		const pedaco = token.trim();
		if (!pedaco) continue;

		const comecaFrase = /^[A-ZÀ-Þ]/.test(pedaco);
		if (!atual || !comecaFrase || atual.length < MIN_TOPICO) {
			atual = atual ? `${atual} ${pedaco}` : pedaco;
			continue;
		}
		partes.push(atual.trim());
		atual = pedaco;
	}
	if (atual.trim()) {
		// A trailing fragment too small to stand alone belongs to the topic before it.
		if (partes.length > 0 && atual.trim().length < MIN_TOPICO) {
			partes[partes.length - 1] = `${partes[partes.length - 1]} ${atual.trim()}`;
		} else {
			partes.push(atual.trim());
		}
	}

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
 * Display siglas
 *
 * A discipline's `codigo` is a technical identifier (D01, D02…) that keys
 * records, activities and reorderings — it must never change. But it says
 * nothing to a reader, and the schedule shows it on every activity chip.
 *
 * `sigla` derives a short mnemonic from the NAME, for display only:
 * "Raciocínio Lógico" → RL, "Banco de Dados" → BD, "Informática" → INF.
 * Nothing is keyed by it.
 * ------------------------------------------------------------------------- */

/** Connectives and generic openers that carry no identity of their own. */
const IGNORADAS = new Set([
	'de','da','do','das','dos','e','em','a','o','as','os','para','com','no','na',
	'nos','nas','nocoes','nocao','aplicada','aplicado','aplicadas','aplicados',
	'geral','gerais','basica','basicas','basico','basicos','introducao','ao','aos'
]);

/** Folds the accented letters Portuguese uses down to ASCII. */
function semAcento(s: string): string {
	return s.normalize('NFD').replace(/[̀-ͯ]/g, '');
}

function palavrasSignificativas(nome: string): string[] {
	const cruas = semAcento(nome)
		.toLowerCase()
		.split(/[^a-z0-9]+/)
		.filter(Boolean);

	const boas = cruas.filter((p) => !IGNORADAS.has(p));

	// A name made entirely of ignored words still has to yield something.
	return boas.length > 0 ? boas : cruas;
}

/* ---------------------------------------------------------------------------
 * One activity's record
 *
 * The editing rules that the activity form and the store must agree on. Kept
 * pure so they can be tested without a DOM.
 * ------------------------------------------------------------------------- */

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
