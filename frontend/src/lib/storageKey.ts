import { browser } from '$app/environment';

/**
 * Browser-storage keys, and the one-time migration from the project's old name.
 *
 * The app used to be called annyGo, so everything it persisted is under an
 * `annygo.*` / `annygo:*` prefix. Renaming the keys outright would sign every
 * existing user out and drop their cached plan — a rename of the project should
 * cost the user nothing, so the old value is moved across on first read and the
 * stale key removed.
 *
 * This can be deleted once no browser is likely to hold the old keys.
 */
const PREFIXO_ANTIGO = 'annygo';
const PREFIXO = 'studygo';

/** Translates a key from the old prefix to the current one. */
export function chave(nome: string): string {
	return `${PREFIXO}${nome}`;
}

function chaveAntiga(nome: string): string {
	return `${PREFIXO_ANTIGO}${nome}`;
}

/** The slice of the Storage API this needs — so the move can be tested. */
export interface Armazenamento {
	getItem(k: string): string | null;
	setItem(k: string, v: string): void;
	removeItem(k: string): void;
}

/**
 * The migration itself, against a given storage.
 *
 * Exported for the tests: `lerMigrando` is the same thing bound to the
 * browser's localStorage, and cannot run outside one.
 */
export function migrar(st: Armazenamento, nome: string): string | null {
	try {
		const atual = st.getItem(chave(nome));
		if (atual !== null) return atual;

		const antigo = st.getItem(chaveAntiga(nome));
		if (antigo === null) return null;

		st.setItem(chave(nome), antigo);
		st.removeItem(chaveAntiga(nome));

		return antigo;
	} catch {
		return null;
	}
}

/**
 * Reads a key, adopting whatever the old name still holds.
 *
 * The move is done once: after it, only the new key exists. A storage that
 * throws (private mode, blocked site data) simply yields null, as before.
 */
export function lerMigrando(nome: string): string | null {
	if (!browser) return null;

	return migrar(localStorage, nome);
}

/** Um armazenamento que dá para percorrer — o que a limpeza por prefixo exige. */
export interface ArmazenamentoIteravel extends Armazenamento {
	readonly length: number;
	key(i: number): string | null;
}

/**
 * Apaga, do armazenamento dado, toda chave que comece por `inicio` — nos dois
 * prefixos. Devolve as chaves removidas.
 *
 * Exportada pelo mesmo motivo que `migrar`: é a parte testável, e
 * `esquecerPorPrefixo` é ela amarrada ao localStorage do navegador.
 */
export function esquecerEm(st: ArmazenamentoIteravel, inicio: string): string[] {
	const alvos = [`${PREFIXO}${inicio}`, `${PREFIXO_ANTIGO}${inicio}`];
	const remover: string[] = [];

	for (let i = 0; i < st.length; i++) {
		const atual = st.key(i);
		if (atual !== null && alvos.some((a) => atual.startsWith(a))) {
			remover.push(atual);
		}
	}

	// Remove só depois de listar: apagar durante a iteração renumera os índices
	// e faz a varredura pular chaves.
	for (const k of remover) st.removeItem(k);

	return remover;
}

/**
 * Apaga toda chave que comece por `inicio`.
 *
 * Existe por causa do logout: o plano fica em cache sob
 * `studygo.plano.<slug>.v1`, uma chave por concurso, e nada apagava nenhuma
 * delas. Num navegador compartilhado, o histórico de estudo de quem saiu
 * continuava no disco — e ia se acumulando, concurso após concurso.
 *
 * Varre o prefixo antigo junto: quem não reabre um concurso desde a renomeação
 * do projeto ainda tem a chave `annygo.*` parada lá.
 */
export function esquecerPorPrefixo(inicio: string): void {
	if (!browser) return;

	try {
		esquecerEm(localStorage, inicio);
	} catch {
		/* modo privado ou site data bloqueado — nada a esquecer */
	}
}
