<script lang="ts">
	import { api } from '$lib/api';
	import { confirmar } from '$lib/stores/confirmacao.svelte';
	import type { ConexaoDoClaude } from '$lib/types';

	/**
	 * A assinatura do Claude que faz os mapas desta conta, conectada pela tela:
	 * o servidor abre o login do Claude Code, a tela mostra o link de
	 * autorização, e o código que a página do Claude mostra volta por aqui.
	 * Nada de terminal. A conexão é desta conta, e só dela.
	 */
	let conexao = $state<ConexaoDoClaude | null>(null);
	let link = $state<string | null>(null);
	let codigo = $state('');
	let ocupado = $state(false);
	let erro = $state<string | null>(null);

	const planos: Record<string, string> = { max: 'Max', pro: 'Pro', team: 'Team', enterprise: 'Enterprise' };

	async function carregar() {
		try {
			conexao = await api.conexaoDoClaude();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível ler a conexão do Claude';
		}
	}
	void carregar();

	async function conectar() {
		erro = null;
		ocupado = true;
		try {
			link = (await api.conectarClaude()).url;
			codigo = '';
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível começar a conexão';
		} finally {
			ocupado = false;
		}
	}

	async function concluir() {
		erro = null;
		ocupado = true;
		try {
			conexao = await api.concluirConexaoDoClaude(codigo);
			link = null;
			codigo = '';
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível concluir a conexão';
		} finally {
			ocupado = false;
		}
	}

	async function desconectar() {
		const ok = await confirmar({
			titulo: 'Desconectar o Claude?',
			texto: 'Os PDFs desta conta param de virar mapa até você conectar de novo.',
			rotulo: 'Desconectar',
			tom: 'perigo'
		});
		if (!ok) return;
		erro = null;
		try {
			await api.desconectarClaude();
			await carregar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível desconectar';
		}
	}
</script>

<p class="page-sub" style="margin-top:0">
	O processador transforma o PDF da aula em mapa com a sua assinatura do Claude. Conecte-a uma vez: autorize na página
	do Claude e cole aqui o código que ela mostrar.
</p>

<p class="estado" role="status">
	{#if conexao === null}
		Carregando…
	{:else if conexao.conectado}
		Claude conectado{#if conexao.email}&nbsp;como <strong>{conexao.email}</strong>{/if}{#if planos[conexao.plano]}&nbsp;(plano {planos[conexao.plano]}){/if}.
	{:else}
		Claude não conectado: os PDFs desta conta não viram mapa.
	{/if}
</p>

{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

{#if conexao?.conectado}
	<button type="button" class="btn danger" onclick={desconectar}>Desconectar</button>
{:else if link}
	<ol class="passos">
		<li>
			<a class="btn primary" href={link} target="_blank" rel="noopener noreferrer">Abrir a página de autorização ↗</a>
			<span class="dica">Entre com a sua conta do Claude e clique em <em>Authorize</em>.</span>
		</li>
		<li>
			<form
				class="linha"
				onsubmit={(e) => {
					e.preventDefault();
					void concluir();
				}}
			>
				<label class="campo">
					<span>Código que a página mostrou</span>
					<input
						type="text"
						autocomplete="off"
						spellcheck="false"
						aria-label="Código de autorização do Claude"
						bind:value={codigo}
						disabled={ocupado}
					/>
				</label>
				<button type="submit" class="btn primary" disabled={ocupado || codigo.trim() === ''}>
					{ocupado ? 'Conectando…' : 'Concluir conexão'}
				</button>
				<button type="button" class="btn" onclick={() => ((link = null), (erro = null))} disabled={ocupado}>
					Cancelar
				</button>
			</form>
		</li>
	</ol>
{:else if conexao}
	<button type="button" class="btn primary" onclick={conectar} disabled={ocupado}>
		{ocupado ? 'Abrindo…' : 'Conectar o Claude'}
	</button>
{/if}

<style>
	.estado {
		margin: 10px 0;
		font-size: 13.5px;
	}
	.passos {
		display: grid;
		gap: 14px;
		margin: 8px 0 0;
		padding-left: 20px;
	}
	.passos li {
		display: grid;
		gap: 6px;
		justify-items: start;
	}
	.dica {
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.linha {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 10px;
		width: 100%;
	}
	.campo {
		display: grid;
		gap: 5px;
		flex: 1 1 260px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	input {
		box-sizing: border-box;
		width: 100%;
		min-height: 36px;
		padding: 7px 10px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	@media (pointer: coarse) {
		input {
			font-size: 16px;
			min-height: 44px;
		}
	}
</style>
