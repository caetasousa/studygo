import { browser } from '$app/environment';
import { api } from '$lib/api';
import { concursoStore } from '$lib/stores/concurso.svelte';
import { aplicarMovimento, blocosComAtividade, diaConcluido, siglas } from '$lib/estudo';
import { chave, lerMigrando } from '$lib/storageKey';
import type {
	AnotacaoInput,
	Caderno,
	ConfigInput,
	Estatisticas,
	ItemDia,
	PlanoResposta,
	PreviewTEC,
	Registro,
	RegistroBloco,
	RegistroBlocoInput,
	RegistroInput
} from '$lib/types';

/**
 * Finds what is recorded for ONE scheduled activity.
 *
 * Prefers the activity id, which is the only key that tells two occurrences of
 * the same discipline in a day apart. Falls back to the discipline for records
 * written before activities were addressable — but only when no block claims an
 * activity id, so a legacy row never shadows a properly keyed one.
 */
export function blocoDaAtividade(
	reg: Registro | null | undefined,
	item: Pick<ItemDia, 'id' | 'disciplina'>,
	itensDoDia?: readonly Pick<ItemDia, 'id' | 'disciplina'>[]
): RegistroBloco | null {
	const blocos = reg?.blocos ?? [];

	if (item.id) {
		const porID = blocos.find((b) => b.atividadeId === item.id);
		if (porID) return porID;
	}

	// A legacy block can only be attributed without guessing when the discipline
	// occurs once on that day. Otherwise showing it in both editors would double
	// hours and make a single historical check look like two completed sessions.
	const ocorrencias =
		itensDoDia?.filter((it) => it.disciplina === item.disciplina).length ?? 1;
	const legado =
		ocorrencias === 1
			? blocos.find((b) => !b.atividadeId && b.disciplina === item.disciplina)
			: undefined;

	return legado ?? null;
}

/** Optimistic local rearrangement, mirroring the backend's move/swap rules. */
function moverLocalmente(
	p: PlanoResposta,
	id: string,
	data: string,
	posicao: number,
	trocar: boolean
): PlanoResposta {
	return { ...p, dias: aplicarMovimento(p.dias, id, data, posicao, trocar) };
}

const cacheSufixo = (slug: string) => `.plano.${slug}.v1`;
const cacheKey = (slug: string) => chave(cacheSufixo(slug));

function readCache(slug: string | null): PlanoResposta | null {
	if (!browser || !slug) return null;
	try {
		const raw = lerMigrando(cacheSufixo(slug));
		return raw ? (JSON.parse(raw) as PlanoResposta) : null;
	} catch {
		return null;
	}
}

interface DiscInfo {
	nome: string;
	sigla: string;
	cor: number;
	bloco: 'esp' | 'ger';
}

class PlanoStore {
	plano = $state<PlanoResposta | null>(readCache(concursoStore.ativoSlug));
	carregando = $state(false);
	movendo = $state(false);
	erro = $state<string | null>(null);
	salvo = $state(false);

	private carregadoSlug: string | null = null;
	private toastTimer: ReturnType<typeof setTimeout> | undefined;

	discIndex = $derived.by<Record<string, DiscInfo>>(() => {
		const m: Record<string, DiscInfo> = {};
		for (const d of this.plano?.concurso.disciplinas ?? []) {
			m[d.codigo] = { nome: d.nome, sigla: d.sigla, cor: d.cor, bloco: d.bloco };
		}
		return m;
	});

	/**
	 * Display siglas by codigo — RL, LP, BD. Derived from the names, never
	 * stored: the codigo stays the technical key everything else is joined on.
	 */
	siglaIndex = $derived.by<Record<string, string>>(() => {
		const disciplinas = this.plano?.concurso.disciplinas ?? [];
		const fallback = siglas(disciplinas);
		const m: Record<string, string> = {};

		for (const d of disciplinas) {
			// New responses own this presentation value. The generated fallback keeps
			// an old localStorage cache readable during the deployment transition.
			m[d.codigo] = d.sigla?.trim() || fallback[d.codigo] || d.codigo;
		}

		return m;
	});

	private get slug(): string {
		const s = concursoStore.ativoSlug;
		if (!s) throw new Error('nenhum concurso ativo');
		return s;
	}

	private commit(p: PlanoResposta, toast = true) {
		this.plano = p;
		this.erro = null;
		if (browser && concursoStore.ativoSlug) {
			try {
				localStorage.setItem(cacheKey(concursoStore.ativoSlug), JSON.stringify(p));
			} catch {
				/* ignore quota / private mode */
			}
		}
		if (toast) this.flashSalvo();
	}

	private flashSalvo() {
		this.salvo = true;
		clearTimeout(this.toastTimer);
		this.toastTimer = setTimeout(() => (this.salvo = false), 1100);
	}

