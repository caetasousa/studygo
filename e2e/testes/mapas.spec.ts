import { readFileSync } from 'node:fs';
import { randomInt } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { request as novoRequest, type APIRequestContext, type Page } from '@playwright/test';
import { test, expect, Api, cadastrar, emailUnico, type Conta, type Disciplina } from './base';

// Os mapas mentais são da conta que os importou, então cada teste (que tem a
// conta dele) importa os seus sem esbarrar nos dos outros. O texto vem de
// e2e/fixtures/mapa-exemplo.md, um mapa sintético e pequeno: o conteúdo de
// verdade deriva de aulas pagas e não vai para o repositório.

const FIXTURE = new URL('../fixtures/mapa-exemplo.md', import.meta.url);
const exemplo = () => readFileSync(FIXTURE, 'utf-8');
const caminhoDoExemplo = fileURLToPath(FIXTURE);

/** O mapa de exemplo, escrito como a API o devolve: é o que a importação tem de reproduzir. */
interface Item {
	texto: string;
	marca: string;
	filhos: Item[];
}

const item = (texto: string, marca = '', filhos: Item[] = []): Item => ({ texto, marca, filhos });

const ESPERADO: Item[] = [
	item('Evaporação', '', [
		item('A **evaporação** leva a água líquida ao estado gasoso', 'def', [
			item('Acontece na superfície dos oceanos'),
			item('Aumenta com o calor')
		]),
		item('Uma poça que seca ao sol', 'ex')
	]),
	item('Condensação', '', [
		item('Forma as nuvens', '', [item('Ocorre quando o vapor esfria', 'cai')]),
		item('Não confundir com a queda da água', 'pegadinha')
	]),
	item('Precipitação', '', [
		item('Chuva'),
		item('Neve', '', [item('A banca troca neve por granizo', 'questao')]),
		item('Granizo')
	]),
	item('Infiltração')
];

/** Os itens como aparecem na tela: sem o `**` do negrito. */
const semNegrito = (t: string) => t.replaceAll('**', '');

function achatar(itens: Item[]): Item[] {
	return itens.flatMap((i) => [i, ...achatar(i.filhos)]);
}

/** O mesmo mapa com outro endereço e título: o teste que importa dois usa isto. */
function variante(texto: string, sufixo: string): string {
	return texto
		.replace('slug: ciclo-da-agua', `slug: ciclo-da-agua-${sufixo}`)
		.replace('# Ciclo da Água', `# Ciclo da Água ${sufixo}`);
}

/** Um mapa que nenhuma matéria destes testes indica. */
const POEMA = '# Poema\nslug: poema\nmateria: Literatura Barroca\n\n- Estrofe\n  - Verso\n';

const MATERIAS: Disciplina[] = [
	{ nome: 'Geografia Física', codigo: 'GEO', bloco: 'esp', questoes: 10, temas: ['Ciclo hidrológico', 'Relevo e solos'] },
	{ nome: 'Direito Constitucional', codigo: 'DIR', bloco: 'esp', questoes: 10, temas: ['Princípios fundamentais', 'Poderes da União'] }
];

const cabecalho = (token: string) => ({ Authorization: `Bearer ${token}` });

async function importar(request: APIRequestContext, token: string, texto: string, concurso = '') {
	const res = await request.post('/api/mapas', { headers: cabecalho(token), data: { texto, concurso } });
	return { status: res.status(), corpo: await res.json() };
}

async function ler(request: APIRequestContext, token: string, slug: string) {
	return request.get(`/api/mapas/${slug}`, { headers: cabecalho(token) });
}

async function catalogo(request: APIRequestContext, token: string): Promise<{ slug: string }[]> {
	return (await (await request.get('/api/mapas', { headers: cabecalho(token) })).json()).mapas;
}

interface MateriaComMapas {
	disciplinaId: string;
	codigo: string;
	mapas: { slug: string; materiaInteira: boolean; temas: string[] }[];
}

async function materiasDo(request: APIRequestContext, token: string, concurso: string): Promise<MateriaComMapas[]> {
	const res = await request.get(`/api/concursos/${concurso}/mapas`, { headers: cabecalho(token) });
	expect(res.ok(), await res.text()).toBeTruthy();
	return (await res.json()).disciplinas;
}

/** Outra conta, com a sessão dela: quem não é dono do mapa. */
async function outraSessao(baseURL: string): Promise<{ request: APIRequestContext; token: string }> {
	const request = await novoRequest.newContext({
		baseURL,
		extraHTTPHeaders: { 'X-Forwarded-For': `10.201.${randomInt(1, 255)}.${randomInt(1, 255)}` }
	});
	const { token } = await cadastrar(request, emailUnico('outra'));
	return { request, token };
}

/** O concurso com as matérias GEO e DIR, e o exemplo já importado e vinculado à GEO. */
async function concursoComMapa(api: Api, conta: Conta, page: Page, nome: string) {
	const concurso = await api.concurso(nome, MATERIAS);
	const r = await importar(page.request, conta.token, exemplo(), concurso);
	expect(r.status).toBe(201);
	return concurso;
}

const linhaDa = (page: Page, codigo: string) =>
	page.locator('.atv', { has: page.locator('.chip', { hasText: codigo }) });

/** Os tópicos do mapa aberto (a lista de cima; as de dentro são dos filhos). */
const topicosDo = (page: Page) => page.getByRole('list', { name: 'Tópicos do mapa' });

/**
 * O item do mapa na tela. O texto de um item marcado vem junto da etiqueta
 * ("Definição"), então ele é achado por um trecho; o sem marca, pelo texto exato —
 * "Neve" não pode casar com "A banca troca neve por granizo".
 */
const itemNaTela = (page: Page, i: Item) =>
	i.marca ? topicosDo(page).getByText(semNegrito(i.texto)) : topicosDo(page).getByText(i.texto, { exact: true });

const semRolagemLateral = (page: Page) =>
	page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);

/** Um mapa com a profundidade que uma aula chega a ter: seis níveis. */
const FUNDO = [
	'# Fundo',
	'slug: fundo',
	'',
	'- Nível um',
	'  - Nível dois',
	'    - Nível três',
	'      - Nível quatro',
	'        - Nível cinco',
	'          - Nível seis, o mais fundo do mapa'
].join('\n');

