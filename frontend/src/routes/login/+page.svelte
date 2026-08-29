<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	let email = $state('');
	let senha = $state('');
	let erro = $state<string | null>(null);
	let enviando = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		erro = null;
		enviando = true;
		try {
			await auth.login(email, senha);
			await goto('/');
		} catch (err) {
			erro = err instanceof Error ? err.message : 'Falha no login';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="auth-wrap">
	<div class="auth-card">
		<h1><span>🏛️</span> Entrar</h1>
		<p class="sub">Seu plano de estudos para o concurso, salvo na nuvem.</p>

		<form class="auth-form" onsubmit={submit}>
			{#if erro}<div class="form-error">{erro}</div>{/if}
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
					autocomplete="current-password"
				/>
			</div>
			<button class="btn primary" type="submit" disabled={enviando}>
				{enviando ? 'Entrando…' : 'Entrar'}
			</button>
		</form>

		<p class="auth-swap">
			Não tem conta? <a href="/registro">Criar conta</a>
		</p>
	</div>
</div>
