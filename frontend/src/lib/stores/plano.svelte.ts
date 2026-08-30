import { browser } from '$app/environment';
import { api } from '$lib/api';
import { concursoStore } from '$lib/stores/concurso.svelte';
import type {
	AnotacaoInput,
	Caderno,
	ConfigInput,
	Estatisticas,
	PlanoResposta,
	PreviewTEC,
	RegistroInput
} from '$lib/types';

const cacheKey = (slug: string) => `annygo.plano.${slug}.v1`;

function readCache(slug: string | null): PlanoResposta | null {
	if (!browser || !slug) return null;
	try {
		const raw = localStorage.getItem(cacheKey(slug));
		return raw ? (JSON.parse(raw) as PlanoResposta) : null;
	} catch {
		return null;
	}
}

interface DiscInfo {
	nome: string;
	cor: number;
	bloco: 'esp' | 'ger';
}

class PlanoStore {
	plano = $state<PlanoResposta | null>(readCache(concursoStore.ativoSlug));
	carregando = $state(false);
	erro = $state<string | null>(null);
	salvo = $state(false);

	private carregadoSlug: string | null = null;
	private toastTimer: ReturnType<typeof setTimeout> | undefined;

	discIndex = $derived.by<Record<string, DiscInfo>>(() => {
		const m: Record<string, DiscInfo> = {};
		for (const d of this.plano?.concurso.disciplinas ?? []) {
			m[d.codigo] = { nome: d.nome, cor: d.cor, bloco: d.bloco };
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

	/**
	 * Marks one discipline of one day done, or undoes it, preserving everything
	 * else already recorded for that day.
	 *
	 * Ticking with no hours logged fills in the day's default share, so a check
	 * alone still counts towards the totals; unticking takes that back only when
	 * it was filled in this way, never hours the user typed.
	 */
	concluirDisciplina = (data: string, codigo: string, marcado: boolean) => {
		const d = this.plano?.dias.find((x) => x.data === data);
		if (!d) return Promise.resolve();

		const reg = d.registro;
		const codigos = d.itens.map((i) => i.disciplina);
		const padrao = this.plano?.config.horasDia ?? null;
		const fatia =
			padrao !== null && codigos.length > 0
				? Math.round((padrao / codigos.length) * 100) / 100
				: null;

		const blocos = codigos.map((c) => {
			const b = reg?.blocos?.find((x) => x.disciplina === c);
			const eu = c === codigo;
			const feito = eu ? marcado : (b?.concluido ?? reg?.concluido ?? false);
			let horas = b?.horas ?? null;

			if (eu && marcado && horas === null) horas = fatia;
			// Undo the automatic fill, but never a value that was typed in.
			if (eu && !marcado && horas !== null && fatia !== null && horas === fatia) horas = null;

			return {
				disciplina: c,
				horas,
				questoes: b?.questoes ?? null,
				acertos: b?.acertos ?? null,
				nota: b?.nota ?? '',
				concluido: feito
			};
		});

		const soma = (f: (b: (typeof blocos)[number]) => number | null): number | null => {
			let t: number | null = null;
			for (const b of blocos) {
				const v = f(b);
				if (v !== null) t = (t ?? 0) + v;
			}
			return t;
		};

		return this.run((s) =>
			api.registrarDia(s, data, {
				horas: soma((b) => b.horas),
				questoes: soma((b) => b.questoes),
				acertos: soma((b) => b.acertos),
				// The day is done once every discipline in it is.
				concluido: blocos.length > 0 && blocos.every((b) => b.concluido),
				nota: reg?.nota ?? '',
				blocos: blocos.filter(
					(b) => b.horas !== null || b.questoes !== null || b.acertos !== null || b.concluido
				)
			})
		);
	};
	limparRegistros = () => this.run((s) => api.limparRegistros(s));
	marcarMarco = (id: string, cumprido: boolean) => this.run((s) => api.marcarMarco(s, id, cumprido));
	registrarRevisao = (id: string, questoes: number, acertos: number) =>
		this.run((s) => api.registrarRevisao(s, id, questoes, acertos));
	reordenar = (a: string, b: string) => this.run((s) => api.reordenar(s, a, b));

	/**
	 * Moves one activity. Returns whether it succeeded, so the caller can show a
	 * message and offer Undo.
	 *
	 * On failure the plan is left exactly as it was: `run` only commits a new
	 * plan when the request resolves, so a rejected move never leaves the UI
	 * showing a position the server did not accept.
	 */
	moverAtividade = async (
		id: string,
		data: string,
		posicao: number,
		trocar = false
	): Promise<boolean> => {
		try {
			this.commit(await api.moverAtividade(this.slug, id, data, posicao, trocar));

			return true;
		} catch (e) {
			this.erro = e instanceof Error ? e.message : 'Não foi possível mover a atividade';

			return false;
		}
	};
	restaurarOrdem = () => this.run((s) => api.restaurarOrdem(s));

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
