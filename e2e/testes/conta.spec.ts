import { test, expect, cadastrar, emailUnico, SENHA } from './base';

test.describe('conta e sessão', () => {
	test('[A1] cadastro cria a conta e leva ao primeiro concurso', async ({ page }) => {
		await page.goto('/registro');
		await page.getByLabel('Nome').fill('Ana E2E');
		await page.getByLabel('Email').fill(emailUnico('a1'));
		await page.getByLabel('Senha').fill(SENHA);
		await page.getByRole('button', { name: 'Criar conta' }).click();

		await expect(page).toHaveURL(/\/concursos\/novo$/);
		await expect(page.getByRole('heading', { name: 'Cadastrar concurso' })).toBeVisible();
		await expect(page.getByRole('complementary', { name: 'Navegação principal' })).toContainText('Ana E2E');
	});

	test('[A2] e-mail já cadastrado é recusado', async ({ page }) => {
		const email = emailUnico('a2');
		await cadastrar(page.context().request, email);
		await page.context().clearCookies();

		await page.goto('/registro');
		await page.getByLabel('Nome').fill('Outra Pessoa');
		await page.getByLabel('Email').fill(email);
		await page.getByLabel('Senha').fill(SENHA);
		await page.getByRole('button', { name: 'Criar conta' }).click();

		await expect(page.getByText(/e-?mail.*(em uso|cadastrado)/i)).toBeVisible();
		await expect(page).toHaveURL(/\/registro$/);
	});

	test('[A3] senha errada não entra e diz o motivo', async ({ page }) => {
		const email = emailUnico('a3');
		await cadastrar(page.context().request, email);
		await page.context().clearCookies();

		await page.goto('/login');
		await page.getByLabel('Email').fill(email);
		await page.getByLabel('Senha').fill('senha-errada-mas-longa');
		await page.getByRole('button', { name: 'Entrar' }).click();

		await expect(page.getByText(/credenciais|senha/i).first()).toBeVisible();
		await expect(page).toHaveURL(/\/login$/);

		await page.getByLabel('Senha').fill(SENHA);
		await page.getByRole('button', { name: 'Entrar' }).click();
		await expect(page).not.toHaveURL(/\/login$/);
	});

	test('[A4] recarregar a página mantém a sessão', async ({ page, conta }) => {
		await page.goto('/concursos');
		await expect(page.getByRole('heading', { name: 'Meus concursos' })).toBeVisible();

		await page.reload();
		await expect(page.getByRole('heading', { name: 'Meus concursos' })).toBeVisible();
		await expect(page).toHaveURL(/\/concursos$/);
		await expect(page.getByRole('complementary', { name: 'Navegação principal' })).toContainText(conta.nome);
	});

	test('[A5] rota interna sem sessão vai para o login', async ({ page }) => {
		await page.goto('/cronograma');
		await expect(page).toHaveURL(/\/login$/);
		await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible();
	});

	test('[A6] sair encerra a sessão no servidor', async ({ page, conta }) => {
		await page.goto('/concursos');
		await expect(page.getByRole('heading', { name: 'Meus concursos' })).toBeVisible();

		// O cookie de refresh antes de sair: depois do logout ele não pode mais
		// abrir sessão, nem reapresentado à mão.
		const antes = (await page.context().cookies()).find((c) => c.name === 'studygo_refresh');
		expect(antes, 'a sessão devia estar no cookie studygo_refresh').toBeTruthy();

		await page.getByRole('button', { name: 'Sair' }).click();
		await expect(page).toHaveURL(/\/login$/);

		await page.goto('/concursos');
		await expect(page).toHaveURL(/\/login$/);

		const reuso = await page.context().request.post('/api/auth/refresh', {
			headers: { Cookie: `studygo_refresh=${antes!.value}` }
		});
		expect(reuso.status(), 'o refresh de uma sessão encerrada ainda abriu sessão').toBe(401);
		expect(conta.email).toContain('@e2e.local');
	});

	test('[A7] nenhum token fica no localStorage', async ({ page }) => {
		const email = emailUnico('a7');
		await page.goto('/registro');
		await page.getByLabel('Nome').fill('Sem Token');
		await page.getByLabel('Email').fill(email);
		await page.getByLabel('Senha').fill(SENHA);
		await page.getByRole('button', { name: 'Criar conta' }).click();
		await expect(page).toHaveURL(/\/concursos\/novo$/);
		await page.reload();
		await expect(page.getByRole('heading', { name: 'Cadastrar concurso' })).toBeVisible();

		const guardado = await page.evaluate(() => JSON.stringify({ ...localStorage }));
		expect(guardado).not.toMatch(/eyJ[\w-]+\.[\w-]+\.[\w-]+/); // cara de JWT
		expect(guardado.toLowerCase()).not.toContain('token');
	});
});
