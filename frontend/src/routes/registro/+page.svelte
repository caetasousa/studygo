<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	let nome = $state('');
	let email = $state('');
	let senha = $state('');
	let erro = $state<string | null>(null);
	let enviando = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		erro = null;
		enviando = true;
		try {
			await auth.register(email, nome, senha);
			await goto('/');
		} catch (err) {
			erro = err instanceof Error ? err.message : 'Falha no cadastro';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="auth-wrap">
	<div class="auth-card">
		<h1><span>🏛️</span> Criar conta</h1>
		<p class="sub">Leva 20 segundos. O plano começa com as datas do edital.</p>

		<form class="auth-form" onsubmit={submit}>
			{#if erro}<div class="form-error">{erro}</div>{/if}
			<div class="field">
				<label for="nome">Nome</label>
				<input id="nome" type="text" bind:value={nome} required autocomplete="name" />
			</div>
			<div class="field">
				<label for="email">Email</label>
				<input id="email" type="email" bind:value={email} required autocomplete="email" />
			</div>
			<div class="field">
				<label for="senha">Senha</label>
				<input
					id="senha"
					type="password"
					bind:value={senha}
					required
					minlength="8"
					autocomplete="new-password"
				/>
			</div>
			<button class="btn primary" type="submit" disabled={enviando}>
				{enviando ? 'Criando…' : 'Criar conta'}
			</button>
		</form>

		<p class="auth-swap">Já tem conta? <a href="/login">Entrar</a></p>
	</div>
</div>
