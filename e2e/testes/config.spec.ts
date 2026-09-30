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

	test('[D12] refazer o cronograma com um lançamento sem conclusão adiante não falha e guarda o lançamento', async ({ page, api, conta }) => {
		const slug = await api.concurso('Lançado adiante E2E');
		const antes = await api.plano(slug);
		const alvo = antes.dias[antes.hojeIndex + 1].itens[0];
		expect(alvo, 'o cenário precisa de uma matéria amanhã').toBeTruthy();
		// Lançado e não concluído: é o rastro de um "já estudei" desmarcado.
		const res = await page.request.put(`/api/concursos/${slug}/plano/atividades/${alvo.id}/registro`, {
			headers: { Authorization: `Bearer ${conta.token}` },
			data: { atividadeId: alvo.id, horas: 0.5, questoes: null, acertos: null, nota: '', concluido: false }
		});
		expect(res.ok(), await res.text()).toBeTruthy();

		await abrirConfig(page);
		const gravou = page.waitForResponse((r) => /\/api\/concursos\/[^/]+\/plano$/.test(r.url()) && r.request().method() === 'PUT');
		await page.getByRole('group', { name: 'Blocos por dia' }).getByRole('button', { name: '3', exact: true }).click();
		const resposta = await gravou;
		expect(resposta.ok(), await resposta.text()).toBeTruthy();

		const depois = await api.plano(slug);
		type Item = { id: string; horas: number | null };
		const lancado = depois.dias.flatMap((d: { itens: Item[] }) => d.itens).find((i: Item) => i.id === alvo.id);
		expect(lancado?.horas, 'o lançamento sumiu do cronograma').toBe(0.5);
		// O cronograma foi refeito: dali em diante, três matérias por dia.
		expect(depois.dias[depois.hojeIndex + 2].itens).toHaveLength(3);
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

	test('[D7] compactar fecha o vão e mantém a ordem manual', async ({ page, api }) => {
		const slug = await api.concurso('Compactar E2E');
		const porDia = async () => {
			const plano = await api.plano(slug);
			const futuros = plano.dias.slice(plano.hojeIndex + 1, plano.hojeIndex + 3);
			const movidas = plano.dias.flatMap((d: { itens: { movida: boolean }[] }) => d.itens).filter((i: { movida: boolean }) => i.movida);
			return { amanha: futuros[0].itens.length, depois: futuros[1].itens.length, movidas: movidas.length };
		};

		// Descer a última matéria de amanhã para o dia seguinte abre um vão amanhã.
		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		const amanha = page
			.locator('main')
			.getByRole('list')
			.filter({ has: page.getByRole('button', { name: /^Registrar estudo de / }) })
			.nth(1);
		const ultima = (await materiasDoDia(page, amanha)).at(-1)!;
		await amanha.getByRole('button', { name: `Descer ${ultima} uma posição` }).click();
		await expect.poll(porDia).toEqual({ amanha: 1, depois: 3, movidas: 1 });

		await abrirConfig(page);
		await page.getByRole('button', { name: '⇡ Compactar o cronograma' }).click();
		await expect.poll(porDia).toEqual({ amanha: 2, depois: 2, movidas: 1 });
		await expect(page.getByText('Nenhuma troca manual ainda.')).toBeHidden();
	});

	test('[D8] reorganizar a partir de hoje refaz o que vem depois e guarda o estudado', async ({ page, api }) => {
		const slug = await api.concurso('Reorganizar E2E');
		await abrirHoje(page);
		const [estudada] = await materiasDoDia(page);
		await registrar(page, estudada, { minutos: 60, questoes: 10, acertos: 7, concluir: true });

		await page.goto('/cronograma');
		await expect(page.getByRole('heading', { name: 'Semana 01' })).toBeVisible();
		const amanha = page
			.locator('main')
			.getByRole('list')
			.filter({ has: page.getByRole('button', { name: /^Registrar estudo de / }) })
			.nth(1);
		const [a] = await materiasDoDia(page, amanha);
		await amanha.getByRole('button', { name: `Descer ${a} uma posição` }).click();
		// Descer a primeira é uma troca: as duas matérias ficam marcadas.
		const movidas = async () =>
			(await api.plano(slug)).dias.flatMap((d: { itens: { movida: boolean }[] }) => d.itens).filter((i: { movida: boolean }) => i.movida).length;
		await expect.poll(movidas).toBeGreaterThan(0);
		const trocadas = await movidas();
		const antes = await api.plano(slug);

		await abrirConfig(page);
		await expect(page.getByLabel('Reorganizar a partir de')).toHaveValue(antes.dias[antes.hojeIndex].data);
		await page.getByRole('button', { name: '⇅ Reorganizar dali em diante' }).click();
		const confirmacao = page.getByRole('alertdialog', { name: 'Refazer o cronograma?' });
		await confirmacao.getByRole('button', { name: 'Cancelar' }).click();
		expect(await movidas(), 'cancelar reorganizou mesmo assim').toBe(trocadas);

		await page.getByRole('button', { name: '⇅ Reorganizar dali em diante' }).click();
		await confirmacao.getByRole('button', { name: 'Reorganizar' }).click();
		await expect(page.getByText('Nenhuma troca manual ainda.')).toBeVisible();

		const depois = await api.plano(slug);
		expect(await movidas(), 'a troca manual sobreviveu à reorganização').toBe(0);
		expect(depois.props.horasTotal, 'reorganizar mexeu no que já foi estudado').toBe(antes.props.horasTotal);
		const hoje = depois.dias[depois.hojeIndex].itens.map((i: { disciplina: string; concluido: boolean }) => [i.disciplina, i.concluido]);
		const codigo = antes.dias[antes.hojeIndex].itens.find((i: { concluido: boolean }) => i.concluido).disciplina;
		expect(hoje, 'a matéria estudada hoje saiu do lugar').toContainEqual([codigo, true]);
		expect(depois.dias[depois.hojeIndex + 1].itens).toHaveLength(2);
	});
});

