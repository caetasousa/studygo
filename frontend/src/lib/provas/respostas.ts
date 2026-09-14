// Onde ficam as respostas do estudante: no navegador, uma chave por prova. O
// servidor não guarda o que ele marcou (é outra entrega, com banco próprio).
// A prova inteira e o treino por matéria leem e gravam o mesmo lugar, então a
// questão resolvida num aparece resolvida no outro.

import { browser } from '$app/environment';
import { chave } from '$lib/storageKey';
import type { Respostas } from './resolucao';

function chaveDaProva(provaId: string): string {
	return chave(`.provas.${provaId}.respostas`);
}

export function lerRespostas(provaId: string): Respostas {
	if (!browser) return {};
	try {
		return JSON.parse(localStorage.getItem(chaveDaProva(provaId)) ?? '{}') as Respostas;
	} catch {
		return {};
	}
}

export function gravarRespostas(provaId: string, respostas: Respostas): void {
	if (!browser) return;
	try {
		localStorage.setItem(chaveDaProva(provaId), JSON.stringify(respostas));
	} catch {
		// Sem armazenamento (aba anônima cheia, bloqueio): segue só em memória.
	}
}
