<script lang="ts">
	import { partesTema } from '$lib/format';

	// Reta-final days carry a whole discipline's ementa in `tema`, joined with
	// " · ". Rendered whole it becomes a 15-topic paragraph; here we show the
	// first few and let the reader open the rest.
	let { tema, limite = 3 }: { tema: string; limite?: number } = $props();

	const partes = $derived(partesTema(tema));
	const longo = $derived(partes.length > limite + 1);

	let aberto = $state(false);

	const visiveis = $derived(longo && !aberto ? partes.slice(0, limite) : partes);
	const ocultos = $derived(partes.length - visiveis.length);
</script>

{#if partes.length <= 1}
	{tema}
{:else}
	<span class="lista">
		{#each visiveis as p, i (i)}
			<span class="parte">{p}</span>
		{/each}
	</span>
	{#if longo}
		<button class="mais" onclick={() => (aberto = !aberto)}>
			{aberto ? 'mostrar menos' : `+${ocultos} tópicos`}
		</button>
	{/if}
{/if}

<style>
	.lista {
		display: flex;
		flex-wrap: wrap;
		gap: 3px 10px;
	}
	.parte {
		position: relative;
		padding-left: 11px;
	}
	.parte::before {
		content: '';
		position: absolute;
		left: 2px;
		top: 0.58em;
		width: 3px;
		height: 3px;
		border-radius: 50%;
		background: var(--text-faint);
	}
	.mais {
		background: transparent;
		border: 0;
		padding: 2px 0 0;
		margin: 0;
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--accent);
		cursor: pointer;
		text-decoration: underline;
		display: block;
	}
	.mais:hover {
		color: var(--accent-strong);
	}
</style>