test.describe('matéria só na reta final', () => {
	const LEGISLACAO = ['Lei Orgânica', 'Regimento Interno', 'Estatuto dos Servidores'];
	const DISCIPLINAS = [
		{ nome: 'Informática', bloco: 'esp' as const, questoes: 10, temas: ['Redes', 'Linux', 'Windows', 'Nuvem', 'Segurança', 'Bancos de dados'] },
		{ nome: 'Língua Portuguesa', bloco: 'ger' as const, questoes: 20, temas: ['Crase', 'Regência', 'Concordância'] },
		{ nome: 'Legislação Institucional', bloco: 'ger' as const, questoes: 5, temas: LEGISLACAO }
	];
	type Item = { disciplina: string; tema: string; passada: number };
	type Dia = { data: string; fase: 'base' | 'reta'; tipo: string; itens: Item[] };
	type Plano = { dias: Dia[]; hojeIndex: number; concurso: { disciplinas: { nome: string; codigo: string }[] } };
	const codigoDe = (p: Plano, nome: string) => p.concurso.disciplinas.find((d) => d.nome === nome)!.codigo;
	/** Os itens da matéria depois de hoje, numa fase. */
	const adiante = (p: Plano, codigo: string, fase: 'base' | 'reta') => {
		const hoje = p.dias[p.hojeIndex].data;
		return p.dias.filter((d) => d.data > hoje && d.fase === fase).flatMap((d) => d.itens).filter((i) => i.disciplina === codigo);
	};
	const quando = (page: Page) => page.getByRole('group', { name: 'Quando estudar Legislação Institucional' });
	const gravar = (page: Page) =>
		page.waitForResponse((r) => /\/api\/concursos\/[^/]+\/plano$/.test(r.url()) && r.request().method() === 'PUT');

	test('[D9][D10] adiar a matéria a tira da fase de aprender e a estuda inteira, uma vez, na reta final', async ({ page, api }) => {
		const slug = await api.concurso('Adiar E2E', DISCIPLINAS);
		await abrirHoje(page);
		const [estudada] = await materiasDoDia(page);
		await registrar(page, estudada, { minutos: 60, questoes: 10, acertos: 8, concluir: true });
		const antes: Plano = await api.plano(slug);
		const leg = codigoDe(antes, 'Legislação Institucional');
		expect(adiante(antes, leg, 'base').length, 'o cenário precisa da matéria na fase de aprender').toBeGreaterThan(0);

		await abrirConfig(page);
		await expect(quando(page).getByRole('button', { name: 'o plano todo' })).toHaveAttribute('aria-pressed', 'true');
		const gravou = gravar(page);
		await quando(page).getByRole('button', { name: 'só na reta final' }).click();
		expect((await gravou).ok()).toBeTruthy();

		const depois: Plano & { props: { horasTotal: number }; balanceamento: { codigo: string; temasCobertos: number }[] } = await api.plano(slug);
		expect(adiante(depois, leg, 'base'), 'a matéria adiada continua na fase de aprender').toHaveLength(0);
		// Na reta final ela é ESTUDADA: cada tópico uma vez, sem o rótulo de revisão.
		const naReta = adiante(depois, leg, 'reta');
		expect(naReta.map((i) => i.tema).sort()).toEqual([...LEGISLACAO].sort());
		expect(naReta.every((i) => i.passada === 1)).toBe(true);
		// As outras continuam sendo revisadas na reta final.
		const outras = depois.dias.filter((d) => d.tipo === 'revd').flatMap((d) => d.itens).filter((i) => i.disciplina !== leg);
		expect(outras.length).toBeGreaterThan(0);
		expect(outras.every((i) => i.tema.startsWith('Revisão dirigida — '))).toBe(true);
		expect(depois.props.horasTotal, 'adiar a matéria apagou o estudo').toBe(antes.props.horasTotal);
		expect(depois.balanceamento.find((l) => l.codigo === leg)!.temasCobertos).toBe(LEGISLACAO.length);

		await page.reload();
		await expect(quando(page).getByRole('button', { name: 'só na reta final' })).toHaveAttribute('aria-pressed', 'true');

		await page.goto('/balanceamento');
		const linha = page.getByRole('row').filter({ hasText: 'Legislação Institucional' }).first();
		await expect(linha).toContainText('só na reta final');
		// Todos os tópicos estão no cronograma: a linha não acusa matéria incompleta.
		await expect(linha).not.toHaveClass(/incompleta/);
	});

	test('[D11] a matéria volta ao plano todo, e adiar todas é recusado', async ({ page, api, conta }) => {
		const slug = await api.concurso('Desadiar E2E', DISCIPLINAS);
		await abrirConfig(page);
		let gravou = gravar(page);
		await quando(page).getByRole('button', { name: 'só na reta final' }).click();
		expect((await gravou).ok()).toBeTruthy();
		const leg = codigoDe(await api.plano(slug), 'Legislação Institucional');
		expect(adiante(await api.plano(slug), leg, 'base')).toHaveLength(0);

		gravou = gravar(page);
		await quando(page).getByRole('button', { name: 'o plano todo' }).click();
		expect((await gravou).ok()).toBeTruthy();
		expect(adiante(await api.plano(slug), leg, 'base').length, 'a matéria não voltou à fase de aprender').toBeGreaterThan(0);

		// Todas adiadas deixariam a fase de aprender sem matéria nenhuma.
		const p: Plano = await api.plano(slug);
		const todas = Object.fromEntries(p.concurso.disciplinas.map((d) => [d.codigo, true]));
		const res = await page.request.put(`/api/concursos/${slug}/plano`, {
			headers: { Authorization: `Bearer ${conta.token}` },
			data: { soNaRetaFinal: todas }
		});
		expect(res.ok()).toBeFalsy();
		expect(await res.text()).toContain('reta final');
	});
});
