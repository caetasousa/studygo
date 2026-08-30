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
	limparRegistros = () => this.run((s) => api.limparRegistros(s));
	marcarMarco = (id: string, cumprido: boolean) => this.run((s) => api.marcarMarco(s, id, cumprido));
	registrarRevisao = (id: string, questoes: number, acertos: number) =>
		this.run((s) => api.registrarRevisao(s, id, questoes, acertos));
	reordenar = (a: string, b: string) => this.run((s) => api.reordenar(s, a, b));
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

/**
 * applyTheme reflects the persisted temaUi onto <html data-theme>.
 * 'system' clears the attribute and lets tokens.css decide: dark unless the OS
 * actively asks for light, so a machine with no stated preference stays dark.
 */
export function applyTheme(tema: 'light' | 'dark' | 'system' | undefined) {
	if (!browser) return;
	const root = document.documentElement;
	if (tema === 'light' || tema === 'dark') {
		root.dataset.theme = tema;
	} else {
		delete root.dataset.theme;
	}
}
