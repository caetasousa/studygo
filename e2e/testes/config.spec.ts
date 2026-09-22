import { readFile } from 'node:fs/promises';
import { test, expect, abrirHoje, materiasDoDia, registrar, cadastrar, emailUnico, Api, type Page } from './base';

async function abrirConfig(page: Page) {
	await page.goto('/config');
	await expect(page.getByRole('heading', { name: 'Configurações', level: 1 })).toBeVisible();
}

test.describe('configurações e dados', () => {
	test('[D1] o tema é aplicado e fica gravado na conta', async ({ page, api }) => {
		await api.concurso('Tema E2E');
		await abrirConfig(page);
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

		const gravou = page.waitForResponse((r) => r.url().endsWith('/api/me/tema') && r.request().method() === 'PUT');
		await page.getByRole('group', { name: 'Tema da interface' }).getByRole('button', { name: 'Claro' }).click();
		expect((await gravou).ok()).toBeTruthy();
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

		await page.reload();
		await expect(page.getByRole('heading', { name: 'Configurações', level: 1 })).toBeVisible();
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
		await expect(page.getByRole('group', { name: 'Tema da interface' }).getByRole('button', { name: 'Claro' })).toHaveAttribute('aria-pressed', 'true');
	});

	test('[D2] mudar os blocos por dia refaz o cronograma sem perder o estudo', async ({ page, api }) => {
		const slug = await api.concurso('Blocos E2E');
		await abrirHoje(page);
		const [a] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 60, questoes: 10, acertos: 8, concluir: true });
		const antes = await api.plano(slug);

		await abrirConfig(page);
		await page.getByRole('group', { name: 'Blocos por dia' }).getByRole('button', { name: '3', exact: true }).click();
		await expect(page.getByRole('status').filter({ hasText: 'h por dia' })).toContainText('3,3 h');

		// Hoje já tem estudo e fica como está; o cronograma se refaz dali para a
		// frente, com três matérias por dia.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		const amanha = page
			.locator('main')
			.getByRole('list')
			.filter({ has: page.getByRole('button', { name: /^Registrar estudo de / }) })
			.nth(1);
		await expect(amanha.getByRole('button', { name: /^Registrar estudo de / })).toHaveCount(3);
		const depois = await api.plano(slug);
		expect(depois.props.horasTotal, 'refazer o cronograma apagou o estudo').toBe(antes.props.horasTotal);
		expect(depois.props.horasTotal).toBeGreaterThan(0);
	});

	test('[D3] a planilha exportada traz o estudo de volta em outra conta', async ({ page, api, browser }, info) => {
		const origem = await api.concurso('Planilha E2E');
		await abrirHoje(page);
		const [a, b] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 45, questoes: 12, acertos: 9, concluir: true });
		await registrar(page, b, { minutos: 30, questoes: 8, acertos: 4, concluir: true });
		const exportado = (await api.plano(origem)).props;

		await abrirConfig(page);
		const [baixado] = await Promise.all([
			page.waitForEvent('download'),
			page.getByRole('button', { name: '⬇ Exportar CSV' }).click()
		]);
		const csv = await readFile(await baixado.path());
		expect(csv.toString()).toContain(a);

		// A outra conta: outro navegador, outro cliente, o mesmo concurso.
		const outro = await browser.newContext({
			baseURL: info.project.use.baseURL,
			locale: 'pt-BR',
			timezoneId: 'America/Sao_Paulo',
			extraHTTPHeaders: { 'X-Forwarded-For': '10.250.0.1' }
		});
		const pagina = await outro.newPage();
		const conta = await cadastrar(outro.request, emailUnico('d3'));
		const slug = await new Api(outro.request, conta.token).concurso('Planilha E2E');

		await abrirConfig(pagina);
		await pagina.locator('input[type="file"][accept*="csv"]').setInputFiles({
			name: 'plano.csv',
			mimeType: 'text/csv',
			buffer: csv
		});
		const importar = pagina.getByRole('button', { name: 'Importar 2 linhas' });
		await expect(importar).toBeEnabled();
		await importar.click();
		await pagina.getByRole('alertdialog', { name: 'Importar 2 linhas?' }).getByRole('button', { name: 'Importar' }).click();
		await expect(pagina.getByRole('button', { name: 'Importar outra' })).toBeVisible();

		const plano = await new Api(outro.request, conta.token).plano(slug);
		expect(plano.props.horasTotal).toBe(exportado.horasTotal);
		expect(plano.props.acertoPct).toBe(exportado.acertoPct);
		expect(plano.props.horasTotal).toBeGreaterThan(0);
		await abrirHoje(pagina);
		await expect(pagina.getByText('Dia concluído')).toBeVisible();
		await outro.close();
	});

	test('[D4] limpar registros pede confirmação e zera o progresso', async ({ page, api }) => {
		const slug = await api.concurso('Limpar E2E');
		await abrirHoje(page);
		const [a] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 60, concluir: true });

		await abrirConfig(page);
		await page.getByRole('button', { name: 'Limpar registros' }).click();
		const confirmacao = page.getByRole('alertdialog', { name: 'Limpar todos os registros?' });
		await confirmacao.getByRole('button', { name: 'Cancelar' }).click();
		expect((await api.plano(slug)).props.horasTotal).toBeGreaterThan(0);

		await page.getByRole('button', { name: 'Limpar registros' }).click();
		await confirmacao.getByRole('button', { name: 'Limpar registros' }).click();
		await expect(confirmacao).toBeHidden();
		await expect.poll(async () => (await api.plano(slug)).props.horasTotal).toBe(0);

		await abrirHoje(page);
		await expect(page.getByText(/Horas\s*0,0/)).toBeVisible();
	});

	test('[D5] restaurar a ordem automática desfaz a troca manual', async ({ page, api }) => {
		const slug = await api.concurso('Ordem E2E');
		const original = (await api.plano(slug)).dias.map((d: { itens: { disciplina: string }[] }) => d.itens.map((i) => i.disciplina).join(','));

		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		const amanha = page
			.locator('main')
			.getByRole('list')
			.filter({ has: page.getByRole('button', { name: /^Registrar estudo de / }) })
			.nth(1);
		const [a] = await materiasDoDia(page, amanha);
		expect(a).toBeTruthy();
		await amanha.getByRole('button', { name: `Descer ${a} uma posição` }).click();
		await expect.poll(async () => (await api.plano(slug)).dias.map((d: { itens: { disciplina: string }[] }) => d.itens.map((i) => i.disciplina).join(','))).not.toEqual(original);

		await abrirConfig(page);
		await page.getByRole('button', { name: '↺ Restaurar ordem automática' }).click();
		const confirmacao = page.getByRole('alertdialog', { name: 'Restaurar a ordem automática?' });
		await confirmacao.getByRole('button', { name: /restaurar/i }).click();
		await expect(confirmacao).toBeHidden();

		await expect.poll(async () => (await api.plano(slug)).dias.map((d: { itens: { disciplina: string }[] }) => d.itens.map((i) => i.disciplina).join(','))).toEqual(original);
		await expect(page.getByText('Nenhuma troca manual ainda.')).toBeVisible();
	});

	test('[D6] o dossiê do NotebookLM leva a ementa e as leis cadastradas', async ({ page, api }) => {
		await api.concurso('Dossiê E2E', [
			{
				nome: 'Direito Administrativo',
				bloco: 'esp',
				questoes: 20,
				temas: ['Atos administrativos', 'Licitações'],
				fontes: [{ titulo: 'Lei 14.133/2021', url: 'https://www.planalto.gov.br/ccivil_03/_ato2019-2022/2021/lei/l14133.htm', tipo: 'lei' }]
			},
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 10 }
		]);
		await page.goto('/caderno');
		await page.getByLabel('Disciplina').selectOption({ label: 'Direito Administrativo' });
		await page.getByRole('button', { name: 'Preparar dossiê' }).click();

		// O dossiê abre na própria página, com o texto pronto para colar.
		const dossie = page.locator('main').filter({ hasText: 'Dossiê para o NotebookLM' });
		await expect(page.getByText('Dossiê para o NotebookLM · Direito Administrativo')).toBeVisible();
		for (const trecho of ['Direito Administrativo', 'Atos administrativos', 'Licitações', 'Lei 14.133/2021']) {
			await expect(dossie).toContainText(trecho);
		}
	});
});
