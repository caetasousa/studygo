<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		rotulo,
		ajuda = '',
		children
	}: {
		rotulo: string;
		/** O que o campo faz e de onde vem o valor, numa frase. */
		ajuda?: string;
		/** Recebe o id que o controle deve usar, e o da ajuda para aria-describedby. */
		children: Snippet<[{ id: string; ajuda: string | undefined }]>;
	} = $props();

	const id = $props.id();
</script>

<!-- Rótulo, uma frase do que o campo faz e o controle: sem a frase, o curador
     via "Caderno" e "Caderno no gabarito" e tinha de adivinhar a diferença. -->
<div class="campo">
	<label for={id}>{rotulo}</label>
	{#if ajuda}<p class="ajuda" id="{id}-ajuda">{ajuda}</p>{/if}
	{@render children({ id, ajuda: ajuda ? `${id}-ajuda` : undefined })}
</div>

<style>
	.campo {
		display: grid;
		gap: 4px;
		align-content: start;
		min-width: 0;
	}
	label {
		font-size: 13px;
		font-weight: 600;
		color: var(--text);
	}
	.ajuda {
		margin: 0 0 2px;
		font-size: 12.5px;
		line-height: 1.45;
		color: var(--text-muted);
	}
	.campo :global(input:not([type='checkbox']):not([type='range']):not([type='file'])),
	.campo :global(select) {
		width: 100%;
		box-sizing: border-box;
	}
</style>
