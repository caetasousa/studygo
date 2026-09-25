import { readFileSync, writeFileSync } from 'node:fs';
import { randomInt, randomUUID } from 'node:crypto';
import { request as novoRequest, type APIRequestContext, type Page } from '@playwright/test';
import { test, expect, Api, cadastrar, emailUnico } from './base';

// O catálogo de leis é global: dois testes importando "lei-e2e" ao mesmo tempo
// disputariam a mesma lei. Cada teste importa uma cópia com slug, número e
// reconhecimento próprios.

interface Dispositivo {
	ref: string;
	texto: string;
	notas: string[];
	anteriores: string[];
}

interface Pacote {
	lei: { slug: string; nome: string; curto: string; reconhecer: string[] };
	dispositivos: Dispositivo[];
	questoes: { id: string; gabarito: string; comentario: string; trecho: string; dispositivos: string[] }[];
}

const FIXTURES = new URL('../fixtures/', import.meta.url);

function pacote(arquivo = 'lei-exemplo.json'): Pacote {
	return JSON.parse(readFileSync(new URL(arquivo, FIXTURES), 'utf-8')) as Pacote;
}

/** Uma cópia só deste teste: slug e "nº" únicos, o resto igual à fixture. */
function copia(p: Pacote, id: string, numero: string): Pacote {
	const c = structuredClone(p);
	c.lei.slug = `lei-e2e-${id}`;
	c.lei.curto = `Lei E2E ${id}`;
	c.lei.reconhecer = [numero];
	return c;
}

function idUnico() {
	return randomUUID().slice(0, 8);
}

function numeroUnico() {
	return `${randomInt(10, 99)}.${randomInt(100, 999)}`;
}

// Quem importa é uma conta à parte da do teste — como na vida real, quem
// publica a lei não é quem a estuda. Qualquer conta importa (L4).
async function outraSessao(baseURL: string): Promise<{ request: APIRequestContext; token: string }> {
	const request = await novoRequest.newContext({
		baseURL,
		extraHTTPHeaders: { 'X-Forwarded-For': `10.200.${randomInt(1, 255)}.${randomInt(1, 255)}` }
	});
	const { token } = await cadastrar(request, emailUnico('importa'));
	return { request, token };
}

async function importar(baseURL: string, p: Pacote) {
	const { request, token } = await outraSessao(baseURL);
	const res = await request.post('/api/leis', { data: p, headers: { Authorization: `Bearer ${token}` } });
	expect(res.ok(), await res.text()).toBeTruthy();
	const corpo = await res.json();
	await request.dispose();
	return corpo;
}

async function abrirLei(page: Page, slug: string) {
	await page.goto(`/leis/${slug}`);
	await expect(page.getByRole('navigation', { name: 'Sumário' })).toBeVisible();
}

function dispositivo(page: Page, ref: string) {
	return page.locator(`[id="${ref}"]`);
}

async function abrirQuestoes(page: Page, rotulo: string) {
	await page.getByRole('button', { name: `Questões do ${rotulo}`, exact: true }).click();
	const painel = page.getByRole('dialog', { name: `Questões — ${rotulo}` });
	await expect(painel).toBeVisible();
	return painel;
}

async function responder(painel: ReturnType<Page['getByRole']>, enunciado: string, letra: string) {
	const questao = painel.getByRole('group', { name: enunciado });
	await questao.getByRole('radio', { name: new RegExp(`^${letra}\\)`) }).check();
	await questao.getByRole('button', { name: 'Responder' }).click();
	await expect(questao.getByText(/^(Certo|Errado)/)).toBeVisible();
	return questao;
}

const ENUNCIADO_1 = 'Segundo a Lei nº 99.999, compete ao Tribunal de Contas:';
const ENUNCIADO_2 = 'O controle externo, conforme a Lei nº 99.999, está a cargo:';
const ENUNCIADO_3 = 'Sobre a organização do Tribunal de Contas, é correto afirmar:';
const ENUNCIADO_4 = 'A Lei nº 99.999 entra em vigor:';