	private async run(fn: (slug: string) => Promise<PlanoResposta>) {
		try {
			this.commit(await fn(this.slug));
		} catch (e) {
			this.erro = e instanceof Error ? e.message : 'Erro inesperado';
		}
	}

	/** Load the active concurso's plan. Re-fetches when the active slug changes;
	 *  a cached copy shows instantly while a fresh one loads. */
	async carregar(force = false) {
		const slug = concursoStore.ativoSlug;
		if (!slug) {
			this.plano = null;
			return;
		}
		if (this.carregadoSlug === slug && !force) return;

		if (this.carregadoSlug !== slug) {
			this.plano = readCache(slug);
		}
		this.carregadoSlug = slug;

		const temCache = this.plano !== null;
		if (!temCache) this.carregando = true;

		try {
			this.commit(await api.getPlano(slug), false);
		} catch (e) {
			if (!temCache) this.erro = e instanceof Error ? e.message : 'Erro ao carregar';
		} finally {
			this.carregando = false;
		}
	}

	salvarConfig = (input: ConfigInput) => this.run((s) => api.salvarConfig(s, input));
	registrarDia = (data: string, input: RegistroInput) =>
		this.run((s) => api.registrarDia(s, data, input));
	limparRegistros = () => this.run((s) => api.limparRegistros(s));
	marcarMarco = (id: string, cumprido: boolean) => this.run((s) => api.marcarMarco(s, id, cumprido));
	/**
	 * Logs one day's review-tail result. Returns the error message on failure
	 * (invalid: acertos > questões) so the form can show it inline and stay
	 * open, instead of closing over a save that did not land.
	 */
	registrarRevisao = async (
		data: string,
		v: { questoes: number | null; acertos: number | null; observacao: string }
	): Promise<string | null> => {
		try {
			this.commit(await api.registrarRevisao(this.slug, data, v));

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível salvar a revisão';
		}
	};
	reordenar = (a: string, b: string) => this.run((s) => api.reordenar(s, a, b));

	/**
	 * Saves ONE scheduled activity's record, leaving every other subject of that
	 * day exactly as it was.
	 *
	 * The day's other blocks are re-sent unchanged (the API replaces the day's
	 * block set), and the day's totals are recomputed from all of them. The day
	 * counts as done only when every activity in it does — the flag is derived,
	 * never set by hand.
	 *
	 * Returns an error message on failure, or null on success, so the caller can
	 * keep the form open and show what went wrong.
	 */
	salvarAtividade = async (
		data: string,
		atividadeId: string,
		disciplina: string,
		v: {
			horas: number | null;
			questoes: number | null;
			acertos: number | null;
			concluido: boolean;
			nota: string;
		}
	): Promise<string | null> => {
		const d = this.plano?.dias.find((x) => x.data === data);
		if (!d) return 'dia não encontrado';

		const reg = d.registro;

		// The activity being recorded may still be scheduled for a later day — a
		// topic finished ahead of time is recorded on the day it was studied, and
		// the backend then moves it here. Include it, or rebuilding the day's
		// blocks from this day's items alone would drop the very record being
		// saved.
		const daqui = d.itens.some((it) => it.id === atividadeId);
		const itens = daqui
			? d.itens
			: [
					...d.itens,
					...(this.plano?.dias
						.flatMap((x) => x.itens)
						.filter((it) => it.id === atividadeId) ?? [])
				];

		// Rebuild the day's block set: this activity from the form, every other
		// from what is already stored. Addressed by atividadeId so two occurrences
		// of the same discipline in a day stay independent.
		const blocosAtividades: RegistroBlocoInput[] = blocosComAtividade(
			itens,
			reg?.blocos ?? [],
			atividadeId,
			v
		).map((b) => ({ ...b, nota: b.nota }));

		// An old row without atividadeId is deliberately left ambiguous when the
		// same discipline occurs more than once. Keep that historical row exactly
		// once while editing a new activity; UpsertRegistro replaces the whole day.
		const contagem = new Map<string, number>();
		for (const it of itens) {
			contagem.set(it.disciplina, (contagem.get(it.disciplina) ?? 0) + 1);
		}
		const legadosAmbiguos: RegistroBlocoInput[] = (reg?.blocos ?? [])
			.filter(
				(b) => !b.atividadeId && (contagem.get(b.disciplina) ?? 0) > 1
			)
			.map((b) => ({
				disciplina: b.disciplina,
				atividadeId: '',
				horas: b.horas,
				questoes: b.questoes,
				acertos: b.acertos,
				nota: b.nota,
				concluido: b.concluido
			}));
		const blocos = [...blocosAtividades, ...legadosAmbiguos];

		const soma = (f: (b: RegistroBlocoInput) => number | null): number | null => {
			let t: number | null = null;
			for (const b of blocos) {
				const x = f(b);
				if (x !== null) t = (t ?? 0) + x;
			}
			return t;
		};

		try {
			this.commit(
				await api.registrarDia(this.slug, data, {
					horas: soma((b) => b.horas),
					questoes: soma((b) => b.questoes),
					acertos: soma((b) => b.acertos),
					concluido: diaConcluido(blocosAtividades),
					nota: reg?.nota ?? '',
					blocos: blocos.filter(
						(b) =>
							b.horas !== null || b.questoes !== null || b.acertos !== null ||
							b.concluido || b.nota !== ''
					)
				})
			);

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível salvar';
		}
	};

