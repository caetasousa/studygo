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
		| 'info';
</script>

<script lang="ts">
	// Stroke icons on a 24px grid. They replace the emoji the nav used to carry:
	// emoji render inconsistently across platforms (and not at all on some Linux
	// setups), and they cannot take the current text colour.
	let { name }: { name: NavIconName } = $props();
</script>

<svg
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.75"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	{#if name === 'hoje'}
		<circle cx="12" cy="12" r="4" />
		<path
			d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"
		/>
	{:else if name === 'cronograma'}
		<rect x="3" y="4" width="18" height="17" rx="2" />
		<path d="M3 9.5h18M3 15h18M9 9.5V21" />
	{:else if name === 'balanceamento'}
		<path d="M12 4v16M8 20h8M4 8h16M6.5 7.5 4 13h5zM17.5 7.5 15 13h5z" />
	{:else if name === 'estatisticas'}
		<path d="M4 20V4M4 20h16" />
		<path d="M8.5 20v-6M13 20v-10M17.5 20v-4" />
	{:else if name === 'caderno'}
		<path d="M5 3h11l3 3v15H5z" />
		<path d="M9.5 10.5l5 5M14.5 10.5l-5 5" />
	{:else if name === 'datas'}
		<rect x="3" y="5" width="18" height="16" rx="2" />
		<path d="M3 10h18M8 3v4M16 3v4" />
		<rect x="11" y="13" width="4" height="4" rx="0.5" fill="currentColor" stroke="none" />
	{:else if name === 'conteudo'}
		<path d="M9 6h11M9 12h11M9 18h11" />
		<path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01" />
	{:else if name === 'config'}
		<circle cx="12" cy="12" r="3" />
		<path
			d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.14.63.7 1.09 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
		/>
	{:else if name === 'concursos'}
		<path d="M4 20V6a2 2 0 0 1 2-2h4l2 3h6a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2z" />
	{:else if name === 'sol'}
		<circle cx="12" cy="12" r="4.5" />
		<path
			d="M12 1.5v2.2M12 20.3v2.2M4.2 4.2l1.6 1.6M18.2 18.2l1.6 1.6M1.5 12h2.2M20.3 12h2.2M4.2 19.8l1.6-1.6M18.2 5.8l1.6-1.6"
		/>
	{:else if name === 'sistema'}
		<rect x="2.5" y="4" width="19" height="13" rx="2" />
		<path d="M8.5 21h7M12 17v4" />
	{:else if name === 'lua'}
		<path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z" />
	{:else if name === 'menu'}
		<path d="M3 6h18M3 12h18M3 18h18" />
	{:else if name === 'prova'}
		<rect x="3" y="4" width="18" height="17" rx="2" />
		<path d="M3 10h18M8 2v4M16 2v4" />
	{:else if name === 'faltam'}
		<circle cx="12" cy="12" r="9" />
		<path d="M12 7v5l3.5 2" />
	{:else if name === 'horas'}
		<circle cx="12" cy="13" r="8" />
		<path d="M12 9v4l2.5 1.5M9 2h6" />
	{:else if name === 'acerto'}
		<circle cx="12" cy="12" r="9" />
		<circle cx="12" cy="12" r="4.5" />
		<circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" />
	{:else if name === 'info'}
		<circle cx="12" cy="12" r="9" />
		<path d="M12 11v5M12 8.2v.1" />
	{:else if name === 'alerta'}
		<path d="M12 3.5 2.5 20h19z" />
		<path d="M12 10v4M12 17.2v.1" />
	{/if}
</svg>

<style>
	svg {
		width: 100%;
		height: 100%;
		display: block;
	}
</style>
