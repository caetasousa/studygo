<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import IconButton from './IconButton.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { semNumeroInicial } from '$lib/estudo';
	import { tagStyle } from '$lib/format';
	import type { Atividade } from '$lib/types';

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
	 *
	 * O código e o assunto formam UM alvo só, que abre o conteúdo programático
	 * da matéria (EmentaModal). Era ali que ficava um balão com o nome completo
	 * da disciplina: o diálogo diz isso no título, então o balão saiu — o
	 * `title` continua cobrindo o hover.
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
		onAbrirMateria,
		concluida = false,
		minutos = null,
		tecUrl = '',
		onRegistrar
	}: {
		item: Atividade;
		data: string;
		/** slot of this activity in its day */
		indice: number;
		podeMover: boolean;
		/** true when there is a slot to swap with above — same day or previous useful day */
		podeSubir: boolean;
		/** true when there is a slot to swap with below — same day or next useful day */
		podeDescer: boolean;
		/** Swap this activity with the one above (crossing days at the top).
		 *  Opcional: a tela Hoje não oferece remanejamento nenhum. */
		onMoverAcima?: (id: string) => void;
		/** Swap this activity with the one below (crossing days at the bottom). */
		onMoverAbaixo?: (id: string) => void;
		/** Abre o conteúdo programático desta matéria, marcando este assunto. */
		onAbrirMateria?: (codigo: string, tema: string) => void;
		/** this activity finished, shown as a quiet mark (edited in its form) */
		concluida?: boolean;
		/** planned length of this activity's block, in minutes */
		minutos?: number | null;
		/** Caderno de questões da banca já filtrado por este assunto, quando a
		 *  matéria tem uma fonte do tipo "questoes". */
		tecUrl?: string;
		/** Opens this activity's form. Receives the trigger so focus can return. */
		onRegistrar?: (gatilho: HTMLElement) => void;
	} = $props();

	const disc = $derived(planoStore.discIndex);
	const nome = $derived(disc[item.disciplina]?.nome ?? item.disciplina);
	// The chip shows the discipline's code (DEV, ENG, BDD…) — a fixed-width badge
	// keeps the topic text starting at the same x on every line.
	// O `codigo` da disciplina JÁ é o mnemônico que o servidor gravou; a tela não
	// deriva sigla própria, ou mostraria algo diferente do que está no banco.
	const sigla = $derived(item.disciplina);
	const cor = $derived(disc[item.disciplina]?.cor ?? 0);
	const tema = $derived(semNumeroInicial(item.tema));

	// An activity the backend has not given an id to cannot be addressed yet, and
	// one already marked done must not move: that would rewrite what was studied.
	const movivel = $derived(podeMover && !!item.id && !concluida);
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

	<!-- Um alvo só: a matéria e o assunto abrem o mesmo painel, e um único ponto
	     de foco por linha mantém o Tab curto num cronograma de centenas delas. -->
	<button
		type="button"
		class="materia"
		title="{nome} — ver o conteúdo programático da matéria"
		aria-label="{nome}: {tema}. Ver o conteúdo programático da matéria"
		onclick={() => onAbrirMateria?.(item.disciplina, item.tema)}
	>
		<span class="chip" style={tagStyle(cor)}>{sigla}</span>
		<span class="txt">
			<span class="tema">{tema}</span>
			{#if item.passada === 2}<span class="meta">2ª passada</span>{/if}
		</span>
	</button>

	<span class="acoes">
		{#if tecUrl}
			<a
				class="tec"
				href={tecUrl}
				target="_blank"
				rel="noopener noreferrer"
				title="Resolver questões de {tema} no caderno da banca"
				aria-label="Resolver questões de {tema} no caderno da banca (link externo)"
			>
				<NavIcon name="link" size="sm" />
			</a>
		{/if}
		{#if concluida}
			<span class="feito-marca" title="Concluída" aria-label="Concluída">
				<NavIcon name="check" size="sm" />
			</span>
		{/if}
		{#if movivel && podeSubir}
			<IconButton
				icon="subir"
				label="Subir {nome} uma posição"
				onclick={() => onMoverAcima?.(item.id)}
			/>
		{/if}
		{#if movivel && podeDescer}
			<IconButton
				icon="descer"
				label="Descer {nome} uma posição"
				onclick={() => onMoverAbaixo?.(item.id)}
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
	/* Three real columns — minutes | matéria | actions — instead of a wrapping
	   flex row. O código e o assunto vivem dentro do mesmo botão, que é o que os
	   torna um alvo só. */
	.atv {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
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
	.materia {
		display: flex;
		align-items: baseline;
		gap: 10px;
		min-width: 0;
		background: transparent;
		border: 0;
		padding: 0;
		margin: 0;
		text-align: left;
		font: inherit;
		color: inherit;
		cursor: pointer;
	}
	.materia:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 3px;
		border-radius: 6px;
	}
	.materia:hover .tema {
		text-decoration: underline;
		text-decoration-color: var(--border-strong);
		text-underline-offset: 3px;
	}
	.chip {
		flex: none;
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
	.tec {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		color: var(--text-faint);
		flex: none;
	}
	.tec:hover {
		color: var(--accent);
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
		/* A duração e os botões dividem a primeira linha; a matéria ocupa a
		   segunda inteira, senão o assunto quebra uma palavra por linha numa
		   coluna espremida. */
		.atv {
			grid-template-columns: minmax(0, 1fr) auto;
			grid-template-areas:
				'min acoes'
				'materia materia';
			align-items: center;
			row-gap: 0;
			column-gap: 8px;
			padding: 6px 8px;
		}
		.min  { grid-area: min; }
		.materia { grid-area: materia; align-items: flex-start; }
		.tema { line-height: 1.35; }
		.acoes { grid-area: acoes; justify-self: end; }
	}
</style>
