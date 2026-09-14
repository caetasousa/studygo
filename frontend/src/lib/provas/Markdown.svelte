<script lang="ts">
	import BlocoDeCodigo from './BlocoDeCodigo.svelte';
	import { lerMarkdown, type Trecho } from './markdown';

	let {
		texto,
		onalternarTarefa
	}: {
		texto: string;
		/** Clique na caixa de uma tarefa, pela linha dela no texto. */
		onalternarTarefa?: (linha: number) => void;
	} = $props();

	const blocos = $derived(lerMarkdown(texto));
</script>

{#snippet emLinha(trechos: Trecho[])}{#each trechos as t, i (i)}{#if t.tipo === 'texto'}{t.texto}{:else if t.tipo === 'codigo'}<code
				>{t.texto}</code
			>{:else if t.tipo === 'negrito'}<strong>{@render emLinha(t.filhos)}</strong>{:else if t.tipo === 'italico'}<em
				>{@render emLinha(t.filhos)}</em
			>{:else if t.tipo === 'riscado'}<s>{@render emLinha(t.filhos)}</s>{:else if t.tipo === 'link'}<a
				href={t.href}
				target="_blank"
				rel="noopener noreferrer">{@render emLinha(t.filhos)}</a
			>{/if}{/each}{/snippet}

<div class="md">
	{#each blocos as b, i (i)}
		{#if b.tipo === 'titulo'}
			{#if b.nivel === 1}<h1>{@render emLinha(b.trechos)}</h1>
			{:else if b.nivel === 2}<h2>{@render emLinha(b.trechos)}</h2>
			{:else}<h3>{@render emLinha(b.trechos)}</h3>{/if}
		{:else if b.tipo === 'paragrafo'}
			<p>
				{#each b.linhas as linha, j (j)}{#if j > 0}<br />{/if}{@render emLinha(linha)}{/each}
			</p>
		{:else if b.tipo === 'lista' && b.ordenada}
			<ol>
				{#each b.itens as item (item.linha)}<li>{@render emLinha(item.trechos)}</li>{/each}
			</ol>
		{:else if b.tipo === 'lista'}
			<ul class:tarefas={b.itens.some((it) => it.tarefa)}>
				{#each b.itens as item (item.linha)}
					{#if item.tarefa}
						<li class="tarefa" class:feita={item.tarefa.feita}>
							<input
								type="checkbox"
								class="checkbox"
								checked={item.tarefa.feita}
								disabled={!onalternarTarefa}
								aria-label="Marcar tarefa"
								onclick={(e) => {
									e.stopPropagation();
									onalternarTarefa?.(item.linha);
								}}
							/>
							<span>{@render emLinha(item.trechos)}</span>
						</li>
					{:else}
						<li>{@render emLinha(item.trechos)}</li>
					{/if}
				{/each}
			</ul>
		{:else if b.tipo === 'citacao'}
			<blockquote>
				{#each b.linhas as linha, j (j)}{#if j > 0}<br />{/if}{@render emLinha(linha)}{/each}
			</blockquote>
		{:else if b.tipo === 'codigo'}
			<BlocoDeCodigo texto={b.texto} linguagem={b.linguagem} />
		{:else}
			<hr />
		{/if}
	{/each}
</div>

<style>
	/* A tipografia do Notion: texto de 16px com respiro, títulos que separam sem
	   gritar, citação com barra, código em linha avermelhado. */
	.md {
		font-size: 15.5px;
		line-height: 1.65;
		overflow-wrap: anywhere;
	}
	.md > :first-child {
		margin-top: 0;
	}
	h1,
	h2,
	h3 {
		margin: 1.1em 0 0.3em;
		line-height: 1.3;
		font-weight: 650;
	}
	h1 {
		font-size: 1.6em;
	}
	h2 {
		font-size: 1.3em;
	}
	h3 {
		font-size: 1.1em;
	}
	p {
		margin: 0.15em 0 0.55em;
	}
	ul,
	ol {
		margin: 0.2em 0 0.6em;
		padding-left: 1.5em;
	}
	li {
		margin: 0.15em 0;
	}
	ul.tarefas {
		list-style: none;
		padding-left: 0.2em;
	}
	/* Item comum no meio de tarefas mantém a bolinha. */
	ul.tarefas > li:not(.tarefa) {
		list-style: disc;
		margin-left: 1.3em;
	}
	.tarefa {
		display: flex;
		gap: 8px;
		align-items: flex-start;
	}
	.tarefa input {
		margin-top: 0.3em;
	}
	.tarefa.feita > span {
		color: var(--text-faint);
		text-decoration: line-through;
	}
	blockquote {
		margin: 0.4em 0 0.7em;
		padding: 0.1em 0 0.1em 0.9em;
		border-left: 3px solid var(--text);
	}
	code {
		font-family: var(--font-mono);
		font-size: 0.85em;
		padding: 0.15em 0.35em;
		border-radius: 4px;
		background: var(--bg-hover);
		color: #eb5757;
	}
	a {
		color: inherit;
		text-decoration: underline;
		text-decoration-color: var(--text-faint);
		text-underline-offset: 2px;
	}
	a:hover {
		text-decoration-color: currentColor;
	}
	hr {
		margin: 1em 0;
		border: 0;
		border-top: 1px solid var(--border);
	}
</style>
