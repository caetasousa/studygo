<script lang="ts">
	import type { Snippet } from 'svelte';
	import Blocos from './Blocos.svelte';
	import { rotuloDoApoio } from './revisao';
	import type { Apoio } from './types';

	let {
		apoio,
		aberto = false,
		acoes,
		onalternar
	}: {
		apoio: Apoio;
		aberto?: boolean;
		/** Botões do curador, no pé do texto aberto. */
		acoes?: Snippet;
		/** Abriu ou fechou: quem mostra várias questões do mesmo texto guarda a escolha. */
		onalternar?: (aberto: boolean) => void;
	} = $props();
</script>

<!-- O toggle do Notion: fechado ocupa uma linha; aberto, o texto vem recuado. -->
<details class="apoio" open={aberto} ontoggle={(e) => onalternar?.(e.currentTarget.open)}>
	<summary><span class="seta" aria-hidden="true"></span>{rotuloDoApoio(apoio)}</summary>
	<div class="corpo">
		<Blocos blocos={apoio.blocos} />
		{#if acoes}<div class="acoes">{@render acoes()}</div>{/if}
	</div>
</details>

<style>
	summary {
		display: flex;
		gap: 6px;
		align-items: center;
		width: fit-content;
		max-width: 100%;
		padding: 3px 8px 3px 4px;
		margin-left: -4px;
		border-radius: 5px;
		font-weight: 600;
		color: var(--text);
		cursor: pointer;
		list-style: none;
		user-select: none;
	}
	summary::-webkit-details-marker {
		display: none;
	}
	summary:hover {
		background: var(--bg-hover);
	}
	.seta {
		width: 0;
		height: 0;
		border-top: 5px solid transparent;
		border-bottom: 5px solid transparent;
		border-left: 7px solid var(--text-muted);
		transition: transform 0.12s;
	}
	.apoio[open] .seta {
		transform: rotate(90deg);
	}
	/* O texto inteiro, sem barra de rolagem: é para ler, como no caderno. A
	   coluna estreita e a linha alta são para ler de corrido, não para caber. */
	.corpo {
		padding: 6px 0 4px 22px;
		max-width: 64ch;
		font-size: 15.5px;
		line-height: 1.75;
	}
	.corpo :global(p) {
		margin-bottom: 14px;
		line-height: inherit;
		text-align: justify;
		hyphens: auto;
	}
	/* O título do texto: a primeira linha, em negrito, respira antes do corpo. */
	.corpo :global(p:first-child .negrito:first-child) {
		display: inline-block;
		margin-bottom: 4px;
		font-size: 1.05em;
	}
	.acoes {
		display: flex;
		gap: 8px;
		padding: 4px 0 8px;
	}
</style>
