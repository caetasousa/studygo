<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import IconButton from './IconButton.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { hojeISO } from '$lib/format';
	import { semNumeroInicial } from '$lib/estudo';
	import { alvoNoPonto, arrastarToque } from '$lib/arrastarToque';
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
		podeMover,
		datasDisponiveis,
		onMover,
		onArrastar,
		onLargar,
		onSobrevoar,
		sobrevoado = false,
		onSoltar,
		concluida = false,
		minutos = null,
		onAntecipar,
		onRegistrar
	}: {
		item: ItemDia;
		data: string;
		/** slot of this activity in its day, used as the drop position */
		indice: number;
		podeMover: boolean;
		/** Dates that accept activities, for the "move to another day" list. */
		datasDisponiveis: string[];
		onMover: (id: string, data: string, posicao: number) => void;
		onArrastar?: (id: string) => void;
		/** drag ended without a drop landing */
		onLargar?: () => void;
		/** touch drag is hovering this slot (or null); lifts the drop affordance */
		onSobrevoar?: (alvo: { data: string; posicao: number } | null) => void;
		/** true while a touch drag hovers THIS row */
		sobrevoado?: boolean;
		onSoltar?: (posicao: number) => void;
		/** this activity finished, shown as a quiet mark (edited in its form) */
		concluida?: boolean;
		/** planned length of this activity's block, in minutes */
		minutos?: number | null;
		/** brings this activity forward to today, when finished ahead of schedule */
		onAntecipar?: (id: string) => void;
		/** Opens this activity's form. Receives the trigger so focus can return. */
		onRegistrar?: (gatilho: HTMLElement) => void;
	} = $props();

	let menuAberto = $state(false);
	let escolhendoData = $state(false);
	// Two-step target picking: first the date, then the exact slot in it. This is
	// the keyboard/touch equivalent of dropping onto a specific row.
	let dataEscolhida = $state<string | null>(null);
	// Tapping the sigla reveals the full name — the touch equivalent of hover.
	let nomeVisivel = $state(false);

	const disc = $derived(planoStore.discIndex);
	const nome = $derived(disc[item.disciplina]?.nome ?? item.disciplina);
	// The chip shows the discipline's code (DEV, ENG, BDD…) — a fixed-width badge
	// keeps the topic text starting at the same x on every line. The full name
	// stays as the tooltip and in the accessible name, so the abbreviation is
	// never the only way to know what it is.
	// Display only: the codigo (D01…) stays the key, the sigla is what a reader
	// can decode at a glance.
	const sigla = $derived(planoStore.siglaIndex[item.disciplina] ?? item.disciplina);
	const cor = $derived(disc[item.disciplina]?.cor ?? 0);
	const tema = $derived(semNumeroInicial(item.tema));

	// An activity the backend has not given an id to cannot be addressed yet, and
	// one already marked done must not move: that would rewrite what was studied.
	const movivel = $derived(podeMover && !!item.id && !concluida);

	function mover(destino: string, posicao: number) {
		menuAberto = false;
		escolhendoData = false;
		dataEscolhida = null;
		onMover(item.id, destino, posicao);
	}

	const proximaData = $derived(datasDisponiveis.find((d) => d > data) ?? null);

	// Only a topic still ahead can be brought forward.
	const ehFuturo = $derived(data > hojeISO());

	function antecipar() {
		menuAberto = false;
		onAntecipar?.(item.id);
	}

	/** The chosen day's activities, so the user can pick an exact slot. */
	const itensDoDestino = $derived(
		dataEscolhida === null
			? []
			: (planoStore.plano?.dias.find((d) => d.data === dataEscolhida)?.itens ?? []).map((it) => ({
					id: it.id,
					rotulo: `${planoStore.discIndex[it.disciplina]?.nome ?? it.disciplina} — ${semNumeroInicial(it.tema)}`
				}))
	);

	// --- drag and drop ----------------------------------------------------
	// `sobre` drives the drop affordance: this row is a swap target when it holds
	// an activity, which is every case here (an empty slot is the day's own drop
	// zone, handled by DiaCard).
	let sobre = $state(false);
	let arrastandoEu = $state(false);

	function inicio(e: DragEvent) {
		if (!movivel) {
			e.preventDefault();
			return;
		}

		arrastandoEu = true;
		onArrastar?.(item.id);

		// A move, not a copy — and carry the id so a drop outside this component
		// still knows what was dragged.
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', item.id);
		}
	}

	function fim() {
		arrastandoEu = false;
		sobre = false;
		onLargar?.();
	}

	// --- touch: press and hold ---------------------------------------------
	// The row the finger is currently over, so the same "trocar" affordance the
	// mouse gets is shown on touch too.
	//
	// ultimoAlvoValido remembers the last row a HIT actually landed on — not
	// just the last point sampled. A real finger is not a mathematically
	// centred point: the instant it lifts, it commonly drifts one or two
	// pixels off the row it was resting on, right into the 2px gap between
	// activities or the strip a neighbouring row's own padding claims. That
	// last, slightly-missed sample used to BE the drop target — a miss by a
	// couple of pixels then read as "dropped on nothing" and moved nothing at
	// all, even though the row had clearly been highlighted a moment before.
	// Falling back to the last point that DID hit something is what makes the
	// drop land on what the person watched get highlighted, not on the exact
	// pixel the finger happened to be leaving from.
	let ultimoAlvoValido: { data: string; posicao: number } | null = null;

	function toqueMoveu(x: number, y: number) {
		const alvo = alvoNoPonto(x, y);
		if (alvo) ultimoAlvoValido = alvo;
		onSobrevoar?.(alvo && alvo.data === data && alvo.posicao === indice ? null : alvo);
	}

	function toqueSoltou(x: number, y: number) {
		const alvo = alvoNoPonto(x, y) ?? ultimoAlvoValido;
		ultimoAlvoValido = null;

		arrastandoEu = false;
		onSobrevoar?.(null);

		if (alvo && !(alvo.data === data && alvo.posicao === indice)) {
			onMover(item.id, alvo.data, alvo.posicao);
		} else {
			onLargar?.();
		}
	}
