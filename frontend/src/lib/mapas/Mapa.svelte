<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { browser } from '$app/environment';
	import Item from './Item.svelte';
	import { caminhosAte, contarAchados, idsComFilhos, paraBusca, type NoDoMapa } from './arvore';

	/**
	 * O mapa mental como uma página de tópicos recolhíveis, no jeito do Notion: a
	 * coluna de leitura, as guias de recuo e os ramos como cabeçalhos de seção.
	 * Foi o modelo escolhido (28/09/2026) porque boa parte do estudo é no celular e
	 * no tablet — lê de cima para baixo, sem arrastar nem dar zoom.
	 */
	let { nos }: { nos: NoDoMapa[] } = $props();

	// No celular o mapa abre como sumário: só os ramos, e cada um abre no toque.
	// Com a primeira camada aberta, o último ramo ficaria a várias telas de
	// distância. Numa tela maior, os ramos já saem abertos.
	const celular = browser && window.matchMedia('(max-width: 620px)').matches;

	// `untrack`: é o estado de saída, lido uma vez. A página recria o mapa quando
	// troca de mapa, então `nos` não muda debaixo dele.
	const abertos = new SvelteSet<string>(
		celular ? [] : untrack(() => nos.filter((n) => n.filhos.length > 0).map((n) => n.id))
	);

	// A busca tem o próprio estado de aberto: ela abre o caminho até cada achado
	// sem mexer no que quem lê tinha aberto, que volta quando o filtro sai.
	let busca = $state('');
	let abertosNaBusca = $state(new SvelteSet<string>());

	const filtro = $derived(paraBusca(busca.trim()));
	const ativos = $derived(filtro === '' ? abertos : abertosNaBusca);
	const achados = $derived(filtro === '' ? 0 : contarAchados(nos, filtro));

	function filtrar(valor: string) {
		busca = valor;
		const f = paraBusca(valor.trim());
		abertosNaBusca = new SvelteSet(f === '' ? [] : caminhosAte(nos, f));
	}

	function alternar(id: string) {
		if (ativos.has(id)) ativos.delete(id);
		else ativos.add(id);
	}

	function abrirTudo() {
		for (const id of idsComFilhos(nos)) ativos.add(id);
	}

	function recolherTudo() {
		ativos.clear();
	}
</script>

<div class="barra" role="toolbar" aria-label="Controles do mapa">
	<input
		type="search"
		class="filtro"
		placeholder="Filtrar itens do mapa"
		aria-label="Filtrar itens do mapa"
		autocomplete="off"
		autocapitalize="off"
		spellcheck="false"
		enterkeyhint="search"
		value={busca}
		oninput={(e) => filtrar(e.currentTarget.value)}
	/>
	<button type="button" class="btn" onclick={abrirTudo}>Abrir tudo</button>
	<button type="button" class="btn" onclick={recolherTudo}>Recolher tudo</button>
</div>

{#if filtro !== ''}
	<p class="achados" role="status">
		{#if achados === 0}
			Nenhum item traz “{busca.trim()}”.
		{:else}
			{achados} {achados === 1 ? 'item traz' : 'itens trazem'} “{busca.trim()}”.
		{/if}
	</p>
{/if}

<ul class="pagina" aria-label="Tópicos do mapa">
	{#each nos as no (no.id)}
		<Item {no} abertos={ativos} {alternar} {filtro} />
	{/each}
</ul>

<style>
	.barra {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
		margin-bottom: 8px;
	}
	.filtro {
		flex: 1 1 220px;
		min-width: 0;
		max-width: 340px;
		box-sizing: border-box;
		padding: 8px 11px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg-card);
		color: var(--text);
		font: inherit;
		font-size: 13.5px;
	}
	.filtro:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.btn {
		padding: 8px 12px;
	}
	.achados {
		margin: 4px 0 0;
		font-size: 13px;
		color: var(--text-muted);
	}
	.pagina {
		margin: 0;
		padding: 0;
		max-width: 860px;
	}

	@media (max-width: 620px) {
		/* O filtro ocupa a linha; os dois botões dividem a de baixo. */
		.filtro {
			flex-basis: 100%;
			max-width: none;
		}
		.btn {
			flex: 1;
		}
	}

	@media (pointer: coarse) {
		/* Abaixo de 16px, o iPhone dá zoom na página ao tocar no campo. */
		.filtro {
			font-size: 16px;
		}
		.btn {
			padding-block: 11px;
		}
	}
</style>