test.describe('legislação', () => {
	test('[L1] o pacote importado vira uma lei legível, com o texto igual ao do pacote', async ({ browser, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		const arquivo = test.info().outputPath(`${p.lei.slug}.json`);
		writeFileSync(arquivo, JSON.stringify(p));

		// A importação pela tela, como em produção.
		const { request, token } = await outraSessao(baseURL!);
		await new Api(request, token).concurso('Importação E2E');
		const contexto = await browser.newContext({ storageState: await request.storageState() });
		const page = await contexto.newPage();
		await page.goto('/legislacao');
		await expect(page.getByRole('heading', { name: 'Legislação', level: 1 })).toBeVisible();
		await page.getByLabel('Pacote da lei (.json)').setInputFiles(arquivo);
		await expect(page.getByRole('status')).toContainText(`${p.lei.curto} importada`);

		await page.getByRole('link', { name: p.lei.curto }).first().click();
		await expect(page.getByRole('heading', { name: p.lei.curto, level: 1 })).toBeVisible();
		for (const d of p.dispositivos) {
			await expect(dispositivo(page, d.ref).locator('.texto')).toHaveText(d.texto);
		}
		await contexto.close();
		await request.dispose();
	});

	test('[L2] importar o mesmo pacote de novo não duplica nada', async ({ api, page, conta, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		const primeira = await importar(baseURL!, p);
		expect(primeira.novaVersao).toBe(true);
		const segunda = await importar(baseURL!, p);
		expect(segunda.novaVersao).toBe(false);
		expect(segunda.questoes.novas).toBe(0);

		const catalogo = await page.request.get('/api/leis', { headers: { Authorization: `Bearer ${conta.token}` } });
		const { leis } = await catalogo.json();
		expect(leis.filter((l: { slug: string }) => l.slug === p.lei.slug)).toHaveLength(1);

		await api.concurso('Leitor E2E');
		await abrirLei(page, p.lei.slug);
		await expect(page.locator('.dispositivo')).toHaveCount(p.dispositivos.length);
		const painel = await abrirQuestoes(page, 'Art. 1º');
		await expect(painel.getByRole('group')).toHaveCount(2);
	});

	test('[L3] a versão nova preserva as respostas das questões que continuam', async ({ api, page, baseURL }) => {
		const id = idUnico();
		const numero = numeroUnico();
		await importar(baseURL!, copia(pacote(), id, numero));
		await api.concurso('Versão E2E');
		await abrirLei(page, `lei-e2e-${id}`);
		const painel = await abrirQuestoes(page, 'Art. 1º');
		await responder(painel, ENUNCIADO_1, 'B');

		const v2 = await importar(baseURL!, copia(pacote('lei-exemplo-v2.json'), id, numero));
		expect(v2.novaVersao).toBe(true);
		expect(v2.questoes.desativadas).toBe(1);

		await page.reload();
		await expect(dispositivo(page, 'art3').locator('.texto')).toHaveText(
			'Art. 3º Esta Lei entra em vigor trinta dias após a data de sua publicação.'
		);
		const depois = await abrirQuestoes(page, 'Art. 1º');
		await expect(depois.getByRole('group', { name: ENUNCIADO_1 }).getByText('Certo')).toBeVisible();
		await page.keyboard.press('Escape');
		// A questão que saiu do pacote some da tela; a resposta dela fica no banco.
		await expect(page.getByRole('button', { name: 'Questões do Art. 2º', exact: true })).toHaveCount(0);
	});

	test('[L4] qualquer conta logada importa, pela tela ou pela API', async ({ page, api, conta }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		const res = await page.request.post('/api/leis', {
			data: p,
			headers: { Authorization: `Bearer ${conta.token}` }
		});
		expect(res.status(), await res.text()).toBe(201);

		await api.concurso('Conta comum E2E');
		await page.goto('/legislacao');
		await expect(page.getByRole('heading', { name: 'Legislação', level: 1 })).toBeVisible();
		await expect(page.getByLabel('Pacote da lei (.json)')).toBeVisible();
		await expect(page.getByRole('link', { name: p.lei.curto })).toBeVisible();
	});

	test('[L5] clicar no artigo traz as questões que o citam, e só elas', async ({ api, page, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		await api.concurso('Artigo E2E');
		await abrirLei(page, p.lei.slug);

		// e2e-01 cita o art. 1º, II; e2e-02, o caput do art. 1º.
		const painel = await abrirQuestoes(page, 'Art. 1º');
		await expect(painel.getByRole('group', { name: ENUNCIADO_1 })).toBeVisible();
		await expect(painel.getByRole('group', { name: ENUNCIADO_2 })).toBeVisible();
		await expect(painel.getByRole('group', { name: ENUNCIADO_3 })).toHaveCount(0);
		await expect(painel.getByRole('group', { name: ENUNCIADO_4 })).toHaveCount(0);
		await page.keyboard.press('Escape');

		const doSegundo = await abrirQuestoes(page, 'Art. 2º');
		await expect(doSegundo.getByRole('group')).toHaveCount(1);
		await expect(doSegundo.getByRole('group', { name: ENUNCIADO_3 })).toBeVisible();
	});

	test('[L6] a resposta fica gravada e mostra gabarito, comentário e trecho', async ({ api, page, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		await api.concurso('Resposta E2E');
		await abrirLei(page, p.lei.slug);

		const painel = await abrirQuestoes(page, 'Art. 1º');
		const questao = await responder(painel, ENUNCIADO_1, 'A');
		const q1 = p.questoes[0];
		await expect(questao.getByText('Errado — gabarito B')).toBeVisible();
		await expect(questao).toContainText(q1.comentario);
		await expect(questao.locator('mark')).toHaveText(q1.trecho);

		await page.reload();
		const depois = await abrirQuestoes(page, 'Art. 1º');
		await expect(depois.getByRole('group', { name: ENUNCIADO_1 }).getByText('Errado — gabarito B')).toBeVisible();
	});

	test('[L7] o selo do artigo e o progresso da unidade seguem as respostas', async ({ api, page, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		await api.concurso('Progresso E2E');
		await abrirLei(page, p.lei.slug);

		const unidade = page.getByRole('listitem').filter({ hasText: 'Arts. 1º e 2º' });
		await expect(unidade).toContainText('0 de 3 respondidas');

		const painel = await abrirQuestoes(page, 'Art. 1º');
		await responder(painel, ENUNCIADO_1, 'B');
		await responder(painel, ENUNCIADO_2, 'A');

		// Só o que errei: sobra a e2e-02.
		await painel.getByLabel('Só o que errei').check();
		await expect(painel.getByRole('group')).toHaveCount(1);
		await expect(painel.getByRole('group', { name: ENUNCIADO_2 })).toBeVisible();
		await page.keyboard.press('Escape');

		await expect(page.getByRole('button', { name: 'Questões do Art. 1º', exact: true })).toContainText('1 de 2 certas');
		await expect(unidade).toContainText('2 de 3 respondidas');
		await page.reload();
		await expect(unidade).toContainText('2 de 3 respondidas');
	});

	test('[L8] o link direto abre a lei no dispositivo', async ({ api, page, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		await api.concurso('Link direto E2E');
		await page.setViewportSize({ width: 1280, height: 400 });

		await page.goto(`/leis/${p.lei.slug}#art3`);
		await expect(dispositivo(page, 'art3')).toBeInViewport();
		await expect(dispositivo(page, 'art3')).toHaveAttribute('aria-current', 'location');
		await expect(dispositivo(page, 'preambulo1')).not.toBeInViewport();
	});

	test('[L9] o tópico que cita a lei sugere o vínculo, e o vínculo fica gravado', async ({ api, page, baseURL }) => {
		const numero = numeroUnico();
		const p = copia(pacote(), idUnico(), numero);
		await importar(baseURL!, p);
		await api.concurso('Vínculo E2E', [
			{ nome: 'Legislação Institucional', bloco: 'esp', questoes: 8, temas: [`Lei Orgânica do Tribunal (Lei nº ${numero}/2026)`] },
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase'] }
		]);

		await page.goto('/legislacao');
		const materia = page.getByRole('region', { name: 'Legislação Institucional' });
		await expect(materia.getByText('Sugerida pelo tópico')).toBeVisible();
		const outra = page.getByRole('region', { name: 'Língua Portuguesa' });
		await expect(outra.getByText('Sugerida pelo tópico')).toHaveCount(0);
		await expect(outra.getByRole('link', { name: p.lei.curto })).toHaveCount(0);

		await materia.getByRole('button', { name: `Vincular ${p.lei.curto}` }).click();
		await expect(materia.getByRole('link', { name: p.lei.curto })).toBeVisible();
		await expect(materia.getByText('Sugerida pelo tópico')).toHaveCount(0);

		await page.reload();
		await expect(page.getByRole('region', { name: 'Legislação Institucional' }).getByRole('link', { name: p.lei.curto })).toBeVisible();
	});

	test('[L10] redação anterior e notas não se misturam ao texto vigente', async ({ api, page, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		await api.concurso('Redação E2E');
		await abrirLei(page, p.lei.slug);

		const par = p.dispositivos.find((d) => d.ref === 'art1.par1')!;
		const bloco = dispositivo(page, 'art1.par1');
		await expect(bloco.locator('.texto')).toHaveText(par.texto);
		await expect(bloco.getByText(par.notas[0])).toBeVisible();
		await expect(bloco.getByText(par.anteriores[0])).toBeHidden();

		await bloco.getByText('Redação anterior').click();
		await expect(bloco.getByText(par.anteriores[0])).toBeVisible();
		await expect(bloco.locator('.texto')).toHaveText(par.texto);
	});
});
