<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { tagStyle } from '$lib/format';
	import { indexar } from '$lib/mapas/arvore';
	import Mapa from '$lib/mapas/Mapa.svelte';
	import Questoes from '$lib/mapas/Questoes.svelte';
	import { descreverPlacar, placar, porBanca, porRamo } from '$lib/mapas/questoes';
	import { confirmar } from '$lib/stores/confirmacao.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { CorrecaoDoMapa, MapaLido, QuestaoDoMapa, QuestoesImportadas } from '$lib/types';

	/**
	 * Um mapa mental aberto: o cabeçalho com as propriedades (matérias, fonte,
	 * tamanho) e o mapa como uma página de tópicos recolhíveis. As questões da
	 * aula vêm depois do mapa, por ramo, e se resolvem num diálogo, como as da
	 * lei. Manter o mapa (importar questões, excluir) fica recolhido no fim.
	 */
	const slug = $derived(page.params.slug ?? '');

	let lido = $state<MapaLido | null>(null);
	let erro = $state<string | null>(null);
	let carregando = $state(true);

	async function carregar(s: string) {
		carregando = true;
		erro = null;
		lido = null;
		try {
			const res = await api.lerMapa(s);
			if (slug === s) lido = res;
		} catch (e) {
			if (slug === s) erro = e instanceof Error ? e.message : 'Não foi possível abrir o mapa';
		} finally {
			if (slug === s) carregando = false;
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	const nos = $derived(lido ? indexar(lido.arvore) : []);

	// --- matérias ---------------------------------------------------------
	const vinculadas = $derived(mapasStore.disciplinas.filter((d) => d.mapas.some((m) => m.slug === slug)));
	const livres = $derived(mapasStore.disciplinas.filter((d) => !vinculadas.includes(d)));
	let erroVinculo = $state<string | null>(null);

	async function vincular(disciplinaId: string, ligar: boolean) {
		const concurso = concursoStore.ativoSlug;
		if (!concurso) return;
		erroVinculo = null;
		try {
			await api.vincularMapa(concurso, disciplinaId, slug, ligar);
			await mapasStore.carregar(true);
		} catch (e) {
			erroVinculo = e instanceof Error ? e.message : 'Não foi possível gravar o vínculo';
		}
	}

	const cor = (codigo: string) => planoStore.discIndex[codigo]?.cor ?? 0;

	// --- questões ----------------------------------------------------------
	const questoes = $derived(lido?.questoes ?? []);
	// Por ramo, na ordem do mapa, ou por banca, para treinar a da prova. A
	// escolha fica no aparelho: quem estuda por banca quer abrir já assim.
	const CHAVE_AGRUPAR = 'studygo:mapas:agrupar-questoes';
	let agrupar = $state<'ramo' | 'banca'>(lerAgrupar());
	function lerAgrupar(): 'ramo' | 'banca' {
		try {
			return localStorage.getItem(CHAVE_AGRUPAR) === 'banca' ? 'banca' : 'ramo';
		} catch {
			return 'ramo';
		}
	}
	function escolherAgrupar(modo: 'ramo' | 'banca') {
		agrupar = modo;
		try {
			localStorage.setItem(CHAVE_AGRUPAR, modo);
		} catch {
			// sem armazenamento, a escolha vale só nesta visita
		}
	}
	const grupos = $derived(
		!lido ? [] : agrupar === 'banca' ? porBanca(questoes) : porRamo(lido.arvore, questoes)
	);
	let aberto = $state<{ titulo: string; ids: string[] } | null>(null);
	const doDialogo = $derived(
		aberto ? questoes.filter((q) => aberto!.ids.includes(q.id)) : ([] as QuestaoDoMapa[])
	);

	function abrirQuestoes(titulo: string, qs: QuestaoDoMapa[]) {
		aberto = { titulo, ids: qs.map((q) => q.id) };
	}

	/** A resposta atualiza o placar sem recarregar o mapa (e sem fechar os tópicos abertos). */
	function respondida(id: string, c: CorrecaoDoMapa) {
		for (const q of lido?.questoes ?? []) {
			if (q.id === id) q.resposta = c;
		}
	}

	let importandoQuestoes = $state(false);
	let avisoQuestoes = $state<string | null>(null);
	let erroQuestoes = $state<string | null>(null);

	function descreverImportacao(r: QuestoesImportadas): string {
		const n = (x: number, um: string, varios: string) => (x === 1 ? `1 ${um}` : `${x} ${varios}`);
		const partes = [
			r.novas && n(r.novas, 'questão nova', 'questões novas'),
			r.atualizadas && n(r.atualizadas, 'atualizada', 'atualizadas'),
			r.desativadas && n(r.desativadas, 'retirada', 'retiradas'),
			r.mantidas && n(r.mantidas, 'sem mudança', 'sem mudança')
		].filter(Boolean);
		return partes.length ? `${partes.join(', ')}.` : 'Nada mudou.';
	}

	// O arquivo vai como está: é o servidor que confere e lista os problemas.
	async function importarQuestoes(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!arquivo) return;
		importandoQuestoes = true;
		avisoQuestoes = null;
		erroQuestoes = null;
		try {
			const r = await api.importarQuestoesDoMapa(slug, await arquivo.text());
			avisoQuestoes = descreverImportacao(r);
			const novo = await api.lerMapa(slug);
			if (lido) lido.questoes = novo.questoes;
		} catch (err) {
			erroQuestoes = err instanceof Error ? err.message : 'A importação das questões falhou';
		} finally {
			importandoQuestoes = false;
		}
	}

	// --- excluir -----------------------------------------------------------
	async function excluir() {
		if (!lido) return;
		const total = lido.questoes.length;
		const junto =
			total === 0 ? 'O mapa' : total === 1 ? 'O mapa, a questão e as suas respostas' : `O mapa, as ${total} questões e as suas respostas`;
		const ok = await confirmar({
			titulo: `Excluir “${lido.mapa.titulo}”?`,
			texto: `${junto} e o vínculo com as matérias saem do app. Para tê-lo de volta, é só importar de novo.`,
			rotulo: 'Excluir mapa',
			tom: 'perigo'
		});
		if (!ok) return;
		try {
			await api.excluirMapa(slug);
			await mapasStore.carregar(true);
			await goto('/mapas');
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A exclusão falhou';
		}
	}

	const nf = new Intl.NumberFormat('pt-BR');
</script>

<svelte:head>
	<title>{lido ? `${lido.mapa.titulo} — Mapas mentais` : 'Mapa mental'} — studygo</title>
</svelte:head>

<a class="voltar" href="/mapas">← Mapas mentais</a>

{#if erro}
	<div class="form-error" role="alert">{erro}</div>
{:else if carregando}
	<p class="page-sub">Carregando…</p>
{:else if lido}
	<h1 class="page-title">
		<span class="title-ic"><NavIcon name="mapa" size="md" /></span>
		<span>{lido.mapa.titulo}</span>
	</h1>

	<dl class="propriedades">
		<div class="linha">
			<dt>Matérias</dt>
			<dd>
				{#each vinculadas as d (d.disciplinaId)}
					<span class="materia">
						<span class="chip" style={tagStyle(cor(d.codigo))}>{d.codigo}</span>
						<span class="nome">{d.nome}</span>
						<button
							type="button"
							class="tirar"
							aria-label="Desvincular {d.nome}"
							title="Desvincular {d.nome}"
							onclick={() => vincular(d.disciplinaId, false)}>×</button
						>
					</span>
				{/each}
				{#if livres.length > 0}
					<select
						class="vincular"
						aria-label="Vincular a uma matéria"
						onchange={(e) => {
							const escolhida = e.currentTarget.value;
							e.currentTarget.value = '';
							if (escolhida) void vincular(escolhida, true);
						}}
					>
						<option value="">{vinculadas.length === 0 ? 'Vincular a uma matéria…' : '+ outra matéria'}</option>
						{#each livres as d (d.disciplinaId)}
							<option value={d.disciplinaId}>{d.codigo} — {d.nome}</option>
						{/each}
					</select>
				{/if}
				{#if vinculadas.length === 0 && lido.mapa.materia}
					<span class="indicada">a fonte indica: {lido.mapa.materia}</span>
				{/if}
			</dd>
		</div>
		{#if lido.mapa.fonte}
			<div class="linha">
				<dt>Fonte</dt>
				<dd class="fonte">{lido.mapa.fonte}</dd>
			</div>
		{/if}
		<div class="linha">
			<dt>Tamanho</dt>
			<dd>{lido.mapa.ramos} {lido.mapa.ramos === 1 ? 'ramo' : 'ramos'} · {nf.format(lido.mapa.itens)} itens</dd>
		</div>
		{#if questoes.length > 0}
			<div class="linha">
				<dt>Questões</dt>
				<dd>
					<span>{descreverPlacar(placar(questoes))}</span>
					<button type="button" class="btn resolver" onclick={() => abrirQuestoes('Todas', questoes)}>
						Resolver todas ({questoes.length})
					</button>
				</dd>
			</div>
		{/if}
	</dl>

	{#if erroVinculo}<div class="form-error" role="alert">{erroVinculo}</div>{/if}

	<div class="mapa">
		{#key slug}
			<Mapa {nos} />
		{/key}
	</div>

	{#if grupos.length > 0}
		<section class="questoes" aria-labelledby="questoes-titulo">
			<div class="q-topo">
				<h2 id="questoes-titulo" class="sec">Questões</h2>
				<div class="agrupar" role="group" aria-label="Agrupar as questões">
					<button type="button" aria-pressed={agrupar === 'ramo'} onclick={() => escolherAgrupar('ramo')}>Por ramo</button>
					<button type="button" aria-pressed={agrupar === 'banca'} onclick={() => escolherAgrupar('banca')}>Por banca</button>
				</div>
			</div>
			<ul class="ramos" aria-label={agrupar === 'banca' ? 'Questões por banca' : 'Questões por ramo'}>
				{#each grupos as g (g.titulo)}
					{@const p = placar(g.questoes)}
					<li>
						<button type="button" class="ramo" onclick={() => abrirQuestoes(g.titulo, g.questoes)}>
							<span class="r-titulo">{g.titulo}</span>
							<span class="r-placar">
								{p.total === 1 ? '1 questão' : `${p.total} questões`}{p.respondidas > 0
									? ` · ${p.respondidas} de ${p.total} respondidas · ${p.certas === 1 ? '1 certa' : `${p.certas} certas`}`
									: ''}
							</span>
							<span class="barra" aria-hidden="true">
								<span class="certas" style="width:{(p.certas / p.total) * 100}%"></span>
								<span class="erradas" style="width:{(p.erradas / p.total) * 100}%"></span>
							</span>
						</button>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	<details class="manter">
		<summary>Manter este mapa</summary>
		<p class="page-sub">
			Para corrigir ou ampliar o mapa, importe o texto de novo em <a href="/mapas">Mapas mentais</a>: o mesmo
			endereço troca o conteúdo e mantém as matérias vinculadas.
		</p>

		<h2 class="sec">Importar questões</h2>
		<p class="page-sub">
			O <code>{slug}.questoes.json</code>, com as questões da aula, cada uma presa a um ramo do mapa. Importar de novo
			não duplica nada, e as respostas das questões que continuam ficam.
		</p>
		{#if avisoQuestoes}<p class="ok" role="status">{avisoQuestoes}</p>{/if}
		{#if erroQuestoes}<div class="form-error" role="alert">{erroQuestoes}</div>{/if}
		<label class="arquivo">
			<span>Questões do mapa (.json)</span>
			<input type="file" accept=".json,application/json" disabled={importandoQuestoes} onchange={importarQuestoes} />
		</label>
		{#if importandoQuestoes}<p class="page-sub">Importando…</p>{/if}

		<h2 class="sec">Excluir</h2>
		<button type="button" class="btn danger" onclick={excluir}>Excluir mapa</button>
	</details>

	{#if aberto}
		<Questoes titulo={aberto.titulo} questoes={doDialogo} onrespondida={respondida} onclose={() => (aberto = null)} />
	{/if}
{/if}

<style>
	.voltar {
		display: inline-block;
		margin-bottom: 10px;
		font-size: 13px;
		color: var(--text-muted);
		text-decoration: none;
	}
	.voltar:hover {
		color: var(--text);
	}
	.title-ic {
		display: grid;
		place-items: center;
		flex: none;
		color: var(--text-muted);
	}

	/* As propriedades no jeito do Notion: um rótulo pequeno e o valor ao lado. */
	.propriedades {
		display: grid;
		gap: 6px;
		margin: 16px 0 0;
		max-width: 860px;
	}
	.linha {
		display: grid;
		grid-template-columns: 84px minmax(0, 1fr);
		gap: 10px;
		align-items: center;
		font-size: 13.5px;
	}
	dt {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	dd {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px 8px;
		margin: 0;
		min-width: 0;
		color: var(--text);
	}
	.fonte {
		color: var(--text-muted);
		font-size: 13px;
	}
	.materia {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 3px 4px 3px 8px;
		border: 1px solid var(--border);
		border-radius: 7px;
		background: var(--bg-soft);
	}
	.chip {
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 2px 7px;
		border-radius: 5px;
	}
	.nome {
		font-size: 13px;
	}
	.tirar {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		padding: 0;
		border: 0;
		border-radius: 5px;
		background: transparent;
		color: var(--text-faint);
		font-size: 15px;
		line-height: 1;
		cursor: pointer;
	}
	.tirar:hover {
		background: var(--bg-hover);
		color: var(--danger);
	}
	.vincular {
		padding: 4px 8px;
		border: 1px dashed var(--border-strong);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 13px;
		max-width: 100%;
		cursor: pointer;
	}
	.vincular:hover {
		background: var(--bg-hover);
	}
	.indicada {
		font-size: 12.5px;
		color: var(--text-faint);
	}

	.mapa {
		margin-top: 20px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}

	.manter {
		margin: 36px 0 24px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.manter > summary {
		cursor: pointer;
		color: var(--text-muted);
		font-size: 13px;
	}
	.manter .page-sub {
		margin: 10px 0 12px;
	}
	.manter .sec {
		margin: 18px 0 0;
		font-size: 14px;
		font-weight: 700;
	}
	.manter code {
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.ok {
		margin: 0 0 10px;
		font-size: 13px;
		color: var(--good);
	}
	.arquivo {
		display: grid;
		gap: 5px;
		margin-bottom: 8px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.resolver {
		padding: 5px 10px;
	}

	/* As questões por ramo, como as questões por unidade da lei. */
	.questoes {
		margin-top: 32px;
		max-width: 860px;
	}
	.questoes .sec {
		margin: 0;
		font-size: 16px;
		font-weight: 700;
	}
	.q-topo {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 8px;
		margin-bottom: 10px;
	}
	.agrupar {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 7px;
		overflow: hidden;
	}
	.agrupar button {
		padding: 5px 12px;
		border: 0;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 12.5px;
		cursor: pointer;
	}
	.agrupar button + button {
		border-left: 1px solid var(--border);
	}
	.agrupar button[aria-pressed='true'] {
		background: var(--bg-hover);
		color: var(--text);
		font-weight: 600;
	}
	@media (pointer: coarse) {
		.agrupar button {
			min-height: 40px;
			padding-inline: 16px;
		}
	}
	.ramos {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 6px;
	}
	.ramo {
		display: grid;
		gap: 4px;
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-card);
		color: var(--text);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	@media (hover: hover) {
		.ramo:hover {
			background: var(--bg-hover);
		}
	}
	.ramo:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.r-titulo {
		font-size: 14px;
		font-weight: 600;
		overflow-wrap: anywhere;
	}
	.r-placar {
		font-size: 12px;
		color: var(--text-muted);
	}
	.barra {
		display: flex;
		height: 4px;
		border-radius: 2px;
		overflow: hidden;
		background: var(--bg-soft);
	}
	.barra .certas {
		background: var(--good);
	}
	.barra .erradas {
		background: var(--danger);
	}

	@media (max-width: 620px) {
		.linha {
			grid-template-columns: minmax(0, 1fr);
			gap: 3px;
		}
	}

	@media (pointer: coarse) {
		/* O × de 20px é pequeno para o dedo; e campo com fonte abaixo de 16px faz
		   o iPhone dar zoom na página ao ser tocado. */
		.tirar {
			width: 32px;
			height: 32px;
		}
		.vincular {
			padding-block: 8px;
			font-size: 16px;
		}
	}
</style>
