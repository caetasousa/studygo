<script lang="ts">
	import { api } from '$lib/api';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { citaNorma, descreverRecorte, especieDaNorma } from '$lib/leis';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { LeiNaMateria, LeisDaMateria } from '$lib/types';

	/**
	 * As leis do concurso, para ler: uma estante por matéria, só com as
	 * vinculadas. Vincular, importar e manter o catálogo ficam em
	 * /legislacao/gerenciar — aqui só o aviso de que há algo a resolver lá.
	 */
	const slug = $derived(concursoStore.ativoSlug);

	let materias = $state<LeisDaMateria[]>([]);
	let erro = $state<string | null>(null);
	let carregado = $state(false);

	async function carregar(s: string) {
		erro = null;
		try {
			materias = (await api.leisDoConcurso(s)).disciplinas;
			carregado = true;
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível carregar a legislação';
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	// A matéria que vale mais na prova vem primeiro.
	const estantes = $derived(
		[...materias.filter((m) => m.vinculadas.length > 0)].sort((a, b) => b.questoes * b.peso - a.questoes * a.peso)
	);

	// A mesma lei pode estar em duas matérias: conta uma vez.
	const unicas = $derived([...new Map(estantes.flatMap((m) => m.vinculadas).map((l) => [l.slug, l])).values()]);
	const totais = $derived({
		normas: unicas.length,
		artigos: unicas.reduce((n, l) => n + l.artigos, 0),
		questoes: unicas.reduce((n, l) => n + l.questoes, 0),
		respondidas: unicas.reduce((n, l) => n + l.respondidas, 0)
	});

	const esperando = $derived(materias.reduce((n, m) => n + m.sugeridas.length, 0));
	const semLei = $derived(
		materias.reduce(
			(n, m) => n + m.temas.filter((t) => citaNorma(t.texto) && t.leis.length === 0 && t.sugeridas.length === 0).length,
			0
		)
	);
	const pendencias = $derived(
		[
			esperando > 0 ? `${esperando} ${esperando === 1 ? 'lei do catálogo esperando vínculo' : 'leis do catálogo esperando vínculo'}` : '',
			semLei > 0 ? `${semLei} ${semLei === 1 ? 'tópico do edital sem lei no catálogo' : 'tópicos do edital sem lei no catálogo'}` : ''
		].filter(Boolean)
	);

	const nf = new Intl.NumberFormat('pt-BR');

	function questoesDa(l: LeiNaMateria): string {
		if (l.questoes === 0) return 'sem questões ainda';
		return `${l.respondidas} de ${l.questoes} ${l.questoes === 1 ? 'questão' : 'questões'}`;
	}

	function pct(n: number, total: number): number {
		return total > 0 ? (n / total) * 100 : 0;
	}
</script>

<svelte:head>
	<title>Legislação — studygo</title>
</svelte:head>

<div class="crumb">Estudos <span class="sep">/</span> Legislação</div>
<div class="head-row">
	<h1 class="page-title"><span class="title-ic"><NavIcon name="lei" size="md" /></span><span>Legislação</span></h1>
	<a class="gerenciar" href="/legislacao/gerenciar">Gerenciar leis</a>
</div>
<p class="page-sub">As normas que o edital cobra, só no recorte que cai na prova.</p>

<div class="page">
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if carregado}
		{#if estantes.length > 0}
			<dl class="numeros">
				<div>
					<dt>normas no recorte do edital</dt>
					<dd>{nf.format(totais.normas)}</dd>
				</div>
				<div>
					<dt>artigos para ler</dt>
					<dd>{nf.format(totais.artigos)}</dd>
				</div>
				<div>
					<dt>
						questões · {totais.respondidas === 0 ? 'nenhuma respondida' : `${nf.format(totais.respondidas)} respondidas`}
					</dt>
					<dd>{nf.format(totais.questoes)}</dd>
				</div>
			</dl>
		{/if}

		{#if pendencias.length > 0}
			<a class="pendencia" href="/legislacao/gerenciar">
				<span class="ponto" aria-hidden="true"></span>
				<span class="pendencia-texto">{pendencias.join(' · ')}</span>
				<span class="resolver">Resolver</span>
			</a>
		{/if}

		{#each estantes as m (m.disciplinaId)}
			<section class="estante" aria-labelledby="est-{m.disciplinaId}">
				<div class="estante-cabeca">
					<h2 id="est-{m.disciplinaId}">{m.nome}</h2>
					<span class="meta">
						{m.questoes}
						{m.questoes === 1 ? 'questão' : 'questões'} na prova · peso {m.peso} · {m.vinculadas.length}
						{m.vinculadas.length === 1 ? 'norma' : 'normas'}
					</span>
				</div>
				<ul class="cartoes">
					{#each m.vinculadas as l (l.slug)}
						<li>
							<a class="cartao" href="/leis/{l.slug}">
								<span class="topo">
									<span class="especie">{especieDaNorma(l)}</span>
									<span class="arts">{nf.format(l.artigos)} arts.</span>
								</span>
								<span class="titulo">
									<span class="curto">{l.curto}</span>
									<span class="nome">{l.nome}</span>
								</span>
								<span class="recorte">{l.recorte.length === 0 ? 'Lei inteira' : descreverRecorte(l.recorte)}</span>
								<span class="progresso">
									<span class="barra" aria-hidden="true">
										<span class="certas" style="width:{pct(l.certas, l.questoes)}%"></span>
										<span class="erradas" style="width:{pct(l.respondidas - l.certas, l.questoes)}%"></span>
									</span>
									<span class="conta">{questoesDa(l)}</span>
								</span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			<div class="vazia">
				<p>Nenhuma lei vinculada a este concurso ainda.</p>
				<p>Em <a href="/legislacao/gerenciar">Gerenciar leis</a>, vincule as do catálogo que o edital cita, ou importe as que faltam.</p>
			</div>
		{/each}
	{:else if !erro}
		<p class="page-sub">Carregando…</p>
	{/if}
</div>

<style>
	.title-ic {
		display: grid;
		place-items: center;
		flex: none;
		color: var(--text-muted);
	}
	.gerenciar {
		flex: none;
		font-size: 13.5px;
		font-weight: 600;
		padding: 7px 14px;
		border-radius: 7px;
		border: 1px solid var(--border-strong);
		color: var(--text);
		text-decoration: none;
	}
	.gerenciar:hover {
		background: var(--bg-hover);
	}

	.numeros {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 12px;
		margin: 22px 0 16px;
	}
	.numeros div {
		display: flex;
		flex-direction: column-reverse;
		gap: 2px;
		padding: 13px 16px;
		border-radius: 10px;
		background: var(--bg-card);
		border: 1px solid var(--border);
	}
	.numeros dt {
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.numeros dd {
		margin: 0;
		font-size: 24px;
		font-weight: 800;
	}

	.pendencia {
		display: flex;
		align-items: center;
		gap: 12px;
		margin: 0 0 8px;
		padding: 10px 14px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--warn) 12%, transparent);
		color: var(--text);
		text-decoration: none;
		font-size: 13.5px;
	}
	.ponto {
		flex: none;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--warn);
	}
	.pendencia-texto {
		flex: 1;
	}
	.resolver {
		font-weight: 600;
		color: var(--warn);
	}
	.pendencia:hover .resolver {
		text-decoration: underline;
		text-underline-offset: 3px;
	}

	.estante {
		margin-top: 26px;
	}
	.estante-cabeca {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex-wrap: wrap;
		padding-bottom: 8px;
		margin-bottom: 14px;
		border-bottom: 1px solid var(--border);
	}
	.estante-cabeca h2 {
		margin: 0;
		font-size: 18px;
		font-weight: 700;
	}
	.meta {
		font-size: 12.5px;
		color: var(--text-faint);
	}

	.cartoes {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
		gap: 12px;
	}
	.cartao {
		display: flex;
		flex-direction: column;
		gap: 10px;
		height: 100%;
		min-height: 150px;
		box-sizing: border-box;
		padding: 14px 16px 13px;
		border-radius: 10px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		color: var(--text);
		text-decoration: none;
	}
	.cartao:hover {
		background: var(--bg-hover);
		border-color: var(--border-strong);
	}
	.topo {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
	}
	.especie {
		font-family: var(--font-mono);
		font-size: 10.5px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.arts {
		font-size: 12px;
		color: var(--text-faint);
	}
	.titulo {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
	}
	.curto {
		font-size: 16px;
		font-weight: 700;
		line-height: 1.3;
	}
	.nome {
		font-size: 12.5px;
		line-height: 1.4;
		color: var(--text-muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.recorte {
		align-self: flex-start;
		max-width: 100%;
		font-size: 12px;
		color: var(--text-muted);
		background: var(--bg-soft);
		padding: 3px 8px;
		border-radius: 5px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		box-sizing: border-box;
	}
	.recorte::first-letter {
		text-transform: uppercase;
	}
	.progresso {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.barra {
		flex: 1;
		display: flex;
		height: 4px;
		border-radius: 2px;
		background: var(--border);
		overflow: hidden;
	}
	.barra .certas {
		background: var(--good);
	}
	.barra .erradas {
		background: var(--danger);
	}
	.conta {
		font-size: 11.5px;
		color: var(--text-faint);
		white-space: nowrap;
	}

	.vazia {
		margin-top: 22px;
		padding: 16px 18px;
		border-radius: 10px;
		background: var(--bg-soft);
		font-size: 14px;
	}
	.vazia p {
		margin: 0;
	}
	.vazia p + p {
		margin-top: 6px;
		color: var(--text-muted);
	}

	@media (max-width: 700px) {
		.numeros {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
