<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import IconButton from './IconButton.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { hojeISO } from '$lib/format';
	import { semNumeroInicial } from '$lib/estudo';
	import { tagStyle } from '$lib/format';
	import type { ItemDia } from '$lib/types';

	/**
	 * One activity in the schedule.
	 *
	 * Rearranging used to be drag-and-drop (mouse + a hold-to-drag gesture for
	 * touch). The gesture leaked corner cases — a scroll that turned into a
	 * drag mid-flight, a drop that landed on air, a touch that never fired the
	 * events pointer relied on — and the fix for one usually broke another.
	 * Now each row carries just two buttons — up and down — that swap it with
	 * its neighbour. Bringing a future subject forward stays as "já terminei
	 * este assunto hoje".
	 */
	let {
		item,
		data,
		indice,
		podeMover,
		podeSubir,
		podeDescer,
		onMoverAcima,
		onMoverAbaixo,
		concluida = false,
		minutos = null,
		onAntecipar,
		onRegistrar
	}: {
		item: ItemDia;
		data: string;
		/** slot of this activity in its day */
		indice: number;
		podeMover: boolean;
		/** true when there is a slot to swap with above — same day or previous useful day */
		podeSubir: boolean;
		/** true when there is a slot to swap with below — same day or next useful day */
		podeDescer: boolean;
		/** Swap this activity with the one above (crossing days at the top). */
		onMoverAcima: (id: string) => void;
		/** Swap this activity with the one below (crossing days at the bottom). */
		onMoverAbaixo: (id: string) => void;
		/** this activity finished, shown as a quiet mark (edited in its form) */
		concluida?: boolean;
		/** planned length of this activity's block, in minutes */
		minutos?: number | null;
		/** brings this activity forward to today. The parent only wires this in
		 *  when everything above the row in the same day is already done, so the
		 *  button shows exactly when "eu já cheguei aqui hoje" is honest. */
		onAntecipar?: (id: string) => void;
		/** Opens this activity's form. Receives the trigger so focus can return. */
		onRegistrar?: (gatilho: HTMLElement) => void;
	} = $props();

	// Tapping the sigla reveals the full name — the touch equivalent of hover.
	let nomeVisivel = $state(false);

	const disc = $derived(planoStore.discIndex);
	const nome = $derived(disc[item.disciplina]?.nome ?? item.disciplina);
	// The chip shows the discipline's code (DEV, ENG, BDD…) — a fixed-width badge
	// keeps the topic text starting at the same x on every line.
	const sigla = $derived(planoStore.siglaIndex[item.disciplina] ?? item.disciplina);
	const cor = $derived(disc[item.disciplina]?.cor ?? 0);
	const tema = $derived(semNumeroInicial(item.tema));

	// An activity the backend has not given an id to cannot be addressed yet, and
	// one already marked done must not move: that would rewrite what was studied.
	const movivel = $derived(podeMover && !!item.id && !concluida);

	// Only a topic still ahead can be brought forward.
	const ehFuturo = $derived(data > hojeISO());
</script>

<div
	class="atv"
	class:movida={item.movida}
	class:feita={concluida}
	data-atv-dia={data}
	data-atv-pos={indice}
	role="listitem"
