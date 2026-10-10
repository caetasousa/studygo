/**
 * O .zip que a exportação dos mapas gera, aberto na tela para voltar mapa a
 * mapa: o de todos passa do que o túnel aceita num envio só (100 MB), e cada
 * mapa, sozinho, cabe.
 *
 * Sem biblioteca: o diretório central do .zip diz os nomes e onde está cada
 * arquivo, e o navegador já descomprime `deflate`. Basta para o .zip que o
 * próprio app escreve (sem zip64: até 65 mil arquivos e 4 GB) e para o que se
 * faz com o compactador do sistema.
 */

const ASSINATURA_DO_FIM = 0x06054b50;
const ASSINATURA_CENTRAL = 0x02014b50;
const ASSINATURA_LOCAL = 0x04034b50;

/** Abre o .zip: nome do arquivo → conteúdo. As pastas ficam de fora. */
export async function lerZip(buffer: ArrayBuffer): Promise<Map<string, Uint8Array>> {
	const bytes = new Uint8Array(buffer);
	const dv = new DataView(buffer);

	// O registro do fim fica nos últimos 22 bytes, mais um comentário de até 64 KB.
	let fim = -1;
	for (let i = bytes.length - 22; i >= Math.max(0, bytes.length - 22 - 0xffff); i--) {
		if (dv.getUint32(i, true) === ASSINATURA_DO_FIM) {
			fim = i;
			break;
		}
	}
	if (fim < 0) throw new Error('o arquivo não é um .zip');

	const total = dv.getUint16(fim + 10, true);
	let p = dv.getUint32(fim + 16, true);
	const nomes = new TextDecoder('utf-8');
	const out = new Map<string, Uint8Array>();

	for (let n = 0; n < total; n++) {
		if (dv.getUint32(p, true) !== ASSINATURA_CENTRAL) throw new Error('o .zip está corrompido');
		const metodo = dv.getUint16(p + 10, true);
		const comprimido = dv.getUint32(p + 20, true);
		const tamNome = dv.getUint16(p + 28, true);
		const tamExtra = dv.getUint16(p + 30, true);
		const tamComentario = dv.getUint16(p + 32, true);
		const local = dv.getUint32(p + 42, true);
		const nome = nomes.decode(bytes.subarray(p + 46, p + 46 + tamNome));
		p += 46 + tamNome + tamExtra + tamComentario;

		if (nome.endsWith('/')) continue;
		if (dv.getUint32(local, true) !== ASSINATURA_LOCAL) throw new Error('o .zip está corrompido');
		// O cabeçalho local tem nome e extra próprios: o dado começa depois deles.
		const inicio = local + 30 + dv.getUint16(local + 26, true) + dv.getUint16(local + 28, true);
		const dado = bytes.subarray(inicio, inicio + comprimido);

		if (metodo === 0) out.set(nome, dado.slice());
		else if (metodo === 8) out.set(nome, await descomprimir(dado));
		else throw new Error(`${nome}: compressão que o app não lê (método ${metodo})`);
	}
	return out;
}

async function descomprimir(dado: Uint8Array): Promise<Uint8Array> {
	const fluxo = new Blob([dado as BlobPart]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
	return new Uint8Array(await new Response(fluxo).arrayBuffer());
}

/** Um mapa do .zip, com tudo o que a exportação levou dele. */
export interface PacoteDoZip {
	/** O caminho do .md sem a extensão: o nome com que o mapa aparece no progresso. */
	nome: string;
	mapa: string;
	questoes?: string;
	conta?: string;
	imagens: { nome: string; dados: Uint8Array }[];
}

const IMAGEM = /\.(png|jpe?g|webp)$/i;

/**
 * Separa o .zip por mapa: `<x>.md` é o mapa, `<x>.questoes.json` as questões,
 * `<x>.conta.json` os vínculos e as respostas, e `<x>/<imagem>` as imagens
 * dele. O README de uma pasta de mapas não é mapa.
 */
export function pacotesDoZip(arquivos: Map<string, Uint8Array>): { pacotes: PacoteDoZip[]; ignorados: string[] } {
	const txt = new TextDecoder('utf-8');
	const pacotes: PacoteDoZip[] = [];
	const ignorados: string[] = [];

	const nomes = [...arquivos.keys()].sort();
	for (const caminho of nomes) {
		if (!/\.(md|markdown)$/i.test(caminho)) continue;
		if (/(^|\/)readme\.(md|markdown)$/i.test(caminho)) {
			ignorados.push(caminho);
			continue;
		}
		const base = caminho.replace(/\.(md|markdown)$/i, '');
		const ler = (n: string) => (arquivos.has(n) ? txt.decode(arquivos.get(n)) : undefined);
		const imagens = nomes
			.filter((n) => n.startsWith(base + '/') && !n.slice(base.length + 1).includes('/') && IMAGEM.test(n))
			.map((n) => ({ nome: n.slice(base.length + 1), dados: arquivos.get(n)! }));
		pacotes.push({
			nome: base,
			mapa: txt.decode(arquivos.get(caminho)),
			questoes: ler(base + '.questoes.json'),
			conta: ler(base + '.conta.json'),
			imagens
		});
	}
	return { pacotes, ignorados };
}
