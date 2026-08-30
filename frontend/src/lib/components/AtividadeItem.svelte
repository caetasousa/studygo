<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import IconButton from './IconButton.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { semNumeroInicial } from '$lib/estudo';
	import { fl, tagStyle } from '$lib/format';
	import type { ItemDia } from '$lib/types';

	/**
	 * One movable activity in the schedule.
	 *
	 * Drag is offered, but never the only way: the menu carries the same moves as
	 * plain buttons, which is what makes this usable by keyboard, on a phone, and
	 * by anyone who does not notice a drag handle.
	 */
	let {
		item,
		data,
		indice,
		total,
		podeMover,
		datasDisponiveis,
		onMover,
		onArrastar,
		onSoltar,
		concluida = false,
		onAlternarConcluida,
		onRegistrar
	}: {
		item: ItemDia;
		data: string;
		indice: number;
		total: number;
		podeMover: boolean;
		/** Dates that accept activities, for the "move to another day" list. */
		datasDisponiveis: string[];
		onMover: (id: string, data: string, posicao: number) => void;
		onArrastar?: (id: string) => void;
		onSoltar?: (posicao: number) => void;
		/** this discipline finished on this day */
		concluida?: boolean;
		onAlternarConcluida?: (marcado: boolean) => void;
		/** opens the day's log focused on this discipline */
		onRegistrar?: () => void;
	} = $props();

	let menuAberto = $state(false);
	let escolhendoData = $state(false);

	const disc = $derived(planoStore.discIndex);
	const nome = $derived(disc[item.disciplina]?.nome ?? item.disciplina);
	// The chip shows the discipline's code (DEV, ENG, BDD…) — a fixed-width badge
	// keeps the topic text starting at the same x on every line. The full name
	// stays as the tooltip and in the accessible name, so the abbreviation is
	// never the only way to know what it is.
	const sigla = $derived(item.disciplina);
	const cor = $derived(disc[item.disciplina]?.cor ?? 0);
	const tema = $derived(semNumeroInicial(item.tema));

	// An activity the backend has not given an id to cannot be addressed yet.
	const movivel = $derived(podeMover && !!item.id);

	function mover(destino: string, posicao: number) {
		menuAberto = false;
		escolhendoData = false;
		onMover(item.id, destino, posicao);
	}

	const proximaData = $derived(datasDisponiveis.find((d) => d > data) ?? null);
</script>

<div
	class="atv"
	class:movida={item.movida}
	class:feita={concluida}
	draggable={movivel}
	ondragstart={() => onArrastar?.(item.id)}
	ondragover={(e) => e.preventDefault()}
	ondrop={(e) => {
		e.preventDefault();
		onSoltar?.(indice);
	}}
	role="listitem"
