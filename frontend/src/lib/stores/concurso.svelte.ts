import { browser } from '$app/environment';
import { api } from '$lib/api';
import { chave, lerMigrando } from '$lib/storageKey';
import type { ConcursoInput, ConcursoResumo } from '$lib/types';

const SUFIXO = '.concurso.ativo.v1';
const ATIVO_KEY = chave(SUFIXO);

function readAtivo(): string | null {
	if (!browser) return null;
	try {
		return lerMigrando(SUFIXO);
	} catch {
		return null;
	}
}

class ConcursoStore {
	lista = $state<ConcursoResumo[]>([]);
	ativoSlug = $state<string | null>(readAtivo());
	importacaoEdital = $state(false);
	carregado = $state(false);

	get ativo(): ConcursoResumo | null {
		return this.lista.find((c) => c.slug === this.ativoSlug) ?? null;
	}

	private persistAtivo() {
		if (!browser) return;
		try {
			if (this.ativoSlug) localStorage.setItem(ATIVO_KEY, this.ativoSlug);
			else localStorage.removeItem(ATIVO_KEY);
		} catch {
			/* ignore */
		}
	}

	setAtivo(slug: string | null) {
		this.ativoSlug = slug;
		this.persistAtivo();
	}

	async carregar() {
		let res;
		try {
			res = await api.listarConcursos();
		} catch {
			// 401 (stale session) etc. — auth store already cleared; the layout
			// redirects to /login. Nothing to load.
			return;
		}
		this.lista = res.concursos;
		this.importacaoEdital = res.importacaoEdital;
		this.carregado = true;

		if (this.ativoSlug && !this.lista.some((c) => c.slug === this.ativoSlug)) {
			this.ativoSlug = null;
		}
		if (!this.ativoSlug && this.lista.length > 0) {
			this.ativoSlug = this.lista[0].slug;
		}
		this.persistAtivo();
	}

	async criar(input: ConcursoInput): Promise<string> {
		const resumo = await api.criarConcurso(input);
		await this.carregar();
		this.setAtivo(resumo.slug);
		return resumo.slug;
	}

	async atualizar(slug: string, input: ConcursoInput): Promise<void> {
		await api.atualizarConcurso(slug, input);
		await this.carregar();
	}

	async remover(slug: string): Promise<void> {
		await api.removerConcurso(slug);
		if (this.ativoSlug === slug) this.ativoSlug = null;
		await this.carregar();
	}

	limpar() {
		this.lista = [];
		this.ativoSlug = null;
		this.carregado = false;
		this.persistAtivo();
	}
}

export const concursoStore = new ConcursoStore();
