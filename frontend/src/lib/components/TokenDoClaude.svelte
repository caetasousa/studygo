<script lang="ts">
	import { api } from '$lib/api';
	import { confirmar } from '$lib/stores/confirmacao.svelte';

	/**
	 * O token do Claude que o processador de mapas usa para fazer os mapas com a sua
	 * assinatura. Ele entra aqui uma vez e nunca volta inteiro para a tela: o
	 * servidor o guarda cifrado e mostra só o fim, para você reconhecer qual é.
	 */
	let situacao = $state<{ configurado: boolean; fim: string } | null>(null);
	let token = $state('');
	let salvando = $state(false);
	let erro = $state<string | null>(null);
	let salvo = $state(false);

	async function carregar() {
		try {
			situacao = await api.situacaoDoTokenDoClaude();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível ler o token guardado';
		}
	}
	void carregar();

	async function salvar() {
		erro = null;
		salvo = false;
		salvando = true;
		try {
			await api.guardarTokenDoClaude(token);
			token = '';
			salvo = true;
			await carregar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível guardar o token';
		} finally {
			salvando = false;
		}
	}

	async function remover() {
		const ok = await confirmar({
			titulo: 'Remover o token do Claude?',
			texto: 'O processador de mapas passa a usar a conexão do Claude desta conta.',
			rotulo: 'Remover',
			tom: 'perigo'
		});
		if (!ok) return;
		erro = null;
		salvo = false;
		try {
			await api.removerTokenDoClaude();
			await carregar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível remover o token';
		}
	}
</script>

<p class="page-sub" style="margin-top:0">
	Quem já tem um token de <code>claude setup-token</code> pode colá-lo aqui: ele vale no lugar da conexão. Fica
	guardado cifrado e não aparece de novo.
</p>

<p class="estado" role="status">
	{#if situacao === null}
		Carregando…
	{:else if situacao.configurado}
		Token guardado, terminado em <code>{situacao.fim}</code>.
	{:else}
		Nenhum token guardado: o processador usa a conexão acima.
	{/if}
</p>

{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}
{#if salvo}<div class="callout" role="status">Token guardado.</div>{/if}

<form
	class="linha"
	onsubmit={(e) => {
		e.preventDefault();
		void salvar();
	}}
>
	<label class="campo">
		<span>{situacao?.configurado ? 'Trocar o token do Claude' : 'Token do Claude'}</span>
		<input
			type="password"
			autocomplete="off"
			spellcheck="false"
			aria-label="Token do Claude"
			bind:value={token}
			disabled={salvando}
		/>
	</label>
	<button type="submit" class="btn primary" disabled={salvando || token.trim() === ''}>
		{salvando ? 'Guardando…' : 'Guardar token'}
	</button>
	{#if situacao?.configurado}
		<button type="button" class="btn danger" onclick={remover}>Remover</button>
	{/if}
</form>

<style>
	.estado {
		margin: 10px 0;
		font-size: 13.5px;
	}
	code {
		font-family: var(--font-mono);
		font-size: 12px;
		padding: 1px 5px;
		border-radius: 4px;
		background: var(--bg-soft);
	}
	.linha {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 10px;
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