>
	{#if movivel}
		<span class="alca" aria-hidden="true" title="Arraste para reorganizar">
			<NavIcon name="balanceamento" size="sm" />
		</span>
	{:else}
		<span class="alca vazia" aria-hidden="true"></span>
	{/if}

	<span class="chip" style={tagStyle(cor)} title={nome}>{sigla}</span>
	<span class="sr-only">{nome}</span>

	<span class="txt">
		<span class="tema">{tema}</span>
		{#if item.passada === 2}<span class="meta">2ª passada</span>{/if}
		{#if item.movida}<span class="meta movida-tag">movida por você</span>{/if}
	</span>

	<span class="acoes">
		{#if onRegistrar}
			<IconButton
				icon="registrar"
				label="Registrar horas e questões de {nome}"
				onclick={onRegistrar}
			/>
		{/if}
		{#if onAlternarConcluida}
			<label class="ok" title="Marcar {nome} como concluída">
				<input
					type="checkbox"
					class="checkbox"
					checked={concluida}
					onchange={(e) => onAlternarConcluida(e.currentTarget.checked)}
				/>
				<span class="sr-only">Concluí {nome} — {tema}</span>
			</label>
		{/if}
		{#if movivel}
			<IconButton
				icon="anterior"
				label="Mover para cima"
				disabled={indice === 0}
				onclick={() => mover(data, indice - 1)}
			/>
			<IconButton
				icon="proximo"
				label="Mover para baixo"
				disabled={indice >= total - 1}
				onclick={() => mover(data, indice + 1)}
			/>
			<IconButton
				icon="chevron"
				label="Mais ações para {nome} — {tema}"
				onclick={() => (menuAberto = !menuAberto)}
			/>
		{/if}
	</span>

	{#if menuAberto}
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			class="menu"
			role="menu"
			tabindex="-1"
			onkeydown={(e) => {
				if (e.key === 'Escape') menuAberto = false;
			}}
		>
			{#if !escolhendoData}
				{#if proximaData}
					<button type="button" role="menuitem" onclick={() => mover(proximaData, 0)}>
						Enviar para o próximo dia disponível
					</button>
				{/if}
				<button type="button" role="menuitem" onclick={() => (escolhendoData = true)}>
					Escolher uma data…
				</button>
				<button type="button" role="menuitem" onclick={() => (menuAberto = false)}>
					Cancelar
				</button>
			{:else}
				<p class="menu-tit">Mover para:</p>
				<div class="datas">
					{#each datasDisponiveis.slice(0, 30) as d (d)}
						<button
							type="button"
							role="menuitem"
							class:atual={d === data}
							onclick={() => mover(d, 0)}
						>
							{fl(d)}
						</button>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	/* Subject and topic read as one sentence on a single flowing line, with the
	   actions pushed to the end. A fixed column for the subject chip is what made
	   long ementas wrap one word at a time. */
	/* Four real columns — handle | code | topic | actions — instead of a wrapping
	   flex row. With flex-wrap, a topic long enough to wrap dropped its second
	   line back to x=0, under the drag handle; in a grid the text column keeps
	   its own left edge, so every line of every activity stays aligned. */
	.atv {
		display: grid;
		grid-template-columns: 18px auto minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 6px 10px;
		padding: 7px 8px;
		border-radius: 8px;
		position: relative;
	}
	.atv:hover {
		background: var(--bg-hover);
	}
	.alca {
		display: grid;
		place-items: center;
		width: 18px;
		height: 18px;
		color: var(--text-faint);
		cursor: grab;
		align-self: center;
		flex: none;
	}
	.alca.vazia {
		cursor: default;
	}
	.atv:active .alca {
		cursor: grabbing;
	}
	/* The code is a fixed-width badge, not a pill that resizes with its text:
	   that is what lets every topic on the card start at the same x. Codes are
	   3–4 characters by convention; a longer one grows the badge rather than
	   being clipped. */
	.chip {
		flex: none;
		min-width: 52px;
		box-sizing: border-box;
		text-align: center;
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 3px 7px;
		border-radius: 5px;
		white-space: nowrap;
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
		/* Long ementas stay readable instead of running past the card. */
		max-width: 78ch;
	}
	.meta {
		font-size: 11px;
		color: var(--text-faint);
	}
	.movida-tag {
		color: var(--accent);
	}
	.acoes {
		display: flex;
		align-items: center;
		gap: 2px;
		align-self: center;
	}
	/* The check gets the same hit area as the icon buttons beside it, so the row
	   of controls reads as one strip rather than a checkbox tacked on. */
	.ok {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		cursor: pointer;
		flex: none;
	}
	/* A finished activity settles back without becoming unreadable. */
	.atv.feita .tema {
		color: var(--text-faint);
		text-decoration: line-through;
		text-decoration-color: var(--border-strong);
	}
	.menu {
		position: absolute;
		right: 8px;
		top: calc(100% - 4px);
		z-index: 30;
		min-width: 240px;
		max-width: min(320px, calc(100vw - 32px));
		background: var(--bg-card);
		border: 1px solid var(--border-strong);
		border-radius: 10px;
		box-shadow: var(--shadow-pop);
		padding: 4px;
	}
	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		border: 0;
		background: transparent;
		color: var(--text);
		font-family: inherit;
		font-size: 13px;
		padding: 8px 10px;
		border-radius: 6px;
		cursor: pointer;
		min-height: 36px;
	}
	.menu button:hover {
		background: var(--bg-hover);
	}
	.menu button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.menu-tit {
		margin: 4px 10px 6px;
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.datas {
		max-height: 240px;
		overflow-y: auto;
	}
	.datas button.atual {
		color: var(--text-faint);
	}

	@media (max-width: 620px) {
		.alca {
			display: none; /* touch: the menu is the reliable path */
		}
		/* Narrow: code and actions share the top line, the topic gets the full
		   width beneath them rather than a squeezed third column. */
		.atv {
			grid-template-columns: auto minmax(0, 1fr);
		}
		.txt {
			grid-column: 1 / -1;
		}
	}
</style>
