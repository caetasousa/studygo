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
	mapas: { slug: string }[];
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

		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		await linhaDa(page, 'GEO').first().getByRole('link', { name: 'Mapas mentais de Geografia Física (2)' }).click();

		await expect(page).toHaveURL(/\/mapas\?materia=GEO$/);
		const geo = page.getByRole('region', { name: 'Geografia Física' });
		await expect(geo.getByRole('link', { name: /^Ciclo da Água\b(?! revisao)/ })).toBeVisible();
		await expect(geo.getByRole('link', { name: /^Ciclo da Água revisao/ })).toBeVisible();

		// Pelo diálogo da matéria, os dois também.
		await page.goto('/cronograma');
		await linhaDa(page, 'GEO').first().getByRole('button', { name: /Ver o conteúdo programático da matéria/ }).click();
		const dialogo = page.getByRole('dialog');
		await expect(dialogo.getByRole('link')).toHaveCount(2);

		// A matéria sem mapa não tem lista.
		await page.goto('/mapas?materia=DIR');
		await expect(page.getByText('Nenhum mapa vinculado a DIR.')).toBeVisible();
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

		// Numa tela de computador os ramos saem abertos, e o que está abaixo deles, recolhido.
		await expect(topicos.getByRole('button', { name: 'Evaporação', exact: true })).toHaveAttribute('aria-expanded', 'true');
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
		await expect(topicos.getByRole('button', { name: 'Evaporação', exact: true })).toHaveAttribute('aria-expanded', 'true');
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

	test('[M15] no tablet o mapa abre com os ramos à vista, tem alvo de dedo e não rola para o lado', async ({ page, api, conta }) => {
		await api.concurso('Tablet E2E', MATERIAS);
		await importar(page.request, conta.token, exemplo());
		await page.goto('/mapas/ciclo-da-agua');
		const topicos = topicosDo(page);

		// Com espaço, os ramos saem abertos, como no computador.
		await expect(topicos.getByRole('button', { name: 'Evaporação', exact: true })).toHaveAttribute('aria-expanded', 'true');
		await expect(topicos.getByText('Uma poça que seca ao sol')).toBeVisible();
		expect((await topicos.getByRole('button', { name: 'Condensação', exact: true }).boundingBox())!.height).toBeGreaterThanOrEqual(36);

		await page.getByRole('button', { name: 'Abrir tudo' }).tap();
		await expect(topicos.getByText('A banca troca neve por granizo')).toBeVisible();
		expect(await semRolagemLateral(page)).toBe(true);
	});
});
