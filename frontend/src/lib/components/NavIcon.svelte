<script lang="ts" module>
	export type NavIconName =
		| 'hoje'
		| 'cronograma'
		| 'balanceamento'
		| 'estatisticas'
		| 'caderno'
		| 'datas'
		| 'conteudo'
		| 'config'
		| 'concursos'
		| 'sol'
		| 'sistema'
		| 'lua'
		| 'menu'
		| 'prova'
		| 'faltam'
		| 'horas'
		| 'acerto'
		| 'alerta'
		| 'info'
		| 'anterior'
		| 'proximo'
		| 'check'
		| 'registrar'
		| 'fechar'
		| 'mais'
		| 'recolher'
		| 'expandir'
		| 'chevron'
		| 'busca'
		| 'topo'
		| 'subir'
		| 'descer'
		| 'link';

	/** Icon sizes, in px. `nav` is the sidebar/menu size; `sm` supports a label
	 *  (metric badges, inline hints); `md` is the default for buttons and titles. */
	export type NavIconSize = 'sm' | 'md' | 'nav';
</script>

<script lang="ts">
	/**
	 * The single icon set for the whole app.
	 *
	 * One geometry for every glyph, which is what makes them look related:
	 *  - 24×24 grid, all artwork inside a 4..20 safe area (a shared optical box,
	 *    so no icon reads bigger than its neighbours);
	 *  - stroke 1.75, round caps and joins, never filled — a filled glyph would
	 *    look heavier than the outline ones beside it;
	 *  - detail kept low: at 18px, anything finer turns to mud, and a busy glyph
	 *    competes with the label it is only meant to support.
	 *
	 * Size and colour come from the caller (via `size` and `currentColor`), never
	 * from inside a glyph, so one icon can never grow heavier than the rest.
	 */
	let { name, size = 'md' }: { name: NavIconName; size?: NavIconSize } = $props();
</script>

