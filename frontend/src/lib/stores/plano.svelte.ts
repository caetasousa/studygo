import { browser } from '$app/environment';
import { api } from '$lib/api';
import { concursoStore } from '$lib/stores/concurso.svelte';
import { chave, lerMigrando } from '$lib/storageKey';
import type {
	AnotacaoInput,
	Atividade,
	Caderno,
	ConfigInput,
	Estatisticas,
	PlanoResposta,
	PreviewTEC,
	RegistroDiaInput
} from '$lib/types';

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
	cor: number;
	bloco: 'esp' | 'ger';
	cadernoUrl: string;
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
			m[d.codigo] = {
				nome: d.nome,
				cor: d.cor,
				bloco: d.bloco,
				cadernoUrl: d.cadernoUrl ?? ''
			};
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
	registrarDia = (data: string, input: RegistroDiaInput) =>
		this.run((s) => api.registrarDia(s, data, input));
	limparRegistros = () => this.run((s) => api.limparRegistros(s));
	marcarMarco = (id: string, cumprido: boolean) => this.run((s) => api.marcarMarco(s, id, cumprido));
	/**
	 * Grava o resultado da cauda de revisão do dia. Devolve a mensagem de erro
	 * quando falha, para que o formulário a mostre e continue aberto em vez de
	 * fechar sobre um save que não aconteceu.
	 */
	registrarRevisao = async (
		data: string,
		v: { questoes: number | null; acertos: number | null; observacao: string }
	): Promise<string | null> => {
		const dia = this.plano?.dias.find((d) => d.data === data);

		try {
			this.commit(
				await api.registrarDia(this.slug, data, { ...v, nota: dia?.nota ?? '' })
			);

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível salvar a revisão';
		}
	};

	/**
	 * Grava o registro de UMA atividade.
	 *
	 * Uma chamada só: a API é por atividade, então as outras matérias do dia não
	 * são reenviadas nem recalculadas aqui. A conclusão do dia vem derivada do
	 * servidor na resposta.
	 *
	 * Devolve a mensagem de erro quando falha, para que o formulário continue
	 * aberto mostrando o que deu errado.
	 */
	salvarAtividade = async (
		atividadeId: string,
		v: {
			horas: number | null;
			questoes: number | null;
			acertos: number | null;
			concluido: boolean;
			nota: string;
		}
	): Promise<string | null> => {
		try {
			this.commit(await api.registrarAtividade(this.slug, { atividadeId, ...v }));

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível salvar';
		}
	};

	/**
	 * Moves one activity to (data, posicao). Waits on the server: the board is
	 * one PATCH away and rearranging optimistically used to hide the exact class
	 * of bug that led to drag being taken out — the local reorder disagreed with
	 * what the backend actually did, and the two flashed past each other.
	 */
	moverAtividade = async (
		id: string,
		data: string,
		posicao: number,
		trocar = false
	): Promise<boolean> => {
		if (this.movendo) return false;
		this.movendo = true;

		try {
			this.commit(await api.moverAtividade(this.slug, id, data, posicao, trocar), false);

			return true;
		} catch (e) {
			this.erro = e instanceof Error ? e.message : 'Não foi possível mover a atividade';

			return false;
		} finally {
			this.movendo = false;
		}
	};
	restaurarOrdem = () => this.run((s) => api.restaurarOrdem(s));
	compactarPlano = () => this.run((s) => api.compactarPlano(s));

	/**
	 * Sets one discipline's error-notebook link, from the schedule. Discipline-
	 * wide, not per-activity. Returns an error message on failure so the form can
	 * show it and stay open.
	 */
	atualizarCadernoDisciplina = async (codigo: string, url: string): Promise<string | null> => {
		try {
			this.commit(await api.atualizarCadernoDisciplina(this.slug, codigo, url), false);

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível salvar o link';
		}
	};

	/**
	 * Pushes a lost day forward. Everything after it slides one study-day along,
	 * so nothing is dropped — the plan just gets tighter at the end, which the
	 * coverage warning already reports.
	 */
	adiarDia = async (data: string): Promise<string | null> => {
		try {
			this.commit(await api.adiarDia(this.slug, data), false);

			return null;
		} catch (e) {
			return e instanceof Error ? e.message : 'Não foi possível adiar o dia';
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