	/**
	 * Moves one activity, optimistically.
	 *
	 * The board is rearranged locally first so the drop feels immediate, then the
	 * server's authoritative plan replaces it. If the request fails the previous
	 * plan is restored exactly, so a rejected move leaves no trace on screen.
	 */
	moverAtividade = async (
		id: string,
		data: string,
		posicao: number,
		trocar = false
	): Promise<boolean> => {
		if (this.movendo) return false;
		this.movendo = true;

		// Snapshot for rollback. A structural clone keeps the optimistic edit from
		// aliasing the object we intend to restore.
		const anterior = this.plano ? structuredClone($state.snapshot(this.plano)) : null;

		if (this.plano) this.plano = moverLocalmente(this.plano, id, data, posicao, trocar);

		try {
			this.commit(await api.moverAtividade(this.slug, id, data, posicao, trocar));

			return true;
		} catch (e) {
			// Put the board back exactly as it was before the optimistic edit.
			if (anterior) this.plano = anterior;
			this.erro = e instanceof Error ? e.message : 'Não foi possível mover a atividade';

			return false;
		} finally {
			this.movendo = false;
		}
	};
	restaurarOrdem = () => this.run((s) => api.restaurarOrdem(s));
	compactarPlano = () => this.run((s) => api.compactarPlano(s));

	/**
	 * Pushes a lost day forward. Everything after it slides one study-day along,
	 * so nothing is dropped — the plan just gets tighter at the end, which the
	 * coverage warning already reports.
	 */
	adiarDia = async (data: string): Promise<string | null> => {
		try {
			this.commit(await api.adiarDia(this.slug, data));

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível adiar o dia';
		}
	};

	/** Brings an activity forward to the day it was actually finished on. */
	anteciparAtividade = async (id: string, data: string): Promise<string | null> => {
		try {
			this.commit(await api.anteciparAtividade(this.slug, id, data));

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível antecipar';
		}
	};

	limpar() {
		this.plano = null;
		this.carregadoSlug = null;
	}

	estatisticas = (): Promise<Estatisticas> => api.estatisticas(this.slug);
	caderno = (): Promise<Caderno> => api.caderno(this.slug);
	criarAnotacao = (input: AnotacaoInput): Promise<Caderno> => api.criarAnotacao(this.slug, input);
	atualizarAnotacao = (id: string, input: AnotacaoInput): Promise<Caderno> =>
		api.atualizarAnotacao(this.slug, id, input);
	removerAnotacao = (id: string): Promise<Caderno> => api.removerAnotacao(this.slug, id);
	dossie = (disciplina: string) => api.dossie(this.slug, disciplina);
	csvUrl = () => api.exportCsvUrl(this.slug);
	previewTec = (csv: string): Promise<PreviewTEC> => api.previewTec(this.slug, csv);
	importarTec = async (csv: string, data: string): Promise<PreviewTEC> => {
		const res = await api.importarTec(this.slug, csv, data);
		await this.carregar(true);
		return res;
	};
}

export const planoStore = new PlanoStore();

/** The themes the app actually implements. 'system' follows the OS via
 *  prefers-color-scheme (see tokens.css) — it is a real option, not a stub. */
export const TEMAS = ['light', 'dark', 'system'] as const;
export type Tema = (typeof TEMAS)[number];

export function ehTema(v: unknown): v is Tema {
	return typeof v === 'string' && (TEMAS as readonly string[]).includes(v);
}

/**
 * applyTheme reflects the chosen theme onto <html data-theme>.
 *
 * 'light'/'dark' pin the attribute; 'system' removes it and lets the
 * prefers-color-scheme rules in tokens.css decide, so the OS switching between
 * light and dark updates the app live with no listener of our own.
 */
export function applyTheme(tema: Tema | undefined) {
	if (!browser) return;
	const root = document.documentElement;
	if (tema === 'light' || tema === 'dark') {
		root.dataset.theme = tema;
	} else {
		delete root.dataset.theme;
	}
}
