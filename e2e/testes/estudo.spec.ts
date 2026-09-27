import { test, expect, abrirHoje, dataEmDias, materiasDoDia, registrar, type Page } from './base';

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

	test('[C11] a revisão do dia abre com o que há para revisar e fica gravada', async ({ page, api }) => {
		await api.concurso('Revisão E2E', [
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase', 'Regência'] },
			{ nome: 'Direito Constitucional', bloco: 'esp', questoes: 15, temas: ['Direitos fundamentais', 'Controle de constitucionalidade'] }
		]);
		await abrirHoje(page);
		for (const m of await materiasDoDia(page)) await registrar(page, m, { minutos: 60, questoes: 10, acertos: 5, concluir: true });

		await abrirCronograma(page);
		const botao = page.getByRole('button', { name: /^Registrar revisão de / }).first();
		const materia = (await botao.getAttribute('aria-label'))!.replace('Registrar revisão de ', '');
		await botao.click();

		// A revisão puxa o que foi mal: o assunto estudado hoje com 50%.
		const dialogo = page.getByRole('dialog', { name: `Registrar revisão — ${materia}` });
		await expect(dialogo.getByRole('heading', { name: 'Volte a estes assuntos, sem consultar antes' })).toBeVisible();
		await expect(dialogo.getByRole('listitem').first()).toContainText('50%');

		const nota = 'Revisei e ainda confundo os conceitos';
		await dialogo.getByLabel('O que ainda precisa de atenção').fill(nota);
		await dialogo.getByLabel('Questões').fill('8');
		await dialogo.getByLabel('Acertos').fill('6');
		await dialogo.getByRole('button', { name: 'Salvar' }).click();
		await expect(dialogo).toBeHidden();

		await page.reload();
		await page.getByRole('button', { name: `Registrar revisão de ${materia}` }).first().click();
		await expect(dialogo.getByLabel('O que ainda precisa de atenção')).toHaveValue(nota);
		await expect(dialogo.getByLabel('Questões')).toHaveValue('8');
		await expect(dialogo.getByLabel('Acertos')).toHaveValue('6');
	});

	test('[C12][C13] marcar na ementa o tópico já estudado o antecipa, e desmarcar desfaz', async ({ page, api }) => {
		const temas = ['Redes', 'Linux', 'Windows', 'Nuvem', 'Segurança', 'Bancos de dados'];
		const slug = await api.concurso('Antecipar tópico E2E', [
			{ nome: 'Informática', bloco: 'esp', questoes: 10, temas },
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase', 'Regência'] }
		]);
		const antes = await api.plano(slug);
		const hoje = antes.dias[antes.hojeIndex].data;
		const codigo = antes.concurso.disciplinas.find((d: { nome: string }) => d.nome === 'Informática').codigo;
		type Item = { id: string; disciplina: string; tema: string; passada: number; concluido: boolean };
		type Dia = { data: string; itens: Item[] };
		const primeiras = (p: { dias: Dia[] }, tema: string) =>
			p.dias.flatMap((d) => d.itens.map((i) => ({ ...i, data: d.data })))
				.filter((i) => i.disciplina === codigo && i.tema === tema && i.passada <= 1);
		// Um tópico que o cronograma só traria depois de hoje.
		const tema = temas.find((t) => primeiras(antes, t).every((i) => i.data > hoje))!;
		expect(tema, 'o cenário precisa de um tópico agendado para a frente').toBeTruthy();
		const alvo = primeiras(antes, tema)[0];

		await abrirCronograma(page);
		await page.getByRole('button', { name: /^Informática: .*Ver o conteúdo programático/ }).first().click();
		const ementa = page.getByRole('dialog', { name: 'Informática' });
		const marca = ementa.getByRole('checkbox', { name: `Já estudei: ${tema}` });
		await expect(marca).not.toBeChecked();
		await marca.check();
		await expect(marca).toBeChecked();

		// A caixa marca na hora; o servidor grava logo depois.
		await expect.poll(async () => primeiras(await api.plano(slug), tema).find((i) => i.id === alvo.id)?.concluido).toBe(true);
		const depois = await api.plano(slug);
		const movida = primeiras(depois, tema).find((i) => i.id === alvo.id)!;
		expect(movida.data).toBe(hoje);
		expect(movida.concluido).toBe(true);
		// Não sobra 1ª passada dele agendada adiante: não reaparece na sequência.
		expect(primeiras(depois, tema).filter((i) => i.data > hoje && !i.concluido)).toHaveLength(0);

		await page.reload();
		await page.getByRole('button', { name: /^Informática: .*Ver o conteúdo programático/ }).first().click();
		await expect(page.getByRole('dialog', { name: 'Informática' }).getByRole('checkbox', { name: `Já estudei: ${tema}` })).toBeChecked();

		// C13: um clique errado se desfaz.
		await page.getByRole('dialog', { name: 'Informática' }).getByRole('checkbox', { name: `Já estudei: ${tema}` }).uncheck();
		await expect(page.getByRole('dialog', { name: 'Informática' }).getByRole('checkbox', { name: `Já estudei: ${tema}` })).not.toBeChecked();
		await expect.poll(async () => primeiras(await api.plano(slug), tema).find((i) => i.id === alvo.id)?.concluido).toBe(false);
	});

	test('[C14] marcar um tópico de um bloco que junta vários marca só ele', async ({ page, api }) => {
		// Mais tópicos que vagas: o motor junta vários numa atividade ("A  ·  B").
		const temas = Array.from({ length: 150 }, (_, i) => `Tópico ${String(i + 1).padStart(3, '0')}`);
		const slug = await api.concurso('Tópico isolado E2E', [
			{ nome: 'Informática', bloco: 'esp', questoes: 10, temas },
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase'] }
		]);
		const antes = await api.plano(slug);
		const hoje = antes.dias[antes.hojeIndex].data;
		const codigo = antes.concurso.disciplinas.find((d: { nome: string }) => d.nome === 'Informática').codigo;
		type Item = { id: string; disciplina: string; tema: string; passada: number; concluido: boolean };
		type Dia = { data: string; itens: Item[] };
		const itens = (p: { dias: Dia[] }) =>
			p.dias.flatMap((d) => d.itens.map((i) => ({ ...i, data: d.data }))).filter((i) => i.disciplina === codigo);
		const partes = (t: string) => t.split('·').map((x) => x.trim()).filter(Boolean);
		const bloco = itens(antes).find((i) => i.data > hoje && i.passada === 1 && partes(i.tema).length >= 2)!;
		expect(bloco, 'o cenário precisa de um bloco com dois tópicos adiante').toBeTruthy();
		const [marcado, vizinho] = partes(bloco.tema);

		await abrirCronograma(page);
		await page.getByRole('button', { name: /^Informática: .*Ver o conteúdo programático/ }).first().click();
		const ementa = page.getByRole('dialog', { name: 'Informática' });
		await ementa.getByRole('checkbox', { name: `Já estudei: ${marcado}` }).check();
		await expect(ementa.getByRole('checkbox', { name: `Já estudei: ${marcado}` })).toBeChecked();
		await expect(ementa.getByRole('checkbox', { name: `Já estudei: ${vizinho}` })).not.toBeChecked();

		await expect.poll(async () => itens(await api.plano(slug)).find((i) => i.tema === marcado)?.concluido).toBe(true);
		const depois = itens(await api.plano(slug));
		const estudado = depois.find((i) => i.tema === marcado)!;
		expect(estudado.concluido).toBe(true);
		expect(estudado.data).toBe(hoje);
		// O resto do bloco continua pendente, sem o tópico marcado.
		const resto = depois.find((i) => i.id === bloco.id)!;
		expect(resto.concluido).toBe(false);
		expect(partes(resto.tema)).toContain(vizinho);
		expect(partes(resto.tema)).not.toContain(marcado);
		expect(depois.filter((i) => !i.concluido && partes(i.tema).includes(marcado) && i.passada === 1)).toHaveLength(0);
	});

	test('[C15] marcar num dia que não é de estudo reorganiza, em vez de deixar feito lá no final', async ({ page, api, conta }) => {
		const temas = ['Redes', 'Linux', 'Windows', 'Nuvem', 'Segurança', 'Bancos de dados'];
		const slug = await api.concurso('Domingo E2E', [
			{ nome: 'Informática', bloco: 'esp', questoes: 10, temas },
			{ nome: 'Língua Portuguesa', bloco: 'ger', questoes: 20, temas: ['Crase', 'Regência'] }
		]);
		// Hoje fora dos dias de estudo, como um domingo num plano de segunda a sexta.
		const hojeSemana = new Date(`${dataEmDias(0)}T12:00:00`).getDay();
		const res = await page.request.put(`/api/concursos/${slug}/plano`, {
			headers: { Authorization: `Bearer ${conta.token}` },
			data: { diasEstudo: [0, 1, 2, 3, 4, 5, 6].filter((d) => d !== hojeSemana), simulados: 'nunca', discursiva: false, revisaoSemanal: false, blocosPorDia: 2 }
		});
		expect(res.ok(), await res.text()).toBeTruthy();

		const antes = await api.plano(slug);
		const codigo = antes.concurso.disciplinas.find((d: { nome: string }) => d.nome === 'Informática').codigo;
		type Item = { id: string; disciplina: string; tema: string; passada: number; concluido: boolean };
		type Dia = { data: string; itens: Item[] };
		const itens = (p: { dias: Dia[] }) =>
			p.dias.flatMap((d) => d.itens.map((i) => ({ ...i, data: d.data }))).filter((i) => i.disciplina === codigo && i.passada === 1);
		const comConteudo = (p: { dias: Dia[] }) => p.dias.filter((d) => d.itens.length > 0).map((d) => d.data);
		const primeiroDia = comConteudo(antes)[0];
		const alvo = itens(antes).filter((i) => i.data > primeiroDia).at(-1)!;
		expect(alvo, 'o cenário precisa de um tópico bem adiante').toBeTruthy();

		await abrirCronograma(page);
		await page.getByRole('button', { name: /^Informática: .*Ver o conteúdo programático/ }).first().click();
		const ementa = page.getByRole('dialog', { name: 'Informática' });
		await ementa.getByRole('checkbox', { name: `Já estudei: ${alvo.tema}` }).check();
		await expect(ementa.getByRole('checkbox', { name: `Já estudei: ${alvo.tema}` })).toBeChecked();

		await expect.poll(async () => itens(await api.plano(slug)).find((i) => i.id === alvo.id)?.concluido).toBe(true);
		const depois = await api.plano(slug);
		const movida = itens(depois).find((i) => i.id === alvo.id)!;
		expect(movida.concluido).toBe(true);
		// Não ficou na data de antes: foi para o dia de estudo mais próximo de hoje.
		expect(movida.data).not.toBe(alvo.data);
		expect(movida.data <= primeiroDia).toBe(true);
		// E não sobrou repetição dele adiante.
		expect(itens(depois).filter((i) => !i.concluido && i.tema.toLowerCase() === alvo.tema.toLowerCase())).toHaveLength(0);
	});
});
