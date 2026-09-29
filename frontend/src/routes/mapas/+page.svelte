<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import PageHead from '$lib/components/PageHead.svelte';
	import { tagStyle } from '$lib/format';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { MapaImportado, MapaResumo } from '$lib/types';

	/**
	 * Os mapas mentais da conta, por matéria: os que já estão vinculados às
	 * matérias do concurso aberto e, à parte, os que ainda esperam uma.
	 *
	 * O mapa é escrito fora do app (conteudo/mapas/README.md) e entra por aqui.
	 * Importar de novo o mesmo mapa troca o conteúdo e mantém os vínculos.
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

	const materiasDo = (slug: string) => materias.filter((d) => d.mapas.some((m) => m.slug === slug));

	const nf = new Intl.NumberFormat('pt-BR');

	// --- importar ----------------------------------------------------------
	let texto = $state('');
	let importando = $state(false);
	let resultado = $state<MapaImportado | null>(null);
	let painelAberto = $state(false);

	// Sem nenhum mapa ainda, o painel de importar já vem aberto.
	$effect(() => {
		if (carregado && catalogo.length === 0) painelAberto = true;
	});

	async function importar(conteudo: string) {
		erro = null;
		resultado = null;
		importando = true;
		try {
			resultado = await api.importarMapa(conteudo, concursoStore.ativoSlug);
			texto = '';
			await carregar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A importação falhou';
		} finally {
			importando = false;
		}
	}

	/** Escolher o arquivo já importa: importar de novo é seguro, e é um passo a menos. */
	async function aoEscolherArquivo(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (arquivo) await importar(await arquivo.text());
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

	{#if resultado}
		<div class="callout aviso-ok" role="status">
			<div>
				<b>{resultado.mapa.titulo}</b>
				{resultado.novo ? 'importado' : 'atualizado'}: {resultado.mapa.ramos} ramos, {nf.format(resultado.mapa.itens)} itens.
				{#if resultado.vinculadas.length > 0}
					Vinculado a {resultado.vinculadas.map((v) => `${v.codigo} — ${v.nome}`).join(', ')}.
				{:else}
					Nenhuma matéria do concurso aberto bate com ele: vincule na página do mapa.
				{/if}
				<a href="/mapas/{resultado.mapa.slug}">Abrir o mapa</a>
			</div>
		</div>
	{/if}

	<details class="importar" bind:open={painelAberto}>
		<summary>Importar mapa</summary>
		<div class="painel">
			<p class="ajuda">
				Um outline de texto: <code># Título</code>, metadados opcionais (<code>materia:</code>,
				<code>fonte:</code>) e os itens como <code>- texto</code>, com 2 espaços por nível. Importar de novo o mesmo
				mapa troca o conteúdo e mantém os vínculos.
			</p>
			<label class="arquivo">
				<span>Arquivo do mapa (.md)</span>
				<input
					type="file"
					accept=".md,.markdown,.txt,text/markdown,text/plain"
					aria-label="Arquivo do mapa (.md)"
					disabled={importando}
					onchange={aoEscolherArquivo}
				/>
			</label>
			<label class="colar">
				<span>Ou cole o texto do mapa</span>
				<textarea rows="6" spellcheck="false" aria-label="Texto do mapa" bind:value={texto}></textarea>
			</label>
			<button
				type="button"
				class="btn primary"
				disabled={importando || texto.trim() === ''}
				onclick={() => importar(texto)}
			>
				{importando ? 'Importando…' : 'Importar'}
			</button>
		</div>
	</details>

	{#if carregado}
		{#if filtro !== ''}
			<p class="filtro">
				Mostrando só os mapas de <b>{filtro}</b>. <a href="/mapas">Ver todos</a>
			</p>
		{/if}

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
					<p>Importe o primeiro acima: o mapa de uma aula, escrito como um outline de texto.</p>
				{/if}
			</div>
		{/if}
	{:else if !erro}
		<p class="page-sub">Carregando…</p>
	{/if}
</div>

<style>
	.aviso-ok {
		margin-bottom: 14px;
	}
	.aviso-ok a {
		margin-left: 6px;
		font-weight: 600;
	}

	.importar {
		margin-bottom: 8px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--bg-card);
	}
	.importar summary {
		padding: 11px 14px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
	}
	.importar summary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
		border-radius: 10px;
	}
	.painel {
		display: grid;
		gap: 12px;
		padding: 0 14px 14px;
	}
	.ajuda {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: var(--text-muted);
		max-width: 76ch;
	}
	.ajuda code {
		font-family: var(--font-mono);
		font-size: 12px;
		padding: 1px 5px;
		border-radius: 4px;
		background: var(--bg-soft);
	}
	.arquivo,
	.colar {
		display: grid;
		gap: 5px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	textarea {
		box-sizing: border-box;
		width: 100%;
		padding: 9px 11px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 12px;
		line-height: 1.5;
		resize: vertical;
	}
	textarea:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.painel .btn {
		justify-self: start;
	}
	@media (pointer: coarse) {
		textarea {
			font-size: 16px;
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