<svg
	class={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.75"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	{#if name === 'hoje'}
		<!-- calendar with today marked: the day you are on -->
		<rect x="4" y="5" width="16" height="15" rx="2.5" />
		<path d="M4 10h16M9 3.5v3M15 3.5v3" />
		<circle cx="12" cy="15" r="1.75" />
	{:else if name === 'cronograma'}
		<!-- calendar + clock: dated activities over time -->
		<path d="M20 11.5V7.5a2.5 2.5 0 0 0-2.5-2.5h-11A2.5 2.5 0 0 0 4 7.5v10A2.5 2.5 0 0 0 6.5 20h5" />
		<path d="M4 10h16M8.5 3.5v3M15.5 3.5v3" />
		<circle cx="17" cy="17" r="3.5" />
		<path d="M17 15.5v1.6l1 .8" />
	{:else if name === 'balanceamento'}
		<!-- sliders: distributing effort between subjects -->
		<path d="M5 7h14M5 12h14M5 17h14" />
		<circle cx="9.5" cy="7" r="1.9" />
		<circle cx="15" cy="12" r="1.9" />
		<circle cx="8" cy="17" r="1.9" />
	{:else if name === 'estatisticas'}
		<!-- bar chart -->
		<path d="M5 19h14" />
		<path d="M8 19v-5M12 19v-9M16 19v-3" />
	{:else if name === 'caderno'}
		<!-- notebook with a cross: the log of what you got wrong -->
		<path d="M6.5 4h11a1.5 1.5 0 0 1 1.5 1.5v13A1.5 1.5 0 0 1 17.5 20h-11" />
		<path d="M6.5 4a1.5 1.5 0 0 0-1.5 1.5v13A1.5 1.5 0 0 0 6.5 20" />
		<path d="M10.5 10.5l4 4M14.5 10.5l-4 4" />
	{:else if name === 'datas'}
		<!-- plain calendar: the edital's dated milestones -->
		<rect x="4" y="5" width="16" height="15" rx="2.5" />
		<path d="M4 10h16M9 3.5v3M15 3.5v3" />
	{:else if name === 'conteudo'}
		<!-- indented list: subjects with their topics -->
		<path d="M9 6.5h10M11.5 12h7.5M11.5 17.5h7.5" />
		<path d="M5.5 6.5h.01M8 12h.01M8 17.5h.01" />
	{:else if name === 'config'}
		<!-- sliders, matching the weight of the rest (a gear could not) -->
		<path d="M6 5.5v13M12 5.5v13M18 5.5v13" />
		<circle cx="6" cy="9" r="1.9" />
		<circle cx="12" cy="15" r="1.9" />
		<circle cx="18" cy="8" r="1.9" />
	{:else if name === 'concursos'}
		<!-- folder: the exams you keep -->
		<path d="M4 8.5V7a2 2 0 0 1 2-2h3.2a1 1 0 0 1 .8.4l1.2 1.6H18a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2z" />
	{:else if name === 'sol'}
		<circle cx="12" cy="12" r="4" />
		<path d="M12 4.5v-1M12 20.5v1M6.7 6.7l-.8-.8M17.3 17.3l.8.8M4.5 12h-1M20.5 12h1M6.7 17.3l-.8.8M17.3 6.7l.8-.8" />
	{:else if name === 'sistema'}
		<rect x="4" y="5" width="16" height="11" rx="2" />
		<path d="M9 20h6M12 16v4" />
	{:else if name === 'lua'}
		<path d="M19 14.2A7.6 7.6 0 0 1 9.8 5a7.7 7.7 0 1 0 9.2 9.2z" />
	{:else if name === 'menu'}
		<path d="M4.5 7h15M4.5 12h15M4.5 17h15" />
	{:else if name === 'prova'}
		<!-- exam day: calendar with a check -->
		<rect x="4" y="5" width="16" height="15" rx="2.5" />
		<path d="M4 10h16M9 3.5v3M15 3.5v3" />
		<path d="M9.5 14.5l1.8 1.8 3.2-3.4" />
	{:else if name === 'faltam'}
		<circle cx="12" cy="12" r="8" />
		<path d="M12 7.5V12l3 1.8" />
	{:else if name === 'horas'}
		<!-- hourglass: time spent -->
		<path d="M7.5 4h9M7.5 20h9" />
		<path d="M8.5 4v3.2c0 1.2 3.5 3.4 3.5 4.8 0-1.4 3.5-3.6 3.5-4.8V4" />
		<path d="M8.5 20v-3.2c0-1.2 3.5-3.4 3.5-4.8 0 1.4 3.5 3.6 3.5 4.8V20" />
	{:else if name === 'acerto'}
		<!-- target: hit rate -->
		<circle cx="12" cy="12" r="8" />
		<circle cx="12" cy="12" r="3.4" />
	{:else if name === 'info'}
		<circle cx="12" cy="12" r="8" />
		<path d="M12 11.2v4.6M12 8.4v.1" />
	{:else if name === 'alerta'}
		<path d="M12 4.5 4 19h16z" />
		<path d="M12 10v3.6M12 16.4v.1" />
	{:else if name === 'anterior'}
		<path d="M14 6.5 8.5 12l5.5 5.5" />
	{:else if name === 'proximo'}
		<path d="M10 6.5 15.5 12 10 17.5" />
	{:else if name === 'check'}
		<path d="M5.5 12.5l4.2 4.2L18.5 7.5" />
	{:else if name === 'registrar'}
		<!-- pencil over a line: log what you studied -->
		<path d="M5 19h14" />
		<path d="M15.2 5.6a1.9 1.9 0 0 1 2.7 2.7L9.6 16.6l-3.4.9.9-3.4z" />
	{:else if name === 'fechar'}
		<path d="M6.5 6.5l11 11M17.5 6.5l-11 11" />
	{:else if name === 'mais'}
		<path d="M12 5.5v13M5.5 12h13" />
	{:else if name === 'recolher'}
		<!-- side panel closing: the rail stays, the labels go -->
		<rect x="4" y="5" width="16" height="14" rx="2.5" />
		<path d="M10 5v14" />
		<path d="M16.5 10.2 14.7 12l1.8 1.8" />
	{:else if name === 'expandir'}
		<!-- side panel opening -->
		<rect x="4" y="5" width="16" height="14" rx="2.5" />
		<path d="M10 5v14" />
		<path d="m14.2 10.2 1.8 1.8-1.8 1.8" />
	{:else if name === 'chevron'}
		<path d="m7.5 10 4.5 4.5 4.5-4.5" />
	{:else if name === 'busca'}
		<circle cx="11" cy="11" r="6" />
		<path d="m15.5 15.5 3.5 3.5" />
	{:else if name === 'topo'}
		<!-- up-arrow to a bar: send this to the very top -->
		<path d="M5 6h14" />
		<path d="M12 10v9" />
		<path d="m8 14 4-4 4 4" />
	{:else if name === 'subir'}
		<!-- plain up chevron+shaft: swap with the row above -->
		<path d="M12 6v13" />
		<path d="m7 11 5-5 5 5" />
	{:else if name === 'descer'}
		<!-- plain down chevron+shaft: swap with the row below -->
		<path d="M12 5v13" />
		<path d="m7 13 5 5 5-5" />
	{:else if name === 'link'}
		<!-- external link: a box with an arrow leaving its top-right corner -->
		<path d="M13 5h6v6" />
		<path d="m19 5-8 8" />
		<path d="M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4" />
	{/if}
</svg>

<style>
	svg {
		display: block;
		flex: none;
	}
	/* Sizes are tokens, not per-call overrides: a caller picks a role, so no
	   single icon can drift bigger than the others in the same context. */
	.sm {
		width: var(--icon-sm);
		height: var(--icon-sm);
	}
	.md {
		width: var(--icon-md);
		height: var(--icon-md);
	}
	.nav {
		width: var(--icon-nav);
		height: var(--icon-nav);
	}
</style>
