import { auth } from '$lib/stores/auth.svelte';
import type { PacoteDoZip } from '$lib/mapas/pacote';
import type {
	ConexaoDoClaude,
	PacoteImportado,
	Caderno,
	AnaliseResposta,
	ConcursoDetalhe,
	ConcursoInput,
	ConcursoLista,
	ConcursoResumo,
	ConfigInput,
	ConteudoEditalResposta,
	Dossie,
	Estatisticas,
	EstruturaResposta,
	ImportacaoCSV,
	PlanoResposta,
	PreviewTEC,
	RegistroInput,
	RegistroDiaInput,
	Usuario,
	CorrecaoDeQuestao,
	CapturaDeLei,
	ImportacaoDeQuestoes,
	LeiResumo,
	LeisDaMateria,
	LeituraDeLei,
	CorrecaoDoMapa,
	MapaImportado,
	MapaLido,
	MapaResumo,
	MapasDaMateria,
	PedidoDeMapa,
	QuestoesImportadas,
	PedidoDeCaptura,
	PedidoDePublicacao,
	PesquisaDoTema,
	ResumoDaExclusao,
	PublicacaoDeLei
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

/**
 * Uma requisição autenticada, com a renovação do token embutida.
 *
 * O access token é curto de propósito, então TODA chamada precisa passar por
 * aqui: quem monta o próprio `fetch` com o token na mão funciona por alguns
 * minutos e depois recebe 401 para sempre. Foi o que aconteceu com o download
 * do CSV, que baixava `{"erro":"não autenticado"}` como se fosse a planilha.
 */
async function fetchAutenticado(
	path: string,
	init: RequestInit = {},
	retry = true
): Promise<Response> {
	const headers = new Headers(init.headers);
	if (auth.accessToken) headers.set('Authorization', `Bearer ${auth.accessToken}`);
	if (init.body && !headers.has('content-type') && typeof init.body === 'string') {
		headers.set('content-type', 'application/json');
	}

	const res = await fetch(path, { ...init, headers });

	if (res.status === 401 && retry && (await auth.refresh())) {
		return fetchAutenticado(path, init, false);
	}

	return res;
}

/** Uma resposta que não é JSON — hoje, o CSV do plano. */
async function requestTexto(path: string): Promise<string> {
	const res = await fetchAutenticado(path);

	if (!res.ok) {
		if (res.status === 401) auth.clear();

		// O corpo de erro é JSON mesmo quando a rota devolve texto.
		const texto = await res.text();
		let erro: string | undefined;
		try {
			erro = (JSON.parse(texto) as { erro?: string })?.erro;
		} catch {
			erro = undefined;
		}

		throw new ApiError(res.status, erro ?? mensagemHTTP(res.status));
	}

	return res.text();
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
	const res = await fetchAutenticado(path, init, retry);

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
 * How the edital enters the wizard's first step: a file or pasted text. After
 * step 1 it is carried by an opaque `documentoId` instead.
 */
export type FonteEdital =
	| { pdf: File; texto?: never }
	| { texto: string; pdf?: never };

interface ExtrasEdital {
	documentoId?: string;
	cargo?: string;
	disciplinas?: string[];
}

/** bodyDe builds multipart for a file upload, JSON otherwise. */
function bodyDe(fonte: FonteEdital | null, extras: ExtrasEdital = {}): RequestInit {
	if (fonte && 'pdf' in fonte && fonte.pdf) {
		const form = new FormData();
		form.append('file', fonte.pdf);
		if (extras.documentoId) form.append('documentoId', extras.documentoId);
		if (extras.cargo) form.append('cargo', extras.cargo);
		if (extras.disciplinas) form.append('disciplinas', JSON.stringify(extras.disciplinas));
		return { method: 'POST', body: form };
	}

	const texto = fonte && 'texto' in fonte ? fonte.texto : undefined;
	return { method: 'POST', body: JSON.stringify({ texto, ...extras }) };
}

export const api = {
	// ---- mapas mentais ----
	listarMapas: () => request<{ mapas: MapaResumo[] }>('/api/mapas'),

	/**
	 * Importa o outline do mapa. Com o concurso aberto, o servidor vincula o mapa
	 * às matérias dele que o texto indica.
	 */
	importarMapa: (texto: string, concurso?: string | null) =>
		request<MapaImportado>('/api/mapas', {
			method: 'POST',
			body: JSON.stringify({ texto, concurso: concurso ?? '' })
		}),

	lerMapa: (slug: string) => request<MapaLido>(`/api/mapas/${encodeURIComponent(slug)}`),

	excluirMapa: (slug: string) => request<void>(`/api/mapas/${encodeURIComponent(slug)}`, { method: 'DELETE' }),

	/**
	 * Importa o <slug>.questoes.json do mapa. O texto do arquivo vai como está:
	 * quem o confere é o servidor, que devolve todos os problemas de uma vez.
	 */
	importarQuestoesDoMapa: (slug: string, conteudo: string) =>
		request<QuestoesImportadas>(`/api/mapas/${encodeURIComponent(slug)}/questoes`, {
			method: 'POST',
			body: conteudo
		}),

	/**
	 * Envia as imagens que o mapa cita. O nome do arquivo é o do outline, e um
	 * nome que já existe troca a imagem. O envio entra inteiro ou não entra.
	 */
	enviarImagensDoMapa: (slug: string, arquivos: File[]) => {
		const form = new FormData();
		for (const a of arquivos) form.append('imagens', a);
		return request<{ gravadas: number }>(`/api/mapas/${encodeURIComponent(slug)}/imagens`, {
			method: 'POST',
			body: form
		});
	},

	/**
	 * Os bytes de uma imagem do mapa. O `<img>` não manda o token, então ela
	 * vem por aqui e vira um endereço `data:` na tela (ver lib/mapas/imagens.ts).
	 */
	imagemDoMapa: async (slug: string, nome: string): Promise<Blob> => {
		const res = await fetchAutenticado(`/api/mapas/${encodeURIComponent(slug)}/imagens/${encodeURIComponent(nome)}`);
		if (!res.ok) throw new ApiError(res.status, mensagemHTTP(res.status));
		return res.blob();
	},

	/**
	 * O mapa (ou todos, sem slug) num .zip, no arranjo que a importação aceita:
	 * o texto, as questões e as imagens. É a saída do banco para fora do app.
	 */
	exportarMapas: async (slug?: string): Promise<Blob> => {
		const caminho = slug ? `/api/mapas/${encodeURIComponent(slug)}/exportacao` : '/api/exportacao-de-mapas';
		const res = await fetchAutenticado(caminho);
		if (!res.ok) throw new ApiError(res.status, mensagemHTTP(res.status));
		return res.blob();
	},

	/** Responde uma questão do mapa: a letra, ou CERTO/ERRADO. */
	responderQuestaoDoMapa: (id: string, resposta: string) =>
		request<CorrecaoDoMapa>(`/api/mapas/questoes/${encodeURIComponent(id)}/respostas`, {
			method: 'POST',
			body: JSON.stringify({ resposta })
		}),

	/**
	 * Tira um tópico do mapa (e o que há dentro dele). O caminho são os índices
	 * desde o ramo; o texto é o que a tela mostrava, e o servidor recusa (409)
	 * se o caminho já aponta outro tópico. `keepalive` deixa o pedido terminar
	 * mesmo com a página sendo fechada.
	 */
	excluirItemDoMapa: (slug: string, caminho: number[], texto: string, keepalive = false) =>
		request<MapaLido>(`/api/mapas/${encodeURIComponent(slug)}/itens/excluir`, {
			method: 'POST',
			body: JSON.stringify({ caminho, texto }),
			keepalive
		}),

	/** Todas as matérias do concurso, cada uma com os mapas vinculados a ela. */
	mapasDoConcurso: (slug: string) =>
		request<{ disciplinas: MapasDaMateria[] }>(`/api/concursos/${encodeURIComponent(slug)}/mapas`),

	/** Liga o mapa à matéria nestes tópicos (nenhum: a matéria inteira), ou o desliga. */
	vincularMapa: (slug: string, disciplinaId: string, mapa: string, ligar: boolean, temas: string[] = []) =>
		request<void>(
			`/api/concursos/${encodeURIComponent(slug)}/disciplinas/${encodeURIComponent(disciplinaId)}/mapas/${encodeURIComponent(mapa)}`,
			ligar ? { method: 'PUT', body: JSON.stringify({ temas }) } : { method: 'DELETE' }
		),

	// ---- pedidos de mapa (o PDF da aula que o processador transforma em mapa) ----
	pedidosDeMapa: () => request<{ pedidos: PedidoDeMapa[] }>('/api/pedidos-de-mapa'),

	/** Põe o PDF na fila; com a matéria, o mapa nasce vinculado a ela. */
	/**
	 * Um mapa do .zip exportado, de volta inteiro: texto, questões, imagens,
	 * vínculos e respostas. O concurso aberto recebe o vínculo cujo concurso de
	 * origem a conta não tem.
	 */
	importarPacote: (p: PacoteDoZip, concurso?: string | null) => {
		const form = new FormData();
		form.append('mapa', p.mapa);
		if (p.questoes) form.append('questoes', p.questoes);
		if (p.conta) form.append('conta', p.conta);
		if (concurso) form.append('concurso', concurso);
		for (const img of p.imagens) form.append('imagens', new Blob([img.dados as BlobPart]), img.nome);
		return request<PacoteImportado>('/api/mapas/pacote', { method: 'POST', body: form });
	},

	pedirMapa: (pdf: File, concurso?: string | null, disciplina?: string | null) => {
		const form = new FormData();
		form.append('pdf', pdf);
		if (concurso && disciplina) {
			form.append('concurso', concurso);
			form.append('disciplina', disciplina);
		}
		return request<PedidoDeMapa>('/api/pedidos-de-mapa', { method: 'POST', body: form });
	},

	/** Devolve à fila o pedido que falhou ou que travou processando. */
	reenfileirarPedido: (id: string) =>
		request<void>(`/api/pedidos-de-mapa/${encodeURIComponent(id)}/fila`, { method: 'POST' }),

	/** O que a tela sabe do token do Claude guardado: se há, e só o fim dele. */
	situacaoDoTokenDoClaude: () => request<{ configurado: boolean; fim: string }>('/api/conta/token-do-claude'),

	/** Guarda (cifrado) o token do Claude que o processador de mapas usa. */
	guardarTokenDoClaude: (token: string) =>
		request<void>('/api/conta/token-do-claude', { method: 'PUT', body: JSON.stringify({ token }) }),

	removerTokenDoClaude: () => request<void>('/api/conta/token-do-claude', { method: 'DELETE' }),

	/** A conta do Claude conectada ao processador de mapas, se houver. */
	conexaoDoClaude: () => request<ConexaoDoClaude>('/api/conta/claude'),

	/** Começa a conexão: devolve o link da página de autorização do Claude. */
	conectarClaude: () => request<{ url: string }>('/api/conta/claude/conexao', { method: 'POST' }),

	/** Entrega o código que a página de autorização mostrou. */
	concluirConexaoDoClaude: (codigo: string) =>
		request<ConexaoDoClaude>('/api/conta/claude/conexao/codigo', {
			method: 'POST',
			body: JSON.stringify({ codigo })
		}),

	desconectarClaude: () => request<void>('/api/conta/claude', { method: 'DELETE' }),

	excluirPedido: (id: string) =>
		request<void>(`/api/pedidos-de-mapa/${encodeURIComponent(id)}`, { method: 'DELETE' }),

	// ---- legislação ----
	catalogoDeLeis: () => request<{ leis: LeiResumo[] }>('/api/leis'),

	/** Começa a captura da lei do link; a prévia vem de `capturaDeLei`. */
	capturarLei: (pedido: PedidoDeCaptura) =>
		request<{ id: string }>('/api/leis/capturas', { method: 'POST', body: JSON.stringify(pedido) }),

	/** Acha a lei que o tópico do edital cita e lê o que ele pede dela. */
	pesquisarTema: (tema: string, link?: string) =>
		request<PesquisaDoTema>('/api/leis/pesquisa', { method: 'POST', body: JSON.stringify({ tema, link: link ?? '' }) }),

	resumirExclusao: (slug: string) => request<ResumoDaExclusao>(`/api/leis/exclusao/${encodeURIComponent(slug)}`),

	excluirLei: (slug: string) => request<void>(`/api/leis/${encodeURIComponent(slug)}`, { method: 'DELETE' }),

	/** Com o concurso, a prévia traz o que o edital dele pede da lei. */
	capturaDeLei: (id: string, concurso?: string | null) =>
		request<CapturaDeLei>(
			`/api/leis/capturas/${encodeURIComponent(id)}${concurso ? `?concurso=${encodeURIComponent(concurso)}` : ''}`
		),

	publicarLei: (id: string, pedido: PedidoDePublicacao) =>
		request<PublicacaoDeLei>(`/api/leis/capturas/${encodeURIComponent(id)}/publicacao`, {
			method: 'POST',
			body: JSON.stringify(pedido)
		}),

	/** O questoes.json vai como está; quem valida é o servidor. */
	importarQuestoes: (slug: string, arquivo: unknown) =>
		request<ImportacaoDeQuestoes>(`/api/leis/${encodeURIComponent(slug)}/questoes`, {
			method: 'POST',
			body: JSON.stringify(arquivo)
		}),

	/** Com o concurso, a leitura traz o recorte que as matérias dele cobram. */
	lerLei: (slug: string, concurso?: string | null) =>
		request<LeituraDeLei>(
			`/api/leis/${encodeURIComponent(slug)}${concurso ? `?concurso=${encodeURIComponent(concurso)}` : ''}`
		),

	responderQuestao: (id: string, alternativa: string) =>
		request<CorrecaoDeQuestao>(`/api/leis/questoes/${encodeURIComponent(id)}/respostas`, {
			method: 'POST',
			body: JSON.stringify({ alternativa })
		}),

	leisDoConcurso: (slug: string) =>
		request<{ disciplinas: LeisDaMateria[] }>(`/api/concursos/${encodeURIComponent(slug)}/leis`),

	/**
	 * Liga (ou desliga) a lei à matéria. Sem `recorte`, o servidor o tira dos
	 * tópicos da matéria; com ele, grava o ajuste de quem estuda ([] = a lei inteira).
	 */
	vincularLei: (
		slug: string,
		disciplinaId: string,
		lei: string,
		ligar: boolean,
		recorte?: string[],
		artigos?: string,
		somar = false
	) =>
		request<void>(
			`/api/concursos/${encodeURIComponent(slug)}/disciplinas/${encodeURIComponent(disciplinaId)}/leis/${encodeURIComponent(lei)}`,
			ligar
				? {
						method: 'PUT',
						...(recorte || artigos ? { body: JSON.stringify({ recorte, artigos: artigos ?? '', somar }) } : {})
					}
				: { method: 'DELETE' }
		),

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
	// Step 1 takes the file or text and returns a documentoId; steps 2 and 3
	// carry only the id (step 3 also accepts a fresh upload for the edit screen).
	analisarEdital: (fonte: FonteEdital) =>
		request<AnaliseResposta>('/api/editais/analisar', bodyDe(fonte)),

	estruturaEdital: (documentoId: string, cargo: string) =>
		request<EstruturaResposta>('/api/editais/estrutura', {
			method: 'POST',
			body: JSON.stringify({ documentoId, cargo })
		}),

	conteudoEdital: (
		src: { documentoId: string } | { fonte: FonteEdital },
		cargo: string,
		disciplinas: string[]
	) =>
		request<ConteudoEditalResposta>(
			'/api/editais/conteudo',
			'documentoId' in src
				? { method: 'POST', body: JSON.stringify({ documentoId: src.documentoId, cargo, disciplinas }) }
				: bodyDe(src.fonte, { cargo, disciplinas })
		),

	// ---- plano ----
	getPlano: (slug: string) => request<PlanoResposta>(planoBase(slug)),

	salvarConfig: (slug: string, input: ConfigInput) =>
		request<PlanoResposta>(planoBase(slug), { method: 'PUT', body: JSON.stringify(input) }),

	/** Lança o resultado de UMA atividade. A conclusão do dia é derivada no
	 *  servidor a partir das atividades daquele dia — o cliente não a envia. */
	registrarAtividade: (slug: string, input: RegistroInput) =>
		request<PlanoResposta>(
			`${planoBase(slug)}/atividades/${encodeURIComponent(input.atividadeId)}/registro`,
			{ method: 'PUT', body: JSON.stringify(input) }
		),

	/** Marca UM tópico de uma atividade como estudado. Quando a atividade junta
	 *  vários, o servidor separa o tópico numa atividade própria antes de
	 *  concluí-la — e a traz para hoje se ela estava adiante. */
	estudarTema: (slug: string, atividadeId: string, tema: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/atividades/${encodeURIComponent(atividadeId)}/estudado`, {
			method: 'POST',
			body: JSON.stringify({ tema })
		}),

	/** Grava o que pertence ao dia: a anotação livre e a cauda de revisão. */
	registrarDia: (slug: string, data: string, input: RegistroDiaInput) =>
		request<PlanoResposta>(`${planoBase(slug)}/dias/${data}`, {
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

	/** Move uma atividade para (data, posicao). Envia só a mudança. */
	moverAtividade: (slug: string, id: string, data: string, posicao: number, trocar = false) =>
		request<PlanoResposta>(`${planoBase(slug)}/atividades/mover`, {
			method: 'POST',
			body: JSON.stringify({ id, data, posicao, trocar })
		}),

	/** Empurra o conteúdo de um dia perdido, deslocando o resto do plano. */
	adiarDia: (slug: string, data: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/dias/${data}/adiar`, { method: 'POST' }),

	/** Fecha os buracos deixados por matérias terminadas antes da hora. */
	reorganizarDesde: (slug: string, data: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/dias/${data}/reorganizar`, { method: 'POST' }),
	compactarPlano: (slug: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/compactar`, { method: 'POST' }),

	/** Descarta as movimentações manuais dos dias à frente. */
	restaurarOrdem: (slug: string) =>
		request<PlanoResposta>(`${planoBase(slug)}/restaurar-ordem`, { method: 'POST' }),

	/**
	 * Grava os links de UMA matéria — caderno de erros e NotebookLM.
	 *
	 * Os dois vão sempre juntos, e o corpo descreve o estado FINAL: mandar um
	 * campo vazio apaga aquele link. É o que permite ao formulário oferecer
	 * "remover" sem uma rota própria para isso.
	 */
	atualizarLinksDisciplina: (
		slug: string,
		codigo: string,
		links: { cadernoUrl: string; notebookUrl: string }
	) =>
		request<PlanoResposta>(
			`${planoBase(slug)}/disciplinas/${encodeURIComponent(codigo)}/links`,
			{ method: 'PATCH', body: JSON.stringify(links) }
		),

	estatisticas: (slug: string) => request<Estatisticas>(`${planoBase(slug)}/estatisticas`),

	caderno: (slug: string) => request<Caderno>(`${planoBase(slug)}/caderno`),

	dossie: (slug: string, disciplina: string) =>
		request<Dossie>(`${planoBase(slug)}/dossie?disciplina=${encodeURIComponent(disciplina)}`),

	exportarCsv: (slug: string) => requestTexto(`${planoBase(slug)}/export.csv`),

	/**
	 * Lê uma planilha do plano. `confirmar: false` é a prévia — o servidor
	 * responde o que entraria sem gravar nada.
	 */
	importarCsv: (slug: string, csv: string, confirmar: boolean) =>
		request<ImportacaoCSV>(`${planoBase(slug)}/importar.csv`, {
			method: 'POST',
			body: JSON.stringify({ csv, confirmar })
		}),

	/** O tema é preferência do USUÁRIO, não do plano. */
	definirTema: (temaUi: string) =>
		request<Usuario>('/api/me/tema', { method: 'PUT', body: JSON.stringify({ temaUi }) })
};
