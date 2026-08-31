<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * One setting: a title, the control, and a short line saying what the value
	 * actually changes. Settings that don't explain their effect are the reason
	 * the config screen read as a wall of anonymous inputs.
	 *
	 * `para` should be the id of the control rendered in the `controle` snippet,
	 * so the visible title is also the field's real label.
	 */
	let {
		titulo,
		descricao,
		para,
		unidade,
		valor,
		controle
	}: {
		titulo: string;
		descricao?: string;
		/** id of the control this labels; omit only for a group of buttons. */
		para?: string;
		/** e.g. "minutos", "dias" — shown next to the control. */
		unidade?: string;
		/** the current value, echoed so the effect of a change is visible. */
		valor?: string;
		controle: Snippet;
	} = $props();
</script>

<div class="ajuste">
	<div class="ajuste-txt">
		{#if para}
			<label class="ajuste-titulo" for={para}>{titulo}</label>
		{:else}
			<span class="ajuste-titulo">{titulo}</span>
		{/if}
		{#if descricao}<p class="ajuste-desc">{descricao}</p>{/if}
	</div>
	<div class="ajuste-ctrl">
		{@render controle()}
		{#if unidade}<span class="ajuste-unidade">{unidade}</span>{/if}
		{#if valor}<output class="ajuste-valor">{valor}</output>{/if}
	</div>
</div>

<style>
	.ajuste {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: start;
		gap: 12px 20px;
		padding: 14px 0;
		border-bottom: 1px solid var(--border);
	}
	.ajuste:last-child {
		border-bottom: 0;
	}
	.ajuste-txt {
		min-width: 0;
	}
	.ajuste-titulo {
		display: block;
		font-size: 13.5px;
		font-weight: 600;
		color: var(--text);
	}
	.ajuste-desc {
		margin: 3px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
		max-width: 62ch;
	}
	.ajuste-ctrl {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: flex-end;
	}
	.ajuste-unidade {
		font-size: 12px;
		color: var(--text-muted);
		white-space: nowrap;
	}
	.ajuste-valor {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-faint);
		white-space: nowrap;
	}

	/* On a narrow screen the control drops under its description instead of being
	   squeezed into a sliver. */
	@media (max-width: 620px) {
		.ajuste {
			grid-template-columns: 1fr;
		}
		.ajuste-ctrl {
			justify-content: flex-start;
		}
	}
</style>
