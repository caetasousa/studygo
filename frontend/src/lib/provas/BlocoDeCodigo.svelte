<script lang="ts">
	let {
		texto,
		linguagem = '',
		copiavel = true
	}: {
		texto: string;
		linguagem?: string;
		/** Dentro de uma alternativa clicável não cabe outro botão. */
		copiavel?: boolean;
	} = $props();

	let copiado = $state(false);

	async function copiar() {
		try {
			await navigator.clipboard.writeText(texto);
			copiado = true;
			setTimeout(() => (copiado = false), 1500);
		} catch {
			// Sem permissão de área de transferência: o texto continua selecionável.
		}
	}
</script>

<div class="codigo" class:compacto={!linguagem && !copiavel}>
	<!-- Sem linguagem nem botão, o cabeçalho só repetiria "código" em cada alternativa. -->
	{#if linguagem || copiavel}
		<div class="topo">
			<span>{linguagem || 'código'}</span>
			{#if copiavel}
				<button type="button" onclick={copiar}>{copiado ? 'Copiado' : 'Copiar'}</button>
			{/if}
		</div>
	{/if}
	<pre><code>{texto}</code></pre>
</div>

<style>
	.codigo {
		margin: 4px 0 12px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
	}
	.topo {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 5px 10px 0 14px;
		font-family: var(--font-mono);
		font-size: 10.5px;
		letter-spacing: 0.04em;
		color: var(--text-faint);
	}
	.topo button {
		font: inherit;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		padding: 3px 4px;
		cursor: pointer;
	}
	.topo button:hover {
		color: var(--text);
	}
	.compacto {
		margin: 0 0 10px;
	}
	.compacto pre {
		padding: 8px 12px;
	}
	pre {
		margin: 0;
		padding: 6px 14px 12px;
		overflow-x: auto;
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 1.55;
		white-space: pre;
		tab-size: 4;
	}
</style>
