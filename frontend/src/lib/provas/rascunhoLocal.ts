// O que a revisão tem de alterado e ainda não salvo, guardado no navegador. No
// celular, a aba que fica em segundo plano é descartada sem aviso e volta
// recarregada: as conferências marcadas e não salvas sumiam, e parecia que a
// revisão tinha se desfeito sozinha.

import { browser } from '$app/environment';
import { chave, esquecerPorPrefixo, type Armazenamento } from '$lib/storageKey';
import type { Rascunho } from './types';

export interface RascunhoLocal {
	versao: number;
	rascunho: Rascunho;
}

const PREFIXO = '.provas.revisao.';

function chaveDaRevisao(id: string): string {
	return chave(`${PREFIXO}${id}`);
}

export function guardarEm(st: Armazenamento, id: string, copia: RascunhoLocal): void {
	try {
		st.setItem(chaveDaRevisao(id), JSON.stringify(copia));
	} catch {
		// Sem espaço ou sem armazenamento: a cópia é só uma rede de segurança.
	}
}

export function lerDe(st: Armazenamento, id: string): RascunhoLocal | null {
	try {
		const copia = JSON.parse(st.getItem(chaveDaRevisao(id)) ?? 'null') as RascunhoLocal | null;
		return copia && Number.isInteger(copia.versao) && Array.isArray(copia.rascunho?.questoes) ? copia : null;
	} catch {
		return null;
	}
}

export function apagarDe(st: Armazenamento, id: string): void {
	try {
		st.removeItem(chaveDaRevisao(id));
	} catch {
		/* nada guardado, nada a apagar */
	}
}

/**
 * O que fazer com a cópia ao abrir a revisão. Ela só vale sobre a mesma versão
 * que estava aberta: se a revisão foi salva depois (outra aba, outro aparelho),
 * aplicar a cópia desfaria o que foi salvo.
 */
export function destinoDaCopia(
	copia: RascunhoLocal | null,
	servidor: RascunhoLocal
): 'nenhuma' | 'recuperar' | 'descartar' {
	if (!copia) return 'nenhuma';
	if (copia.versao !== servidor.versao) return 'descartar';
	return JSON.stringify(copia.rascunho) === JSON.stringify(servidor.rascunho) ? 'nenhuma' : 'recuperar';
}

function local(): Storage | null {
	try {
		return browser ? localStorage : null;
	} catch {
		return null;
	}
}

export const guardarRascunhoLocal = (id: string, copia: RascunhoLocal) => {
	const st = local();
	if (st) guardarEm(st, id, copia);
};
export const lerRascunhoLocal = (id: string) => {
	const st = local();
	return st ? lerDe(st, id) : null;
};
export const apagarRascunhoLocal = (id: string) => {
	const st = local();
	if (st) apagarDe(st, id);
};
/** No logout: a revisão de quem saiu não fica no navegador. */
export const esquecerRascunhosLocais = () => esquecerPorPrefixo(PREFIXO);
