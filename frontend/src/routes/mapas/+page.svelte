<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import PageHead from '$lib/components/PageHead.svelte';
	import { cobertura } from '$lib/mapas/cobertura';
	import { tagStyle } from '$lib/format';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { MapaResumo } from '$lib/types';

	/**
	 * Os mapas mentais da conta, para ler: por matéria — os vinculados às
	 * matérias do concurso aberto e, à parte, os que ainda esperam uma — e o que
	 * da ementa ainda não tem mapa.
	 *
	 * Importar e exportar ficam em /mapas/importar: aqui eles vinham antes das
	 * estantes, e no celular o primeiro mapa ficava telas abaixo.
	 */
	let catalogo = $state<MapaResumo[]>([]);
	let erro = $state<string | null>(null);
	let carregado = $state(false);

	async function carregar() {
		erro = null;
		try {
			const [lista] = await Promise.all([api.listarMapas(), mapasStore.carregar(true)]);
			catalogo = lista.mapas;
			carregado = true;
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível carregar os mapas mentais';
		}
	}

	$effect(() => {
		// Recarrega ao trocar de concurso: os vínculos são de lá.
		void concursoStore.ativoSlug;
		void untrack(carregar);
	});

	// `?materia=GTI`, que o cronograma manda quando a matéria tem mais de um mapa.
	const filtro = $derived(page.url.searchParams.get('materia') ?? '');

	const materias = $derived(mapasStore.disciplinas);
	const cor = (codigo: string) => planoStore.discIndex[codigo]?.cor ?? 0;

	/** As estantes: uma por matéria que tem mapa (ou só a do filtro). */
	const estantes = $derived(
		materias.filter((d) => d.mapas.length > 0 && (filtro === '' || d.codigo === filtro))
	);
	const vinculados = $derived(new Set(materias.flatMap((d) => d.mapas.map((m) => m.slug))));
	const semMateria = $derived(filtro === '' ? catalogo.filter((m) => !vinculados.has(m.slug)) : []);

	const nf = new Intl.NumberFormat('pt-BR');

	// --- o que falta ---------------------------------------------------------
	const cob = $derived(cobertura(materias.filter((d) => filtro === '' || d.codigo === filtro)));
	const pendentes = $derived(cob.materias.filter((m) => m.faltam.length > 0));
	const cobertas = $derived(cob.materias.filter((m) => m.faltam.length === 0));
	const faltam = $derived(cob.topicos - cob.cobertos);

	// --- abas -------------------------------------------------------------
	// A aba fica no endereço: voltar, recarregar ou mandar o link abre onde estava.
	type Aba = 'materias' | 'sem-mapa';
	const aba = $derived<Aba>(page.url.searchParams.get('aba') === 'sem-mapa' ? 'sem-mapa' : 'materias');

	function escolherAba(a: Aba) {
		const url = new URL(page.url);
		if (a === 'materias') url.searchParams.delete('aba');
		else url.searchParams.set('aba', a);
		void goto(url, { replaceState: true, noScroll: true, keepFocus: true });
	}

	// As setas trocam de aba, como em toda lista de abas.
	function teclasDasAbas(e: KeyboardEvent) {
		if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
		e.preventDefault();
		const outra: Aba = aba === 'materias' ? 'sem-mapa' : 'materias';
		escolherAba(outra);
		document.getElementById(`aba-${outra}`)?.focus();
	}
</script>

<svelte:head>
	<title>Mapas mentais — studygo</title>
</svelte:head>

<PageHead
	icone="mapa"
	titulo="Mapas mentais"
	sub="O conteúdo de cada aula em galhos: para revisar de relance, matéria por matéria."
	mostrarProps={false}
/>

