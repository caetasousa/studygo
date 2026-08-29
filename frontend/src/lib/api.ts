import { auth } from '$lib/stores/auth.svelte';
import type {
	AnotacaoInput,
	Caderno,
	CargosResposta,
	ConcursoDetalhe,
	ConcursoInput,
	ConcursoLista,
	ConcursoResumo,
	ConfigInput,
	ConteudoEditalResposta,
	Dossie,
	Estatisticas,
	EstruturaResposta,
	PlanoResposta,
	PreviewTEC,
	RegistroInput
} from '$lib/types';

export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

function mensagemHTTP(status: number): string {
	switch (status) {
		case 413:
			return 'o arquivo é grande demais (máx. ~20 MB)';
		case 502:
		case 503:
			return 'servidor indisponível no momento — tente de novo';
		case 504:
			return 'a leitura do edital demorou demais e expirou — tente de novo ou cadastre manualmente';
		default:
			return `erro ${status}`;
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
	let body: unknown;
	try {
		body = text ? JSON.parse(text) : undefined;
	} catch {
		// Non-JSON body (e.g. an nginx 413/504 error page).
		if (!res.ok) {
			if (res.status === 401) auth.clear();
			throw new ApiError(res.status, mensagemHTTP(res.status));
		}
		throw new ApiError(res.status, 'resposta inesperada do servidor');
	}

	if (!res.ok) {
		if (res.status === 401) auth.clear();
		const erro = (body as { erro?: string })?.erro;
		throw new ApiError(res.status, erro ?? mensagemHTTP(res.status));
	}

	return body as T;
}

const planoBase = (slug: string) => `/api/concursos/${encodeURIComponent(slug)}/plano`;

/**
 * The edital as the wizard carries it between steps: the file on the first call,
 * then whatever cheap handle came back — the extracted text, or the URI of the
 * PDF the backend uploaded to the provider (scanned files have no text layer).
 */
export type FonteEdital =
	| { pdf: File; texto?: never; arquivoUri?: never; mime?: never }
	| { texto: string; pdf?: never; arquivoUri?: never; mime?: never }
	| { arquivoUri: string; mime: string; pdf?: never; texto?: never };

interface ExtrasEdital {
	cargo?: string;
	disciplinas?: string[];
}

/** bodyDe builds multipart for a file upload, JSON for text or a file URI. */
function bodyDe(fonte: FonteEdital, extras: ExtrasEdital = {}): RequestInit {
	if (fonte.pdf) {
		const form = new FormData();
		form.append('pdf', fonte.pdf);
		if (extras.cargo) form.append('cargo', extras.cargo);
		if (extras.disciplinas) form.append('disciplinas', JSON.stringify(extras.disciplinas));
		return { method: 'POST', body: form };
	}

	const { pdf: _pdf, ...resto } = fonte;
	return { method: 'POST', body: JSON.stringify({ ...resto, ...extras }) };
}

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

	// ---- edital import wizard ----
	// The edital travels on every step: as text when the PDF had a text layer,
	// otherwise as the file itself (scanned PDFs have no text to reuse).
	analisarEdital: (fonte: FonteEdital) =>
		request<CargosResposta>('/api/editais/analisar', bodyDe(fonte)),

	estruturaEdital: (fonte: FonteEdital, cargo: string) =>
		request<EstruturaResposta>('/api/editais/estrutura', bodyDe(fonte, { cargo })),

	conteudoEdital: (fonte: FonteEdital, disciplinas: string[]) =>
		request<ConteudoEditalResposta>('/api/editais/conteudo', bodyDe(fonte, { disciplinas })),

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

	registrarRevisao: (slug: string, id: string, questoes: number, acertos: number) =>
		request<PlanoResposta>(`${planoBase(slug)}/revisoes/${id}`, {
			method: 'PATCH',
			body: JSON.stringify({ questoes, acertos })
		}),

	previewTec: (slug: string, csv: string) =>
		request<PreviewTEC>(`${planoBase(slug)}/tec/preview`, {
			method: 'POST',
			body: JSON.stringify({ csv })
		}),

	importarTec: (slug: string, csv: string, data: string) =>
		request<PreviewTEC>(`${planoBase(slug)}/tec`, {
			method: 'POST',
			body: JSON.stringify({ csv, data })
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
