import { test, expect, abrirHoje, materiasDoDia, registrar, type Page } from './base';

/** As listas de matérias do cronograma, uma por dia, na ordem da tela. */
function diasDoCronograma(page: Page) {
	return page
		.locator('main')
		.getByRole('list')
		.filter({ has: page.getByRole('button', { name: /^Registrar estudo de / }) });
}

async function abrirCronograma(page: Page) {
	await page.goto('/cronograma');
	await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
}

test.describe('estudo do dia', () => {
	test('[C1] registrar todas as matérias conclui o dia e move os números', async ({ page, api }) => {
		await api.concurso('Dia E2E');
		await abrirHoje(page);
		await expect(page.getByText(/Progresso 0%/)).toBeVisible();
		await expect(page.getByText('Dia concluído')).toBeHidden();

		const materias = await materiasDoDia(page);
		expect(materias.length).toBeGreaterThan(0);
		for (const m of materias) await registrar(page, m, { minutos: 60, questoes: 10, acertos: 8, concluir: true });

		await expect(page.getByText('Dia concluído')).toBeVisible();
		await expect(page.getByText(new RegExp(`Horas\\s*${materias.length},0\\s*/`))).toBeVisible();
		await expect(page.getByText(/Acerto 80%/)).toBeVisible();
		await expect(page.getByText(/Progresso 0%/)).toBeHidden();
	});

	test('[C2] o dia só conclui quando todas as matérias concluem', async ({ page, api }) => {
		await api.concurso('Parcial E2E');
		await abrirHoje(page);
		const [primeira, segunda] = await materiasDoDia(page);
		expect(segunda, 'o dia de teste precisa de duas matérias').toBeTruthy();

		await registrar(page, primeira, { minutos: 60, concluir: true });
		await registrar(page, segunda, { minutos: 30, concluir: false });
		await expect(page.getByText('Dia concluído')).toBeHidden();

		await registrar(page, segunda, { minutos: 60, concluir: true });
		await expect(page.getByText('Dia concluído')).toBeVisible();
	});

	test('[C3] o registro fica gravado depois de recarregar', async ({ page, api }) => {
		await api.concurso('Gravar E2E');
		await abrirHoje(page);
		const [materia] = await materiasDoDia(page);
		await registrar(page, materia, { minutos: 50, questoes: 14, acertos: 11, observacao: 'Revisar crase' });

		await page.reload();
		await page.getByRole('button', { name: `Registrar estudo de ${materia}` }).first().click();
		const dialogo = page.getByRole('dialog', { name: `Registrar estudo — ${materia}` });
		await expect(dialogo.getByLabel('Minutos estudados')).toHaveValue('50');
		await expect(dialogo.getByLabel('Questões')).toHaveValue('14');
		await expect(dialogo.getByLabel('Acertos')).toHaveValue('11');
		await expect(dialogo.getByLabel('Observação')).toHaveValue('Revisar crase');
	});

	test('[C4] adiar o dia leva as matérias para o dia seguinte', async ({ page, api }) => {
		const slug = await api.concurso('Adiar E2E');
		const antes = await api.plano(slug);
		const hoje = antes.dias[antes.hojeIndex];
		const deHoje = hoje.itens.map((i: { disciplina: string }) => i.disciplina);

		await abrirHoje(page);
		await page.getByRole('button', { name: /^Adiar este dia/ }).click();
		const confirmacao = page.getByRole('alertdialog', { name: 'Adiar este dia?' });
		await confirmacao.getByRole('button', { name: 'Cancelar' }).click();
		expect((await api.plano(slug)).dias[antes.hojeIndex].itens).toHaveLength(deHoje.length);

		await page.getByRole('button', { name: /^Adiar este dia/ }).click();
		await confirmacao.getByRole('button', { name: 'Adiar o dia' }).click();
		await expect(confirmacao).toBeHidden();
		await expect(page.getByRole('button', { name: /^Registrar estudo de / })).toHaveCount(0);

		const depois = await api.plano(slug);
		const diaDeHoje = depois.dias.find((d: { data: string }) => d.data === hoje.data);
		const seguinte = depois.dias.find((d: { data: string; itens: unknown[] }) => d.data > hoje.data && d.itens.length > 0);
		expect(diaDeHoje?.itens ?? []).toHaveLength(0);
		expect(seguinte.itens.map((i: { disciplina: string }) => i.disciplina)).toEqual(deHoje);
	});

	test('[C5] mover uma matéria troca a ordem e a troca fica gravada', async ({ page, api }) => {
		await api.concurso('Mover E2E');
		await abrirCronograma(page);
		const amanha = diasDoCronograma(page).nth(1);
		const [a, b] = await materiasDoDia(page, amanha);
		expect(b, 'o dia de teste precisa de duas matérias').toBeTruthy();

		await amanha.getByRole('button', { name: `Descer ${a} uma posição` }).click();
		await expect.poll(() => materiasDoDia(page, diasDoCronograma(page).nth(1))).toEqual([b, a]);

		await page.reload();
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		expect(await materiasDoDia(page, diasDoCronograma(page).nth(1))).toEqual([b, a]);
	});

	test('[C6] matéria concluída não se move', async ({ page, api }) => {
		await api.concurso('Travar E2E');
		await abrirHoje(page);
		const [a, b] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 60, concluir: true });

		await abrirCronograma(page);
		const hoje = diasDoCronograma(page).first();
		await expect(hoje.getByRole('button', { name: `Registrar estudo de ${a}` })).toBeVisible();
		await expect(hoje.getByRole('button', { name: new RegExp(`^(Subir|Descer) ${a} `) })).toHaveCount(0);
		if (b) await expect(hoje.getByRole('button', { name: new RegExp(`^(Subir|Descer) ${b} `) }).first()).toBeVisible();
	});

	test('[C7] as estatísticas batem com o que foi registrado', async ({ page, api }) => {
		await api.concurso('Estatística E2E');
		await abrirHoje(page);
		const [a, b] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 60, questoes: 10, acertos: 7, concluir: true });
		await registrar(page, b, { minutos: 60, questoes: 12, acertos: 9, concluir: true });
		// 16 de 22 = 72,7%: a tela Hoje e as estatísticas têm de dizer o mesmo.
		await expect(page.getByText(/Acerto 73%/)).toBeVisible();

		await page.getByRole('link', { name: 'Estatísticas' }).click();
		const topo = page.locator('main');
		await expect(topo.getByText(/Horas\s*2,0/).first()).toBeVisible();
		await expect(topo.getByText(/Questões\s*22/).first()).toBeVisible();
		await expect(topo.getByText(/Acerto\s*73%/).first()).toBeVisible();
		await expect(page.getByRole('row', { name: new RegExp(`^${a} 1,0 h .* 70%$`) })).toBeVisible();
		await expect(page.getByRole('row', { name: new RegExp(`^${b} 1,0 h .* 75%$`) })).toBeVisible();
	});

	test('[C8] o balanceamento reflete as horas lançadas', async ({ page, api }) => {
		await api.concurso('Balanço E2E');
		await abrirHoje(page);
		const [a] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 90, concluir: true });

		await page.getByRole('link', { name: 'Balanceamento' }).click();
		const linha = page.getByRole('row').filter({ hasText: a }).filter({ hasText: /\d+,\d h/ }).last();
		await expect(linha).toContainText('1,5 h');
	});

	test('[C9] matéria abaixo de 70% entra no caderno de erros, e a boa não', async ({ page, api }) => {
		await api.concurso('Caderno E2E');
		await abrirHoje(page);
		const [a, b] = await materiasDoDia(page);
		await registrar(page, a, { minutos: 60, questoes: 10, acertos: 5, concluir: true });
		await registrar(page, b, { minutos: 60, questoes: 10, acertos: 9, concluir: true });

		await page.getByRole('link', { name: 'Caderno de erros' }).click();
		const main = page.locator('main');
		// "Caderno por matéria" é por assunto: só o de 50% entra. As "baterias"
		// são por DIA — 14 de 20 dá exatamente 70%, que não é fraco.
		await expect(main.getByText(new RegExp(`${a}.*50% 5/10`)).first()).toBeVisible();
		await expect(main.getByText(/90% 9\/10/)).toHaveCount(0);
		await expect(page.getByRole('heading', { name: /Baterias com aproveitamento abaixo de 70% \(0\)/ })).toBeVisible();
	});

	test('[C10] o link do caderno colado no registro vale para a matéria toda', async ({ page, api }) => {
		const slug = await api.concurso('Link E2E');
		const url = 'https://www.tecconcursos.com.br/questoes/caderno/e2e-123';
		await abrirHoje(page);
		const [a] = await materiasDoDia(page);

		await page.getByRole('button', { name: `Registrar estudo de ${a}` }).first().click();
		const dialogo = page.getByRole('dialog', { name: `Registrar estudo — ${a}` });
		await dialogo.getByRole('button', { name: 'Adicionar link do Caderno de erros' }).click();
		await dialogo.getByPlaceholder('https://www.tecconcursos.com.br/questoes/caderno/…').fill(url);
		await dialogo.getByRole('button', { name: 'pronto' }).click();
		await dialogo.getByLabel('Minutos estudados').fill('30');
		await dialogo.getByRole('button', { name: 'Salvar' }).click();
		await expect(dialogo).toBeHidden();

		await page.goto(`/concursos/${slug}/editar`);
		const nomes = page.getByLabel('Disciplina *');
		await expect(nomes.first()).not.toHaveValue('');
		const indice = await nomes.evaluateAll((els, nome) => els.findIndex((e) => (e as HTMLInputElement).value === nome), a);
		await expect(page.getByLabel('Caderno de erros — link').nth(indice)).toHaveValue(url);
	});
});
