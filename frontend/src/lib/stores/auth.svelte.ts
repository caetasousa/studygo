import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { esquecerPorPrefixo } from '$lib/storageKey';
import type { AuthResponse, Usuario } from '$lib/types';

/**
 * A sessão não é mais gravada no navegador.
 *
 * O refresh token vive num cookie HttpOnly — fora do alcance de qualquer
 * script, inclusive deste — e o access token fica só em memória, pelos 15
 * minutos que dura. Um XSS deixa de encontrar credencial guardada: consegue
 * agir enquanto a aba está aberta, não levar a conta embora por 30 dias.
 *
 * O preço é que, a cada carga da página, o app começa sem saber quem está
 * logado. Daí `iniciar()`: uma renovação ANTES de decidir qualquer rota, e um
 * `pronto` que o layout espera — sem ele, quem tem sessão válida veria a tela
 * de login piscar a cada F5.
 */

/** As chaves em que a sessão era gravada antes do cookie. */
const LEGADO = '.auth.';

class AuthStore {
	accessToken = $state<string | null>(null);
	usuario = $state<Usuario | null>(null);

	/** Falso até a primeira renovação responder — ver `iniciar()`. */
	pronto = $state(!browser);

	private renovando: Promise<boolean> | null = null;
	private iniciando: Promise<void> | null = null;

	get isAuthenticated(): boolean {
		return this.accessToken !== null;
	}

	private aplicar(res: AuthResponse) {
		this.accessToken = res.accessToken;
		this.usuario = res.usuario;
	}

	/**
	 * Restaura a sessão a partir do cookie. Roda uma vez por carga da página, e
	 * chamadas repetidas dividem a mesma promessa.
	 */
	async iniciar(): Promise<void> {
		if (!browser) return;
		if (this.iniciando) return this.iniciando;

		this.iniciando = (async () => {
			// Quem entrou na versão anterior tem os dois tokens no localStorage.
			// Eles não servem mais para nada — o backend só aceita o cookie — e
			// deixá-los ali seria manter exposto justamente o que esta mudança
			// veio esconder. O custo é um login a mais, uma única vez.
			esquecerPorPrefixo(LEGADO);

			await this.refresh();
			this.pronto = true;
		})();

		return this.iniciando;
	}

	async login(email: string, senha: string): Promise<void> {
		const res = await fetch('/api/auth/login', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			credentials: 'same-origin',
			body: JSON.stringify({ email, senha })
		});
		await this.receber(res);
	}

	async register(email: string, nome: string, senha: string): Promise<void> {
		const res = await fetch('/api/auth/register', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			credentials: 'same-origin',
			body: JSON.stringify({ email, nome, senha })
		});
		await this.receber(res);
	}

	private async receber(res: Response) {
		const body = await res.json().catch(() => ({}));
		if (!res.ok) {
			throw new Error(body.erro ?? 'Não foi possível autenticar');
		}
		this.aplicar(body as AuthResponse);
		this.pronto = true;
	}

	/**
	 * Renova o access token apresentando o cookie. Não há corpo a mandar: a
	 * credencial é o cookie, e o navegador a anexa sozinho.
	 *
	 * Chamadas concorrentes dividem uma única ida — e aqui isso deixou de ser
	 * só economia: como o servidor GIRA o refresh, duas renovações em paralelo
	 * fariam a segunda chegar com um token que a primeira já invalidou.
	 */
	async refresh(): Promise<boolean> {
		if (this.renovando) return this.renovando;

		this.renovando = (async () => {
			try {
				const res = await fetch('/api/auth/refresh', {
					method: 'POST',
					credentials: 'same-origin'
				});
				if (!res.ok) {
					this.clear();
					return false;
				}
				this.aplicar((await res.json()) as AuthResponse);
				return true;
			} catch {
				// Falha de rede não é sessão inválida: o que está em memória
				// continua valendo e a próxima chamada tenta de novo.
				return false;
			} finally {
				this.renovando = null;
			}
		})();

		return this.renovando;
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
		} catch {
			/* o tema local continua valendo; a próxima carga reconcilia */
		}
	}

	async logout(): Promise<void> {
		this.clear();

		// Sem corpo: o servidor revoga o que o cookie apontar e manda o
		// navegador apagá-lo. É o único jeito de encerrar a sessão de verdade —
		// o token não passa mais por aqui para ser esquecido na mão.
		await fetch('/api/auth/logout', {
			method: 'POST',
			credentials: 'same-origin'
		}).catch(() => {});

		await goto('/login');
	}

	clear() {
		this.accessToken = null;
		this.usuario = null;
	}
}

export const auth = new AuthStore();
