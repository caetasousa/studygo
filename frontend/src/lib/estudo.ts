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
export const PESO_PADRAO: Record<Bloco, number> = { esp: 2, ger: 1 };

export function pesoPadrao(bloco: string): number {
	return PESO_PADRAO[bloco as Bloco] ?? 1;
}

/** Whether a stored topic looks like an un-split prose ementa worth offering to split. */
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

/**
 * A display sigla for one discipline name.
 *
 * Multi-word names take each word's initial (Raciocínio Lógico → RL,
 * Direito Constitucional → DC), capped at 4 letters so a long title stays a
 * badge. A single word takes its first three letters (Informática → INF).
 * Returns '' when the name has nothing to build on.
 */
export function sigla(nome: string): string {
	const palavras = palavrasSignificativas(nome);
	if (palavras.length === 0) return '';

	if (palavras.length === 1) {
		return palavras[0].slice(0, 3).toUpperCase();
	}

	return palavras
		.slice(0, 4)
		.map((p) => p[0])
		.join('')
		.toUpperCase();
}

/**
 * Assigns a unique display sigla to every discipline, keyed by its codigo.
 *
 * Collisions are resolved deterministically — by lengthening the first word
 * before falling back to a numeric suffix — so two disciplines never share a
 * badge and the result does not depend on iteration luck. Input order decides
 * who keeps the shorter form, so the mapping is stable for a given concurso.
 */
export function siglas(
	disciplinas: readonly { codigo: string; nome: string }[]
): Record<string, string> {
	const out: Record<string, string> = {};
	const usadas = new Set<string>();

	for (const d of disciplinas) {
		const base = sigla(d.nome) || d.codigo.toUpperCase();
		let escolhida = base;

		if (usadas.has(escolhida)) {
			// Lengthen with the first word's next letters before resorting to a
			// digit: "DC"/"DCO" reads better than "DC"/"DC2".
			const [primeira = ''] = palavrasSignificativas(d.nome);
			for (let n = 2; n <= 4 && usadas.has(escolhida); n++) {
				const alongada = (primeira.slice(0, n) + base.slice(1)).toUpperCase();
				if (alongada !== base) escolhida = alongada;
			}
		}

		for (let n = 2; usadas.has(escolhida); n++) {
			escolhida = base + String(n);
		}

		usadas.add(escolhida);
		out[d.codigo] = escolhida;
	}

	return out;
}

/* ---------------------------------------------------------------------------
 * Optimistic activity moves
 *
 * Mirrors the backend's MoverAtividade / TrocarAtividades so a drop can be
 * shown before the server answers. The server's response is still the
 * authority — this only has to agree with it, and be reverted if it fails.
 * ------------------------------------------------------------------------- */

/** Minimal shape this needs; the real Dia/ItemDia carry much more. */
interface ItemMovivel {
	id: string;
}

interface DiaMovivel<I extends ItemMovivel> {
	data: string;
	itens: I[];
}

/**
 * Applies a move (or a swap) to a day list, returning new arrays.
 *
 * `trocar` exchanges the activity with whatever holds the target slot, so
 * neither day changes size. Otherwise it is removed from its day and inserted
 * at `posicao`. Nothing is duplicated or dropped in either case.
 */
