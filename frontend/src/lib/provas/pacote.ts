// O pacote de provas exportado de outro ambiente: um .zip sem compressão, com
// uma pasta por prova (prova.json, prova.pdf, gabarito.pdf e figuras/). O
// navegador o lê aqui e a importação envia uma prova por vez — o arquivo
// inteiro, com dez cadernos, passaria do limite de envio do servidor.
//
// Só o que o backend escreve (entradas Store, sem zip64) precisa ser lido: é
// isso que dispensa uma biblioteca de zip.

export interface ProvaNoPacote {
	/** A pasta da prova no .zip, como "tjce-2026-e05". */
	pasta: string;
	manifesto: Blob;
	/** Os PDFs e as figuras, pelo nome sem a pasta — o que o prova.json cita. */
	arquivos: { nome: string; conteudo: Blob }[];
}

const FIM_DO_DIRETORIO = 0x06054b50;
const ENTRADA_DO_DIRETORIO = 0x02014b50;
const CABECALHO_LOCAL = 0x04034b50;

export class PacoteInvalido extends Error {}

async function bytes(arquivo: Blob, inicio: number, fim: number): Promise<DataView> {
	return new DataView(await arquivo.slice(inicio, fim).arrayBuffer());
}

/** As provas do pacote, cada uma com os arquivos da sua pasta. */
export async function provasDoPacote(arquivo: Blob): Promise<ProvaNoPacote[]> {
	const naoE = () => new PacoteInvalido('este arquivo não é um pacote de provas exportado pelo studygo');

	// O fim do diretório central fica nos últimos 22 bytes, mais um comentário de até 64 KiB.
	const inicioDoFim = Math.max(0, arquivo.size - 65_557);
	const fim = await bytes(arquivo, inicioDoFim, arquivo.size);
	let eocd = -1;
	for (let i = fim.byteLength - 22; i >= 0; i--) {
		if (fim.getUint32(i, true) === FIM_DO_DIRETORIO) {
			eocd = i;
			break;
		}
	}
	if (eocd < 0) throw naoE();
	const total = fim.getUint16(eocd + 10, true);
	const tamanho = fim.getUint32(eocd + 12, true);
	const inicio = fim.getUint32(eocd + 16, true);
	if (total === 0xffff || inicio === 0xffffffff || inicio + tamanho > arquivo.size) throw naoE();

	const diretorio = await bytes(arquivo, inicio, inicio + tamanho);
	const decodificar = new TextDecoder();
	const entradas = new Map<string, Blob>();
	for (let p = 0, n = 0; n < total; n++) {
		if (p + 46 > diretorio.byteLength || diretorio.getUint32(p, true) !== ENTRADA_DO_DIRETORIO) throw naoE();
		const metodo = diretorio.getUint16(p + 10, true);
		const comprimido = diretorio.getUint32(p + 20, true);
		const tamNome = diretorio.getUint16(p + 28, true);
		const tamExtra = diretorio.getUint16(p + 30, true);
		const tamComentario = diretorio.getUint16(p + 32, true);
		const local = diretorio.getUint32(p + 42, true);
		const nome = decodificar.decode(new Uint8Array(diretorio.buffer, diretorio.byteOffset + p + 46, tamNome));
		p += 46 + tamNome + tamExtra + tamComentario;
		if (nome.endsWith('/')) continue;
		// Comprimido é .zip de outro programa — o pacote foi descompactado e refeito.
		if (metodo !== 0) throw naoE();

		// Os dados começam depois do cabeçalho local, cujo extra pode diferir do diretório.
		const cabecalho = await bytes(arquivo, local, local + 30);
		if (cabecalho.getUint32(0, true) !== CABECALHO_LOCAL) throw naoE();
		const dados = local + 30 + cabecalho.getUint16(26, true) + cabecalho.getUint16(28, true);
		entradas.set(nome, arquivo.slice(dados, dados + comprimido));
	}

	const provas: ProvaNoPacote[] = [];
	for (const [nome, manifesto] of entradas) {
		const m = /^([^/]+)\/prova\.json$/.exec(nome);
		if (!m) continue;
		const pasta = m[1];
		const arquivos = [...entradas]
			.filter(([n]) => n.startsWith(`${pasta}/`) && n !== nome)
			.map(([n, conteudo]) => ({ nome: n.slice(n.lastIndexOf('/') + 1), conteudo }));
		provas.push({ pasta, manifesto, arquivos });
	}
	if (provas.length === 0) throw new PacoteInvalido('este .zip não tem nenhuma prova exportada pelo studygo');

	return provas.sort((a, b) => a.pasta.localeCompare(b.pasta));
}
