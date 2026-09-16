import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { expect, it } from 'vitest';
import { PacoteInvalido, provasDoPacote } from './pacote';

// Gerado pelo zip.Writer do Go, com entradas Store e o data descriptor que ele
// põe — o que o backend escreve ao exportar.
const exemplo = new Blob([readFileSync(fileURLToPath(new URL('./testdata/pacote-de-exemplo.zip', import.meta.url)))]);

it('lê cada prova do pacote com os arquivos da pasta dela', async () => {
	const provas = await provasDoPacote(exemplo);

	expect(provas.map((p) => p.pasta)).toEqual(['tjce-2026-e05', 'trt-18-2023-l12']);
	const tjce = provas[0];
	expect(JSON.parse(await tjce.manifesto.text())).toEqual({ formato: 'studygo.prova/2', pdfDaProva: 'prova.pdf' });
	expect(tjce.arquivos.map((a) => a.nome).sort()).toEqual([
		'gabarito.pdf',
		'prova.pdf',
		'questao-11-11111111-2222-3333-4444-555555555555.png'
	]);
	const figura = tjce.arquivos.find((a) => a.nome.endsWith('.png'))!;
	expect(Buffer.from(await figura.conteudo.arrayBuffer())).toEqual(Buffer.from('\x89PNG\r\n\x1a\nfigura', 'latin1'));
	expect(await provas[1].arquivos[0].conteudo.text()).toBe('%PDF-1.7 outro caderno');
});

it('recusa o que não é pacote do studygo', async () => {
	await expect(provasDoPacote(new Blob(['não sou zip']))).rejects.toBeInstanceOf(PacoteInvalido);
	// Um .zip sem prova.json em pasta nenhuma.
	const bytes = readFileSync(fileURLToPath(new URL('./testdata/pacote-de-exemplo.zip', import.meta.url)));
	const semProva = new Blob(
		[bytes.toString('latin1').replaceAll('prova.json', 'outra.json')].map((s) => Buffer.from(s, 'latin1'))
	);
	await expect(provasDoPacote(semProva)).rejects.toThrow('não tem nenhuma prova');
});
