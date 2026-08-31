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