<div class="page">
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	<div class="barra">
		<div class="abas" role="tablist" aria-label="Mapas mentais">
			<button
				type="button"
				role="tab"
				id="aba-materias"
				aria-selected={aba === 'materias'}
				aria-controls="painel-materias"
				tabindex={aba === 'materias' ? 0 : -1}
				onclick={() => escolherAba('materias')}
				onkeydown={teclasDasAbas}
			>
				Por matéria
			</button>
			<button
				type="button"
				role="tab"
				id="aba-sem-mapa"
				aria-selected={aba === 'sem-mapa'}
				aria-controls="painel-sem-mapa"
				tabindex={aba === 'sem-mapa' ? 0 : -1}
				onclick={() => escolherAba('sem-mapa')}
				onkeydown={teclasDasAbas}
			>
				Sem mapa {#if carregado}<span class="contagem">{faltam}</span>{/if}
			</button>
		</div>
		<a class="btn manter" href="/mapas/importar">Importar e exportar</a>
	</div>

	{#if filtro !== ''}
		<p class="filtro">
			Mostrando só <b>{filtro}</b>. <a href={aba === 'sem-mapa' ? '/mapas?aba=sem-mapa' : '/mapas'}>Ver todas</a>
		</p>
	{/if}

	<div id="painel-materias" role="tabpanel" aria-labelledby="aba-materias" hidden={aba !== 'materias'}>
		{#if carregado}
			{#each estantes as m (m.disciplinaId)}
				<section class="estante" aria-labelledby="est-{m.disciplinaId}">
					<div class="estante-cabeca">
						<span class="chip" style={tagStyle(cor(m.codigo))}>{m.codigo}</span>
						<h2 id="est-{m.disciplinaId}">{m.nome}</h2>
						<span class="meta">{m.mapas.length} {m.mapas.length === 1 ? 'mapa' : 'mapas'}</span>
					</div>
					<ul class="cartoes">
						{#each m.mapas as mapa (mapa.slug)}
							<li>
								<a class="cartao" href="/mapas/{mapa.slug}">
									<span class="titulo">{mapa.titulo}</span>
									<span class="conta">{mapa.ramos} ramos · {nf.format(mapa.itens)} itens</span>
									{#if mapa.fonte}<span class="fonte">{mapa.fonte}</span>{/if}
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{/each}

			{#if semMateria.length > 0}
				<section class="estante" aria-labelledby="sem-materia">
					<div class="estante-cabeca">
						<h2 id="sem-materia">Sem matéria vinculada</h2>
						<span class="meta">abra o mapa para vinculá-lo a uma matéria</span>
					</div>
					<ul class="cartoes">
						{#each semMateria as mapa (mapa.slug)}
							<li>
								<a class="cartao" href="/mapas/{mapa.slug}">
									<span class="titulo">{mapa.titulo}</span>
									<span class="conta">{mapa.ramos} ramos · {nf.format(mapa.itens)} itens</span>
									{#if mapa.materia}<span class="fonte">a fonte indica: {mapa.materia}</span>{/if}
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if estantes.length === 0 && semMateria.length === 0}
				<div class="vazia">
					{#if filtro !== ''}
						<p>Nenhum mapa vinculado a {filtro}.</p>
					{:else}
						<p>Nenhum mapa mental ainda.</p>
						<p>
							Envie o PDF de uma aula, ou o texto de um mapa pronto:
							<a href="/mapas/importar">Importar o primeiro mapa</a>
						</p>
					{/if}
				</div>
			{/if}
		{:else if !erro}
			<p class="page-sub">Carregando…</p>
		{/if}
	</div>

	<div id="painel-sem-mapa" role="tabpanel" aria-labelledby="aba-sem-mapa" hidden={aba !== 'sem-mapa'}>
		{#if carregado}
			{#if cob.topicos === 0}
				<div class="vazia">
					<p>A ementa do concurso aberto não tem tópicos para conferir.</p>
				</div>
			{:else}
				<p class="resumo">
					<b>{cob.cobertos} de {cob.topicos} tópicos com mapa mental</b>
					<span>— coberto é o tópico escolhido na página de algum mapa, como a ementa marca. Toque num tópico para pedir o mapa dele, mesmo sem
							aula.</span
						>
				</p>

				{#each pendentes as m (m.disciplinaId)}
					<section class="estante" aria-labelledby="falta-{m.disciplinaId}">
						<div class="estante-cabeca">
							<span class="chip" style={tagStyle(cor(m.codigo))}>{m.codigo}</span>
							<h2 id="falta-{m.disciplinaId}">{m.nome}</h2>
							<span class="meta">{m.cobertos} de {m.topicos} tópicos com mapa</span>
							<a class="enviar" href="/mapas/importar?materia={encodeURIComponent(m.codigo)}">Enviar o PDF de uma aula</a>
						</div>
						{#if m.daMateriaInteira.length > 0}
							<p class="inteira">
								{#each m.daMateriaInteira as mapa, i (mapa.slug)}{#if i > 0},
									{/if}<a href="/mapas/{mapa.slug}">{mapa.titulo}</a>{/each}
								{m.daMateriaInteira.length === 1 ? 'vale' : 'valem'} para a matéria inteira: escolha, na página do mapa, os
								tópicos que ele cobre.
							</p>
						{/if}
						<ul class="faltam" aria-label="Tópicos sem mapa">
							{#each m.faltam as t (t)}
								<li>
									<a
										href="/mapas/importar?materia={encodeURIComponent(m.codigo)}&assunto={encodeURIComponent(t)}"
										aria-label="Pedir um mapa de “{t}”">{t}</a
									>
								</li>
							{/each}
						</ul>
					</section>
				{/each}

				{#if cobertas.length > 0}
					<p class="cobertas">
						Com todos os tópicos cobertos: {cobertas.map((m) => `${m.codigo} — ${m.nome}`).join(', ')}.
					</p>
				{/if}
			{/if}
		{:else if !erro}
			<p class="page-sub">Carregando…</p>
		{/if}
	</div>
</div>

<style>
	.barra {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 10px;
		flex-wrap: wrap;
		margin-top: 4px;
		border-bottom: 1px solid var(--border);
	}
	.abas {
		display: flex;
		gap: 4px;
	}
	.abas button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-bottom: -1px;
		padding: 8px 12px;
		border: 0;
		border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
	}
	.abas button[aria-selected='true'] {
		border-bottom-color: var(--text);
		color: var(--text);
	}
	@media (hover: hover) {
		.abas button:hover {
			color: var(--text);
		}
	}
	.abas button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.contagem {
		padding: 1px 7px;
		border-radius: 999px;
		background: var(--bg-soft);
		font-size: 11.5px;
		font-variant-numeric: tabular-nums;
	}
	.manter {
		margin-bottom: 6px;
	}
	@media (pointer: coarse) {
		.abas button {
			min-height: 44px;
		}
	}

	.filtro {
		margin: 14px 0 0;
		font-size: 13.5px;
		color: var(--text-muted);
	}

	.estante {
		margin-top: 26px;
	}
	.estante-cabeca {
		display: flex;
		align-items: baseline;
		gap: 10px;
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
	.chip {
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 3px 8px;
		border-radius: 5px;
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
		gap: 6px;
		height: 100%;
		min-height: 104px;
		box-sizing: border-box;
		padding: 14px 16px;
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
	.titulo {
		font-size: 16px;
		font-weight: 700;
		line-height: 1.3;
	}
	.conta {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}
	.fonte {
		font-size: 12.5px;
		line-height: 1.4;
		color: var(--text-muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.resumo {
		margin: 18px 0 0;
		font-size: 14px;
	}
	.resumo span {
		color: var(--text-muted);
		font-size: 13px;
	}
	.enviar {
		margin-left: auto;
		font-size: 13px;
		font-weight: 600;
	}
	.inteira {
		margin: 0 0 10px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.faltam {
		margin: 0;
		padding-left: 20px;
		display: grid;
		gap: 4px;
		font-size: 14px;
		line-height: 1.45;
	}
	.faltam a {
		color: var(--text);
		text-decoration: underline dotted var(--text-faint);
		text-underline-offset: 3px;
	}
	.faltam a:hover {
		color: var(--accent);
		text-decoration-color: currentColor;
	}
	@media (pointer: coarse) {
		.faltam a {
			display: inline-block;
			padding: 6px 0;
		}
	}
	.cobertas {
		margin: 26px 0 0;
		font-size: 13px;
		color: var(--text-muted);
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
</style>
