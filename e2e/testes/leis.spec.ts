import { readFileSync, writeFileSync } from 'node:fs';
import { randomInt, randomUUID } from 'node:crypto';
import { request as novoRequest, type APIRequestContext, type Page } from '@playwright/test';
import { test, expect, Api, cadastrar, emailUnico } from './base';

// O catálogo de leis é global: dois testes publicando a mesma lei ao mesmo
// tempo disputariam o mesmo slug. Cada teste publica uma cópia com nome curto
// (e portanto slug), número e reconhecimento próprios. O texto vem do dublê do
// processador, que devolve a lei da fixture para links do Planalto em /e2e/.

interface Dispositivo {
	ref: string;
	texto: string;
	notas: string[];
	anteriores: string[];
}

interface Pacote {
	lei: { slug: string; nome: string; curto: string; reconhecer: string[] };
	dispositivos: Dispositivo[];
	unidades: { ref: string; titulo: string; dispositivos: string[]; hash: string }[];
	questoes: { id: string; gabarito: string; comentario: string; trecho: string; dispositivos: string[] }[];
}

const FIXTURES = new URL('../fixtures/', import.meta.url);

function pacote(arquivo = 'lei-exemplo.json'): Pacote {
	return JSON.parse(readFileSync(new URL(arquivo, FIXTURES), 'utf-8')) as Pacote;
}

/** Uma cópia só deste teste: nome curto e "nº" únicos, o resto igual à fixture. */
function copia(p: Pacote, id: string, numero: string): Pacote {
	const c = structuredClone(p);
	c.lei.slug = `lei-e2e-${id}`;
	c.lei.curto = `Lei E2E ${id}`;
	c.lei.reconhecer = [numero];
	return c;
}

/** O link que o dublê reconhece; `marca` produz aviso ou bloqueio na captura. */
function linkDaLei(arquivo = 'lei-exemplo', marca = '') {
	return `https://www.planalto.gov.br/e2e/${arquivo}${marca ? `-${marca}` : ''}.htm`;
}

function idUnico() {
	return randomUUID().slice(0, 8);
}

function numeroUnico() {
	return `${randomInt(10, 99)}.${randomInt(100, 999)}`;
}

// Quem publica é uma conta à parte da do teste — como na vida real, quem
// publica a lei não é quem a estuda. Qualquer conta publica (L4).
async function outraSessao(baseURL: string): Promise<{ request: APIRequestContext; token: string }> {
	const request = await novoRequest.newContext({
		baseURL,
		extraHTTPHeaders: { 'X-Forwarded-For': `10.200.${randomInt(1, 255)}.${randomInt(1, 255)}` }
	});
	const { token } = await cadastrar(request, emailUnico('publica'));
	return { request, token };
}

/** Captura o link e espera a prévia, como a tela faz. */
async function capturar(request: APIRequestContext, token: string, link: string) {
	const headers = { Authorization: `Bearer ${token}` };
	const inicio = await request.post('/api/leis/capturas', { data: { link }, headers });
	expect(inicio.status(), await inicio.text()).toBe(202);
	const { id } = await inicio.json();
	for (let i = 0; i < 40; i++) {
		const c = await (await request.get(`/api/leis/capturas/${id}`, { headers })).json();
		if (c.estado !== 'rodando') return c;
		await new Promise((r) => setTimeout(r, 100));
	}
	throw new Error('a captura não terminou');
}

/**
 * Publica a lei da fixture e importa as questões dela, pela API. Com
 * `atualizar`, é o texto novo da lei que já existe.
 */
async function importar(baseURL: string, p: Pacote, { arquivo = 'lei-exemplo', atualizar = false } = {}) {
	const { request, token } = await outraSessao(baseURL);
	const headers = { Authorization: `Bearer ${token}` };
	const c = await capturar(request, token, linkDaLei(arquivo));
	const pub = await request.post(`/api/leis/capturas/${c.id}/publicacao`, {
		headers,
		data: {
			slug: atualizar ? p.lei.slug : undefined,
			nome: p.lei.nome,
			curto: p.lei.curto,
			reconhecer: p.lei.reconhecer,
			aceitos: c.resultado.avisos.map((a: { id: string }) => a.id)
		}
	});
	expect(pub.ok(), await pub.text()).toBeTruthy();
	const publicacao = await pub.json();
	expect(publicacao.slug).toBe(p.lei.slug);

	const q = await request.post(`/api/leis/${p.lei.slug}/questoes`, {
		headers,
		data: { unidades: p.unidades, questoes: p.questoes }
	});
	expect(q.ok(), await q.text()).toBeTruthy();
	const questoes = await q.json();
	await request.dispose();
	return { publicacao, questoes };
}