</script>

<div
	class="atv"
	class:movida={item.movida}
	class:feita={concluida}
	class:arrastando={arrastandoEu}
	class:alvo-troca={(sobre || sobrevoado) && !arrastandoEu}
	data-atv-dia={data}
	data-atv-pos={indice}
	use:arrastarToque={{
		ativo: movivel,
		onInicio: () => {
			arrastandoEu = true;
			onArrastar?.(item.id);
		},
		onMover: toqueMoveu,
		onSoltar: toqueSoltou,
		onCancelar: fim
	}}
	draggable={movivel}
	ondragstart={inicio}
	ondragend={fim}
	ondragover={(e) => {
		if (arrastandoEu) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		sobre = true;
	}}
	ondragleave={() => (sobre = false)}
	ondrop={(e) => {
		e.preventDefault();
		e.stopPropagation();
		sobre = false;
		if (!arrastandoEu) onSoltar?.(indice);
	}}
	role="listitem"
>
	{#if (sobre || sobrevoado) && !arrastandoEu}
		<span class="dica-troca" aria-hidden="true">trocar</span>
	{/if}
	{#if movivel}
		<span class="alca" aria-hidden="true" title="Arraste para reorganizar (no celular, segure e arraste)">
			<NavIcon name="balanceamento" size="sm" />
		</span>
	{:else}
		<span class="alca vazia" aria-hidden="true"></span>
	{/if}

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
		{#if onRegistrar}
			<!-- The pointerdown guard stops the row's drag from starting when the
			     press lands on this button, which otherwise makes the icon
			     unreliable on touch and with a slow mouse. -->
			<IconButton
				icon="registrar"
				label="Registrar estudo de {nome}"
				onclick={(e) => onRegistrar?.(e.currentTarget as HTMLElement)}
				onpointerdown={(e) => e.stopPropagation()}
			/>
		{/if}
		{#if movivel}
			<!-- Pointer and touch both drag (hold to lift). This stays for keyboard
			     and screen readers, which have no drag gesture at all — it is
			     reachable by Tab but does not add a visible control to the row. -->
			<button
				type="button"
				class="mover-teclado"
				aria-expanded={menuAberto}
				onclick={() => (menuAberto = !menuAberto)}
			>
				Mover ou trocar {nome} — {tema}
			</button>
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
				{#if onAntecipar && ehFuturo}
					<!-- Two blocks of one subject in a single sitting: the topic moves to
					     the day it was actually finished on, not the day it was planned. -->
					<button type="button" role="menuitem" onclick={antecipar}>
						Já terminei este assunto hoje
					</button>
				{/if}
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
			{:else if dataEscolhida === null}
				<p class="menu-tit">Mover para qual data?</p>
				<div class="datas">
					<!-- Every valid date, not a truncated window: the schedule runs for
					     months and the useful target is often far from today. -->
					{#each datasDisponiveis as d (d)}
						<button
							type="button"
							role="menuitem"
							class:atual={d === data}
							onclick={() => (dataEscolhida = d)}
						>
							{fl(d)}
						</button>
					{/each}
				</div>
			{:else}
				<p class="menu-tit">Onde, em {fl(dataEscolhida)}?</p>
				<div class="datas">
					<!-- Landing ON an activity swaps the two; landing at the end moves. -->
					{#each itensDoDestino as alvo, i (alvo.id || i)}
						<button type="button" role="menuitem" onclick={() => mover(dataEscolhida!, i)}>
							Trocar com {alvo.rotulo}
						</button>
					{/each}
					<button
						type="button"
						role="menuitem"
						onclick={() => mover(dataEscolhida!, itensDoDestino.length)}
					>
						Mover para o fim do dia
					</button>
					<button type="button" role="menuitem" onclick={() => (dataEscolhida = null)}>
						Voltar às datas
					</button>
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
		/* handle | minutes | code | topic | actions */
		grid-template-columns: 18px auto auto minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 6px 10px;
		padding: 7px 8px;
		border-radius: 8px;
		position: relative;
	}
	.atv:hover {
		background: var(--bg-hover);
	}
	.atv[draggable='true'] {
		cursor: grab;
		/* `pan-y` used to sit here on the theory that the hold gesture's own
		   preventDefault() would suppress the native pan once a drag begins.
		   Measured on a real touch sequence, it does not: touch-action is
		   decided once, for the touch's whole lifetime, from the value in
		   effect at the first contact — a later preventDefault() cannot claw
		   it back. The page kept scrolling out from under the finger mid-drag
		   (confirmed via window.scrollY moving during the gesture), landing
		   the drop on whatever row scrolled into place rather than the one
		   under the thumb. `none` costs a swipe-to-scroll that starts exactly
		   on a row — scrolling from anywhere else on the card still works —
		   in exchange for the drop actually landing where it was released.
		   See arrastarToque.ts for the hold/cancel timing this still uses. */
		touch-action: none;
		/* A long press must not raise the text-selection or callout UI. */
		-webkit-touch-callout: none;
		user-select: none;
	}
	.atv[draggable='true']:active {
		cursor: grabbing;
	}
	/* The row being dragged stays visible but recedes, so the cursor's own drag
	   image reads as the thing in motion. */
	.atv.arrastando {
		opacity: 0.4;
		/* Lifted: on touch there is no browser-drawn drag image, so the row itself
		   has to read as picked up. */
		transform: scale(0.99);
		box-shadow: var(--shadow-pop);
	}
	/* A drop here swaps the two activities — said in words as well as colour. */
	.atv.alvo-troca {
		background: var(--accent-soft);
		outline: 2px solid var(--accent);
		outline-offset: -2px;
		/* a slight lift, so the target reads as raised toward the pointer */
		transform: translateY(-1px);
	}
	/* Visible only to keyboard and assistive tech: the row is dragged by pointer
	   or by holding on touch, so this control does not need to occupy space —
	   but it must never be unreachable. It appears when focused. */
	.mover-teclado {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
		background: transparent;
		color: inherit;
		font: inherit;
	}
	.mover-teclado:focus-visible {
		position: static;
		width: auto;
		height: auto;
		margin: 0;
		clip: auto;
		overflow: visible;
		padding: 6px 10px;
		border: 1px solid var(--accent);
		border-radius: 7px;
		background: var(--bg-card);
		font-size: 12px;
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.dica-troca {
		position: absolute;
		top: -9px;
		right: 10px;
		z-index: 2;
		font-family: var(--font-mono);
		font-size: 9px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		font-weight: 600;
		padding: 2px 7px;
		border-radius: 4px;
		background: var(--accent);
		color: var(--bg);
	}
	/* Confirms the landing without animating the whole list. `.movida` marks an
	   activity the user placed; it is used only for this brief settle, never as a
	   permanent badge. */
	@keyframes assentar {
		from {
			background: var(--accent-soft);
		}
		to {
			background: transparent;
		}
	}
	.atv.movida {
		animation: assentar 0.45s ease-out;
	}
	@media (prefers-reduced-motion: reduce) {
		.atv.alvo-troca {
			transform: none;
		}
		.atv.movida {
			animation: none;
		}
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
	/* Same treatment as the review block's duration, so a day reads as one
	   sequence of timed blocks rather than a list with one odd member. */
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
		font-family: var(--font-mono);
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
	/* The full name, revealed by hover, keyboard focus or a tap. `title` alone
	   reaches neither touch nor most screen-reader/keyboard paths. */
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
		/* Long ementas stay readable instead of running past the card. */
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
	/* A completed activity shows a quiet check; the state itself is edited in the
	   activity's own form. */
	.feito-marca {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		color: var(--good);
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
			grid-template-columns: auto auto minmax(0, 1fr);
		}
		.txt {
			grid-column: 1 / -1;
		}
	}
</style>
