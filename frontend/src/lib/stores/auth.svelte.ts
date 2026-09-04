import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { chave, lerMigrando } from '$lib/storageKey';
import type { AuthResponse, Usuario } from '$lib/types';

const SUFIXO = '.auth.v1';
const STORAGE_KEY = chave(SUFIXO);

interface Persisted {
	accessToken: string;
	refreshToken: string;
	usuario: Usuario;
}

function load(): Persisted | null {
	if (!browser) return null;
	try {
		const raw = lerMigrando(SUFIXO);
		return raw ? (JSON.parse(raw) as Persisted) : null;
	} catch {
		return null;
	}
}

const initial = load();

class AuthStore {
	accessToken = $state<string | null>(initial?.accessToken ?? null);
	refreshToken = $state<string | null>(initial?.refreshToken ?? null);
	usuario = $state<Usuario | null>(initial?.usuario ?? null);

	private refreshing: Promise<boolean> | null = null;

	get isAuthenticated(): boolean {
		return this.accessToken !== null;
	}

	private persist() {
		if (!browser) return;
		try {
			if (this.accessToken && this.refreshToken && this.usuario) {
				localStorage.setItem(
					STORAGE_KEY,
					JSON.stringify({
						accessToken: this.accessToken,
						refreshToken: this.refreshToken,
						usuario: this.usuario
					})
				);
			} else {
				localStorage.removeItem(STORAGE_KEY);
			}
		} catch {
			/* private mode — session-only */
		}
	}

	private apply(res: AuthResponse) {
		this.accessToken = res.accessToken;
		this.refreshToken = res.refreshToken;
		this.usuario = res.usuario;
		this.persist();
	}

	async login(email: string, senha: string): Promise<void> {
		const res = await fetch('/api/auth/login', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ email, senha })
		});
		await this.handleAuth(res);
	}

	async register(email: string, nome: string, senha: string): Promise<void> {
		const res = await fetch('/api/auth/register', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ email, nome, senha })
		});
		await this.handleAuth(res);
	}

	private async handleAuth(res: Response) {
		const body = await res.json().catch(() => ({}));
		if (!res.ok) {
			throw new Error(body.erro ?? 'Não foi possível autenticar');
		}
		this.apply(body as AuthResponse);
	}

	/** Refresh the access token. Concurrent callers share one in-flight request. */
	async refresh(): Promise<boolean> {
		if (this.refreshing) return this.refreshing;
		if (!this.refreshToken) return false;

		this.refreshing = (async () => {
			try {
				const res = await fetch('/api/auth/refresh', {
					method: 'POST',
					headers: { 'content-type': 'application/json' },
					body: JSON.stringify({ refreshToken: this.refreshToken })
				});
				if (!res.ok) {
					this.clear();
					return false;
				}
				const body = await res.json();
				this.accessToken = body.accessToken;
				this.refreshToken = body.refreshToken;
				this.persist();
				return true;
			} catch {
				return false;
			} finally {
				this.refreshing = null;
			}
		})();

		return this.refreshing;
	}

	/**
	 * Grava a preferência visual da conta. O tema é do USUÁRIO, não do plano:
	 * quem estuda para dois concursos não quer dois temas.
	 *
	 * A tela já aplicou o tema localmente; aqui só se persiste, e uma falha de
	 * rede não desfaz o que o usuário acabou de ver.
	 */
	async definirTema(temaUi: string): Promise<void> {
		try {
			const { api } = await import('$lib/api');
			this.usuario = await api.definirTema(temaUi);
			this.persist();
		} catch {
			/* o tema local continua valendo; a próxima carga reconcilia */
		}
	}

	async logout(): Promise<void> {
		const token = this.refreshToken;
		this.clear();
		if (token) {
			await fetch('/api/auth/logout', {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ refreshToken: token })
			}).catch(() => {});
		}
		await goto('/login');
	}

	clear() {
		this.accessToken = null;
		this.refreshToken = null;
		this.usuario = null;
		this.persist();
	}
}

export const auth = new AuthStore();
