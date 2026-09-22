import { test, expect, abrirHoje, dataEmDias, materiasDoDia, registrar } from './base';

test.describe('concurso', () => {
	test('[B1] cadastro manual gera o plano', async ({ page, conta }) => {
		void conta;
		await page.goto('/concursos/novo');
		await page.getByRole('button', { name: 'Cadastrar manualmente' }).click();
		await page.getByLabel('Nome *').fill('TRT E2E');
		await page.getByLabel('Data da prova *').fill(dataEmDias(60));
		await page.getByLabel('Banca').fill('FCC');

		const linhas: [string, 'Gerais' | 'Específicas', string][] = [
			['Língua Portuguesa', 'Gerais', '20'],
			['Direito Administrativo', 'Específicas', '15']
		];
		for (const [i, [nome, grupo, questoes]] of linhas.entries()) {
			if (i > 0) await page.getByRole('button', { name: '+ disciplina' }).click();
			await page.getByLabel('Disciplina *').nth(i).fill(nome);
			await page.getByRole('button', { name: grupo, exact: true }).nth(i).click();
			await page.getByLabel('Questões').nth(i).fill(questoes);
		}
		await expect(page.getByRole('heading', { name: /20 gerais \+ 15 específicas/i })).toBeVisible();
		await page.getByRole('button', { name: 'Criar concurso' }).click();

		await expect(page).toHaveURL(/\/$/);
		await expect(page.getByRole('heading', { name: 'Hoje', level: 1 })).toBeVisible();
		await expect(page.getByRole('button', { name: /^Registrar estudo de / }).first()).toBeVisible();

		await page.getByRole('link', { name: 'Cronograma' }).click();
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Registrar estudo de Direito Administrativo' }).first()).toBeVisible();
	});

	test('[B2] concurso sem data ou sem disciplina não é criado', async ({ page, conta }) => {
		void conta;
		await page.goto('/concursos/novo');
		await page.getByRole('button', { name: 'Cadastrar manualmente' }).click();
		await page.getByLabel('Nome *').fill('Sem Data');
		await page.getByLabel('Disciplina *').first().fill('Matemática');
		await page.getByLabel('Questões').first().fill('10');
		await page.getByRole('button', { name: 'Criar concurso' }).click();
		await expect(page).toHaveURL(/\/concursos\/novo$/);

		await page.getByLabel('Data da prova *').fill(dataEmDias(60));
		await page.getByLabel('Disciplina *').first().fill('');
		await page.getByRole('button', { name: 'Criar concurso' }).click();
		await expect(page).toHaveURL(/\/concursos\/novo$/);

		const res = await page.context().request.get('/api/concursos', { headers: { Authorization: `Bearer ${conta.token}` } });
		expect((await res.json()).concursos ?? (await res.json())).toHaveLength(0);
	});

	test('[B3] a tag escolhida aparece no cronograma, e tag repetida é recusada', async ({ page, api }) => {
		const slug = await api.concurso('Tags E2E', [
			{ nome: 'Raciocínio Lógico', bloco: 'ger', questoes: 10, codigo: 'RLM' },
			{ nome: 'Direito Penal', bloco: 'esp', questoes: 10 }
		]);
		await page.goto('/cronograma');
		await expect(page.getByRole('button', { name: /^Raciocínio Lógico: / }).first()).toContainText('RLM');

		await page.goto(`/concursos/${slug}/editar`);
		await page.getByLabel('Tag').nth(1).fill('RLM');
		await page.getByRole('button', { name: /salvar/i }).click();
		await expect(page.getByText(/RLM/).and(page.getByText(/tag|repetid|já/i)).first()).toBeVisible();

		const plano = await api.plano(slug);
		const codigos = plano.concurso.disciplinas.map((d: { codigo: string }) => d.codigo);
		expect(new Set(codigos).size).toBe(codigos.length);
	});

	test('[B4] renomear a disciplina preserva o que já foi estudado', async ({ page, api }) => {
		const slug = await api.concurso('Renomear E2E');
		await abrirHoje(page);
		const [primeira] = await materiasDoDia(page);
		await registrar(page, primeira, { minutos: 45, questoes: 10, acertos: 8, concluir: true });

		const antes = await api.plano(slug);

		await page.goto(`/concursos/${slug}/editar`);
		const nomes = page.getByLabel('Disciplina *');
		await expect(nomes.first()).not.toHaveValue('');
		const indice = await nomes.evaluateAll((els, nome) => els.findIndex((e) => (e as HTMLInputElement).value === nome), primeira);
		expect(indice, `a disciplina ${primeira} não está no formulário de edição`).toBeGreaterThanOrEqual(0);
		const tagAntes = await page.getByLabel('Tag').nth(indice).inputValue();
		await nomes.nth(indice).fill(`${primeira} renomeada`);
		await page.getByRole('button', { name: /salvar/i }).click();
		await expect(page).not.toHaveURL(/\/editar$/);

		// A matéria continua a mesma: mesma tag, mesmo estudo, o dia ainda concluído
		// pela parte dela — só o nome mudou.
		await abrirHoje(page);
		await expect(page.getByRole('button', { name: new RegExp(`^${primeira} renomeada: `) }).first()).toContainText(tagAntes);
		const depois = await api.plano(slug);
		expect(depois.props.horasTotal, 'o estudo registrado antes de renomear sumiu').toBe(antes.props.horasTotal);
		expect(depois.props.horasTotal).toBeGreaterThan(0);
	});

	test('[B5] os tópicos chegam ao conteúdo programático e ao dia', async ({ page, api }) => {
		await api.concurso('Temas E2E', [
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase', 'Concordância verbal', 'Regência'] },
			{ nome: 'Direito Constitucional', bloco: 'esp', questoes: 15, temas: ['Direitos fundamentais', 'Controle de constitucionalidade'] }
		]);
		await page.goto('/conteudo');
		for (const tema of ['Crase', 'Concordância verbal', 'Direitos fundamentais']) {
			await expect(page.getByText(tema).first()).toBeVisible();
		}

		await page.goto('/cronograma');
		await expect(page.getByRole('button', { name: /: (Crase|Direitos fundamentais)\. Ver o conteúdo/ }).first()).toBeVisible();

		await page.getByRole('button', { name: /^Língua Portuguesa: .*Ver o conteúdo/ }).first().click();
		const ementa = page.getByRole('dialog', { name: 'Língua Portuguesa' });
		await expect(ementa).toContainText('3 tópicos');
		await expect(ementa).toContainText('Regência');
	});

	test('[B6] as datas do edital aparecem e "cumprido" fica gravado', async ({ page, api }) => {
		await api.concurso('Datas E2E', undefined, {
			marcos: [
				{ data: dataEmDias(5), dataFim: dataEmDias(10), titulo: 'Inscrições', exigeAcao: true },
				{ data: dataEmDias(20), dataFim: '', titulo: 'Pagamento da taxa', exigeAcao: true }
			]
		});
		await page.goto('/datas');
		const main = page.locator('main');
		await expect(main.getByText('Inscrições').first()).toBeVisible();
		await expect(main.getByText('Pagamento da taxa').first()).toBeVisible();

		const cumprido = page.getByRole('checkbox', { name: 'Cumprido' }).first();
		const gravou = page.waitForResponse((r) => r.url().includes('/marcos/') && r.request().method() === 'PUT');
		await cumprido.check();
		expect((await gravou).ok()).toBeTruthy();
		await expect(cumprido).toBeChecked();

		await page.reload();
		await expect(page.getByRole('checkbox', { name: 'Cumprido' }).first()).toBeChecked();
	});

	test('[B7] dois concursos não se misturam', async ({ page, api }) => {
		await api.concurso('Primeiro E2E', [{ nome: 'Contabilidade', bloco: 'esp', questoes: 20 }, { nome: 'Português', bloco: 'ger', questoes: 10 }]);
		await api.concurso('Segundo E2E', [{ nome: 'Estatística', bloco: 'esp', questoes: 20 }, { nome: 'Inglês', bloco: 'ger', questoes: 10 }]);

		await page.goto('/concursos');
		// O cartão mais interno que tem o nome e o botão: o último, na ordem do documento.
		await page
			.locator('main *')
			.filter({ hasText: 'Primeiro E2E' })
			.filter({ has: page.getByRole('button', { name: 'abrir' }) })
			.last()
			.getByRole('button', { name: 'abrir' })
			.click();
		await abrirHoje(page);
		const primeiro = await materiasDoDia(page);
		expect(primeiro.every((m) => ['Contabilidade', 'Português'].includes(m))).toBeTruthy();
		await registrar(page, primeiro[0], { minutos: 30, concluir: true });

		await page.getByRole('button', { name: 'Plano de estudos' }).click();
		await page.getByText('Segundo E2E').first().click();
		await abrirHoje(page);
		const segundo = await materiasDoDia(page);
		expect(segundo.every((m) => ['Estatística', 'Inglês'].includes(m))).toBeTruthy();
		await expect(page.getByText(/Horas 0,0/)).toBeVisible();
	});

	test('[B8] excluir o concurso pede confirmação', async ({ page, api }) => {
		await api.concurso('Excluir E2E');
		await page.goto('/concursos');
		const main = page.locator('main');
		await expect(main.getByText('Excluir E2E')).toBeVisible();

		await page.getByRole('button', { name: 'Excluir concurso' }).click();
		const confirmacao = page.getByRole('alertdialog').or(page.getByRole('dialog'));
		await expect(confirmacao).toBeVisible();
		await confirmacao.getByRole('button', { name: /cancelar|voltar|não/i }).click();
		await expect(main.getByText('Excluir E2E')).toBeVisible();

		await page.getByRole('button', { name: 'Excluir concurso' }).click();
		await confirmacao.getByRole('button', { name: /excluir/i }).click();
		await expect(main.getByText('Excluir E2E')).toBeHidden();
		await page.reload();
		await expect(page.locator('main').getByText('Excluir E2E')).toBeHidden();
	});

	test('[B9] a análise do edital sem cargo avisa e oferece o cadastro manual', async ({ page, conta }) => {
		void conta;
		await page.goto('/concursos/novo');
		// O dublê do processador devolve zero cargos para este texto.
		await page.getByLabel('…ou cole o texto do edital').fill('TRIBUNAL REGIONAL. EDITAL 01/2026, sem a lista de cargos.');
		await page.getByRole('button', { name: 'Analisar edital →' }).click();

		// Sem a IA (ou quando ela não acha cargo nenhum no texto), a análise volta
		// sem cargos. O assistente não pode parar numa lista vazia e muda: tem de
		// dizer o que houve e oferecer o cadastro manual ali mesmo.
		const main = page.locator('main');
		await expect(main.getByText(/nenhum cargo|não (encontr|ach)/i).first()).toBeVisible({ timeout: 30_000 });
		await main.getByRole('button', { name: 'Cadastrar manualmente' }).click();
		await expect(page.getByLabel('Nome *')).toBeVisible();
	});

	test('[B10] o assistente do edital leva ao plano o que a leitura trouxe', async ({ page, api }) => {
		await page.goto('/concursos/novo');
		await page.getByLabel('…ou cole o texto do edital').fill('TRIBUNAL REGIONAL DO TRABALHO. EDITAL 01/2026. Cargos A01 e B02.');
		await page.getByRole('button', { name: 'Analisar edital →' }).click();

		// O segundo cargo, de propósito: a estrutura do dublê muda com o cargo, e
		// é isso que mostra que o assistente pediu a do cargo escolhido.
		await page.getByRole('radio', { name: /^B02 — Técnico Judiciário/ }).check();
		await page.getByRole('button', { name: 'Continuar →' }).click();

		const main = page.locator('main');
		await expect(page.getByRole('heading', { name: 'Conhecimentos gerais' })).toBeVisible();
		await expect(main.getByRole('textbox').nth(1)).toHaveValue('Matemática');
		await expect(main.getByText('Soma informada: 40 / 40 do grupo')).toBeVisible();
		await page.getByRole('button', { name: 'Próximo: específicas →' }).click();

		await expect(page.getByRole('heading', { name: 'Conhecimentos específicos do cargo' })).toBeVisible();
		await expect(main.getByRole('textbox').first()).toHaveValue('Direito Administrativo');
		await page.getByRole('button', { name: 'Buscar conteúdo programático →' }).click();

		// A revisão chega preenchida com o que foi lido; nada aqui é digitado.
		await expect(page.getByLabel('Nome *')).toHaveValue('TRT E2E — Técnico Judiciário');
		await expect(page.getByLabel('Data da prova *')).toHaveValue(dataEmDias(75));
		await expect(page.getByLabel('Tópicos — um por linha').nth(2)).toHaveValue(/Atos administrativos\s+Licitações/);
		await page.getByRole('button', { name: 'Criar concurso' }).click();
		await expect(page.getByRole('heading', { name: 'Hoje', level: 1 })).toBeVisible();

		// O plano nasceu do que a leitura trouxe: disciplinas, tópicos e datas.
		await page.goto('/conteudo');
		for (const tema of ['Crase', 'Porcentagem', 'Atos administrativos', 'Gestão de documentos']) {
			await expect(page.locator('main').getByText(tema).first()).toBeVisible();
		}
		await page.goto('/datas');
		for (const marco of ['Inscrições', 'Pagamento da taxa']) {
			await expect(page.locator('main').getByText(marco).first()).toBeVisible();
		}
		const { dados } = await api.unicoConcurso();
		const gravadas = dados.disciplinas.map((d: { nome: string; bloco: string; questoes: number }) => [d.nome, d.bloco, d.questoes]);
		expect(gravadas).toEqual([
			['Língua Portuguesa', 'ger', 25],
			['Matemática', 'ger', 15],
			['Direito Administrativo', 'esp', 20],
			['Arquivologia', 'esp', 10]
		]);
		expect(dados.prova).toBe(dataEmDias(75));
	});
});