/** "Adicionar lei" fica recolhido quando o catálogo já tem leis. */
async function abrirAdicionar(page: Page) {
	const campo = page.getByLabel('Link da lei na fonte oficial');
	if (!(await campo.isVisible())) await page.getByText('Adicionar lei', { exact: true }).click();
	await expect(campo).toBeVisible();
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
	test('[L1][L16] a lei capturada pela tela vira uma lei legível, com o texto que o processador leu', async ({ browser, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		const arquivo = test.info().outputPath(`${p.lei.slug}-questoes.json`);
		writeFileSync(arquivo, JSON.stringify({ unidades: p.unidades, questoes: p.questoes }));

		const { request, token } = await outraSessao(baseURL!);
		await new Api(request, token).concurso('Captura E2E');
		const contexto = await browser.newContext({ storageState: await request.storageState() });
		const page = await contexto.newPage();
		await page.goto('/legislacao');
		await expect(page.getByRole('heading', { name: 'Legislação', level: 1 })).toBeVisible();

		await abrirAdicionar(page);
		await page.getByLabel('Link da lei na fonte oficial').fill(linkDaLei());
		await page.getByRole('button', { name: 'Capturar', exact: true }).click();
		// A captura ainda rodando aparece como andamento, e a tela espera (L16).
		await expect(page.getByRole('status')).toContainText('Capturando');
		await expect(page.getByRole('heading', { name: 'Prévia da captura' })).toBeVisible();
		await expect(page.getByText(`3 artigos, ${p.dispositivos.length} dispositivos`)).toBeVisible();

		await page.getByLabel('Nome da lei').fill(p.lei.nome);
		await page.getByLabel('Nome curto').fill(p.lei.curto);
		await page.getByRole('button', { name: 'Publicar lei' }).click();
		await expect(page.getByRole('status')).toContainText(`${p.lei.curto} publicada`);

		await page.getByRole('link', { name: 'Abrir a lei' }).click();
		await expect(page.getByRole('heading', { name: p.lei.curto, level: 1 })).toBeVisible();
		for (const d of p.dispositivos) {
			await expect(dispositivo(page, d.ref).locator('.texto')).toHaveText(d.texto);
		}

		// As questões, escritas fora do app, entram pela página da lei.
		await page.getByText('Manter esta lei').click();
		await page.getByLabel('Questões da lei (questoes.json)').setInputFiles(arquivo);
		await expect(page.getByRole('status')).toContainText('Questões importadas: 4 novas');
		await expect(page.getByRole('button', { name: 'Questões do Art. 1º', exact: true })).toBeVisible();
		await contexto.close();
		await request.dispose();
	});

	test('[L2] publicar e importar de novo a mesma versão não duplica nada', async ({ api, page, conta, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		const primeira = await importar(baseURL!, p);
		expect(primeira.publicacao.novaVersao).toBe(true);
		const segunda = await importar(baseURL!, p, { atualizar: true });
		expect(segunda.publicacao.novaVersao).toBe(false);
		expect(segunda.questoes.novas).toBe(0);
		expect(segunda.questoes.mantidas).toBe(4);

		const catalogo = await page.request.get('/api/leis', { headers: { Authorization: `Bearer ${conta.token}` } });
		const { leis } = await catalogo.json();
		expect(leis.filter((l: { slug: string }) => l.slug === p.lei.slug)).toHaveLength(1);

		await api.concurso('Leitor E2E');
		await abrirLei(page, p.lei.slug);
		await expect(page.locator('.dispositivo')).toHaveCount(p.dispositivos.length);
		const painel = await abrirQuestoes(page, 'Art. 1º');
		await expect(painel.getByRole('group')).toHaveCount(2);
	});

	test('[L3] o texto novo e as questões novas preservam as respostas das que continuam', async ({ api, page, baseURL }) => {
		const id = idUnico();
		const numero = numeroUnico();
		await importar(baseURL!, copia(pacote(), id, numero));
		await api.concurso('Versão E2E');
		await abrirLei(page, `lei-e2e-${id}`);
		const painel = await abrirQuestoes(page, 'Art. 1º');
		await responder(painel, ENUNCIADO_1, 'B');

		const v2 = await importar(baseURL!, copia(pacote('lei-exemplo-v2.json'), id, numero), {
			arquivo: 'lei-exemplo-v2',
			atualizar: true
		});
		expect(v2.publicacao.novaVersao).toBe(true);
		expect(v2.questoes.desativadas).toBe(1);

		await page.reload();
		await expect(dispositivo(page, 'art3').locator('.texto')).toHaveText(
			'Art. 3º Esta Lei entra em vigor trinta dias após a data de sua publicação.'
		);
		const depois = await abrirQuestoes(page, 'Art. 1º');
		await expect(depois.getByRole('group', { name: ENUNCIADO_1 }).getByText('Certo')).toBeVisible();
		await page.keyboard.press('Escape');
		// A questão que saiu do arquivo some da tela; a resposta dela fica no banco.
		await expect(page.getByRole('button', { name: 'Questões do Art. 2º', exact: true })).toHaveCount(0);
	});

	test('[L4] qualquer conta logada captura, pela tela ou pela API', async ({ page, api, conta }) => {
		const c = await capturar(page.request, conta.token, linkDaLei());
		expect(c.estado).toBe('pronta');
		expect(c.resultado.publicavel).toBe(true);

		await api.concurso('Conta comum E2E');
		await page.goto('/legislacao');
		await abrirAdicionar(page);
		await expect(page.getByLabel('Link da lei na fonte oficial')).toBeEditable();
	});

	test('[L11] a captura com bloqueio não pode ser publicada', async ({ page, api, conta }) => {
		await api.concurso('Bloqueio E2E');
		await page.goto('/legislacao');
		await abrirAdicionar(page);
		await page.getByLabel('Link da lei na fonte oficial').fill(linkDaLei('lei-exemplo', 'bloqueio'));
		await page.getByRole('button', { name: 'Capturar', exact: true }).click();
		await expect(page.getByRole('alert')).toContainText('Esta captura não pode ser publicada');
		await expect(page.getByRole('alert')).toContainText('sha256 diferente');
		await expect(page.getByRole('button', { name: 'Publicar lei' })).toHaveCount(0);

		// A tela esconde o botão; quem recusa de verdade é o servidor.
		const c = await capturar(page.request, conta.token, linkDaLei('lei-exemplo', 'bloqueio'));
		const res = await page.request.post(`/api/leis/capturas/${c.id}/publicacao`, {
			headers: { Authorization: `Bearer ${conta.token}` },
			data: { nome: 'Lei quebrada', curto: `Quebrada ${idUnico()}`, reconhecer: [], aceitos: [] }
		});
		expect(res.status()).toBe(422);
		expect((await res.json()).erro).toContain('impedem a publicação');
	});

	test('[L12] o aviso da captura aparece e só sai marcado como revisado', async ({ page, api, conta }) => {
		const curto = `Lei Aviso ${idUnico()}`;
		const c = await capturar(page.request, conta.token, linkDaLei('lei-exemplo', 'aviso'));
		const headers = { Authorization: `Bearer ${conta.token}` };
		const semRevisar = await page.request.post(`/api/leis/capturas/${c.id}/publicacao`, {
			headers,
			data: { nome: 'Lei com aviso', curto, reconhecer: [], aceitos: ['outro aviso qualquer'] }
		});
		expect(semRevisar.status()).toBe(422);
		expect((await semRevisar.json()).erro).toContain('marque como revisado');

		await api.concurso('Aviso E2E');
		await page.goto('/legislacao');
		await abrirAdicionar(page);
		await page.getByLabel('Link da lei na fonte oficial').fill(linkDaLei('lei-exemplo', 'aviso'));
		await page.getByRole('button', { name: 'Capturar', exact: true }).click();
		const aviso = page.getByRole('checkbox', { name: /Revisei: p0003: a regra diz artigo, o Gemini diz solto/ });
		await expect(aviso).toBeVisible();
		await page.getByLabel('Nome da lei').fill('Lei com aviso');
		await page.getByLabel('Nome curto').fill(curto);
		const publicar = page.getByRole('button', { name: 'Publicar lei' });
		await expect(publicar).toBeDisabled();
		await aviso.check();
		await publicar.click();
		await expect(page.getByRole('status')).toContainText(`${curto} publicada`);
	});

	test('[L13] link fora das fontes oficiais é recusado com o motivo', async ({ page, api }) => {
		await api.concurso('Fonte E2E');
		await page.goto('/legislacao');
		await abrirAdicionar(page);
		await page.getByLabel('Link da lei na fonte oficial').fill('https://www.exemplo.com/lei.htm');
		await page.getByRole('button', { name: 'Capturar', exact: true }).click();
		await expect(page.getByRole('alert')).toContainText('não é uma fonte oficial');
		await expect(page.getByRole('heading', { name: 'Prévia da captura' })).toHaveCount(0);
	});

	test('[L14] lei nova com o nome curto de outra não a sobrescreve', async ({ page, conta, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);

		const c = await capturar(page.request, conta.token, linkDaLei('lei-exemplo-v2'));
		const res = await page.request.post(`/api/leis/capturas/${c.id}/publicacao`, {
			headers: { Authorization: `Bearer ${conta.token}` },
			data: { nome: 'Outra lei', curto: p.lei.curto, reconhecer: [], aceitos: [] }
		});
		expect(res.status()).toBe(409);
		expect((await res.json()).erro).toContain('Atualizar texto');

		const leitura = await (
			await page.request.get(`/api/leis/${p.lei.slug}`, { headers: { Authorization: `Bearer ${conta.token}` } })
		).json();
		expect(leitura.lei.nome).toBe(p.lei.nome);
		expect(leitura.versao).toBe('e2e-v1');
	});

	test('[L15] questão sem apoio literal na lei, ou escrita para outra redação, é recusada', async ({ page, conta, baseURL }) => {
		const p = copia(pacote(), idUnico(), numeroUnico());
		await importar(baseURL!, p);
		const headers = { Authorization: `Bearer ${conta.token}` };

		const inventada = structuredClone(p);
		inventada.questoes[0].trecho = 'um trecho que a lei não tem';
		const r1 = await page.request.post(`/api/leis/${p.lei.slug}/questoes`, {
			headers,
			data: { unidades: inventada.unidades, questoes: inventada.questoes }
		});
		expect(r1.status()).toBe(422);
		expect((await r1.json()).erro).toContain('não está, literalmente');

		// As unidades da v2 trazem o hash do texto da v2; a lei publicada é a v1.
		const v2 = pacote('lei-exemplo-v2.json');
		const r2 = await page.request.post(`/api/leis/${p.lei.slug}/questoes`, {
			headers,
			data: { unidades: v2.unidades, questoes: v2.questoes }
		});
		expect(r2.status()).toBe(422);
		expect((await r2.json()).erro).toContain('desatualizada');

		// Sem hash, a unidade é escrita para o texto publicado, e passa.
		const semHash = p.unidades.map((u) => ({ ...u, hash: '' }));
		const r3 = await page.request.post(`/api/leis/${p.lei.slug}/questoes`, {
			headers,
			data: { unidades: semHash, questoes: p.questoes }
		});
		expect(r3.ok(), await r3.text()).toBeTruthy();
		expect((await r3.json()).mantidas).toBe(4);
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

	test('[L17][L18] a prévia mostra o que o edital pede, e publicar vincula a matéria com esse recorte', async ({ page, api }) => {
		const curto = `Lei Recorte ${idUnico()}`;
		// O dublê devolve a lei da fixture, cuja epígrafe é a Lei nº 99.999.
		await api.concurso('Recorte E2E', [
			{ nome: 'Legislação E2E', bloco: 'esp', questoes: 8, temas: ['Lei nº 99.999/2026: controle externo'] },
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase'] }
		]);
		await page.goto('/legislacao');
		await abrirAdicionar(page);
		await page.getByLabel('Link da lei na fonte oficial').fill(linkDaLei());
		await page.getByRole('button', { name: 'Capturar', exact: true }).click();

		const edital = page.getByRole('group', { name: 'O que o edital pede desta lei' });
		await expect(edital).toContainText('Legislação E2E');
		await expect(edital).toContainText('CAPÍTULO I — Do Controle Externo (arts. 1º a 2º)');
		await expect(edital).not.toContainText('Língua Portuguesa');
		await expect(edital.getByRole('checkbox')).toBeChecked();
		// O nome vem da epígrafe; o curto é de quem publica.
		await expect(page.getByLabel('Nome da lei')).toHaveValue(/99\.999/);
		await page.getByLabel('Nome curto').fill(curto);
		await page.getByRole('button', { name: 'Publicar lei' }).click();
		await expect(page.getByRole('status')).toContainText(`${curto} publicada`);

		const materia = page.getByRole('region', { name: 'Legislação E2E' });
		await expect(materia.getByRole('link', { name: curto })).toBeVisible();
		await expect(materia.getByRole('listitem').filter({ hasText: curto })).toContainText('arts. 1º a 2º');
		await page.reload();
		await expect(
			page.getByRole('region', { name: 'Legislação E2E' }).getByRole('listitem').filter({ hasText: curto })
		).toContainText('arts. 1º a 2º');
	});

	/** Publica a lei (outra conta) e a vincula à matéria deste teste pela API, sem recorte: o servidor o lê do tópico. */
	async function leiNoConcurso(api: Api, page: Page, token: string, baseURL: string) {
		const numero = numeroUnico();
		const p = copia(pacote(), idUnico(), numero);
		await importar(baseURL, p);
		const slug = await api.concurso('Leitor com recorte E2E', [
			{ nome: 'Legislação E2E', bloco: 'esp', questoes: 8, temas: [`Lei nº ${numero}/2026: controle externo`] }
		]);
		const headers = { Authorization: `Bearer ${token}` };
		const { disciplinas } = await (await page.request.get(`/api/concursos/${slug}/leis`, { headers })).json();
		const res = await page.request.put(`/api/concursos/${slug}/disciplinas/${disciplinas[0].disciplinaId}/leis/${p.lei.slug}`, { headers });
		expect(res.status(), await res.text()).toBe(204);
		return p;
	}

	test('[L19] aberta no concurso, a lei mostra só o recorte, e a lei inteira a um clique', async ({ api, page, conta, baseURL }) => {
		const p = await leiNoConcurso(api, page, conta.token, baseURL!);
		await abrirLei(page, p.lei.slug);

		const callout = page.getByRole('complementary', { name: 'Recorte do edital' });
		await expect(callout).toContainText('O edital cobra esta parte da lei');
		await expect(callout).toContainText('CAPÍTULO I — Do Controle Externo');
		await expect(dispositivo(page, 'art1')).toBeVisible();
		await expect(dispositivo(page, 'art3')).toHaveCount(0);

		await callout.getByRole('button', { name: 'Lei inteira' }).click();
		await expect(dispositivo(page, 'art3')).toBeVisible();
		await callout.getByRole('button', { name: 'Só o que cai' }).click();
		await expect(dispositivo(page, 'art3')).toHaveCount(0);
		await page.getByRole('button', { name: 'Ler a lei inteira' }).click();
		await expect(dispositivo(page, 'art3')).toBeVisible();
	});

	test('[L20] o recorte ajustado à mão fica gravado', async ({ api, page, conta, baseURL }) => {
		const p = await leiNoConcurso(api, page, conta.token, baseURL!);
		await abrirLei(page, p.lei.slug);

		await page.getByRole('button', { name: 'Ajustar recorte' }).click();
		// Dentro do capítulo marcado, o art. 1º já vem junto e não se desmarca sozinho.
		await expect(page.getByRole('checkbox', { name: 'No recorte: Art. 1º' })).toBeDisabled();
		await page.getByRole('checkbox', { name: 'No recorte: Art. 3º' }).check();
		await page.getByRole('button', { name: 'Salvar recorte' }).click();

		const callout = page.getByRole('complementary', { name: 'Recorte do edital' });
		await expect(callout).toContainText('O edital cobra estas partes da lei');
		await expect(dispositivo(page, 'art3')).toBeVisible();
		await page.reload();
		await expect(page.getByRole('complementary', { name: 'Recorte do edital' })).toContainText('Art. 3º');
		await expect(dispositivo(page, 'art3')).toBeVisible();
	});

	test('[L21] o link direto para fora do recorte abre o dispositivo', async ({ api, page, conta, baseURL }) => {
		const p = await leiNoConcurso(api, page, conta.token, baseURL!);
		await page.goto(`/leis/${p.lei.slug}#art3`);
		await expect(dispositivo(page, 'art3')).toBeInViewport();
		await expect(dispositivo(page, 'art3')).toHaveAttribute('aria-current', 'location');
		await expect(page.getByRole('button', { name: 'Lei inteira' })).toHaveAttribute('aria-pressed', 'true');
	});
});
