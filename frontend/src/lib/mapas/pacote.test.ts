/**
 * O .zip exportado aberto na tela, para voltar mapa a mapa.
 *
 * Como isto quebra (escrito antes do código):
 * - o arquivo comprimido (o texto e o JSON vão com deflate) chega cru, ou o
 *   guardado sem compressão (as imagens) chega comprimido de novo;
 * - o nome com acento sai embaralhado;
 * - a imagem de um mapa vai para outro de nome parecido ("rede" e "redes");
 * - as questões ou o estudo (`.conta.json`) não acompanham o mapa;
 * - o README de uma pasta de mapas vira um "mapa" que a importação recusa;
 * - um arquivo que não é .zip passa sem dizer nada.
 */
import { deflateRawSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';
import { lerZip, pacotesDoZip } from './pacote';

type Entrada = { nome: string; dados: Uint8Array; comprimir: boolean };

/** Um .zip mínimo, como o archive/zip do Go escreve (sem zip64). */
function zip(entradas: Entrada[]): ArrayBuffer {
	const enc = new TextEncoder();
	const locais: Uint8Array[] = [];
	const central: Uint8Array[] = [];
	let offset = 0;
	for (const e of entradas) {
		const nome = enc.encode(e.nome);
		const corpo = e.comprimir ? new Uint8Array(deflateRawSync(e.dados)) : e.dados;
		const metodo = e.comprimir ? 8 : 0;
		const local = new Uint8Array(30 + nome.length + corpo.length);
		const lv = new DataView(local.buffer);
		lv.setUint32(0, 0x04034b50, true);
		lv.setUint16(6, 0x0800, true);
		lv.setUint16(8, metodo, true);
		lv.setUint32(18, corpo.length, true);
		lv.setUint32(22, e.dados.length, true);
		lv.setUint16(26, nome.length, true);
		local.set(nome, 30);
		local.set(corpo, 30 + nome.length);
		locais.push(local);
		const c = new Uint8Array(46 + nome.length);
		const cv = new DataView(c.buffer);
		cv.setUint32(0, 0x02014b50, true);
		cv.setUint16(8, 0x0800, true);
		cv.setUint16(10, metodo, true);
		cv.setUint32(20, corpo.length, true);
		cv.setUint32(24, e.dados.length, true);
		cv.setUint16(28, nome.length, true);
		cv.setUint32(42, offset, true);
		c.set(nome, 46);
		central.push(c);
		offset += local.length;
	}
	const tamCentral = central.reduce((t, c) => t + c.length, 0);
	const fim = new Uint8Array(22);
	const fv = new DataView(fim.buffer);
	fv.setUint32(0, 0x06054b50, true);
	fv.setUint16(8, entradas.length, true);
	fv.setUint16(10, entradas.length, true);
	fv.setUint32(12, tamCentral, true);
	fv.setUint32(16, offset, true);
	const partes = [...locais, ...central, fim];
	const out = new Uint8Array(partes.reduce((t, p) => t + p.length, 0));
	let i = 0;
	for (const p of partes) {
		out.set(p, i);
		i += p.length;
	}
	return out.buffer;
}

const texto = (s: string) => new TextEncoder().encode(s);
const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 1, 2, 3]);

describe('lerZip', () => {
	it('lê o comprimido e o guardado, com acento no nome', async () => {
		const arquivos = await lerZip(
			zip([
				{ nome: 'ciência.md', dados: texto('# Ciência\nslug: ciencia\n\n- Ramo\n'), comprimir: true },
				{ nome: 'ciencia/figura.png', dados: PNG, comprimir: false }
			])
		);
		expect(new TextDecoder().decode(arquivos.get('ciência.md'))).toBe('# Ciência\nslug: ciencia\n\n- Ramo\n');
		expect(arquivos.get('ciencia/figura.png')).toEqual(PNG);
	});

	it('recusa o que não é .zip', async () => {
		await expect(lerZip(texto('# só um texto').buffer as ArrayBuffer)).rejects.toThrow('não é um .zip');
	});
});

describe('pacotesDoZip', () => {
	it('junta a cada mapa as questões, o estudo e só as imagens dele', () => {
		const arquivos = new Map<string, Uint8Array>([
			['rede.md', texto('# Rede')],
			['rede.questoes.json', texto('{"mapa":"rede"}')],
			['rede.conta.json', texto('{"versao":1}')],
			['rede/a.png', PNG],
			['redes.md', texto('# Redes')],
			['redes/b.png', PNG],
			['README.md', texto('# Mapas mentais: o formato')]
		]);
		const { pacotes, ignorados } = pacotesDoZip(arquivos);
		expect(pacotes.map((p) => p.nome)).toEqual(['rede', 'redes']);
		const [rede, redes] = pacotes;
		expect(rede.mapa).toBe('# Rede');
		expect(rede.questoes).toBe('{"mapa":"rede"}');
		expect(rede.conta).toBe('{"versao":1}');
		expect(rede.imagens.map((i) => i.nome)).toEqual(['a.png']);
		expect(redes.questoes).toBeUndefined();
		expect(redes.imagens.map((i) => i.nome)).toEqual(['b.png']);
		expect(ignorados).toEqual(['README.md']);
	});
});