test.describe('mapas mentais', () => {
	test('[M1] o texto importado pela tela vira um mapa fiel, com cada item no seu lugar', async ({ page, api, conta }) => {
		await api.concurso('Importar E2E', MATERIAS);
		await page.goto('/mapas');
		await expect(page.getByRole('heading', { name: 'Mapas mentais', level: 1 })).toBeVisible();

		await page.getByLabel('Arquivo do mapa (.md)').setInputFiles(caminhoDoExemplo);
		await expect(page.getByRole('status')).toContainText('Ciclo da Água importado: 4 ramos, 15 itens');

		// A árvore gravada é exatamente a escrita: nada some, nada troca de pai ou de ordem.
		const res = await ler(page.request, conta.token, 'ciclo-da-agua');
		expect(res.status()).toBe(200);
		const lido = await res.json();
		expect(lido.mapa).toMatchObject({
			slug: 'ciclo-da-agua',
			titulo: 'Ciclo da Água',
			fonte: 'Material sintético dos testes E2E',
			materia: 'Geografia Física',
			ramos: 4,
			itens: 15
		});
		expect(lido.arvore).toEqual(ESPERADO);

		await page.getByRole('link', { name: 'Abrir o mapa' }).click();
		await expect(page.getByRole('heading', { name: 'Ciclo da Água', level: 1 })).toBeVisible();
	});

	test('[M2] importar de novo o mesmo mapa troca o conteúdo e mantém o vínculo com a matéria', async ({ page, api, conta }) => {
		const concurso = await api.concurso('Reimportar E2E', MATERIAS);

		const primeira = await importar(page.request, conta.token, exemplo(), concurso);
		expect(primeira.status).toBe(201);
		expect(primeira.corpo.novo).toBe(true);
		expect(primeira.corpo.vinculadas.map((v: { codigo: string }) => v.codigo)).toEqual(['GEO']);

		const maior = exemplo().replace('- Infiltração', '- Infiltração\n  - Lençol freático');
		const segunda = await importar(page.request, conta.token, maior, concurso);
		expect(segunda.status).toBe(200);
		expect(segunda.corpo.novo).toBe(false);
		expect(segunda.corpo.mapa.itens).toBe(16);

		// Nem duplicou o mapa, nem o vínculo se perdeu…
		expect(await catalogo(page.request, conta.token)).toHaveLength(1);
		expect((await materiasDo(page.request, conta.token, concurso)).find((m) => m.codigo === 'GEO')?.mapas).toHaveLength(1);

		// …nem mesmo quando a importação não diz de que concurso é.
		const terceira = await importar(page.request, conta.token, exemplo());
		expect(terceira.corpo.novo).toBe(false);
		expect(terceira.corpo.mapa.itens).toBe(15);
		expect((await materiasDo(page.request, conta.token, concurso)).find((m) => m.codigo === 'GEO')?.mapas).toHaveLength(1);

		const lido = await (await ler(page.request, conta.token, 'ciclo-da-agua')).json();
		expect(lido.arvore).toEqual(ESPERADO);
	});

	test('[M3] o texto com problema é recusado inteiro, e a mensagem diz a linha', async ({ page, api, conta }) => {
		// Sem concurso o app manda para o cadastro: a tela dos mapas só existe com um aberto.
		await api.concurso('Recusa E2E', MATERIAS);
		const ruins: [string, string, RegExp][] = [
			['sem título', '- um item\n', /linha 1: o texto precisa começar com o título/],
			['recuo que pula um nível', '# T\n- a\n    - c\n', /linha 3: o item pula do nível 1 para o 2/],
			['recuo ímpar', '# T\n- a\n   - b\n', /linha 3: recuo de 3 espaços/],
			['marca desconhecida', '# T\n- [xyz] a\n', /linha 2: marca desconhecida \[xyz\] — use def, pegadinha, cai, ex ou questao/],
			['item vazio', '# T\n- \n', /linha 2: item vazio/],
			['tabulação no recuo', '# T\n- a\n\t- b\n', /linha 3: tabulação no recuo/],
			['metadado desconhecido', '# T\nfoo: bar\n- a\n', /linha 2: metadado desconhecido "foo"/],
			['sem nenhum item', '# T\n', /não tem nenhum item/],
			['slug fora do formato', '# T\nslug: Ab C\n- a\n', /slug "Ab C" inválido/],
			['título sem letra nem número', '# ---\n- a\n', /informe slug:/]
		];

		for (const [nome, texto, mensagem] of ruins) {
			const r = await importar(page.request, conta.token, texto);
			expect(r.status, nome).toBe(422);
			expect(r.corpo.erro, nome).toMatch(mensagem);
		}

		// Todos os problemas de uma vez, e nenhum mapa pela metade.
		const varios = await importar(page.request, conta.token, '# T\n- a\n      - b\n- [xyz] c\n');
		expect(varios.status).toBe(422);
		expect(varios.corpo.erro).toContain('linha 3');
		expect(varios.corpo.erro).toContain('linha 4');
		expect(await catalogo(page.request, conta.token)).toEqual([]);

		// Pela tela, o mesmo aviso, com a linha.
		await page.goto('/mapas');
		await page.getByLabel('Texto do mapa').fill('# T\n- a\n    - c\n');
		await page.getByRole('button', { name: 'Importar', exact: true }).click();
		await expect(page.getByRole('alert')).toContainText('linha 3: o item pula do nível 1 para o 2');
		await expect(page.getByText('Nenhum mapa mental ainda.')).toBeVisible();
	});

	test('[M4] o mapa de uma conta não aparece, abre nem é mexido por outra conta', async ({ page, api, conta, baseURL }) => {
		const concursoA = await api.concurso('Dona E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo(), concursoA);

		const { request, token } = await outraSessao(baseURL!);
		expect(await catalogo(request, token)).toEqual([]);
		expect((await ler(request, token, 'ciclo-da-agua')).status()).toBe(404);
		expect((await request.delete('/api/mapas/ciclo-da-agua', { headers: cabecalho(token) })).status()).toBe(404);

		// O concurso de outra conta também não é dela para vincular, listar ou importar contra.
		const concursoB = await new Api(request, token).concurso('Outra E2E', MATERIAS);
		const [materiaB] = await materiasDo(request, token, concursoB);
		const vincular = await request.put(
			`/api/concursos/${concursoB}/disciplinas/${materiaB.disciplinaId}/mapas/ciclo-da-agua`,
			{ headers: cabecalho(token) }
		);
		expect(vincular.status()).toBe(404);
		expect((await request.get(`/api/concursos/${concursoA}/mapas`, { headers: cabecalho(token) })).status()).toBe(404);
		expect((await importar(request, token, exemplo(), concursoA)).status).toBe(404);
		expect(await catalogo(request, token)).toEqual([]);

		// O mesmo endereço, na outra conta, é outro mapa: nada colide, nada é tocado.
		const proprio = await importar(request, token, exemplo());
		expect(proprio.status).toBe(201);
		expect(await catalogo(request, token)).toHaveLength(1);
		expect(await catalogo(page.request, conta.token)).toHaveLength(1);
		expect((await ler(page.request, conta.token, 'ciclo-da-agua')).status()).toBe(200);
		await request.dispose();

		// Na tela, o mapa que não é da conta (ou não existe) diz isso, sem quebrar.
		await page.goto('/mapas/nao-existe');
		await expect(page.getByRole('alert')).toContainText('mapa mental não encontrado');
	});

	test('[M5] as marcas viram etiquetas e o negrito vira negrito, sem sinais soltos no texto', async ({ page, api, conta }) => {
		await api.concurso('Marcas E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());

		await page.goto('/mapas/ciclo-da-agua');
		await page.getByRole('button', { name: 'Abrir tudo' }).click();
		const corpo = page.locator('main');

		for (const etiqueta of ['Definição', 'Exemplo', 'Cai em prova', 'Pegadinha', 'Questão']) {
			await expect(corpo.getByText(etiqueta, { exact: true })).toBeVisible();
		}
		await expect(corpo.locator('strong', { hasText: 'evaporação' })).toBeVisible();
		await expect(corpo).not.toContainText('**');
		await expect(corpo).not.toContainText('[def]');
		await expect(corpo).not.toContainText('[pegadinha]');
	});

	test('[M6] a lista agrupa os mapas pela matéria e mostra à parte o que ainda não tem matéria', async ({ page, api, conta }) => {
		const concurso = await api.concurso('Lista E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo(), concurso);
		await importar(page.request, conta.token, POEMA, concurso);

		await page.goto('/mapas');
		const geo = page.getByRole('region', { name: 'Geografia Física' });
		await expect(geo.getByRole('link', { name: /Ciclo da Água/ })).toBeVisible();
		await expect(geo.getByRole('link', { name: /Poema/ })).toHaveCount(0);

		const sem = page.getByRole('region', { name: 'Sem matéria vinculada' });
		await expect(sem.getByRole('link', { name: /Poema/ })).toBeVisible();
		await expect(sem.getByText('a fonte indica: Literatura Barroca')).toBeVisible();
		await expect(sem.getByRole('link', { name: /Ciclo da Água/ })).toHaveCount(0);
	});

	test('[M7] vincular e desvincular pela página do mapa fica gravado, e só vale para o concurso dele', async ({ page, api, conta }) => {
		const concurso = await api.concurso('Vínculo E2E', MATERIAS);
		await importar(page.request, conta.token, POEMA, concurso);

		await page.goto('/mapas/poema');
		await expect(page.getByRole('heading', { name: 'Poema', level: 1 })).toBeVisible();
		await page.getByLabel('Vincular a uma matéria').selectOption({ label: 'DIR — Direito Constitucional' });
		const tirar = page.getByRole('button', { name: 'Desvincular Direito Constitucional' });
		await expect(tirar).toBeVisible();

		await page.reload();
		await expect(tirar).toBeVisible();
		await tirar.click();
		await expect(page.getByRole('button', { name: /^Desvincular/ })).toHaveCount(0);
		await page.reload();
		await expect(page.getByRole('heading', { name: 'Poema', level: 1 })).toBeVisible();
		await expect(page.getByRole('button', { name: /^Desvincular/ })).toHaveCount(0);

		// A matéria de outro concurso da mesma conta não vale para este.
		const outro = await api.concurso('Outro concurso E2E', MATERIAS);
		const [deOutro] = await materiasDo(page.request, conta.token, outro);
		const res = await page.request.put(
			`/api/concursos/${concurso}/disciplinas/${deOutro.disciplinaId}/mapas/poema`,
			{ headers: cabecalho(conta.token) }
		);
		expect(res.status()).toBe(404);
		expect((await materiasDo(page.request, conta.token, outro)).flatMap((m) => m.mapas)).toEqual([]);
	});

	test('[M8] importar com o concurso aberto vincula às matérias que o texto indica, e a nenhuma outra', async ({ page, api, conta }) => {
		const concurso = await api.concurso('Indicação E2E', [
			{ nome: 'Geografia Física', codigo: 'GEO', bloco: 'esp', questoes: 10, temas: ['Relevo e solos'] },
			{ nome: 'Clima e Águas', codigo: 'CLI', bloco: 'esp', questoes: 10, temas: ['Hidrologia continental', 'Massas de ar'] },
			{ nome: 'Direito Constitucional', codigo: 'DIR', bloco: 'esp', questoes: 10, temas: ['Princípios fundamentais'] }
		]);

		// GEO pelo nome (`materia:`); CLI porque um tópico cita "hidrologia" (`reconhecer:`); DIR, nada.
		const r = await importar(page.request, conta.token, exemplo(), concurso);
		expect(r.corpo.vinculadas.map((v: { codigo: string }) => v.codigo).sort()).toEqual(['CLI', 'GEO']);

		const porCodigo = Object.fromEntries(
			(await materiasDo(page.request, conta.token, concurso)).map((m) => [m.codigo, m.mapas.map((x) => x.slug)])
		);
		expect(porCodigo).toEqual({ GEO: ['ciclo-da-agua'], CLI: ['ciclo-da-agua'], DIR: [] });

		// Um texto que nada indica não vincula a nada — e importar sem concurso também não.
		const sem = await importar(page.request, conta.token, POEMA, concurso);
		expect(sem.status).toBe(201);
		expect(sem.corpo.vinculadas).toEqual([]);
		const semConcurso = await importar(page.request, conta.token, variante(exemplo(), 'b'));
		expect(semConcurso.status).toBe(201);
		expect(semConcurso.corpo.vinculadas).toEqual([]);
		expect((await materiasDo(page.request, conta.token, concurso)).flatMap((m) => m.mapas.map((x) => x.slug)).sort()).toEqual([
			'ciclo-da-agua',
			'ciclo-da-agua'
		]);
	});

	test('[M9] o cronograma e o Hoje oferecem o mapa da matéria que tem mapa — e só o dela', async ({ page, api, conta }) => {
		await concursoComMapa(api, conta, page, 'Cronograma E2E');

		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();

		const geo = linhaDa(page, 'GEO');
		const dir = linhaDa(page, 'DIR');
		expect(await dir.count()).toBeGreaterThan(0);
		await expect(geo.first().getByRole('link', { name: 'Abrir o mapa mental: Ciclo da Água' })).toBeVisible();
		await expect(dir.getByRole('link', { name: /mapa mental/i })).toHaveCount(0);

		// Todo acesso ao mapa do cronograma está numa linha da GEO.
		const acessos = await page.getByRole('link', { name: /Abrir o mapa mental/ }).count();
		expect(acessos).toBe(await geo.getByRole('link', { name: /Abrir o mapa mental/ }).count());
		expect(acessos).toBeGreaterThan(0);

		// Pelo diálogo da matéria também, e só na que tem mapa.
		await geo.first().getByRole('button', { name: /Ver o conteúdo programático da matéria/ }).click();
		const dialogo = page.getByRole('dialog');
		await expect(dialogo.getByRole('link', { name: /Ciclo da Água/ })).toBeVisible();
		await page.keyboard.press('Escape');
		await dir.first().getByRole('button', { name: /Ver o conteúdo programático da matéria/ }).click();
		await expect(dialogo.getByRole('heading', { name: 'Direito Constitucional' })).toBeVisible();
		await expect(dialogo.getByRole('link')).toHaveCount(0);
		await page.keyboard.press('Escape');

		// O Hoje mostra as atividades do dia com o mesmo componente: o mapa está lá também.
		await page.goto('/');
		await expect(page.getByRole('heading', { name: 'Hoje', level: 1 })).toBeVisible();
		await expect(linhaDa(page, 'GEO').first().getByRole('link', { name: 'Abrir o mapa mental: Ciclo da Água' })).toBeVisible();
		await expect(linhaDa(page, 'DIR').getByRole('link', { name: /mapa mental/i })).toHaveCount(0);

		// E o acesso leva ao mapa.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		await geo.first().getByRole('link', { name: /Abrir o mapa mental/ }).click();
		await expect(page).toHaveURL(/\/mapas\/ciclo-da-agua$/);
		await expect(page.getByRole('heading', { name: 'Ciclo da Água', level: 1 })).toBeVisible();
	});

	test('[M10] a matéria com dois mapas leva à lista dos dois, e nenhum fica fora de alcance', async ({ page, api, conta }) => {
		const concurso = await concursoComMapa(api, conta, page, 'Dois mapas E2E');
		expect((await importar(page.request, conta.token, variante(exemplo(), 'revisao'), concurso)).status).toBe(201);

		// Dois mapas no assunto da linha: o ícone abre a ementa, que lista os dois.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		await linhaDa(page, 'GEO').first().getByRole('button', { name: /^Mapas mentais de .* \(2\)$/ }).click();
		const dialogo = page.getByRole('dialog');
		await expect(dialogo.getByRole('link', { name: /^Ciclo da Água\b(?! revisao)/ })).toBeVisible();
		await expect(dialogo.getByRole('link', { name: /^Ciclo da Água revisao/ })).toBeVisible();
		await expect(dialogo.getByRole('link')).toHaveCount(2);

		// A lista da matéria também tem os dois.
		await page.goto('/mapas?materia=GEO');
		const geo = page.getByRole('region', { name: 'Geografia Física' });
		await expect(geo.getByRole('link', { name: /^Ciclo da Água\b(?! revisao)/ })).toBeVisible();
		await expect(geo.getByRole('link', { name: /^Ciclo da Água revisao/ })).toBeVisible();

		// A matéria sem mapa não tem lista.
		await page.goto('/mapas?materia=DIR');
		await expect(page.getByText('Nenhum mapa vinculado a DIR.')).toBeVisible();
	});

	test('[M27] o mapa aparece só nos tópicos dele: a importação marca o que ele cita, a ementa mostra, e a escolha na página fica gravada', async ({ page, api, conta }) => {
		const concurso = await api.concurso('Tópicos E2E', MATERIAS);
		const relevo = '# Relevo\nslug: relevo\nreconhecer: Relevo\n\n- Formas\n  - Planalto\n';
		expect((await importar(page.request, conta.token, relevo, concurso)).status).toBe(201);

		const abrir = { name: 'Abrir o mapa mental: Relevo' };

		// No cronograma, só a linha do tópico que o mapa cita tem o mapa.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		const comRelevo = linhaDa(page, 'GEO').filter({ hasText: 'Relevo e solos' });
		const semRelevo = linhaDa(page, 'GEO').filter({ hasNotText: 'Relevo e solos' });
		expect(await comRelevo.count()).toBeGreaterThan(0);
		expect(await semRelevo.count()).toBeGreaterThan(0);
		await expect(comRelevo.first().getByRole('link', abrir)).toBeVisible();
		await expect(semRelevo.getByRole('link', abrir)).toHaveCount(0);
		await expect(page.getByRole('link', abrir)).toHaveCount(await comRelevo.count());

		// A ementa marca o tópico com mapa, e só ele.
		await semRelevo.first().getByRole('button', { name: /Ver o conteúdo programático da matéria/ }).click();
		const dialogo = page.getByRole('dialog');
		await expect(dialogo.getByText('1 de 2 tópicos com mapa mental')).toBeVisible();
		const topico = (t: string) => dialogo.getByRole('listitem').filter({ hasText: t });
		await expect(topico('Relevo e solos').getByRole('link', { name: 'Relevo' })).toBeVisible();
		await expect(topico('Ciclo hidrológico').getByRole('link')).toHaveCount(0);
		await topico('Relevo e solos').getByRole('link', { name: 'Relevo' }).click();
		await expect(page).toHaveURL(/\/mapas\/relevo$/);

		// Na página do mapa, a escolha dos tópicos fica gravada.
		await expect(page.getByText('Relevo e solos', { exact: true })).toBeVisible();
		await page.getByRole('button', { name: 'Escolher tópicos' }).click();
		await page.getByRole('checkbox', { name: 'Ciclo hidrológico' }).check();
		await expect(page.getByText('Ciclo hidrológico · Relevo e solos')).toBeVisible();
		await page.reload();
		await expect(page.getByText('Ciclo hidrológico · Relevo e solos')).toBeVisible();

		// Importar o mapa de novo não desfaz a escolha.
		expect((await importar(page.request, conta.token, relevo, concurso)).status).toBe(200);
		const geo = (await materiasDo(page.request, conta.token, concurso)).find((d) => d.codigo === 'GEO');
		expect(geo?.mapas.find((m) => m.slug === 'relevo')).toMatchObject({
			materiaInteira: false,
			temas: ['Ciclo hidrológico', 'Relevo e solos']
		});

		// E agora a linha do ciclo hidrológico também tem o mapa.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		await expect(semRelevo.first().getByRole('link', abrir)).toBeVisible();

		// Desmarcar tudo volta à matéria inteira, e o vínculo continua.
		await page.goto('/mapas/relevo');
		await page.getByRole('button', { name: 'Escolher tópicos' }).click();
		await page.getByRole('checkbox', { name: 'Ciclo hidrológico' }).uncheck();
		await expect(page.getByRole('checkbox', { name: 'Ciclo hidrológico' })).toBeEnabled();
		await page.getByRole('checkbox', { name: 'Relevo e solos' }).uncheck();
		await expect(page.getByText('a matéria inteira')).toBeVisible();
		await page.reload();
		await expect(page.getByText('a matéria inteira')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Desvincular Geografia Física' })).toBeVisible();
	});

	test('[M11] excluir o mapa pede confirmação e tira o acesso do cronograma', async ({ page, api, conta }) => {
		const concurso = await concursoComMapa(api, conta, page, 'Excluir mapa E2E');

		await page.goto('/mapas/ciclo-da-agua');
		await expect(page.getByRole('heading', { name: 'Ciclo da Água', level: 1 })).toBeVisible();

		// Excluir fica recolhido em "Manter este mapa", no fim da página, como na lei.
		const manter = page.getByText('Manter este mapa');
		await manter.click();
		await page.getByRole('button', { name: 'Excluir mapa' }).click();
		const dialogo = page.getByRole('alertdialog');
		await expect(dialogo).toContainText('Excluir “Ciclo da Água”?');
		await dialogo.getByRole('button', { name: 'Cancelar' }).click();
		await expect(dialogo).toBeHidden();
		await page.reload();
		await expect(page.getByRole('heading', { name: 'Ciclo da Água', level: 1 })).toBeVisible();
		expect((await ler(page.request, conta.token, 'ciclo-da-agua')).status()).toBe(200);

		await manter.click();
		await page.getByRole('button', { name: 'Excluir mapa' }).click();
		await dialogo.getByRole('button', { name: 'Excluir mapa' }).click();
		await expect(page).toHaveURL(/\/mapas$/);
		await expect(page.getByRole('link', { name: /Ciclo da Água/ })).toHaveCount(0);

		expect((await ler(page.request, conta.token, 'ciclo-da-agua')).status()).toBe(404);
		expect((await materiasDo(page.request, conta.token, concurso)).flatMap((m) => m.mapas)).toEqual([]);

		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		expect(await linhaDa(page, 'GEO').count()).toBeGreaterThan(0);
		await expect(page.getByRole('link', { name: /mapa mental/i })).toHaveCount(0);
	});

	test('[M12] todo item do mapa tem caminho até a tela: o tópico abre, "Abrir tudo" abre tudo e "Recolher tudo" deixa os ramos', async ({ page, api, conta }) => {
		await api.concurso('Tópicos E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);

		// O mapa abre recolhido, em qualquer tela: só os ramos à vista.
		const evaporacao = topicos.getByRole('button', { name: 'Evaporação', exact: true });
		await expect(evaporacao).toHaveAttribute('aria-expanded', 'false');
		await expect(topicos.getByText('Uma poça que seca ao sol')).toHaveCount(0);
		await evaporacao.click();
		await expect(topicos.getByText('Uma poça que seca ao sol')).toBeVisible();
		await expect(topicos.getByText('Acontece na superfície dos oceanos')).toHaveCount(0);

		// Um clique abre o tópico, e outro o fecha.
		const definicao = topicos.getByRole('button', { name: /evaporação leva a água/ });
		await definicao.click();
		await expect(topicos.getByText('Acontece na superfície dos oceanos')).toBeVisible();
		await definicao.click();
		await expect(topicos.getByText('Acontece na superfície dos oceanos')).toHaveCount(0);

		// "Abrir tudo" mostra cada item do mapa.
		const todos = achatar(ESPERADO);
		expect(todos).toHaveLength(15);
		await page.getByRole('button', { name: 'Abrir tudo' }).click();
		for (const i of todos) await expect(itemNaTela(page, i), i.texto).toBeVisible();

		// "Recolher tudo" deixa só os ramos, que continuam abrindo.
		await page.getByRole('button', { name: 'Recolher tudo' }).click();
		for (const r of ESPERADO) await expect(itemNaTela(page, r), r.texto).toBeVisible();
		await expect(topicos.getByText('Uma poça que seca ao sol')).toHaveCount(0);
		await topicos.getByRole('button', { name: 'Precipitação', exact: true }).click();
		await expect(topicos.getByText('Granizo', { exact: true })).toBeVisible();
	});

	test('[M14] o filtro acha sem acento e em outra caixa, abre o caminho até o achado e mostra o que há nele', async ({ page, api, conta }) => {
		await api.concurso('Filtro E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);
		const filtro = page.getByLabel('Filtrar itens do mapa');
		const achados = page.locator('main').getByRole('status');

		// Sem acento e em maiúsculas, e com o negrito no meio: acha o ramo e a definição.
		await filtro.fill('EVAPORACAO');
		await expect(achados).toHaveText('2 itens trazem “EVAPORACAO”.');
		await expect(topicos.getByText('evaporação leva a água')).toBeVisible();
		// O que está dentro de um achado aparece, mesmo sem trazer o termo…
		await expect(topicos.getByText('Uma poça que seca ao sol')).toBeVisible();
		// …e o que não tem a ver some.
		await expect(topicos.getByText('Condensação', { exact: true })).toHaveCount(0);

		// Um termo fundo abre o caminho até ele — e só até ele.
		await filtro.fill('vapor esfria');
		await expect(achados).toHaveText('1 item traz “vapor esfria”.');
		await expect(topicos.getByText('Ocorre quando o vapor esfria')).toBeVisible();
		await expect(topicos.getByText('Forma as nuvens', { exact: true })).toBeVisible();
		await expect(topicos.getByText('Não confundir com a queda da água')).toHaveCount(0);
		await expect(topicos.getByText('Evaporação', { exact: true })).toHaveCount(0);

		// O achado que tem filhos mostra o que há nele, a um clique.
		await filtro.fill('condensação');
		await expect(achados).toHaveText('1 item traz “condensação”.');
		await topicos.getByRole('button', { name: 'Condensação', exact: true }).click();
		await expect(topicos.getByText('Forma as nuvens', { exact: true })).toBeVisible();
		await expect(topicos.getByText('Não confundir com a queda da água')).toBeVisible();

		// Nada achado: a tela diz isso.
		await filtro.fill('vulcão');
		await expect(achados).toHaveText('Nenhum item traz “vulcão”.');
		await expect(topicos.getByRole('listitem')).toHaveCount(0);

		// Sem filtro, o mapa volta como estava antes da busca.
		await filtro.fill('');
		await expect(achados).toHaveCount(0);
		await expect(topicos.getByRole('button', { name: 'Evaporação', exact: true })).toHaveAttribute('aria-expanded', 'false');
		await expect(topicos.getByText('Ocorre quando o vapor esfria')).toHaveCount(0);
	});

	test('[M13] o texto sem limite é recusado: itens demais, fundo demais, linha enorme', async ({ page, conta }) => {
		const itensDemais = `# Grande\n${'- x\n'.repeat(5001)}`;
		const r1 = await importar(page.request, conta.token, itensDemais);
		expect(r1.status).toBe(422);
		expect(r1.corpo.erro).toContain('passa de 5000 itens');

		const linhaEnorme = `# Grande\n- ${'x'.repeat(501)}\n`;
		const r2 = await importar(page.request, conta.token, linhaEnorme);
		expect(r2.status).toBe(422);
		expect(r2.corpo.erro).toContain('item com 501 caracteres');

		const fundo = `# Grande\n${Array.from({ length: 11 }, (_, n) => `${'  '.repeat(n)}- n`).join('\n')}\n`;
		const r3 = await importar(page.request, conta.token, fundo);
		expect(r3.status).toBe(422);
		expect(r3.corpo.erro).toContain('fundo demais');

		// O teto no limite (5000 itens, 500 caracteres, 10 níveis) ainda passa.
		const cadeia = Array.from({ length: 10 }, (_, n) => `${'  '.repeat(n)}- ${n === 9 ? 'y'.repeat(500) : `nível ${n + 1}`}`);
		const nolimite = `# No limite\n${cadeia.join('\n')}\n${'- x\n'.repeat(4990)}`;
		const r4 = await importar(page.request, conta.token, nolimite);
		expect(r4.status).toBe(201);
		expect(r4.corpo.mapa.itens).toBe(5000);
		expect(await catalogo(page.request, conta.token)).toHaveLength(1);
	});
});

// O mapa é lido boa parte do tempo no celular e no tablet (escolha de 28/09/2026).
// O Chromium com toque emulado responde como o aparelho às regras de CSS de toque
// (pointer: coarse), então o que se mede aqui é o que o dedo encontra.
test.describe('mapas mentais no celular', () => {
	test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

	test('[M15] no celular o mapa abre como sumário, tem alvo de dedo, não dá zoom no filtro e não rola para o lado', async ({ page, api, conta }) => {
		await api.concurso('Celular E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());
		await importar(page.request, conta.token, FUNDO);
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);

		// Abre como sumário: os ramos, todos recolhidos, sem o último a várias telas de distância.
		for (const r of ESPERADO.filter((r) => r.filhos.length > 0)) {
			await expect(topicos.getByRole('button', { name: r.texto, exact: true })).toHaveAttribute('aria-expanded', 'false');
		}
		await expect(topicos.getByText('Uma poça que seca ao sol')).toHaveCount(0);

		// O alvo do toque tem altura de dedo, e o toque abre o ramo.
		const precipitacao = topicos.getByRole('button', { name: 'Precipitação', exact: true });
		expect((await precipitacao.boundingBox())!.height).toBeGreaterThanOrEqual(36);
		await precipitacao.tap();
		await expect(topicos.getByText('Granizo', { exact: true })).toBeVisible();
		expect((await topicos.getByRole('button', { name: 'Neve', exact: true }).boundingBox())!.height).toBeGreaterThanOrEqual(36);

		// Campo com fonte menor que 16px faz o iPhone dar zoom na página ao tocar nele.
		for (const campo of ['Filtrar itens do mapa', 'Vincular a uma matéria']) {
			const fonte = await page.getByLabel(campo).evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
			expect(fonte, campo).toBeGreaterThanOrEqual(16);
		}

		// Tudo aberto, a página não rola para o lado.
		await page.getByRole('button', { name: 'Abrir tudo' }).tap();
		await expect(topicos.getByText('A banca troca neve por granizo')).toBeVisible();
		expect(await semRolagemLateral(page)).toBe(true);

		// O tópico mais fundo que uma aula costuma ter ainda tem largura de leitura.
		await page.goto('/mapas/fundo');
		await page.getByRole('button', { name: 'Abrir tudo' }).tap();
		const maisFundo = page.getByText('Nível seis, o mais fundo do mapa');
		await expect(maisFundo).toBeVisible();
		expect((await maisFundo.boundingBox())!.width).toBeGreaterThanOrEqual(180);
		expect(await semRolagemLateral(page)).toBe(true);
	});
});

test.describe('mapas mentais no tablet', () => {
	test.use({ viewport: { width: 820, height: 1180 }, isMobile: true, hasTouch: true });

	test('[M15] no tablet o mapa abre recolhido, tem alvo de dedo e não rola para o lado', async ({ page, api, conta }) => {
		await api.concurso('Tablet E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);

		// Recolhido, como no computador e no celular: só os ramos à vista.
		await expect(topicos.getByRole('button', { name: 'Evaporação', exact: true })).toHaveAttribute('aria-expanded', 'false');
		await expect(topicos.getByText('Uma poça que seca ao sol')).toHaveCount(0);
		expect((await topicos.getByRole('button', { name: 'Condensação', exact: true }).boundingBox())!.height).toBeGreaterThanOrEqual(36);

		await page.getByRole('button', { name: 'Abrir tudo' }).tap();
		await expect(topicos.getByText('A banca troca neve por granizo')).toBeVisible();
		expect(await semRolagemLateral(page)).toBe(true);
	});
});

// As questões das aulas ficam fora do mapa, num arquivo à parte importado na
// página dele (e2e/fixtures/mapa-exemplo.questoes.json, sintético como o mapa).
const QUESTOES = new URL('../fixtures/mapa-exemplo.questoes.json', import.meta.url);
const questoesDoExemplo = () => JSON.parse(readFileSync(QUESTOES, 'utf-8'));
const caminhoDasQuestoes = fileURLToPath(QUESTOES);

async function importarQuestoes(request: APIRequestContext, token: string, arquivo: unknown, slug = 'ciclo-da-agua') {
	const res = await request.post(`/api/mapas/${slug}/questoes`, {
		headers: { ...cabecalho(token), 'Content-Type': 'application/json' },
		data: typeof arquivo === 'string' ? arquivo : JSON.stringify(arquivo)
	});
	return { status: res.status(), corpo: await res.json() };
}

async function responderQuestao(request: APIRequestContext, token: string, id: string, resposta: string) {
	const res = await request.post(`/api/mapas/questoes/${id}/respostas`, { headers: cabecalho(token), data: { resposta } });
	return { status: res.status(), corpo: await res.json() };
}

interface QuestaoLida {
	id: string;
	ramo: string;
	origem: string;
	banca: string;
	enunciado: string;
	alternativas: string[];
	resposta: null | { escolhida: string; acertou: boolean; gabarito: string; comentario: string; explicacoes: string[] };
}

async function questoesDo(request: APIRequestContext, token: string, slug = 'ciclo-da-agua'): Promise<QuestaoLida[]> {
	const res = await ler(request, token, slug);
	expect(res.status()).toBe(200);
	return (await res.json()).questoes;
}

/** A aba das questões, aberta, e a lista de grupos dela. */
async function listaDeQuestoes(page: Page, nome = 'Questões por conteúdo') {
	const aba = page.getByRole('tab', { name: /^Questões/ });
	if ((await aba.getAttribute('aria-selected')) !== 'true') await aba.click();
	return page.getByRole('list', { name: nome });
}

/** O exemplo importado e as questões dele, pela API. */
async function mapaComQuestoes(api: Api, page: Page, conta: Conta, nome: string) {
	await api.concurso(nome, MATERIAS);
	expect((await importar(page.request, conta.token, exemplo())).status).toBe(201);
	const r = await importarQuestoes(page.request, conta.token, questoesDoExemplo());
	expect(r.status, JSON.stringify(r.corpo)).toBe(200);
}

test.describe('questões dos mapas', () => {
	test('[M16] as questões importadas ficam fiéis, na ordem e no ramo; reimportar não duplica, não perde resposta e tira a que saiu', async ({ page, api, conta }) => {
		await api.concurso('Questões E2E', MATERIAS);
		expect((await importar(page.request, conta.token, exemplo())).status).toBe(201);

		// Pela tela, em "Manter este mapa", como na lei.
		await page.goto('/mapas/ciclo-da-agua');
		await page.getByText('Manter este mapa').click();
		await page.getByLabel('Questões do mapa (.json)').setInputFiles(caminhoDasQuestoes);
		await expect(page.getByText('4 questões novas.')).toBeVisible();

		const lidas = await questoesDo(page.request, conta.token);
		expect(lidas.map((q) => [q.enunciado.slice(0, 20), q.ramo, q.alternativas.length])).toEqual([
			['A evaporação leva a ', 'Evaporação', 5],
			['A condensação forma ', 'Condensação', 0],
			['Neve e granizo são a', 'Precipitação', 0],
			['Qual destas NÃO é um', 'Precipitação', 4]
		]);
		expect(lidas[0]).toMatchObject({ origem: 'FGV · 2024 · Sintética E2E', banca: 'FGV', alternativas: questoesDoExemplo().questoes[0].alternativas });

		// Importar de novo o mesmo arquivo não muda nada.
		const igual = await importarQuestoes(page.request, conta.token, questoesDoExemplo());
		expect(igual.corpo).toMatchObject({ novas: 0, atualizadas: 0, desativadas: 0, mantidas: 4 });
		expect(await questoesDo(page.request, conta.token)).toHaveLength(4);

		// A q1 respondida; depois o arquivo muda: q1 reescrita, q2 retirada, q5 nova.
		const [q1] = lidas;
		expect((await responderQuestao(page.request, conta.token, q1.id, 'A')).status).toBe(201);
		const arquivo = questoesDoExemplo();
		arquivo.questoes[0].enunciado = 'Ao evaporar, a água líquida passa ao estado';
		arquivo.questoes.splice(1, 1);
		arquivo.questoes.push({
			id: 'q5', ramo: 'Infiltração', origem: 'FCC · 2021 · Sintética E2E',
			enunciado: 'A infiltração abastece o lençol freático.', gabarito: 'Certo',
			comentario: 'A água que infiltra no solo recarrega os aquíferos.'
		});
		const mudou = await importarQuestoes(page.request, conta.token, arquivo);
		expect(mudou.corpo).toMatchObject({ novas: 1, atualizadas: 1, desativadas: 1, mantidas: 2 });

		const depois = await questoesDo(page.request, conta.token);
		expect(depois.map((q) => q.ramo)).toEqual(['Evaporação', 'Precipitação', 'Precipitação', 'Infiltração']);
		expect(depois.some((q) => q.enunciado.startsWith('A condensação'))).toBe(false);
		// A q1 é a mesma questão: o id e a resposta ficaram.
		expect(depois[0]).toMatchObject({ id: q1.id, enunciado: 'Ao evaporar, a água líquida passa ao estado' });
		expect(depois[0].resposta).toMatchObject({ escolhida: 'A', acertou: false });

		// A retirada volta se o arquivo a trouxer de novo, com o mesmo id.
		const volta = await importarQuestoes(page.request, conta.token, questoesDoExemplo());
		expect(volta.corpo).toMatchObject({ novas: 0, desativadas: 1 });
		expect((await questoesDo(page.request, conta.token)).map((q) => q.id)).toContain(lidas[1].id);
	});

	test('[M17] o arquivo de questões com problema é recusado inteiro, e a mensagem diz a questão e o quê', async ({ page, api, conta }) => {
		await api.concurso('Questões ruins E2E', MATERIAS);
		expect((await importar(page.request, conta.token, exemplo())).status).toBe(201);

		const com = (mexer: (a: ReturnType<typeof questoesDoExemplo>) => void) => {
			const a = questoesDoExemplo();
			mexer(a);
			return a;
		};
		const ruins: [string, unknown, RegExp][] = [
			['não é JSON', '{ questoes: ', /não é um JSON válido/],
			['de outro mapa', com((a) => (a.mapa = 'outro-mapa')), /é do mapa "outro-mapa", e esta página é do "ciclo-da-agua"/],
			['sem questões', { mapa: 'ciclo-da-agua', questoes: [] }, /nenhuma questão/],
			['ramo que o mapa não tem', com((a) => (a.questoes[0].ramo = 'Vulcanismo')), /questão q1: o ramo "Vulcanismo" não existe no mapa/],
			['gabarito fora', com((a) => (a.questoes[0].gabarito = 'F')), /questão q1: gabarito "F" fora das alternativas \(A–E\)/],
			['gabarito além das 4', com((a) => (a.questoes[3].gabarito = 'E')), /questão q4: gabarito "E" fora das alternativas \(A–D\)/],
			['Certo/Errado com alternativas', com((a) => (a.questoes[1].alternativas = ['x', 'y'])), /questão q2: Certo\/Errado não leva alternativas/],
			['gabarito de julgar errado', com((a) => (a.questoes[1].gabarito = 'Talvez')), /questão q2: gabarito "Talvez" — use Certo ou Errado/],
			['alternativa repetida', com((a) => (a.questoes[0].alternativas[2] = 'gasoso.')), /questão q1: alternativa C repete outra/],
			['uma alternativa só', com((a) => (a.questoes[0].alternativas = ['sólido.'])), /questão q1: precisa de 2 a 5 alternativas, tem 1/],
			['enunciado vazio', com((a) => (a.questoes[2].enunciado = '  ')), /questão q3: enunciado vazio/],
			['comentário vazio', com((a) => (a.questoes[2].comentario = '')), /questão q3: comentário vazio/],
			['chave repetida', com((a) => (a.questoes[3].id = 'q1')), /chave "q1" repetida/],
			['sem chave', com((a) => (a.questoes[3].id = '')), /questão 4: sem id/]
		];
		for (const [nome, arquivo, mensagem] of ruins) {
			const r = await importarQuestoes(page.request, conta.token, arquivo);
			expect(r.status, nome).toBe(422);
			expect(r.corpo.erro, nome).toMatch(mensagem);
		}

		// Vários problemas de uma vez, e nenhuma questão pela metade.
		const varios = com((a) => {
			a.questoes[0].ramo = 'Vulcanismo';
			a.questoes[2].comentario = '';
		});
		const r = await importarQuestoes(page.request, conta.token, varios);
		expect(r.corpo.erro).toContain('questão q1');
		expect(r.corpo.erro).toContain('questão q3');
		expect(await questoesDo(page.request, conta.token)).toEqual([]);

		// Pela tela, o aviso aparece e o mapa segue sem a seção de questões.
		await page.goto('/mapas/ciclo-da-agua');
		await page.getByText('Manter este mapa').click();
		await page.getByLabel('Questões do mapa (.json)').setInputFiles({
			name: 'ruim.questoes.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(varios))
		});
		await expect(page.getByRole('alert')).toContainText('o ramo "Vulcanismo" não existe no mapa');
		await expect(page.getByRole('tab', { name: /^Questões/ })).toHaveCount(0);

		// Alternativas que só diferem na caixa ou na pontuação são outras alternativas:
		// é o que uma questão de tokenização cobra.
		const tokens = com((a) => {
			a.questoes[3].alternativas = ['Chuva!', "['Chuva', '!']", 'CHUVA!', 'chuva'];
		});
		const aceita = await importarQuestoes(page.request, conta.token, tokens);
		expect(aceita.status, JSON.stringify(aceita.corpo)).toBe(200);
		expect((await questoesDo(page.request, conta.token))[3].alternativas).toEqual(tokens.questoes[3].alternativas);
	});

	test('[M18] o gabarito só vem com a resposta, a correção acerta nos dois tipos, e responder de novo conta a nova', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Responder E2E');
		const [q1, q2, q3, q4] = await questoesDo(page.request, conta.token);

		// Antes de responder, nada de gabarito nem comentário no que a tela recebe.
		const cru = await (await ler(page.request, conta.token, 'ciclo-da-agua')).text();
		expect(cru).not.toContain('Evaporar é passar do líquido');
		expect(cru).not.toContain('"gabarito"');
		expect(q1.resposta).toBeNull();

		const errada = await responderQuestao(page.request, conta.token, q1.id, 'A');
		expect(errada).toMatchObject({ status: 201, corpo: { escolhida: 'A', acertou: false, gabarito: 'B' } });
		expect(errada.corpo.comentario).toContain('Evaporar é passar do líquido');
		expect((await responderQuestao(page.request, conta.token, q2.id, 'CERTO')).corpo).toMatchObject({ acertou: true, gabarito: 'CERTO' });
		expect((await responderQuestao(page.request, conta.token, q3.id, 'CERTO')).corpo).toMatchObject({ acertou: false, gabarito: 'ERRADO' });
		expect((await responderQuestao(page.request, conta.token, q4.id, 'D')).corpo).toMatchObject({ acertou: true });

		// Resposta que não cabe na questão é recusada, e não conta.
		expect((await responderQuestao(page.request, conta.token, q4.id, 'E')).status).toBe(422);
		expect((await responderQuestao(page.request, conta.token, q2.id, 'A')).status).toBe(422);
		expect((await responderQuestao(page.request, conta.token, q1.id, 'CERTO')).status).toBe(422);

		// Gravado: ao recarregar, cada questão traz a última resposta; responder de novo conta a nova.
		expect((await responderQuestao(page.request, conta.token, q1.id, 'B')).corpo.acertou).toBe(true);
		const lidas = await questoesDo(page.request, conta.token);
		expect(lidas.map((q) => q.resposta?.acertou)).toEqual([true, true, false, true]);

		// Pela tela: escolher, responder, ver o veredito e o comentário, e responder de novo.
		await page.goto('/mapas/ciclo-da-agua');
		await (await listaDeQuestoes(page)).getByRole('button', { name: /^Precipitação/ }).click();
		const dialogo = page.getByRole('dialog', { name: 'Questões — Precipitação' });
		const neve = dialogo.getByRole('group', { name: /Neve e granizo/ });
		await expect(neve.getByText('Errou — o item está Errado')).toBeVisible();
		await neve.getByRole('button', { name: 'Responder de novo' }).click();
		await expect(neve.getByText(/A banca troca uma pela outra/)).toHaveCount(0);
		await neve.getByRole('radio', { name: 'Errado' }).check();
		await neve.getByRole('button', { name: 'Responder', exact: true }).click();
		await expect(neve.getByText('Acertou')).toBeVisible();
		await expect(neve.getByText(/A banca troca uma pela outra/)).toBeVisible();
		expect((await questoesDo(page.request, conta.token))[2].resposta).toMatchObject({ escolhida: 'ERRADO', acertou: true });
	});

	test('[M19] as questões de uma conta não aparecem, nem são importadas ou respondidas por outra', async ({ page, api, conta, baseURL }) => {
		await mapaComQuestoes(api, page, conta, 'Questões da dona E2E');
		const [q1] = await questoesDo(page.request, conta.token);

		const { request, token } = await outraSessao(baseURL!);
		expect((await importarQuestoes(request, token, questoesDoExemplo())).status).toBe(404);
		expect((await responderQuestao(request, token, q1.id, 'B')).status).toBe(404);

		// A outra conta com o mesmo mapa tem as questões dela, separadas.
		expect((await importar(request, token, exemplo())).status).toBe(201);
		expect((await importarQuestoes(request, token, questoesDoExemplo())).corpo.novas).toBe(4);
		const dela = await questoesDo(request, token);
		expect(dela.map((q) => q.id)).not.toContain(q1.id);
		expect((await responderQuestao(request, token, dela[0].id, 'B')).status).toBe(201);
		await request.dispose();

		expect((await questoesDo(page.request, conta.token))[0].resposta).toBeNull();
	});

	test('[M20] a página agrupa as questões pelo ramo, o placar anda ao responder e "Só o que errei" filtra', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Placar E2E');
		await page.goto('/mapas/ciclo-da-agua');

		const lista = await listaDeQuestoes(page);
		await expect(lista.getByRole('button')).toHaveText([
			/^Evaporação\s*1 questão/,
			/^Condensação\s*1 questão/,
			/^Precipitação\s*2 questões/
		]);

		await lista.getByRole('button', { name: /^Precipitação/ }).click();
		const dialogo = page.getByRole('dialog', { name: 'Questões — Precipitação' });
		await expect(dialogo.getByText('2 questões · 0 respondidas · 0 certas')).toBeVisible();

		const neve = dialogo.getByRole('group', { name: /Neve e granizo/ });
		await neve.getByRole('radio', { name: 'Certo' }).check();
		await neve.getByRole('button', { name: 'Responder', exact: true }).click();
		await expect(neve.getByText('Errou — o item está Errado')).toBeVisible();

		const orvalho = dialogo.getByRole('group', { name: /NÃO é uma forma/ });
		await orvalho.getByRole('radio', { name: 'D) Orvalho.' }).check();
		await orvalho.getByRole('button', { name: 'Responder', exact: true }).click();
		await expect(orvalho.getByText('Acertou')).toBeVisible();
		await expect(dialogo.getByText('2 questões · 2 respondidas · 1 certa')).toBeVisible();

		await dialogo.getByLabel('Só o que errei').check();
		await expect(dialogo.getByRole('group')).toHaveCount(1);
		await expect(dialogo.getByRole('group', { name: /Neve e granizo/ })).toBeVisible();

		await dialogo.getByRole('button', { name: 'Fechar as questões' }).click();
		await expect(lista.getByRole('button', { name: /^Precipitação/ })).toContainText('2 de 2 respondidas · 1 certa');

		// "Todas as questões" abre as quatro, e o placar da seção soma tudo.
		await page.getByRole('button', { name: 'Resolver 4' }).click();
		await expect(page.getByRole('dialog', { name: 'Questões — Todas' }).getByRole('group')).toHaveCount(4);
	});

	test('[M21] o mapa sem questões não mostra a seção, e excluir o mapa leva questões e respostas, avisando', async ({ page, api, conta }) => {
		await api.concurso('Sem questões E2E', MATERIAS);
		expect((await importar(page.request, conta.token, exemplo())).status).toBe(201);
		await page.goto('/mapas/ciclo-da-agua');
		await expect(page.getByRole('heading', { name: 'Ciclo da Água', level: 1 })).toBeVisible();
		await expect(page.getByRole('tab', { name: /^Questões/ })).toHaveCount(0);

		const r = await importarQuestoes(page.request, conta.token, questoesDoExemplo());
		expect(r.status).toBe(200);
		const [q1] = await questoesDo(page.request, conta.token);
		await responderQuestao(page.request, conta.token, q1.id, 'B');

		await page.reload();
		await page.getByText('Manter este mapa').click();
		await page.getByRole('button', { name: 'Excluir mapa' }).click();
		const confirmar = page.getByRole('alertdialog');
		await expect(confirmar).toContainText('as 4 questões e as suas respostas');
		await confirmar.getByRole('button', { name: 'Excluir mapa' }).click();
		await expect(page).toHaveURL(/\/mapas$/);

		// A questão respondida some com o mapa: nem ela nem a resposta ficam à mão.
		expect((await responderQuestao(page.request, conta.token, q1.id, 'B')).status).toBe(404);
		expect((await importar(page.request, conta.token, exemplo())).status).toBe(201);
		expect(await questoesDo(page.request, conta.token)).toEqual([]);
	});
});

test.describe('explicação por alternativa', () => {
	test('[M23] a explicação de cada alternativa fica debaixo dela: aberta a da certa ao acertar, a da marcada ao errar, e as outras a um toque', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Explicações E2E');
		const [q1, q2, , q4] = await questoesDo(page.request, conta.token);

		// O comentário escrito alternativa por alternativa chega separado, na ordem delas.
		const certa = await responderQuestao(page.request, conta.token, q1.id, 'B');
		expect(certa.corpo.comentario).toBe('Evaporar é passar do líquido ao gasoso.');
		expect(certa.corpo.explicacoes).toEqual([
			'Errada. Passar ao sólido é solidificar.',
			"Correta. O vapor d'água é a água no estado gasoso.",
			'Errada. Plasma pede temperatura de estrela, não de chuva.',
			'Errada. Líquido superaquecido continua líquido.',
			'Errada. Cristalino é o gelo, de novo sólido.'
		]);
		// O que não separa (prosa, ou Certo/Errado) vem inteiro, sem explicação por alternativa.
		const prosa = await responderQuestao(page.request, conta.token, q4.id, 'A');
		expect(prosa.corpo).toMatchObject({ comentario: questoesDoExemplo().questoes[3].comentario, explicacoes: [] });
		expect((await responderQuestao(page.request, conta.token, q2.id, 'CERTO')).corpo.explicacoes).toEqual([]);

		// Pela tela, ao acertar: a explicação da certa aberta, as outras a um toque.
		await page.goto('/mapas/ciclo-da-agua');
		await (await listaDeQuestoes(page)).getByRole('button', { name: /^Evaporação/ }).click();
		const dialogo = page.getByRole('dialog', { name: 'Questões — Evaporação' });
		const questao = dialogo.getByRole('group', { name: /A evaporação leva/ });
		const opcao = (letra: string) => questao.locator('.opcao').filter({ has: page.getByRole('radio', { name: new RegExp(`^${letra}\\)`) }) });

		await expect(questao.getByText('Acertou')).toBeVisible();
		await expect(questao.getByText('Evaporar é passar do líquido ao gasoso.')).toBeVisible();
		await expect(opcao('B').getByText("Correta. O vapor d'água é a água no estado gasoso.")).toBeVisible();
		await expect(questao.getByText(/^Errada\./)).toHaveCount(0);
		await expect(questao.getByRole('button', { name: /^Ver explicação/ })).toHaveCount(4);
		await opcao('C').getByRole('button', { name: 'Ver explicação da C' }).click();
		await expect(opcao('C').getByText('Errada. Plasma pede temperatura de estrela, não de chuva.')).toBeVisible();
		await opcao('C').getByRole('button', { name: 'Ocultar explicação da C' }).click();
		await expect(questao.getByText(/^Errada\./)).toHaveCount(0);

		// Ao errar: a certa destacada, a explicação da marcada aberta, e a da certa a um toque.
		await questao.getByRole('button', { name: 'Responder de novo' }).click();
		await expect(questao.getByText(/^Correta\./)).toHaveCount(0);
		await questao.getByRole('radio', { name: 'A) sólido.' }).check();
		await questao.getByRole('button', { name: 'Responder', exact: true }).click();
		await expect(questao.getByText('Errou — gabarito B')).toBeVisible();
		await expect(opcao('B')).toHaveClass(/gabarito/);
		await expect(opcao('A')).toHaveClass(/marcada/);
		await expect(opcao('A').getByText('Errada. Passar ao sólido é solidificar.')).toBeVisible();
		await expect(questao.getByText(/^Correta\./)).toHaveCount(0);
		await opcao('B').getByRole('button', { name: 'Ver explicação da B' }).click();
		await expect(opcao('B').getByText("Correta. O vapor d'água é a água no estado gasoso.")).toBeVisible();

		// Recarregado, a resposta gravada traz as explicações do mesmo jeito.
		await page.reload();
		await (await listaDeQuestoes(page)).getByRole('button', { name: /^Evaporação/ }).click();
		await expect(opcao('A').getByText('Errada. Passar ao sólido é solidificar.')).toBeVisible();

		// A questão em prosa mostra o comentário inteiro e nenhum botão.
		await dialogo.getByRole('button', { name: 'Fechar as questões' }).click();
		await (await listaDeQuestoes(page)).getByRole('button', { name: /^Precipitação/ }).click();
		const orvalho = page.getByRole('dialog', { name: 'Questões — Precipitação' }).getByRole('group', { name: /NÃO é uma forma/ });
		await expect(orvalho.getByText(/O orvalho se forma na superfície/)).toBeVisible();
		await expect(orvalho.getByRole('button', { name: /explicação/ })).toHaveCount(0);
	});
});

test.describe('questões dos mapas por banca', () => {
	test('[M22] as questões se agrupam pela banca da origem, com placar, e o diálogo da banca só traz as dela', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Bancas E2E');

		// A banca é o começo da origem, sem o "(CESPE)" e sem o "ADAPTADA -".
		expect((await questoesDo(page.request, conta.token)).map((q) => q.banca)).toEqual(['FGV', 'CEBRASPE', 'FGV', 'CEBRASPE']);

		await page.goto('/mapas/ciclo-da-agua');
		await listaDeQuestoes(page);
		await page.getByLabel('Ver por').selectOption('banca');
		const lista = page.getByRole('list', { name: 'Questões por banca' });
		await expect(lista.getByRole('button')).toHaveText([/^CEBRASPE\s*2 questões/, /^FGV\s*2 questões/]);

		await lista.getByRole('button', { name: /^FGV/ }).click();
		const dialogo = page.getByRole('dialog', { name: 'Questões — FGV' });
		await expect(dialogo.getByRole('group')).toHaveCount(2);
		await expect(dialogo.getByRole('group', { name: /A evaporação leva/ })).toBeVisible();
		await expect(dialogo.getByRole('group', { name: /Neve e granizo/ })).toBeVisible();

		const neve = dialogo.getByRole('group', { name: /Neve e granizo/ });
		await neve.getByRole('radio', { name: 'Errado' }).check();
		await neve.getByRole('button', { name: 'Responder', exact: true }).click();
		await expect(neve.getByText('Acertou')).toBeVisible();
		await dialogo.getByRole('button', { name: 'Fechar as questões' }).click();
		await expect(lista.getByRole('button', { name: /^FGV/ })).toContainText('1 de 2 respondidas · 1 certa');
		await expect(lista.getByRole('button', { name: /^CEBRASPE/ })).not.toContainText('respondidas');

		// Voltar a ver por conteúdo mostra os ramos de novo.
		await page.getByLabel('Ver por').selectOption('ramo');
		await expect(page.getByRole('list', { name: 'Questões por conteúdo' })).toBeVisible();
	});
});

test.describe('edição do mapa e aba de questões', () => {
	test('[M24] no modo de edição a lixeira tira o tópico e o que há dentro dele, "Desfazer" o devolve, e a exclusão fica gravada', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Edição E2E');
		let exclusoes = 0;
		page.on('request', (r) => {
			if (r.url().includes('/itens/excluir')) exclusoes++;
		});

		await page.goto('/mapas/ciclo-da-agua');
		const topicos = page.getByRole('list', { name: 'Tópicos do mapa' });
		const filhosDaPrecipitacao = topicos.getByText(/^(Chuva|Neve|Granizo)$/);

		// Fora do modo de edição não há lixeira: nada se apaga por acidente.
		await page.getByRole('button', { name: 'Abrir tudo' }).click();
		await expect(page.getByRole('button', { name: /^Excluir “/ })).toHaveCount(0);
		await page.getByRole('button', { name: 'Editar' }).click();
		await expect(page.getByRole('button', { name: 'Excluir “Chuva”' })).toBeVisible();

		// Um tópico sem nada dentro sai sem pergunta; "Desfazer" o devolve ao
		// mesmo lugar, e nada vai ao servidor.
		await page.getByRole('button', { name: 'Excluir “Chuva”' }).click();
		await expect(filhosDaPrecipitacao).toHaveText(['Neve', 'Granizo']);
		await page.getByRole('status').getByRole('button', { name: 'Desfazer' }).click();
		await expect(filhosDaPrecipitacao).toHaveText(['Chuva', 'Neve', 'Granizo']);
		await page.waitForTimeout(6500);
		expect(exclusoes).toBe(0);

		// O que tem conteúdo dentro pergunta antes, dizendo quanto vai junto.
		await page.getByRole('button', { name: 'Excluir “Neve”' }).click();
		const confirmacao = page.getByRole('alertdialog').or(page.getByRole('dialog'));
		await expect(confirmacao).toContainText('Sai junto o item que há dentro dele');
		const gravou = page.waitForResponse((r) => r.url().includes('/itens/excluir'), { timeout: 15_000 });
		await confirmacao.getByRole('button', { name: 'Excluir' }).click();
		await expect(filhosDaPrecipitacao).toHaveText(['Chuva', 'Granizo']);
		expect((await gravou).status()).toBe(200);

		// Gravado: recarregar não traz de volta, e só ele saiu.
		await page.reload();
		await page.getByRole('button', { name: 'Abrir tudo' }).click();
		await expect(filhosDaPrecipitacao).toHaveText(['Chuva', 'Granizo']);
		await expect(topicos.getByText('A banca troca neve por granizo')).toHaveCount(0);
		await expect(topicos.getByText('Uma poça que seca ao sol')).toBeVisible();

		// O ramo leva as questões dele junto, e avisa. Sair da página logo depois
		// não perde a exclusão.
		await page.getByRole('button', { name: 'Editar' }).click();
		await page.getByRole('button', { name: 'Excluir “Precipitação”' }).click();
		await expect(confirmacao).toContainText('as 2 questões do ramo (as respostas ficam guardadas)');
		await confirmacao.getByRole('button', { name: 'Excluir' }).click();
		await expect(topicos.getByText('Precipitação', { exact: true })).toHaveCount(0);
		const saiu = page.waitForResponse((r) => r.url().includes('/itens/excluir'));
		await page.getByRole('link', { name: '← Mapas mentais' }).click();
		expect((await saiu).status()).toBe(200);

		const lido = await (await ler(page.request, conta.token, 'ciclo-da-agua')).json();
		expect(lido.arvore.map((r: { texto: string }) => r.texto)).toEqual(['Evaporação', 'Condensação', 'Infiltração']);
		expect(lido.questoes.map((q: QuestaoLida) => q.ramo)).toEqual(['Evaporação', 'Condensação']);
	});

	test('[M25] as questões ficam numa aba, por conteúdo ou por banca, com filtro, e o ramo do mapa leva às dele', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Aba de questões E2E');
		const [q1, q2] = await questoesDo(page.request, conta.token);
		await responderQuestao(page.request, conta.token, q1.id, 'A');
		await responderQuestao(page.request, conta.token, q2.id, 'CERTO');

		await page.goto('/mapas/ciclo-da-agua');
		const topicos = page.getByRole('list', { name: 'Tópicos do mapa' });

		// O ramo diz quantas questões tem e abre as dele.
		await topicos.getByRole('button', { name: 'Resolver as questões de Precipitação' }).click();
		const doRamo = page.getByRole('dialog', { name: 'Questões — Precipitação' });
		await expect(doRamo.getByRole('group')).toHaveCount(2);
		await doRamo.getByRole('button', { name: 'Fechar as questões' }).click();
		await expect(topicos.getByRole('button', { name: /^Resolver as questões de Infiltração/ })).toHaveCount(0);

		// A aba das questões fica no endereço, e o mapa sai da frente.
		await page.getByRole('tab', { name: /^Questões/ }).click();
		await expect(page).toHaveURL(/aba=questoes/);
		await expect(topicos).toBeHidden();
		await expect(page.getByText('4 questões · 2 respondidas · 1 certa · 50% de acerto')).toBeVisible();

		const porConteudo = page.getByRole('list', { name: 'Questões por conteúdo' });
		await page.getByLabel('Mostrar').selectOption('erradas');
		await expect(porConteudo.getByRole('button')).toHaveText([/^Evaporação\s*1 questão/]);
		await page.getByRole('button', { name: 'Resolver 1' }).click();
		const erradas = page.getByRole('dialog', { name: 'Questões — Que errei' });
		await expect(erradas.getByRole('group')).toHaveCount(1);
		await expect(erradas.getByRole('group', { name: /A evaporação leva/ })).toBeVisible();
		await erradas.getByRole('button', { name: 'Fechar as questões' }).click();

		await page.getByLabel('Mostrar').selectOption('sem-resposta');
		await expect(porConteudo.getByRole('button')).toHaveText([/^Precipitação\s*2 questões/]);
		await page.getByLabel('Ver por').selectOption('banca');
		await expect(page.getByRole('list', { name: 'Questões por banca' }).getByRole('button')).toHaveText([
			/^CEBRASPE\s*1 questão/,
			/^FGV\s*1 questão/
		]);

		// Recarregar volta à mesma aba, com as mesmas escolhas.
		await page.reload();
		await expect(page.getByRole('tab', { name: /^Questões/ })).toHaveAttribute('aria-selected', 'true');
		await expect(page.getByLabel('Ver por')).toHaveValue('banca');
		await expect(page.getByLabel('Mostrar')).toHaveValue('sem-resposta');
	});
});

test.describe('questões dos mapas no celular', () => {
	test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

	test('[M20] no celular o diálogo das questões cabe na tela e cada alternativa tem alvo de dedo', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Questões no celular E2E');
		await page.goto('/mapas/ciclo-da-agua');
		await (await listaDeQuestoes(page)).getByRole('button', { name: /^Evaporação/ }).tap();

		const dialogo = page.getByRole('dialog', { name: 'Questões — Evaporação' });
		await expect(dialogo).toBeVisible();
		const caixa = (await dialogo.boundingBox())!;
		expect(caixa.x).toBeGreaterThanOrEqual(0);
		expect(caixa.x + caixa.width).toBeLessThanOrEqual(390);

		const gasoso = dialogo.getByRole('radio', { name: 'B) gasoso.' });
		const alvo = (await dialogo.locator('label').filter({ hasText: 'B) gasoso.' }).boundingBox())!;
		expect(alvo.height).toBeGreaterThanOrEqual(40);
		await gasoso.tap();
		await dialogo.getByRole('button', { name: 'Responder', exact: true }).tap();
		await expect(dialogo.getByText('Acertou')).toBeVisible();
		expect(await semRolagemLateral(page)).toBe(true);
	});

	test('[M25][M24] no celular a aba das questões está na primeira tela, e a lixeira tem alvo de dedo', async ({ page, api, conta }) => {
		await mapaComQuestoes(api, page, conta, 'Edição no celular E2E');
		await page.goto('/mapas/ciclo-da-agua');

		const aba = (await page.getByRole('tab', { name: /^Questões/ }).boundingBox())!;
		expect(aba.y + aba.height).toBeLessThanOrEqual(844);
		expect(aba.height).toBeGreaterThanOrEqual(40);

		await page.getByRole('button', { name: 'Editar' }).tap();
		const lixeira = (await page.getByRole('button', { name: 'Excluir “Evaporação”' }).boundingBox())!;
		expect(lixeira.height).toBeGreaterThanOrEqual(40);
		expect(lixeira.width).toBeGreaterThanOrEqual(40);
		expect(await semRolagemLateral(page)).toBe(true);
	});
});

