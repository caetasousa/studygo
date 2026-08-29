<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';

	const plano = $derived(planoStore.plano);

	// The syllabus lives on the disciplines' temas — that is what the edital import
	// fills in. `concurso.conteudo` is the artifact's free-form block list, kept as
	// an optional preamble for whoever pasted the edital text by hand.
	const grupos = $derived.by(() => {
		const discs = plano?.concurso.disciplinas ?? [];
		return [
			{ rotulo: 'Conhecimentos específicos', itens: discs.filter((d) => d.bloco === 'esp') },
			{ rotulo: 'Conhecimentos gerais', itens: discs.filter((d) => d.bloco === 'ger') }
		].filter((g) => g.itens.length > 0);
	});

	const totalTemas = $derived(
		(plano?.concurso.disciplinas ?? []).reduce((a, d) => a + d.temas.length, 0)
	);
	const semTemas = $derived(totalTemas === 0 && (plano?.concurso.conteudo.length ?? 0) === 0);
</script>

<PageHead
	emoji="📖"
	titulo="Conteúdo programático"
	sub="A ementa do edital, matéria por matéria."
	mostrarProps={false}
/>

{#if plano && semTemas}
	<div class="page">
		<div class="callout">
			<span class="em">📖</span>
			<div>
				Nenhum tema cadastrado ainda. O plano funciona sem eles — o dia mostra o nome da
				disciplina —, mas com a ementa cada dia recebe um tema específico. Adicione os temas em
				<a href="/concursos/{plano.concurso.slug}/editar">editar o concurso</a>, dentro de
				<b>“Temas e fontes”</b> de cada disciplina.
			</div>
		</div>
	</div>
{:else if plano}
	<div class="page prog">
		{#if plano.concurso.conteudo.length > 0}
			{#each plano.concurso.conteudo as item, i (i)}
				{#if item.tipo === 'ficha'}
					<div class="prog-card">{item.texto}</div>
				{:else if item.tipo === 'rot'}
					<h3>{item.texto}</h3>
				{:else if item.tipo === 'h'}
					<h4>{item.texto}</h4>
				{:else}
					<p>{item.texto}</p>
				{/if}
			{/each}
		{/if}

		{#each grupos as g (g.rotulo)}
			<h3>{g.rotulo}</h3>
			{#each g.itens as d (d.codigo)}
				<div class="ementa">
					<h4>
						<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>{d.nome}
						<span class="qtd">{d.temas.length} {d.temas.length === 1 ? 'tema' : 'temas'}</span>
					</h4>
					{#if d.temas.length > 0}
						<ol>
							{#each d.temas as t, i (i)}
								<li>{t}</li>
							{/each}
						</ol>
					{:else}
						<p class="vazia">
							Sem temas cadastrados — os dias desta disciplina mostram só o nome dela.
						</p>
					{/if}
				</div>
			{/each}
		{/each}

		<p class="rodape">
			{totalTemas} temas no total. Para editar,
			<a href="/concursos/{plano.concurso.slug}/editar">edite o concurso</a> — os temas ficam em
			“Temas e fontes” de cada disciplina.
		</p>
	</div>
{/if}

<style>
	.ementa {
		margin-top: 18px;
	}
	.ementa h4 {
		display: flex;
		align-items: baseline;
		gap: 0;
		margin-bottom: 8px;
	}
	.qtd {
		margin-left: auto;
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 500;
		letter-spacing: 0.04em;
		color: var(--text-faint);
		white-space: nowrap;
		padding-left: 12px;
	}
	.ementa ol {
		margin: 0;
		padding-left: 26px;
		max-width: 74ch;
	}
	.ementa li {
		font-size: 14px;
		line-height: 1.55;
		color: var(--text-muted);
		margin-bottom: 3px;
	}
	.ementa li::marker {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
	}
	.vazia {
		font-style: italic;
	}
	.rodape {
		margin-top: 34px;
		padding-top: 14px;
		border-top: 1px solid var(--border);
		font-size: 13px;
		color: var(--text-faint);
	}
</style>
