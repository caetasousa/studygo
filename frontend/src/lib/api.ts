import { auth } from '$lib/stores/auth.svelte';
import type {
	AnotacaoInput,
	Caderno,
	ConcursoDetalhe,
	ConcursoInput,
	ConcursoLista,
	ConcursoResumo,
	ConfigInput,
	Dossie,
	Estatisticas,
	ImportarEditalResposta,
	PlanoResposta,
	RegistroInput
} from '$lib/types';

export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
	const headers = new Headers(init.headers);
	if (auth.accessToken) headers.set('Authorization', `Bearer ${auth.accessToken}`);
	if (init.body && !headers.has('content-type') && typeof init.body === 'string') {
		headers.set('content-type', 'application/json');
	}

	const res = await fetch(path, { ...init, headers });

	if (res.status === 401 && retry && (await auth.refresh())) {
		return request<T>(path, init, false);
	}

	if (res.status === 204) return undefined as T;

	const text = await res.text();
	const body = text ? JSON.parse(text) : undefined;

	if (!res.ok) {
		if (res.status === 401) auth.clear();
		throw new ApiError(res.status, body?.erro ?? `Erro ${res.status}`);
	}

	return body as T;
}

const planoBase = (slug: string) => `/api/concursos/${encodeURIComponent(slug)}/plano`;

export const api = {
	// ---- concursos ----
	listarConcursos: () => request<ConcursoLista>('/api/concursos'),

	getConcurso: (slug: string) => request<ConcursoDetalhe>(`/api/concursos/${encodeURIComponent(slug)}`),

	criarConcurso: (input: ConcursoInput) =>
		request<ConcursoResumo>('/api/concursos', { method: 'POST', body: JSON.stringify(input) }),

	atualizarConcurso: (slug: string, input: ConcursoInput) =>
		request<ConcursoResumo>(`/api/concursos/${encodeURIComponent(slug)}`, {
			method: 'PUT',
			body: JSON.stringify(input)
		}),

	removerConcurso: (slug: string) =>
		request<void>(`/api/concursos/${encodeURIComponent(slug)}`, { method: 'DELETE' }),

	importarEditalTexto: (texto: string) =>
		request<ImportarEditalResposta>('/api/concursos/importar', {
			method: 'POST',
			body: JSON.stringify({ texto })
		}),

	importarEditalPDF: (file: File) => {
		const form = new FormData();
		form.append('pdf', file);
		return request<ImportarEditalResposta>('/api/concursos/importar', { method: 'POST', body: form });
	},

	// ---- plano ----
	getPlano: (slug: string) => request<PlanoResposta>(planoBase(slug)),

	salvarConfig: (slug: string, input: ConfigInput) =>
		request<PlanoResposta>(planoBase(slug), { method: 'PUT', body: JSON.stringify(input) }),

	registrarDia: (slug: string, data: string, input: RegistroInput) =>
		request<PlanoResposta>(`${planoBase(slug)}/registros/${data}`, {
			method: 'PATCH',
			body: JSON.stringify(input)
		}),

	limparRegistros: (slug: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/registros`, { method: 'DELETE' }),

	marcarMarco: (slug: string, id: string, cumprido: boolean) =>
		request<PlanoResposta>(`${planoBase(slug)}/marcos/${id}`, {
			method: 'PUT',
			body: JSON.stringify({ cumprido })
		}),

	reordenar: (slug: string, dataA: string, dataB: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/reordenar`, {
			method: 'POST',
			body: JSON.stringify({ dataA, dataB })
		}),

	restaurarOrdem: (slug: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/restaurar-ordem`, { method: 'POST' }),

	estatisticas: (slug: string) => request<Estatisticas>(`${planoBase(slug)}/estatisticas`),

	caderno: (slug: string) => request<Caderno>(`${planoBase(slug)}/caderno`),

	criarAnotacao: (slug: string, input: AnotacaoInput) =>
		request<Caderno>(`${planoBase(slug)}/anotacoes`, { method: 'POST', body: JSON.stringify(input) }),

	atualizarAnotacao: (slug: string, id: string, input: AnotacaoInput) =>
		request<Caderno>(`${planoBase(slug)}/anotacoes/${id}`, {
			method: 'PATCH',
			body: JSON.stringify(input)
		}),

	removerAnotacao: (slug: string, id: string) =>
		request<Caderno>(`${planoBase(slug)}/anotacoes/${id}`, { method: 'DELETE' }),

	dossie: (slug: string, disciplina: string) =>
		request<Dossie>(`${planoBase(slug)}/dossie?disciplina=${encodeURIComponent(disciplina)}`),

	exportCsvUrl: (slug: string) => `${planoBase(slug)}/export.csv`
};
