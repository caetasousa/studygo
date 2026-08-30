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
		<path d="M3 10h18M8 2v4M16 2v4" />
	{:else if name === 'balanceamento'}
		<path d="M12 3v18M5 21V10M19 21V6M3 21h18" />
	{:else if name === 'estatisticas'}
		<path d="M3 3v18h18" />
		<path d="M7 14l4-4 3 3 5-6" />
	{:else if name === 'caderno'}
		<path d="M4 4h13a2 2 0 0 1 2 2v14H6a2 2 0 0 1-2-2z" />
		<path d="M4 4v14a2 2 0 0 0 2 2M9 9h6M9 13h6" />
	{:else if name === 'datas'}
		<path d="M12 21s-7-4.4-7-10a7 7 0 0 1 14 0c0 5.6-7 10-7 10z" />
		<circle cx="12" cy="11" r="2.5" />
	{:else if name === 'conteudo'}
		<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
		<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
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
