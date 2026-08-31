<script lang="ts">
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { agruparPorBloco, numeroHierarquico, pareceEmentaCorrida, semNumeroInicial } from '$lib/estudo';

	const plano = $derived(planoStore.plano);

	// The syllabus lives on the disciplines' temas — that is what the edital import
	// fills in. `concurso.conteudo` is the artifact's free-form block list, kept as
	// an optional preamble for whoever pasted the edital text by hand.
	//
	// Order and numbering come from the shared study domain: conhecimentos gerais
	// first, then específicas, with "1", "1.1" derived from position rather than
	// read out of the stored text.
	const grupos = $derived(agruparPorBloco(plano?.concurso.disciplinas ?? []));

	// Matéria numbers run 1..N across the whole syllabus, so "3.2" is unambiguous
	// no matter which group the matéria sits in.
	const offset = $derived.by<Record<string, number>>(() => {
		const m: Record<string, number> = {};
		let acc = 0;
		for (const g of grupos) {
			m[g.bloco] = acc;
			acc += g.itens.length;
		}
		return m;
	});

	function indiceGlobal(bloco: string, i: number): number {
		return (offset[bloco] ?? 0) + i;
	}

	const totalTemas = $derived(
		(plano?.concurso.disciplinas ?? []).reduce((a, d) => a + d.temas.length, 0)
	);
	const semTemas = $derived(totalTemas === 0 && (plano?.concurso.conteudo.length ?? 0) === 0);
</script>

<PageHead
	icone="conteudo"
	titulo="Conteúdo programático"
	sub="A ementa do edital, matéria por matéria."
	mostrarProps={false}
/>

{#if plano && semTemas}
	<div class="page">
		<div class="callout">
			<span class="em"><NavIcon name="info" /></span>
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
		<!-- `concurso.conteudo` is the raw edital text; the grouped ementa below is
		     the same syllabus, structured. Showing both repeats every topic twice,
		     so the raw block only stands in when there are no temas to group. -->
		{#if totalTemas === 0 && plano.concurso.conteudo.length > 0}
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

		{#each grupos as g (g.bloco)}
			<h3>{g.rotulo}</h3>
			{#each g.itens as d, di (d.codigo)}
				<div class="ementa">
					<h4>
						<span class="num-mat">{numeroHierarquico(indiceGlobal(g.bloco, di))}</span>
						<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>
						<span class="mat-nome">{d.nome}</span>
						<span class="qtd">
							{d.temas.length}
							{d.temas.length === 1 ? 'tópico' : 'tópicos'}
						</span>
					</h4>
					{#if d.temas.length > 0}
						<ol class="topicos">
							{#each d.temas as t, i (i)}
								<li>
									<span class="num-top">
										{numeroHierarquico(indiceGlobal(g.bloco, di), i)}
									</span>
									<span class="top-txt">{semNumeroInicial(t)}</span>
									{#if pareceEmentaCorrida(t)}
										<a
											class="dividir"
											href="/concursos/{plano.concurso.slug}/editar"
											title="Este tópico reúne a ementa inteira; abra a edição para dividi-lo"
										>
											dividir
										</a>
									{/if}
								</li>
							{/each}
						</ol>
					{:else}
						<p class="vazia">
							Sem tópicos cadastrados — os dias desta matéria mostram só o nome dela.
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
		gap: 7px;
		margin-bottom: 8px;
	}
	.mat-nome {
		min-width: 0;
	}
	/* Numbers are their own column so they never crowd the text they label. */
	.num-mat {
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		color: var(--accent-strong);
		flex: none;
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
	/* The list carries its own numbers, so the browser marker is off: a single
	   running counter would contradict the "matéria.tópico" hierarchy. */
	.topicos {
		list-style: none;
		margin: 0;
		padding-left: 20px;
		max-width: 78ch;
	}
	.topicos li {
		display: grid;
		grid-template-columns: 3.2em minmax(0, 1fr) auto;
		gap: 8px;
		align-items: baseline;
		font-size: 14px;
		line-height: 1.55;
		color: var(--text-muted);
		padding: 2px 0;
	}
	.num-top {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}
	.top-txt {
		min-width: 0;
	}
	.dividir {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--warn);
		white-space: nowrap;
	}
	@media (max-width: 560px) {
		.topicos {
			padding-left: 4px;
		}
		.topicos li {
			grid-template-columns: 3.2em minmax(0, 1fr);
		}
		.dividir {
			grid-column: 2;
		}
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
