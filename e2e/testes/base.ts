import { test as base, expect, type APIRequestContext, type Page } from '@playwright/test';
import { randomInt, randomUUID } from 'node:crypto';

// O que todo teste recebe: uma conta nova, só dele, já com a sessão no cookie
// do navegador — e um IP próprio.
//
// Por que o IP: o backend limita /api/auth por cliente (20 por minuto, rajada
// de 10), e cada carga de página renova a sessão por ali. Com todos os testes
// saindo do mesmo IP, a suíte esbarraria no limite e falharia com 429 sem
// nada estar quebrado. O limitador conta pelo primeiro X-Forwarded-For
// (middleware.ClienteIP), então cada teste se apresenta como um cliente
// distinto — que é o que ele simula. Se ClienteIP deixar de confiar nesse
// cabeçalho, os testes passam a ver 429 e é aqui que se ajusta.

export interface Conta {
	email: string;
	senha: string;
	nome: string;
	/** Access token da sessão, para o preparo pela API. A tela usa o dela. */
	token: string;
}

export interface Disciplina {
	nome: string;
	bloco: 'ger' | 'esp';
	questoes: number;
	codigo?: string;
	temas?: string[];
	fontes?: { titulo: string; url: string; tipo: string }[];
}

export const DISCIPLINAS_PADRAO: Disciplina[] = [
	{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20 },
	{ nome: 'Direito Constitucional', bloco: 'esp', questoes: 15 },
	{ nome: 'Informática', bloco: 'ger', questoes: 10 }
];

export const SENHA = 'senha-do-e2e-bem-comprida';

/** A data de hoje em Brasília, somada de `dias`, como YYYY-MM-DD. */
export function dataEmDias(dias: number): string {
	const agora = new Date(Date.now() + dias * 86_400_000);
	return new Intl.DateTimeFormat('en-CA', { timeZone: 'America/Sao_Paulo' }).format(agora);
}

function ipDoTeste(): string {
	return `10.${randomInt(1, 255)}.${randomInt(1, 255)}.${randomInt(1, 255)}`;
}

export function emailUnico(prefixo: string): string {
	return `${prefixo}-${randomUUID().slice(0, 8)}@e2e.local`;
}

/** Cadastra pela API; a resposta grava o cookie de refresh no contexto. */
export async function cadastrar(request: APIRequestContext, email: string, nome = 'Estudante E2E'): Promise<Conta> {
	const res = await request.post('/api/auth/register', { data: { email, nome, senha: SENHA } });
	expect(res.status(), await res.text()).toBe(201);
	const corpo = await res.json();
	return { email, senha: SENHA, nome, token: corpo.accessToken };
}

export class Api {
	constructor(
		private readonly request: APIRequestContext,
		private readonly token: string
	) {}

	private async chamar(metodo: 'GET' | 'POST' | 'PUT', caminho: string, data?: unknown) {
		const res = await this.request.fetch(caminho, {
			method: metodo,
			headers: { Authorization: `Bearer ${this.token}` },
			data
		});
		expect(res.ok(), `${metodo} ${caminho}: ${res.status()} ${await res.text()}`).toBeTruthy();
		return res.json();
	}

	/**
	 * Cria o concurso e deixa o plano previsível em qualquer dia da semana:
	 * estudo nos sete dias, sem simulado, sem discursiva e sem dia fixo de
	 * revisão. Sem isso, rodar a suíte num sábado dava um "Hoje" vazio.
	 */
	async concurso(nome: string, disciplinas: Disciplina[] = DISCIPLINAS_PADRAO, extra: Record<string, unknown> = {}): Promise<string> {
		const criado = await this.chamar('POST', '/api/concursos', {
			nome,
			banca: 'FCC',
			cargo: 'Analista',
			emoji: '📚',
			prova: dataEmDias(60),
			retaFinalDias: 14,
			disciplinas: disciplinas.map((d) => ({
				nome: d.nome,
				codigo: d.codigo ?? '',
				bloco: d.bloco,
				questoes: d.questoes,
				peso: 0,
				cadernoUrl: '',
				notebookUrl: '',
				temas: d.temas ?? [],
				fontes: d.fontes ?? []
			})),
			marcos: [],
			conteudo: [],
			...extra
		});
		await this.chamar('PUT', `/api/concursos/${criado.slug}/plano`, {
			diasEstudo: [0, 1, 2, 3, 4, 5, 6],
			simulados: 'nunca',
			discursiva: false,
			revisaoSemanal: false,
			blocosPorDia: 2
		});
		return criado.slug;
	}

	plano(slug: string) {
		return this.chamar('GET', `/api/concursos/${slug}/plano`);
	}

	/** O concurso como foi gravado: dados.disciplinas traz questões e temas. */
	async unicoConcurso() {
		const { concursos } = await this.chamar('GET', '/api/concursos');
		expect(concursos).toHaveLength(1);
		return this.chamar('GET', `/api/concursos/${concursos[0].slug}`);
	}
}

export const test = base.extend<{ conta: Conta; api: Api }>({
	extraHTTPHeaders: async ({}, use) => {
		await use({ 'X-Forwarded-For': ipDoTeste() });
	},
	conta: async ({ page }, use, info) => {
		const prefixo = info.titlePath.at(-1)?.match(/\[([A-Z]\d+)\]/)?.[1]?.toLowerCase() ?? 'e2e';
		await use(await cadastrar(page.context().request, emailUnico(prefixo)));
	},
	api: async ({ page, conta }, use) => {
		await use(new Api(page.context().request, conta.token));
	}
});

export { expect };

/** Abre o Hoje e espera o plano carregar. */
export async function abrirHoje(page: Page) {
	await page.goto('/');
	await expect(page.getByRole('heading', { name: 'Hoje', level: 1 })).toBeVisible();
	await expect(page.getByRole('button', { name: /^Registrar estudo de / }).first()).toBeVisible();
}

/** As matérias do dia, pela ordem da tela: o nome de cada "Registrar estudo de …". */
export async function materiasDoDia(page: Page, escopo = page.locator('main')): Promise<string[]> {
	const nomes = await escopo.getByRole('button', { name: /^Registrar estudo de / }).evaluateAll((els) =>
		els.map((e) => (e.getAttribute('aria-label') ?? e.textContent ?? '').replace(/^Registrar estudo de /, '').trim())
	);
	return nomes;
}

/** Preenche e salva o diálogo de registro de UMA matéria. */
export async function registrar(
	page: Page,
	materia: string,
	valores: { minutos: number; questoes?: number; acertos?: number; concluir?: boolean; observacao?: string },
	escopo = page.locator('main')
) {
	await escopo.getByRole('button', { name: `Registrar estudo de ${materia}` }).first().click();
	const dialogo = page.getByRole('dialog', { name: `Registrar estudo — ${materia}` });
	await expect(dialogo).toBeVisible();
	await dialogo.getByLabel('Minutos estudados').fill(String(valores.minutos));
	if (valores.questoes !== undefined) await dialogo.getByLabel('Questões').fill(String(valores.questoes));
	if (valores.acertos !== undefined) await dialogo.getByLabel('Acertos').fill(String(valores.acertos));
	if (valores.observacao) await dialogo.getByLabel('Observação').fill(valores.observacao);
	if (valores.concluir) await dialogo.getByLabel('Concluí esta matéria').check();
	await dialogo.getByRole('button', { name: 'Salvar' }).click();
	await expect(dialogo).toBeHidden();
}