test.describe('imagens dos mapas', () => {
	// Duas imagens de verdade, pequenas e de larguras diferentes: a troca se vê
	// pela largura da que a tela mostra.
	const PNG_1x1 = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNwaDgAAAKEAYEml6crAAAAAElFTkSuQmCC', 'base64');
	const PNG_2x1 = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAIAAAB7QOjdAAAADUlEQVR4nGNwaDgARAAJhwMBFKhZ+wAAAABJRU5ErkJggg==', 'base64');

	/** O exemplo com uma imagem na Evaporação e outra, que nunca chega, na Condensação. */
	const comImagens = (texto: string) =>
		texto
			.replace('  - [ex] Uma poça que seca ao sol', '  - [ex] Uma poça que seca ao sol\n  - ![Fluxo da evaporação](fluxo.png)')
			.replace('  - Forma as nuvens', '  - ![Nuvem que falta](nuvem.png)\n  - Forma as nuvens');

	const enviar = (request: APIRequestContext, token: string, slug: string, arquivo: { name: string; mimeType: string; buffer: Buffer }) =>
		request.post(`/api/mapas/${slug}/imagens`, { headers: cabecalho(token), multipart: { imagens: arquivo } });

	// A política de segurança que a borda do servidor manda (ansible), aplicada
	// aqui a cada página: o stack do E2E não tem a borda, e a imagem que ela
	// bloqueia passaria no teste e ficaria em branco no ar.
	const CSP_DA_BORDA = /Content-Security-Policy "([^"]+)"/.exec(
		readFileSync(new URL('../../ansible/templates/app.conf.j2', import.meta.url), 'utf-8')
	)![1];

	// Aplicada como <meta>, e não reescrevendo o cabeçalho da resposta: a
	// página servida pelo route.fulfill não carregava os próprios módulos JS
	// (ERR_FAILED) quando o app vinha de http://frontend:5173, como na pipeline.
	// A política vale para tudo o que carrega depois de inserida, e as imagens
	// do mapa só carregam ao abrir o ramo.
	const comACspDaBorda = (page: Page) =>
		page.addInitScript((csp) => {
			document.addEventListener('DOMContentLoaded', () => {
				const meta = document.createElement('meta');
				meta.httpEquiv = 'Content-Security-Policy';
				meta.content = csp;
				document.head.prepend(meta);
			});
		}, CSP_DA_BORDA);

	const larguraDa = (page: Page, legenda: string) =>
		topicosDo(page).getByRole('img', { name: legenda }).evaluate((img: HTMLImageElement) => img.naturalWidth);

	test('[M26] a imagem citada aparece no tópico dela depois de enviada, a que falta avisa, o filtro acha pela legenda e enviar de novo troca', async ({ page, api, conta, baseURL }) => {
		await api.concurso('Imagens E2E', MATERIAS);
		expect((await importar(page.request, conta.token, comImagens(exemplo()))).status).toBe(201);
		// Outro mapa da mesma conta cita o mesmo nome de arquivo: a imagem é do mapa.
		expect((await importar(page.request, conta.token, variante(comImagens(exemplo()), 'b'))).status).toBe(201);

		await comACspDaBorda(page);
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);

		// Antes do envio, o tópico diz o que falta — e a página segue de pé.
		await topicos.getByRole('button', { name: /^Evaporação/ }).click();
		await expect(topicos.getByText('Imagem não enviada: fluxo.png')).toBeVisible();
		await expect(topicos.getByText('Fluxo da evaporação', { exact: true })).toBeVisible();
		await page.getByText('Manter este mapa').click();
		await expect(page.getByText('Faltam 2 de 2: fluxo.png, nuvem.png.')).toBeVisible();

		// Pela tela, em "Manter este mapa".
		await page.getByLabel('Imagens do mapa').setInputFiles({ name: 'fluxo.png', mimeType: 'image/png', buffer: PNG_1x1 });
		await expect(page.getByText('1 imagem enviada.')).toBeVisible();
		await expect(page.getByText('Faltam 1 de 2: nuvem.png.')).toBeVisible();
		await expect(topicos.getByRole('img', { name: 'Fluxo da evaporação' })).toBeVisible();
		expect(await larguraDa(page, 'Fluxo da evaporação')).toBe(1);

		// Tocar na imagem a abre em tela cheia, na própria página; tocar de novo fecha.
		await topicos.getByRole('button', { name: 'Ampliar: Fluxo da evaporação' }).click();
		const telaCheia = page.getByRole('dialog', { name: 'Fluxo da evaporação' });
		await expect(telaCheia.getByRole('img', { name: 'Fluxo da evaporação' })).toBeVisible();
		await telaCheia.click();
		await expect(telaCheia).toBeHidden();

		// No tópico dela, e não no vizinho: a da Condensação continua faltando.
		const evaporacao = topicos.getByRole('listitem').filter({ has: page.getByRole('button', { name: /^Evaporação/ }) });
		await expect(evaporacao.getByRole('img', { name: 'Fluxo da evaporação' })).toBeVisible();
		await topicos.getByRole('button', { name: /^Condensação/ }).click();
		await expect(topicos.getByText('Imagem não enviada: nuvem.png')).toBeVisible();

		// O filtro acha o tópico pela legenda, sem acento.
		await page.getByLabel('Filtrar itens do mapa').fill('fluxo da evaporacao');
		await expect(page.locator('main').getByRole('status').first()).toHaveText('1 item traz “fluxo da evaporacao”.');
		await page.getByLabel('Filtrar itens do mapa').fill('');

		// O arquivo que não é imagem é recusado com o motivo, e nada muda.
		await page.getByLabel('Imagens do mapa').setInputFiles({ name: 'nuvem.png', mimeType: 'image/png', buffer: Buffer.from('<svg onload="alert(1)"/>') });
		await expect(page.getByRole('alert')).toContainText('nuvem.png: não é uma imagem PNG, JPEG ou WebP');
		await expect(page.getByText('Faltam 1 de 2: nuvem.png.')).toBeVisible();

		// Enviar de novo o mesmo nome troca a imagem, sem duplicar.
		await page.getByLabel('Imagens do mapa').setInputFiles({ name: 'fluxo.png', mimeType: 'image/png', buffer: PNG_2x1 });
		await expect(page.getByText('1 imagem enviada.')).toBeVisible();
		await expect.poll(() => larguraDa(page, 'Fluxo da evaporação')).toBe(2);
		expect((await (await ler(page.request, conta.token, 'ciclo-da-agua')).json()).imagens).toEqual(['fluxo.png']);

		// O PNG com nome de JPEG e o arquivo grande demais também ficam de fora.
		const jpgFalso = await enviar(page.request, conta.token, 'ciclo-da-agua', { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: PNG_1x1 });
		expect(jpgFalso.status()).toBe(422);
		expect((await jpgFalso.json()).erro).toContain('o conteúdo é png, mas a extensão diz outra coisa');
		const grande = Buffer.concat([PNG_1x1, Buffer.alloc(2 << 20)]);
		const enorme = await enviar(page.request, conta.token, 'ciclo-da-agua', { name: 'grande.png', mimeType: 'image/png', buffer: grande });
		expect(enorme.status()).toBe(422);
		expect((await enorme.json()).erro).toContain('grande.png: maior que 2 MiB');

		// O outro mapa, que cita o mesmo nome, não a mostra…
		await page.goto('/mapas/ciclo-da-agua-b');
		await topicosDo(page).getByRole('button', { name: /^Evaporação/ }).click();
		await expect(topicosDo(page).getByText('Imagem não enviada: fluxo.png')).toBeVisible();

		// …e outra conta não a alcança.
		const outra = await outraSessao(baseURL!);
		const alheia = await outra.request.get('/api/mapas/ciclo-da-agua/imagens/fluxo.png', { headers: cabecalho(outra.token) });
		expect(alheia.status()).toBe(404);
		const envioAlheio = await enviar(outra.request, outra.token, 'ciclo-da-agua', { name: 'fluxo.png', mimeType: 'image/png', buffer: PNG_1x1 });
		expect(envioAlheio.status()).toBe(404);
		await outra.request.dispose();
	});
});