export function aplicarMovimento<I extends ItemMovivel, D extends DiaMovivel<I>>(
	dias: readonly D[],
	id: string,
	destinoData: string,
	posicao: number,
	trocar: boolean
): D[] {
	const origem = dias.find((d) => d.itens.some((i) => i.id === id));
	const destino = dias.find((d) => d.data === destinoData);
	if (!origem || !destino || !id) return dias as D[];

	const iOrigem = origem.itens.findIndex((i) => i.id === id);
	const movida = origem.itens[iOrigem];

	// Swap: only meaningful when the slot is actually occupied by someone else.
	if (trocar && posicao < destino.itens.length && destino.itens[posicao].id !== id) {
		const alvo = destino.itens[posicao];

		return dias.map((d) => {
			if (d.data === origem.data && d.data === destino.data) {
				const itens = [...d.itens];
				itens[iOrigem] = alvo;
				itens[posicao] = movida;
				return { ...d, itens };
			}
			if (d.data === origem.data) {
				const itens = [...d.itens];
				itens[iOrigem] = alvo;
				return { ...d, itens };
			}
			if (d.data === destino.data) {
				const itens = [...d.itens];
				itens[posicao] = movida;
				return { ...d, itens };
			}
			return d;
		}) as D[];
	}

	// Plain move: take it out, then put it back at the target slot.
	return dias.map((d) => {
		if (d.data === origem.data && d.data === destino.data) {
			const itens = d.itens.filter((i) => i.id !== id);
			const at = Math.max(0, Math.min(posicao, itens.length));
			itens.splice(at, 0, movida);
			return { ...d, itens };
		}
		if (d.data === origem.data) {
			return { ...d, itens: d.itens.filter((i) => i.id !== id) };
		}
		if (d.data === destino.data) {
			const itens = [...d.itens];
			const at = Math.max(0, Math.min(posicao, itens.length));
			itens.splice(at, 0, movida);
			return { ...d, itens };
		}
		return d;
	}) as D[];
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
 * Rebuilds a day's block set with ONE activity replaced.
 *
 * Every other activity is carried through untouched, which is what keeps saving
 * or completing one subject from disturbing the others. Blocks are addressed by
 * activity id, so two occurrences of the same discipline stay independent.
 */
export function blocosComAtividade<
	I extends { id: string; disciplina: string },
	B extends Partial<ValoresAtividade> & { atividadeId?: string; disciplina?: string }
>(
	itens: readonly I[],
	blocosAtuais: readonly B[],
	atividadeId: string,
	v: ValoresAtividade
): (ValoresAtividade & { disciplina: string; atividadeId: string })[] {
	const ocorrencias = new Map<string, number>();
	for (const it of itens) {
		ocorrencias.set(it.disciplina, (ocorrencias.get(it.disciplina) ?? 0) + 1);
	}

	return itens.map((it) => {
		const porID = blocosAtuais.find((b) => b.atividadeId && b.atividadeId === it.id);
		// A legacy block only says "discipline X on this date". It is safe to
		// adopt when X occurs once; with two occurrences there is no honest way to
		// decide which activity owns it. Reusing it for both would double the
		// recorded hours and make one check complete two activities.
		const legado =
			(ocorrencias.get(it.disciplina) ?? 0) === 1
				? blocosAtuais.find((b) => !b.atividadeId && b.disciplina === it.disciplina)
				: undefined;
		const atual = porID ?? legado;

		const base = it.id === atividadeId ? v : valoresIniciais(atual);

		return { ...base, disciplina: it.disciplina, atividadeId: it.id };
	});
}

/** A day is done when every activity in it is — never set directly. */
export function diaConcluido(blocos: readonly { concluido: boolean }[]): boolean {
	return blocos.length > 0 && blocos.every((b) => b.concluido);
}

/**
 * Whether one scheduled activity counts as done.
 *
 * The block's own record decides it. The day-level flag is only a fallback for
 * LEGACY records — the ones written before activities were addressable, which
 * carry no per-block state at all.
 *
 * Falling back to the day for an activity that simply has no record is what
 * made a subject brought forward into an already-finished day show up struck
 * through, as if it had been studied: the student moved it there precisely
 * because it still had to be done. An activity with an id and no record of its
 * own is NOT done, whatever the day says.
 */
export function atividadeFeita(
	bloco: { concluido: boolean } | null | undefined,
	temRegistroPorAtividade: boolean,
	diaConcluidoFlag: boolean | undefined
): boolean {
	if (bloco) return bloco.concluido;

	// Nenhum bloco por atividade em lugar nenhum do dia: registro legado.
	if (!temRegistroPorAtividade) return diaConcluidoFlag ?? false;

	return false;
}
