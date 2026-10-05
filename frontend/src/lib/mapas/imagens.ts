import { getContext, setContext } from 'svelte';
import { api } from '$lib/api';

/**
 * O que a imagem de um item precisa saber do mapa aberto: de qual mapa ela é,
 * quais arquivos já chegaram e a versão do envio. A página fornece; cada item
 * de imagem, fundo na árvore, lê daqui em vez de receber de mão em mão.
 */
export interface ImagensDoMapa {
	readonly slug: string;
	readonly enviadas: string[];
	/** Sobe a cada envio: a imagem trocada é buscada de novo. */
	readonly versao: number;
}

const CHAVE = Symbol('imagens-do-mapa');

export function fornecerImagens(imagens: ImagensDoMapa) {
	setContext(CHAVE, imagens);
}

export function imagensDoMapa(): ImagensDoMapa | undefined {
	return getContext<ImagensDoMapa | undefined>(CHAVE);
}

// A imagem vira um endereço data:, e não blob:: a política de segurança da
// borda (img-src 'self' data:) bloqueia blob:, e a tela ficava em branco no ar.
// Abrir e fechar o ramo remonta a imagem: o arquivo baixado fica guardado
// enquanto a versão do envio for a mesma.
const baixadas = new Map<string, Promise<string>>();

function paraDataURL(blob: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const leitor = new FileReader();
		leitor.onload = () => resolve(leitor.result as string);
		leitor.onerror = () => reject(leitor.error);
		leitor.readAsDataURL(blob);
	});
}

export function baixarImagem(slug: string, nome: string, versao: number): Promise<string> {
	const chave = `${slug}\n${nome}\n${versao}`;
	let url = baixadas.get(chave);
	if (!url) {
		url = api.imagemDoMapa(slug, nome).then(paraDataURL);
		url.catch(() => baixadas.delete(chave));
		baixadas.set(chave, url);
	}
	return url;
}
