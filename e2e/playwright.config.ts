import { defineConfig } from '@playwright/test';

// Os testes rodam contra o stack isolado que `make e2e` sobe (projeto compose
// studygo-e2e, imagens de produção). E2E_BASE_URL aponta para outro lugar só
// quando alguém quer rodar contra um stack já de pé.
export default defineConfig({
	testDir: './testes',
	outputDir: './relatorio/artefatos',
	fullyParallel: true,
	workers: 4,
	// Sem nova tentativa: um teste que passa na segunda vez esconde exatamente
	// a intermitência que o E2E existe para mostrar.
	retries: 0,
	timeout: 60_000,
	expect: { timeout: 10_000 },
	reporter: [
		['list'],
		['html', { outputFolder: './relatorio/html', open: 'never' }],
		['json', { outputFile: './relatorio/resultados.json' }]
	],
	use: {
		baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:25173',
		locale: 'pt-BR',
		// O dia vira à meia-noite de Brasília no backend; o navegador precisa
		// concordar, ou "hoje" muda de dia entre a tela e a API.
		timezoneId: 'America/Sao_Paulo',
		viewport: { width: 1280, height: 900 },
		// Todo teste termina com um print: é o que deixa o relatório conferível
		// por quem não rodou a suíte.
		screenshot: 'on',
		trace: 'retain-on-failure'
	}
});
