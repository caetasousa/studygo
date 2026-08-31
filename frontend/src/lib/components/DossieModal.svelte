<script lang="ts">
	import IconButton from '$lib/components/IconButton.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { Dossie } from '$lib/types';

	let { codigo, onclose }: { codigo: string; onclose: () => void } = $props();

	let dossie = $state<Dossie | null>(null);
	let erro = $state<string | null>(null);
	let copiado = $state<'' | 'texto' | 'links'>('');

	$effect(() => {
		const c = codigo;
		dossie = null;
		erro = null;
		planoStore
			.dossie(c)
			.then((d) => (dossie = d))
			.catch((e) => (erro = e instanceof Error ? e.message : 'Erro'));
	});

	async function copiar(o: 'texto' | 'links') {
		if (!dossie) return;
		const txt =
			o === 'texto' ? dossie.markdown : dossie.fontes.map((f) => f.url).filter(Boolean).join('\n');
		try {
			await navigator.clipboard.writeText(txt);
			copiado = o;
			setTimeout(() => (copiado = ''), 1500);
		} catch {
			/* clipboard blocked */
		}
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
<div
	style="position:fixed;inset:0;background:rgba(20,18,14,.45);z-index:60;display:grid;place-items:center;padding:20px"
	onclick={onclose}
>
	<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
	<div
		class="card"
		style="max-width:640px;width:100%;max-height:88vh;overflow:auto"
		onclick={(e) => e.stopPropagation()}
	>
		<div class="card-top">
			Dossiê para o NotebookLM {#if dossie}· {dossie.disciplina}{/if}
			<span style="margin-left:auto">
				<IconButton icon="fechar" label="Fechar" onclick={onclose} />
			</span>
		</div>
		<div class="card-body">
			{#if erro}
				<div class="form-error">{erro}</div>
			{:else if !dossie}
				<p class="page-sub" style="margin:0">Montando…</p>
			{:else}
				<ol style="font-size:13px;color:var(--text-muted);line-height:1.7;padding-left:18px;margin:0 0 14px">
					<li>Abra o <a href="https://notebooklm.google.com" target="_blank" rel="noreferrer">NotebookLM</a> e crie um notebook.</li>
					<li>Cole o texto abaixo como uma fonte (Adicionar fonte → Texto copiado).</li>
					<li>Cole os links abaixo como fontes (um por linha).</li>
					<li>Peça um <b>Guia de Estudos</b> e um <b>Áudio Overview em português</b>.</li>
				</ol>

				<div style="display:flex;gap:8px;margin-bottom:8px">
					<button class="btn" onclick={() => copiar('texto')}>
						{copiado === 'texto' ? '✓ copiado' : 'Copiar texto'}
					</button>
					{#if dossie.fontes.some((f) => f.url)}
						<button class="btn" onclick={() => copiar('links')}>
							{copiado === 'links' ? '✓ copiado' : 'Copiar links'}
						</button>
					{/if}
					<a class="btn primary" href="https://notebooklm.google.com" target="_blank" rel="noreferrer"
						>Abrir NotebookLM</a
					>
				</div>

				<pre
					style="background:var(--bg-soft);border:1px solid var(--border);border-radius:8px;padding:12px;font-family:var(--font-mono);font-size:11.5px;white-space:pre-wrap;overflow-x:auto;max-height:40vh;overflow-y:auto">{dossie.markdown}</pre>
			{/if}
		</div>
	</div>
</div>
