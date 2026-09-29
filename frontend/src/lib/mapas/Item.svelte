<script lang="ts">
	import type { SvelteSet } from 'svelte/reactivity';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import Self from './Item.svelte';
	import Texto from './Texto.svelte';
	import { casa, type NoDoMapa } from './arvore';

	/**
	 * Um item do mapa, com os filhos recolhíveis logo abaixo.
	 *
	 * Com um filtro, aparece o que casa com ele e o caminho até lá. O item achado
	 * traz também o que há dentro dele, recolhido como sempre: achar o título de
	 * uma prática e não poder ler o que ela diz obrigaria a limpar a busca e
	 * procurar de novo.
	 */
	let {
		no,
		abertos,
		alternar,
		filtro,
		dentroDeAchado = false
	}: {
		no: NoDoMapa;
		/** Os itens abertos agora: sem filtro, os de quem lê; com filtro, os da busca. */
		abertos: SvelteSet<string>;
		alternar: (id: string) => void;
		/** O filtro já passado por `paraBusca`; vazio quando não há. */
		filtro: string;
		/** Um item acima deste casa com o filtro: este aparece de qualquer jeito. */
		dentroDeAchado?: boolean;
	} = $props();

	const achado = $derived(filtro !== '' && no.busca.includes(filtro));
	const visivel = $derived(filtro === '' || dentroDeAchado || casa(no, filtro));
	const aberto = $derived(abertos.has(no.id));
</script>

{#if visivel}
	<li
		class="item n{Math.min(no.nivel, 2)}"
		style={no.nivel === 0 ? `--cor:var(--c${no.cor}-tx);--cor-bg:var(--c${no.cor}-bg)` : undefined}
	>
		{#if no.filhos.length > 0}
			<button type="button" class="linha" aria-expanded={aberto} onclick={() => alternar(no.id)}>
				<span class="seta" class:aberta={aberto}><NavIcon name="proximo" size="sm" /></span>
				<span class="txt"><Texto texto={no.item.texto} marca={no.item.marca} /></span>
				{#if !aberto}<span class="conta" aria-hidden="true" title="{no.total} itens recolhidos">{no.total}</span>{/if}
			</button>
			{#if aberto}
				<ul class="filhos">
					{#each no.filhos as f (f.id)}
						<Self no={f} {abertos} {alternar} {filtro} dentroDeAchado={dentroDeAchado || achado} />
					{/each}
				</ul>
			{/if}
		{:else}
			<div class="linha folha">
				<span class="ponto" aria-hidden="true"></span>
				<span class="txt"><Texto texto={no.item.texto} marca={no.item.marca} /></span>
			</div>
		{/if}
	</li>
{/if}

<style>
	.item {
		list-style: none;
	}
	.linha {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		width: 100%;
		box-sizing: border-box;
		padding: 4px 8px;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--text);
		font: inherit;
		font-size: 14px;
		line-height: 1.55;
		text-align: left;
	}
	button.linha {
		cursor: pointer;
	}
	/* Só onde há cursor: no toque, o fundo do hover ficaria preso na linha
	   tocada até o próximo toque. */
	@media (hover: hover) {
		button.linha:hover {
			background: var(--bg-hover);
		}
		.n0 > button.linha:hover {
			background: color-mix(in srgb, var(--cor-bg) 100%, var(--text) 6%);
		}
	}
	button.linha:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.txt {
		flex: 1;
		min-width: 0;
		overflow-wrap: anywhere;
	}
	.seta,
	.ponto {
		flex: none;
		display: grid;
		place-items: center;
		width: 18px;
		height: 22px;
		color: var(--text-faint);
	}
	.seta :global(svg) {
		transition: transform 0.12s ease;
	}
	.seta.aberta :global(svg) {
		transform: rotate(90deg);
	}
	@media (prefers-reduced-motion: reduce) {
		.seta :global(svg) {
			transition: none;
		}
	}
	.ponto::after {
		content: '';
		width: 4px;
		height: 4px;
		border-radius: 50%;
		background: var(--text-faint);
	}
	.conta {
		flex: none;
		align-self: center;
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}

	/* As guias de recuo, como no Notion: uma linha fina que mostra a que pai o
	   item pertence. */
	.filhos {
		margin: 1px 0 3px 17px;
		padding: 0 0 0 10px;
		border-left: 1px solid var(--border);
	}

	/* O ramo principal é um cabeçalho de seção, na cor dele. */
	.n0 {
		margin-top: 14px;
	}
	.n0 > .linha {
		padding: 9px 12px;
		border-radius: 8px;
		background: var(--cor-bg);
		color: var(--cor);
		font-size: 16px;
		font-weight: 700;
		line-height: 1.35;
	}
	.n0 > .linha .seta {
		color: var(--cor);
	}
	.n0 > .linha .conta {
		color: var(--cor);
	}
	.n0 > .filhos {
		margin-left: 22px;
		border-left-color: color-mix(in srgb, var(--cor) 35%, var(--border));
	}
	.n1 > .linha {
		font-weight: 600;
	}

	/* No toque, a linha tem altura de dedo (~38px): a de leitura, com 30px,
	   faz tocar no item de cima ou de baixo. */
	@media (pointer: coarse) {
		.linha {
			padding-block: 8px;
		}
	}

	/* Tela estreita: o recuo de cada nível encolhe, senão o sexto nível de uma
	   aula vira uma coluna de poucas palavras. */
	@media (max-width: 620px) {
		.linha {
			gap: 6px;
			padding-inline: 6px;
		}
		.filhos {
			margin-left: 8px;
			padding-left: 8px;
		}
		.n0 > .filhos {
			margin-left: 10px;
		}
		.n0 > .linha {
			padding-inline: 10px;
		}
	}
</style>
