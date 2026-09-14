<script lang="ts">
	let {
		marcar
	}: {
		/** Age no último campo em foco; nulo antes de o curador clicar em algum. */
		marcar: ((marca: string) => void) | null;
	} = $props();

	const BOTOES = [
		{ marca: '**', rotulo: 'N', titulo: 'Negrito (Ctrl+B)', estilo: 'font-weight: 700' },
		{ marca: '*', rotulo: 'I', titulo: 'Itálico (Ctrl+I)', estilo: 'font-style: italic' },
		{ marca: '__', rotulo: 'S', titulo: 'Sublinhado (Ctrl+U)', estilo: 'text-decoration: underline' },
		{ marca: 'codigo', rotulo: '</>', titulo: 'Código (Ctrl+E)', estilo: '' }
	];
</script>

<div class="barra" role="toolbar" aria-label="Destaque">
	{#each BOTOES as b (b.marca)}
		<!-- mousedown sem foco: o clique não tira a seleção do campo. -->
		<button
			type="button"
			title={b.titulo}
			style={b.estilo}
			disabled={!marcar}
			onmousedown={(e) => e.preventDefault()}
			onclick={() => marcar?.(b.marca)}
		>
			{b.rotulo}
		</button>
	{/each}
	<span class="dica">
		Selecione o trecho e escolha o destaque. <code>[figura 1]</code> numa linha marca onde a
		figura entra.
	</span>
</div>

<style>
	.barra {
		position: sticky;
		top: 64px;
		z-index: 2;
		display: flex;
		flex-wrap: wrap;
		gap: 4px 6px;
		align-items: center;
		padding: 6px 8px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	button {
		min-width: 32px;
		height: 28px;
		padding: 0 8px;
		font: inherit;
		font-size: 13px;
		color: var(--text);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
	}
	button:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	button:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.dica {
		font-size: 12px;
		color: var(--text-faint);
	}
	.dica code {
		font-family: var(--font-mono);
		font-size: 11.5px;
	}
</style>