>
	{#if minutos}
		<span class="min">{minutos} min</span>
	{/if}

	<!-- The name is reachable three ways, since `title` alone is invisible to
	     touch and unreliable with a keyboard: a focusable chip that reveals the
	     name on hover AND focus, plus the always-present accessible name. -->
	<button
		type="button"
		class="chip"
		style={tagStyle(cor)}
		title={nome}
		aria-label={nome}
		onclick={() => (nomeVisivel = !nomeVisivel)}
	>
		{sigla}
	</button>
	<span class="nome-balao" class:visivel={nomeVisivel} aria-hidden="true">{nome}</span>

	<span class="txt">
		<span class="tema">{tema}</span>
		{#if item.passada === 2}<span class="meta">2ª passada</span>{/if}
	</span>

	<span class="acoes">
		{#if concluida}
			<span class="feito-marca" title="Concluída" aria-label="Concluída">
				<NavIcon name="check" size="sm" />
			</span>
		{/if}
		{#if movivel && podeSubir}
			<IconButton
				icon="subir"
				label="Subir {nome} uma posição"
				onclick={() => onMoverAcima(item.id)}
			/>
		{/if}
		{#if movivel && podeDescer}
			<IconButton
				icon="descer"
				label="Descer {nome} uma posição"
				onclick={() => onMoverAbaixo(item.id)}
			/>
		{/if}
		{#if onAntecipar && ehFuturo && movivel}
			<IconButton
				icon="adiantar"
				label="Já terminei {nome} hoje"
				onclick={() => onAntecipar?.(item.id)}
			/>
		{/if}
		{#if onRegistrar}
			<IconButton
				icon="registrar"
				label="Registrar estudo de {nome}"
				onclick={(e) => onRegistrar?.(e.currentTarget as HTMLElement)}
			/>
		{/if}
	</span>
</div>

<style>
	/* Four real columns — minutes | code | topic | actions — instead of a
	   wrapping flex row. */
	.atv {
		display: grid;
		grid-template-columns: auto auto minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 6px 10px;
		padding: 7px 8px;
		border-radius: 8px;
		position: relative;
	}
	.atv:hover {
		background: var(--bg-hover);
	}
	/* Confirms the landing without animating the whole list. `.movida` marks an
	   activity the user placed; it is used only for this brief settle. */
	@keyframes assentar {
		from { background: var(--accent-soft); }
		to   { background: transparent; }
	}
	.atv.movida {
		animation: assentar 0.45s ease-out;
	}
	@media (prefers-reduced-motion: reduce) {
		.atv.movida { animation: none; }
	}
	.min {
		flex: none;
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.chip {
		flex: none;
		border: 0;
		cursor: help;
		font-family: var(--font-mono);
		min-width: 52px;
		box-sizing: border-box;
		text-align: center;
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 3px 7px;
		border-radius: 5px;
		white-space: nowrap;
	}
	.chip:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.nome-balao {
		position: absolute;
		top: -6px;
		left: 34px;
		z-index: 5;
		display: none;
		padding: 4px 9px;
		border-radius: 6px;
		background: var(--bg-card);
		border: 1px solid var(--border-strong);
		box-shadow: var(--shadow-pop);
		font-size: 12px;
		color: var(--text);
		white-space: nowrap;
		pointer-events: none;
	}
	.chip:hover ~ .nome-balao,
	.chip:focus-visible ~ .nome-balao,
	.nome-balao.visivel {
		display: block;
	}
	.txt {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 4px 10px;
		min-width: 0;
	}
	.tema {
		font-size: 14px;
		line-height: 1.5;
		color: var(--text);
		max-width: 78ch;
	}
	.meta {
		font-size: 11px;
		color: var(--text-faint);
	}
	.acoes {
		display: flex;
		align-items: center;
		gap: 2px;
		align-self: center;
	}
	.feito-marca {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		color: var(--good);
		flex: none;
	}
	.atv.feita .tema {
		color: var(--text-faint);
		text-decoration: line-through;
		text-decoration-color: var(--border-strong);
	}

	@media (max-width: 620px) {
		.atv {
			grid-template-columns: auto auto 1fr auto;
			grid-template-areas:
				'min chip . acoes'
				'txt txt txt txt';
			align-items: center;
			row-gap: 0;
			column-gap: 8px;
			padding: 6px 8px;
		}
		.min  { grid-area: min; }
		.chip { grid-area: chip; }
		.txt  { grid-area: txt; align-self: start; }
		.tema { line-height: 1.35; }
		.acoes { grid-area: acoes; justify-self: end; }
	}
</style>
