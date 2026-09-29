<script lang="ts">
	import { partesDoTexto, ROTULO_MARCA } from './arvore';
	import type { MarcaDoItem } from '$lib/types';

	/**
	 * O texto de um item do mapa: a marca (definição, pegadinha…) como uma
	 * etiqueta na frente e o **negrito** em pedaços de texto — nunca como HTML.
	 * O tópico que abre e a folha o mostram do mesmo jeito, com a regra num lugar só.
	 */
	let { texto, marca = '' }: { texto: string; marca?: MarcaDoItem } = $props();

	const partes = $derived(partesDoTexto(texto));
</script>

{#if marca}<span class="marca {marca}">{ROTULO_MARCA[marca]}</span>{/if}{#each partes as p, i (i)}{#if p.negrito}<strong>{p.texto}</strong>{:else}{p.texto}{/if}{/each}

<style>
	/* Uma etiqueta discreta, no tom do resto do app: o que cai em prova se vê de
	   relance, sem gritar. */
	.marca {
		display: inline-block;
		margin-right: 7px;
		padding: 1px 6px;
		border-radius: 4px;
		font-family: var(--font-mono);
		font-size: 9.5px;
		font-weight: 600;
		letter-spacing: 0.07em;
		text-transform: uppercase;
		vertical-align: 1px;
		white-space: nowrap;
	}
	.marca.def {
		background: var(--accent-soft);
		color: var(--accent-strong);
	}
	.marca.pegadinha {
		background: var(--danger-soft);
		color: var(--danger);
	}
	.marca.cai {
		background: var(--warn-soft);
		color: var(--warn);
	}
	.marca.ex {
		background: var(--good-soft);
		color: var(--good);
	}
	.marca.questao {
		background: var(--c3-bg);
		color: var(--c3-tx);
	}
	strong {
		font-weight: 700;
	}
</style>
