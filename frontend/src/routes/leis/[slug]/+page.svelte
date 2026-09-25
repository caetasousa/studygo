<script lang="ts">
	import { page } from '$app/state';
	import { tick } from 'svelte';
	import { api } from '$lib/api';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import QuestoesDaLei from '$lib/components/QuestoesDaLei.svelte';
	import { AGRUPAMENTOS, comRotulo, placar, questoesPorArtigo, recuo } from '$lib/leis';
	import type { CorrecaoDeQuestao, Dispositivo, LeituraDeLei, QuestaoDeLei } from '$lib/types';

	/**
	 * A lei seca, dispositivo a dispositivo.
	 *
	 * Cada dispositivo é um bloco com id = ref, e é isso que faz o link direto
	 * funcionar (/leis/cf88#art71.inc2): o cronograma, o caderno e as questões
	 * apontam para o artigo pelo mesmo endereço que a lei usa. O texto vigente
	 * fica sozinho no `.texto`; notas e redação anterior ficam ao lado, nunca
	 * dentro — quem decora lei não pode confundir as duas coisas.
	 */
	const slug = $derived(page.params.slug ?? '');

	let leitura = $state<LeituraDeLei | null>(null);
	let erro = $state<string | null>(null);
	let alvo = $state<string | null>(null);
	let aberto = $state<{ titulo: string; questoes: QuestaoDeLei[] } | null>(null);

	const porRef = $derived(new Map((leitura?.dispositivos ?? []).map((d) => [d.ref, d])));
	const porArtigo = $derived(questoesPorArtigo(leitura?.questoes ?? [], porRef));
	const sumario = $derived.by(() => {
		const ds = leitura?.dispositivos ?? [];
		const divisoes = ds.filter((d) => AGRUPAMENTOS.includes(d.tipo));
		return divisoes.length > 0 ? divisoes : ds.filter((d) => d.tipo === 'artigo');
	});

	async function carregar(s: string) {
		erro = null;
		try {
			leitura = await api.lerLei(s);
			await tick();
			irParaAncora();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível abrir a lei';
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	function irParaAncora() {
		const ref = decodeURIComponent(location.hash.slice(1));
		if (!ref) return;
		alvo = ref;
		document.getElementById(ref)?.scrollIntoView({ block: 'start' });
	}

	/** A questão respondida atualiza selo e progresso sem recarregar a lei. */
	function respondida(id: string, c: CorrecaoDeQuestao) {
		if (!leitura) return;
		for (const q of leitura.questoes) {
			if (q.id === id) q.resposta = c;
		}
		if (aberto) {
			aberto = { ...aberto, questoes: aberto.questoes.map((q) => (q.id === id ? { ...q, resposta: c } : q)) };
		}
	}

	function abrirArtigo(d: Dispositivo) {
		aberto = { titulo: d.rotulo, questoes: porArtigo.get(d.ref) ?? [] };
	}

	function abrirUnidade(ref: string, titulo: string) {
		aberto = { titulo, questoes: (leitura?.questoes ?? []).filter((q) => q.unidade === ref) };
	}

	function selo(questoes: QuestaoDeLei[]): string {
		const p = placar(questoes);
		if (p.respondidas === 0) return `${p.total} ${p.total === 1 ? 'questão' : 'questões'}`;
		return `${p.certas} de ${p.total} certas`;
	}
</script>

<svelte:window onhashchange={irParaAncora} />

<svelte:head>
	<title>{leitura?.lei.curto ?? 'Lei'} — studygo</title>
</svelte:head>

{#if erro}
	<div class="page"><div class="form-error" role="alert">{erro}</div></div>
{:else if !leitura}
	<p class="page-sub" style="padding:32px">Carregando a lei…</p>
{:else}
	<div class="crumb"><a href="/legislacao">Legislação</a> <span class="sep">/</span> {leitura.lei.curto}</div>
	<div class="head-row">
		<h1 class="page-title">
			<span class="title-ic"><NavIcon name="lei" size="md" /></span><span>{leitura.lei.curto}</span>
		</h1>
	</div>
	<p class="page-sub">
		{leitura.lei.nome}
		{#if leitura.lei.fonte}· <a href={leitura.lei.fonte} target="_blank" rel="noopener noreferrer">texto na fonte oficial</a>{/if}
	</p>

	<div class="leitor">
		<aside class="lateral">
			<nav aria-label="Sumário">
				<h2 class="sec">Sumário</h2>
				<ol class="sumario">
					{#each sumario as d (d.ref)}
						<li class="nivel-{d.tipo}">
							<a href="#{d.ref}">{d.rotulo || d.texto}{#if d.nome}<span class="nome-div">{' — '}{d.nome}</span>{/if}</a>
						</li>
					{/each}
				</ol>
			</nav>

			{#if leitura.unidades.length > 0}
				<section aria-labelledby="unidades-t">
					<h2 class="sec" id="unidades-t">Questões por unidade</h2>
					<ul class="unidades">
						{#each leitura.unidades as u (u.ref)}
							{@const qs = leitura.questoes.filter((q) => q.unidade === u.ref)}
							{@const p = placar(qs)}
							<li>
								<button type="button" class="unidade" onclick={() => abrirUnidade(u.ref, u.titulo)}>
									<span class="u-titulo">{u.titulo}</span>
									<span class="u-placar">{p.respondidas} de {p.total} respondidas · {p.certas} certas</span>
								</button>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		</aside>

		<article class="texto-lei">
			{#each leitura.dispositivos as d (d.ref)}
				{@const qs = d.tipo === 'artigo' ? porArtigo.get(d.ref) : undefined}
				{@const [rotulo, resto] = comRotulo(d)}
				<div
					id={d.ref}
					class="dispositivo tipo-{d.tipo}"
					class:revogado={d.revogado}
					class:alvo={alvo === d.ref}
					style="--recuo:{recuo(d, porRef)}"
					aria-current={alvo === d.ref ? 'location' : undefined}
				>
					{#if AGRUPAMENTOS.includes(d.tipo)}
						<p class="divisao"><span class="texto">{d.texto}</span></p>
						{#if d.nome}<p class="nome-divisao">{d.nome}</p>{/if}
					{:else}
						<div class="linha">
							<p class="texto">{#if rotulo}<b>{rotulo}</b>{/if}{resto}</p>
							{#if qs && qs.length > 0}
								{@const pl = placar(qs)}
								<button
									type="button"
									class="selo"
									class:tudo-certo={pl.respondidas === pl.total && pl.erradas === 0}
									class:com-erro={pl.erradas > 0}
									aria-label="Questões do {d.rotulo}"
									onclick={() => abrirArtigo(d)}
								>
									{selo(qs)}
								</button>
							{/if}
						</div>
					{/if}
					{#if d.notas.length > 0}
						<ul class="notas">
							{#each d.notas as n, i (i)}<li>{n}</li>{/each}
						</ul>
					{/if}
					{#if d.anteriores.length > 0}
						<details class="anteriores">
							<summary>Redação anterior{d.anteriores.length > 1 ? ` (${d.anteriores.length})` : ''}</summary>
							{#each d.anteriores as a, i (i)}<p>{a}</p>{/each}
						</details>
					{/if}
				</div>
			{/each}
		</article>
	</div>

	{#if aberto}
		<QuestoesDaLei
			titulo={aberto.titulo}
			questoes={aberto.questoes}
			{porRef}
			onrespondida={respondida}
			onclose={() => (aberto = null)}
		/>
	{/if}
{/if}

<style>
	.crumb a {
		color: inherit;
	}
	.leitor {
		display: grid;
		grid-template-columns: 260px minmax(0, 1fr);
		gap: 28px;
		padding: 8px 0 60px;
		align-items: start;
	}
	@media (max-width: 900px) {
		.leitor {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	.lateral {
		position: sticky;
		top: 12px;
		max-height: calc(100vh - 24px);
		overflow-y: auto;
		font-size: 12.5px;
	}
	@media (max-width: 900px) {
		.lateral {
			position: static;
			max-height: 40vh;
		}
	}
	.sumario,
	.unidades {
		list-style: none;
		margin: 0 0 18px;
		padding: 0;
	}
	.sumario a {
		display: block;
		padding: 3px 6px;
		border-radius: 5px;
		color: var(--text-muted);
		text-decoration: none;
		line-height: 1.35;
	}
	.sumario a:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.sumario .nivel-capitulo a {
		padding-left: 14px;
	}
	.sumario .nivel-secao a {
		padding-left: 22px;
	}
	.sumario .nivel-subsecao a {
		padding-left: 30px;
	}
	.nome-div {
		color: var(--text-faint);
	}
	.unidade {
		display: flex;
		flex-direction: column;
		gap: 2px;
		width: 100%;
		text-align: left;
		background: none;
		border: 1px solid var(--border);
		border-radius: 7px;
		padding: 7px 9px;
		margin-bottom: 6px;
		color: var(--text);
		font: inherit;
		cursor: pointer;
	}
	.unidade:hover {
		background: var(--bg-hover);
	}
	.u-placar {
		color: var(--text-muted);
		font-size: 11.5px;
	}
	.texto-lei {
		max-width: 760px;
		font-size: 15px;
		line-height: 1.65;
	}
	.dispositivo {
		padding: 2px 6px 2px calc(6px + var(--recuo) * 22px);
		border-radius: 6px;
		scroll-margin-top: 16px;
	}
	.dispositivo.alvo {
		background: var(--accent-soft);
	}
	.dispositivo.revogado .texto {
		color: var(--text-faint);
	}
	.tipo-artigo {
		margin-top: 14px;
	}
	.linha {
		display: flex;
		gap: 10px;
		align-items: flex-start;
	}
	.texto {
		margin: 0;
		flex: 1;
	}
	.divisao {
		margin: 26px 0 0;
		text-align: center;
		font-weight: 700;
		letter-spacing: 0.04em;
		font-size: 13px;
		color: var(--text-muted);
	}
	.nome-divisao {
		margin: 2px 0 6px;
		text-align: center;
		font-weight: 700;
	}
	.tipo-preambulo .texto,
	.tipo-fecho .texto {
		color: var(--text-muted);
		font-size: 13.5px;
	}
	.selo {
		flex: none;
		margin-top: 3px;
		font-family: var(--font-mono);
		font-size: 10.5px;
		letter-spacing: 0.02em;
		white-space: nowrap;
		border: 1px solid var(--border-strong);
		border-radius: 999px;
		padding: 2px 9px;
		background: var(--accent-soft);
		color: var(--text);
		cursor: pointer;
	}
	.selo.tudo-certo {
		background: var(--good-soft);
		border-color: var(--good);
	}
	.selo.com-erro {
		background: var(--danger-soft);
		border-color: var(--danger);
	}
	.notas {
		list-style: none;
		margin: 2px 0 0;
		padding: 0;
		font-size: 11.5px;
		color: var(--text-faint);
	}
	.anteriores {
		margin: 2px 0 4px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.anteriores summary {
		cursor: pointer;
		font-size: 11.5px;
	}
	.anteriores p {
		margin: 4px 0 0;
		padding-left: 10px;
		border-left: 2px solid var(--border-strong);
		text-decoration: line-through;
		text-decoration-color: var(--text-faint);
	}
</style>
