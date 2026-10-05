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

// Abrir e fechar o ramo remonta a imagem: o arquivo baixado fica guardado
// enquanto a versão do envio for a mesma.
const baixadas = new Map<string, Promise<Blob>>();

export function baixarImagem(slug: string, nome: string, versao: number): Promise<Blob> {
	const chave = `${slug}\n${nome}\n${versao}`;
	let blob = baixadas.get(chave);
	if (!blob) {
		blob = api.imagemDoMapa(slug, nome);
		blob.catch(() => baixadas.delete(chave));
		baixadas.set(chave, blob);
	}
	return blob;
}
