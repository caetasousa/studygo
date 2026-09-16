// A lista da curadoria depois de excluir ou renomear uma prova: as importações
// que a publicaram ou revisam andam juntas, porque são a mesma prova.

import type { ImportacaoResumo } from './types';

/** Tira as importações da prova excluída; as outras ficam como estavam. */
export function semAProva(lista: ImportacaoResumo[], provaId: string): ImportacaoResumo[] {
	return lista.filter((i) => i.provaId !== provaId);
}

/** O título novo nas importações publicadas da prova — as em revisão têm o nome que o curador edita lá. */
export function comTitulo(lista: ImportacaoResumo[], provaId: string, cargoNome: string): ImportacaoResumo[] {
	return lista.map((i) => (i.provaId === provaId && i.estado === 'publicada' ? { ...i, cargoNome } : i));
}

/** O título como o servidor vai guardar: espaços repetidos viram um. */
export function tituloDigitado(texto: string): string {
	return texto.split(/\s+/).filter(Boolean).join(' ');
}
