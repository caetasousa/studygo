<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { tick } from 'svelte';
	import { api } from '$lib/api';
	import CapturaDeLei from '$lib/components/CapturaDeLei.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import QuestoesDaLei from '$lib/components/QuestoesDaLei.svelte';
	import {
		AGRUPAMENTOS,
		comRotulo,
		descreverRecorte,
		nomeLegivel,
		placar,
		questoesPorArtigo,
		recuo,
		visiveisNoRecorte
	} from '$lib/leis';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type {
		CorrecaoDeQuestao,
		Dispositivo,
		ImportacaoDeQuestoes,
		LeituraDeLei,
		PublicacaoDeLei,
		QuestaoDeLei,
		ResumoDaExclusao
	} from '$lib/types';

	/**
	 * A lei seca, como uma página de leitura.
	 *
	 * Cada dispositivo é um bloco com id = ref, e é isso que faz o link direto
	 * funcionar (/leis/cf88#art71.inc2). O texto vigente fica sozinho no
	 * `.texto`; notas e redação anterior ficam ao lado, nunca dentro.
	 *
	 * Aberta no concurso ativo, a lei mostra só o que o edital cobra dela (o
	 * recorte das matérias que a vinculam), com a lei inteira a um clique. O
	 * recorte se ajusta aqui mesmo, marcando divisões e artigos.
	 */
	const slug = $derived(page.params.slug ?? '');
	const concurso = $derived(concursoStore.ativoSlug);

	let leitura = $state<LeituraDeLei | null>(null);
	let erro = $state<string | null>(null);
	let alvo = $state<string | null>(null);
	let aberto = $state<{ titulo: string; questoes: QuestaoDeLei[] } | null>(null);
	let inteira = $state(false);
	let ajustando = $state(false);
	let marcados = $state<string[]>([]);
	let salvandoRecorte = $state(false);
	let atual = $state<string | null>(null);

	const porRef = $derived(new Map((leitura?.dispositivos ?? []).map((d) => [d.ref, d])));
	const porArtigo = $derived(questoesPorArtigo(leitura?.questoes ?? [], porRef));
	const recorte = $derived(leitura?.recorte ?? null);
	const temRecorte = $derived(!!recorte && recorte.refs.length > 0);
	// A versão guarda só parte da lei (importada pelo tópico do edital).
	const parcial = $derived((leitura?.guardado.length ?? 0) > 0);
	const visiveis = $derived(
		temRecorte && !inteira && !ajustando ? visiveisNoRecorte(leitura?.dispositivos ?? [], recorte!.refs) : null
	);
	const mostrados = $derived(
		(leitura?.dispositivos ?? []).filter((d) => visiveis === null || visiveis.has(d.ref))
	);
	const divisoes = $derived(mostrados.filter((d) => AGRUPAMENTOS.includes(d.tipo)));
	const total = $derived(placar(leitura?.questoes ?? []));
	// No ajuste, o que está dentro de uma divisão marcada já vem junto.
	const cobertos = $derived(
		ajustando ? (visiveisNoRecorte(leitura?.dispositivos ?? [], marcados) ?? new Set<string>()) : new Set<string>()
	);

	async function carregar(s: string, c: string | null) {
		erro = null;
		try {
			leitura = await api.lerLei(s, c);
			await tick();
			irParaAncora();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível abrir a lei';
		}
	}

	$effect(() => {
		if (slug) void carregar(slug, concurso);
	});

	function irParaAncora() {
		const ref = decodeURIComponent(location.hash.slice(1));
		if (!ref) return;
		alvo = ref;
		// O link direto vale mesmo para o que está fora do recorte (L21).
		if (visiveis && !visiveis.has(ref) && porRef.has(ref)) inteira = true;
		void tick().then(() => document.getElementById(ref)?.scrollIntoView({ block: 'start' }));
	}

	// A divisão em que a leitura está, para o sumário acompanhar.
	function acompanhar() {
		let ultima: string | null = null;
		for (const d of divisoes) {
			const el = document.getElementById(d.ref);
			if (el && el.getBoundingClientRect().top < 120) ultima = d.ref;
		}
		atual = ultima;
	}
	let quadro = 0;
	function aoRolar() {
		cancelAnimationFrame(quadro);
		quadro = requestAnimationFrame(acompanhar);
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

	// ---- recorte -------------------------------------------------------------

	function ajustar() {
		marcados = [...(recorte?.refs ?? [])];
		ajustando = true;
	}

	function marcar(ref: string, sim: boolean) {
		marcados = sim ? [...marcados, ref] : marcados.filter((r) => r !== ref);
	}

	async function salvarRecorte() {
		if (!recorte || !concurso) return;
		salvandoRecorte = true;
		erro = null;
		try {
			for (const m of recorte.materias) {
				await api.vincularLei(concurso, m.disciplinaId, slug, true, marcados);
			}
			ajustando = false;
			inteira = false;
			await carregar(slug, concurso);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível gravar o recorte';
		} finally {
			salvandoRecorte = false;
		}
	}

	// ---- manter a lei --------------------------------------------------------
	// As questões chegam pelo questoes.json escrito fora do app, e o texto novo,
	// por uma captura nova da mesma fonte.
	let importando = $state(false);
	let manutencao = $state<string | null>(null);
	let erroManutencao = $state<string | null>(null);

	function descreverImportacao(r: ImportacaoDeQuestoes): string {
		const partes = [
			`${r.novas} novas`,
			r.atualizadas ? `${r.atualizadas} atualizadas` : '',
			r.desativadas ? `${r.desativadas} desativadas` : '',
			r.mantidas ? `${r.mantidas} sem mudança` : ''
		].filter(Boolean);
		return `Questões importadas: ${partes.join(', ')}.`;
	}

	async function importarQuestoes(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!arquivo) return;
		importando = true;
		manutencao = null;
		erroManutencao = null;
		try {
			let conteudo: unknown;
			try {
				conteudo = JSON.parse(await arquivo.text());
			} catch {
				throw new Error('o arquivo não é JSON — use o questoes.json da lei');
			}
			manutencao = descreverImportacao(await api.importarQuestoes(slug, conteudo));
			await carregar(slug, concurso);
		} catch (err) {
			erroManutencao = err instanceof Error ? err.message : 'A importação falhou';
		} finally {
			importando = false;
		}
	}

	async function textoPublicado(r: PublicacaoDeLei) {
		erroManutencao = null;
		manutencao = r.novaVersao
			? `Texto novo publicado.${r.unidadesDesatualizadas ? ` ${r.unidadesDesatualizadas} unidade(s) têm questões escritas para a redação anterior: reveja-as e importe de novo.` : ''}`
			: 'O texto na fonte é o mesmo já publicado: nada mudou.';
		await carregar(slug, concurso);
	}

	let excluindo = $state<ResumoDaExclusao | null>(null);

	async function pedirExclusao() {
		erroManutencao = null;
		try {
			excluindo = await api.resumirExclusao(slug);
		} catch (e) {
			erroManutencao = e instanceof Error ? e.message : 'Não foi possível preparar a exclusão';
		}
	}

	async function excluir() {
		try {
			await api.excluirLei(slug);
			await goto('/legislacao');
		} catch (e) {
			erroManutencao = e instanceof Error ? e.message : 'A exclusão falhou';
		}
	}

	function dominio(url: string): string {
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	}
</script>

<svelte:window onhashchange={irParaAncora} onscroll={aoRolar} />

<svelte:head>
	<title>{leitura?.lei.curto ?? 'Lei'} — studygo</title>
</svelte:head>

{#if erro && !leitura}
	<div class="page"><div class="form-error" role="alert">{erro}</div></div>
{:else if !leitura}
	<p class="page-sub" style="padding:32px">Carregando a lei…</p>
{:else}
	<div class="crumb"><a href="/legislacao">Legislação</a> <span class="sep">/</span> {leitura.lei.curto}</div>
	<h1 class="page-title">
		<span class="title-ic"><NavIcon name="lei" size="md" /></span><span>{leitura.lei.curto}</span>
	</h1>
	<p class="nome-lei">{leitura.lei.nome}</p>

	<dl class="props">
		{#if leitura.lei.fonte}
			<dt>Fonte</dt>
			<dd><a href={leitura.lei.fonte} target="_blank" rel="noopener noreferrer">{dominio(leitura.lei.fonte)} ↗</a></dd>
		{/if}
		<dt>Questões</dt>
		<dd>
			{#if total.total === 0}
				<span class="vazio">nenhuma ainda</span>
			{:else}
				{total.total} · {total.respondidas} respondidas{#if total.respondidas > 0}
					· <span class:bom={total.erradas === 0}>{total.certas} certas</span>{/if}
			{/if}
		</dd>
		{#if parcial}
			<dt>Importado</dt>
			<dd>
				só {descreverRecorte(leitura.guardado)}
				{#if leitura.lei.fonte}· <a href={leitura.lei.fonte} target="_blank" rel="noopener noreferrer">lei inteira na fonte ↗</a>{/if}
			</dd>
		{/if}
		{#if recorte}
			<dt>Cobrada em</dt>
			<dd>
				{#each recorte.materias as m, i (m.disciplinaId)}<span class="tag">{m.nome}</span>{#if i < recorte.materias.length - 1}{' '}{/if}{/each}
			</dd>
		{/if}
	</dl>

	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if recorte}
		<aside class="callout" aria-label="Recorte do edital">
			<span class="callout-ic" aria-hidden="true">📌</span>
			<div class="callout-corpo">
				{#if temRecorte}
					<p class="callout-titulo">O edital cobra {recorte.trechos.length === 1 ? 'esta parte' : 'estas partes'} da lei</p>
					<ul class="trechos">
						{#each recorte.trechos as t (t.ref)}
							<li>
								<a href="#{t.ref}">{t.rotulo}{t.nome ? ` — ${nomeLegivel(t.nome)}` : ''}</a>
								{#if t.artigos && t.artigos !== `art. ${t.rotulo.replace(/^Art\.\s*/, '')}`}<span class="artigos">{t.artigos}</span>{/if}
							</li>
						{/each}
					</ul>
				{:else}
					<p class="callout-titulo">O edital cobra a lei inteira</p>
				{/if}
				{#if !ajustando}
					<div class="acoes">
						{#if temRecorte}
							<div class="alternar" role="group" aria-label="O que mostrar">
								<button type="button" aria-pressed={!inteira} onclick={() => (inteira = false)}>Só o que cai</button>
								<button type="button" aria-pressed={inteira} onclick={() => (inteira = true)}>
									{parcial ? 'Tudo o que foi importado' : 'Lei inteira'}
								</button>
							</div>
						{/if}
						<button type="button" class="link" onclick={ajustar}>Ajustar recorte</button>
					</div>
				{/if}
			</div>
		</aside>
	{/if}

	{#if leitura.unidades.length > 0}
		<details class="toggle" open>
			<summary>Questões por unidade <span class="conta">{leitura.unidades.length}</span></summary>
			<ul class="unidades">
				{#each leitura.unidades as u (u.ref)}
					{@const qs = leitura.questoes.filter((q) => q.unidade === u.ref)}
					{@const p = placar(qs)}
					<li>
						<button type="button" class="unidade" onclick={() => abrirUnidade(u.ref, u.titulo)}>
							<span class="u-titulo">{u.titulo}</span>
							<span class="u-placar">{p.respondidas} de {p.total} respondidas · {p.certas} certas</span>
							<span class="barra" aria-hidden="true">
								<span class="certas" style="width:{p.total ? (p.certas / p.total) * 100 : 0}%"></span>
								<span class="erradas" style="width:{p.total ? (p.erradas / p.total) * 100 : 0}%"></span>
							</span>
						</button>
					</li>
				{/each}
			</ul>
		</details>
	{/if}

	<div class="leitor">
		<article class="texto-lei" class:ajustando>
			{#each mostrados as d (d.ref)}
				{@const qs = d.tipo === 'artigo' ? porArtigo.get(d.ref) : undefined}
				{@const [rotulo, resto] = comRotulo(d)}
				{@const marcavel = ajustando && (d.tipo === 'artigo' || AGRUPAMENTOS.includes(d.tipo))}
				<div
					id={d.ref}
					class="dispositivo tipo-{d.tipo}"
					class:revogado={d.revogado}
					class:alvo={alvo === d.ref}
					class:no-recorte={ajustando && cobertos.has(d.ref)}
					style="--recuo:{recuo(d, porRef)}"
					aria-current={alvo === d.ref ? 'location' : undefined}
				>
					{#if marcavel}
						<input
							class="marca"
							type="checkbox"
							aria-label="No recorte: {d.rotulo || d.texto}"
							checked={marcados.includes(d.ref)}
							disabled={cobertos.has(d.ref) && !marcados.includes(d.ref)}
							onchange={(e) => marcar(d.ref, e.currentTarget.checked)}
						/>
					{/if}
					{#if AGRUPAMENTOS.includes(d.tipo)}
						<p class="rotulo-divisao"><span class="texto">{d.texto}</span></p>
						{#if d.nome}<p class="nome-divisao">{nomeLegivel(d.nome)}</p>{/if}
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
			{#if visiveis}
				<p class="fim-recorte">
					Fim do que o edital cobra. <button type="button" class="link" onclick={() => (inteira = true)}>
					{parcial ? 'Ler tudo o que foi importado' : 'Ler a lei inteira'}
				</button>
				</p>
			{/if}
		</article>

		{#if divisoes.length > 0}
			<nav class="sumario" aria-label="Sumário">
				<p class="sumario-titulo">Sumário</p>
				<ol>
					{#each divisoes as d (d.ref)}
						<li class="nivel-{d.tipo}" class:atual={atual === d.ref}>
							<a href="#{d.ref}" title={d.nome ? nomeLegivel(d.nome) : d.texto}>
								<span class="s-linhas">
									<span class="s-rotulo">{d.rotulo || d.texto}</span>
									{#if d.nome}<span class="s-nome">{nomeLegivel(d.nome)}</span>{/if}
								</span>
							</a>
						</li>
					{/each}
				</ol>
			</nav>
		{/if}
	</div>

	{#if ajustando}
		<div class="barra-ajuste" role="region" aria-label="Ajuste do recorte">
			<span>
				{marcados.length === 0 ? 'Nada marcado: a lei inteira' : `${marcados.length} ${marcados.length === 1 ? 'parte marcada' : 'partes marcadas'}`}
				· vale para {recorte?.materias.map((m) => m.nome).join(', ')}
			</span>
			<button type="button" class="btn" onclick={() => (ajustando = false)} disabled={salvandoRecorte}>Cancelar</button>
			<button type="button" class="btn primary" onclick={salvarRecorte} disabled={salvandoRecorte}>Salvar recorte</button>
		</div>
	{/if}

	<details class="manter">
		<summary>Manter esta lei</summary>
		{#if manutencao}<p class="ok" role="status">{manutencao}</p>{/if}
		{#if erroManutencao}<div class="form-error" role="alert">{erroManutencao}</div>{/if}

		<h2 class="sec">Importar questões</h2>
		<p class="page-sub">
			O <code>questoes.json</code> da lei, escrito para o texto publicado. Importar de novo não duplica nada, e as
			respostas das questões que continuam ficam.
		</p>
		<label class="arquivo">
			<span>Questões da lei (questoes.json)</span>
			<input type="file" accept=".json,application/json" disabled={importando} onchange={importarQuestoes} />
		</label>
		{#if importando}<p class="page-sub">Importando…</p>{/if}

		<h2 class="sec">Excluir</h2>
		{#if excluindo}
			<div class="confirmar" role="alertdialog" aria-label="Excluir {excluindo.curto}">
				<p>
					Excluir <b>{excluindo.curto}</b> do catálogo? Vão junto {excluindo.questoes}
					{excluindo.questoes === 1 ? 'questão' : 'questões'} e {excluindo.respostas}
					{excluindo.respostas === 1 ? 'resposta' : 'respostas'}, de quem quer que as tenha respondido. Não dá
					para desfazer.
				</p>
				<button class="btn danger" type="button" onclick={excluir}>Excluir de vez</button>
				<button class="btn" type="button" onclick={() => (excluindo = null)}>Cancelar</button>
			</div>
		{:else}
			<p class="page-sub">
				Tira a lei do catálogo, com as questões e as respostas. Para importar de novo, use o tópico do edital
				em Legislação.
			</p>
			<button class="btn danger" type="button" onclick={pedirExclusao}>Excluir esta lei</button>
		{/if}

		<h2 class="sec">Atualizar texto</h2>
		<p class="page-sub">
			Captura a lei de novo na fonte. As questões continuam; as de trechos que mudaram ficam marcadas para revisão.
		</p>
		<CapturaDeLei
			atual={{
				slug: leitura.lei.slug,
				nome: leitura.lei.nome,
				curto: leitura.lei.curto,
				fonte: leitura.lei.fonte,
				reconhecer: leitura.lei.reconhecer
			}}
			aoPublicar={textoPublicado}
		/>
	</details>

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
	.nome-lei {
		margin: 4px 0 14px;
		color: var(--text-muted);
		font-size: 14px;
		max-width: 68ch;
	}

	/* Propriedades, como no topo de uma página do Notion. */
	.props {
		display: grid;
		grid-template-columns: 120px minmax(0, 1fr);
		gap: 6px 12px;
		margin: 0 0 18px;
		font-size: 13.5px;
	}
	.props dt {
		color: var(--text-faint);
	}
	.props dd {
		margin: 0;
		color: var(--text);
	}
	.props a {
		color: var(--text-muted);
	}
	.props .vazio {
		color: var(--text-faint);
	}
	.props .bom {
		color: var(--good);
	}
	.tag {
		display: inline-block;
		font-size: 12px;
		padding: 1px 8px;
		border-radius: 4px;
		background: var(--c3-bg);
		color: var(--c3-tx);
	}

	.callout {
		display: flex;
		gap: 12px;
		padding: 14px 16px;
		margin: 0 0 18px;
		border-radius: 8px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
	}
	.callout-ic {
		font-size: 18px;
		line-height: 1.3;
	}
	.callout-corpo {
		flex: 1;
		min-width: 0;
	}
	.callout-titulo {
		margin: 0 0 6px;
		font-weight: 600;
		font-size: 14px;
	}
	.trechos {
		list-style: none;
		margin: 0;
		padding: 0;
		font-size: 14px;
	}
	.trechos li {
		padding: 2px 0;
	}
	.trechos a {
		color: var(--text);
		text-decoration: none;
		border-bottom: 1px solid var(--border-strong);
	}
	.trechos a:hover {
		border-color: var(--text-muted);
	}
	.artigos {
		margin-left: 8px;
		color: var(--text-muted);
		font-size: 12.5px;
	}
	.acoes {
		display: flex;
		align-items: center;
		gap: 14px;
		flex-wrap: wrap;
		margin-top: 10px;
	}
	.alternar {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
	}
	.alternar button {
		font: inherit;
		font-size: 12.5px;
		padding: 4px 11px;
		background: none;
		border: 0;
		color: var(--text-muted);
		cursor: pointer;
	}
	.alternar button + button {
		border-left: 1px solid var(--border);
	}
	.alternar button[aria-pressed='true'] {
		background: var(--bg-hover);
		color: var(--text);
		font-weight: 600;
	}
	.link {
		font: inherit;
		font-size: 12.5px;
		background: none;
		border: 0;
		padding: 0;
		color: var(--text-muted);
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}
	.link:hover {
		color: var(--text);
	}

	/* Toggle do Notion: seta, título, conteúdo recolhível. */
	.toggle {
		margin: 0 0 18px;
	}
	.toggle > summary {
		cursor: pointer;
		font-weight: 600;
		font-size: 14px;
		list-style: none;
		padding: 4px 0;
	}
	.toggle > summary::before {
		content: '▸';
		display: inline-block;
		width: 16px;
		color: var(--text-faint);
		transition: transform 0.15s;
	}
	.toggle[open] > summary::before {
		transform: rotate(90deg);
	}
	.conta {
		color: var(--text-faint);
		font-weight: 400;
		margin-left: 4px;
	}
	.unidades {
		list-style: none;
		margin: 6px 0 0 16px;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
		gap: 8px;
	}
	.unidade {
		display: flex;
		flex-direction: column;
		gap: 4px;
		width: 100%;
		height: 100%;
		text-align: left;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 9px 11px;
		color: var(--text);
		font: inherit;
		font-size: 13px;
		cursor: pointer;
	}
	.unidade:hover {
		background: var(--bg-hover);
	}
	.u-placar {
		color: var(--text-muted);
		font-size: 11.5px;
	}
	.barra {
		display: flex;
		height: 3px;
		border-radius: 2px;
		background: var(--border);
		overflow: hidden;
		margin-top: 2px;
	}
	.barra .certas {
		background: var(--good);
	}
	.barra .erradas {
		background: var(--danger);
	}

	/* Texto à esquerda, sumário à direita. */
	.leitor {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 210px;
		gap: 32px;
		padding: 4px 0 40px;
		align-items: start;
	}
	@media (max-width: 1000px) {
		.leitor {
			grid-template-columns: minmax(0, 1fr);
		}
		.sumario {
			display: none;
		}
	}
	.texto-lei {
		font-family: Georgia, 'Source Serif 4', 'Times New Roman', serif;
		font-size: 16.5px;
		line-height: 1.7;
		color: var(--text);
		min-width: 0;
	}
	.dispositivo {
		position: relative;
		padding: 1px 8px 1px calc(8px + var(--recuo) * 24px);
		border-radius: 6px;
		scroll-margin-top: 16px;
	}
	.dispositivo.alvo {
		background: var(--accent-soft);
	}
	.dispositivo.revogado .texto {
		color: var(--text-faint);
		text-decoration: line-through;
		text-decoration-color: var(--border-strong);
	}
	.tipo-artigo {
		margin-top: 16px;
	}
	.linha {
		display: flex;
		gap: 12px;
		align-items: baseline;
	}
	.texto {
		margin: 0;
		flex: 1;
	}
	.texto b {
		font-family: var(--font-ui);
		font-weight: 700;
		font-size: 0.92em;
	}
	/* As divisões: rótulo miúdo, nome como título de seção. */
	.rotulo-divisao {
		margin: 34px 0 0;
		font-family: var(--font-ui);
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.nome-divisao {
		margin: 2px 0 6px;
		font-family: var(--font-ui);
		font-weight: 700;
		line-height: 1.3;
	}
	.tipo-titulo .nome-divisao {
		font-size: 22px;
	}
	.tipo-capitulo .nome-divisao {
		font-size: 19px;
	}
	.tipo-secao .nome-divisao,
	.tipo-subsecao .nome-divisao {
		font-size: 16px;
	}
	.tipo-titulo .rotulo-divisao {
		margin-top: 44px;
		padding-top: 14px;
		border-top: 1px solid var(--border);
	}
	.tipo-preambulo .texto,
	.tipo-fecho .texto {
		color: var(--text-muted);
		font-size: 14.5px;
	}
	.selo {
		flex: none;
		font-family: var(--font-ui);
		font-size: 11.5px;
		white-space: nowrap;
		border: 0;
		border-radius: 4px;
		padding: 1px 8px;
		background: var(--accent-soft);
		color: var(--text-muted);
		cursor: pointer;
	}
	.selo:hover {
		color: var(--text);
	}
	.selo.tudo-certo {
		background: var(--good-soft);
		color: var(--good);
	}
	.selo.com-erro {
		background: var(--danger-soft);
		color: var(--danger);
	}
	.notas {
		list-style: none;
		margin: 1px 0 0;
		padding: 0;
		font-family: var(--font-ui);
		font-size: 11.5px;
		line-height: 1.45;
		color: var(--text-faint);
	}
	.anteriores {
		margin: 2px 0 4px;
		font-family: var(--font-ui);
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
	.fim-recorte {
		margin: 36px 0 0;
		padding-top: 14px;
		border-top: 1px dashed var(--border-strong);
		font-family: var(--font-ui);
		font-size: 13px;
		color: var(--text-muted);
	}

	/* Ajuste do recorte: uma caixa na margem de cada divisão e artigo. */
	.ajustando .dispositivo {
		padding-left: calc(34px + var(--recuo) * 24px);
	}
	.marca {
		position: absolute;
		left: 8px;
		top: 0.55em;
	}
	.ajustando .rotulo-divisao + .nome-divisao,
	.ajustando .rotulo-divisao {
		margin-top: 18px;
	}
	.dispositivo.no-recorte {
		background: var(--accent-soft);
	}
	.barra-ajuste {
		position: sticky;
		bottom: 12px;
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
		padding: 10px 14px;
		margin: 0 0 20px;
		border-radius: 8px;
		background: var(--bg-card);
		border: 1px solid var(--border-strong);
		box-shadow: var(--shadow-pop);
		font-size: 13px;
	}
	.barra-ajuste span {
		flex: 1;
		color: var(--text-muted);
	}

	/* Sumário à direita: rótulo e nome numa linha, a divisão atual marcada. */
	.sumario {
		position: sticky;
		top: 16px;
		max-height: calc(100vh - 32px);
		overflow-y: auto;
		font-size: 12px;
	}
	.sumario-titulo {
		margin: 0 0 6px;
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.sumario ol {
		list-style: none;
		margin: 0;
		padding: 0;
		border-left: 1px solid var(--border);
	}
	.sumario a {
		display: block;
		padding: 3px 8px;
		margin-left: -1px;
		border-left: 2px solid transparent;
		color: var(--text-faint);
		text-decoration: none;
		line-height: 1.35;
	}
	.s-linhas {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.sumario a:hover {
		color: var(--text);
	}
	.sumario .atual a {
		color: var(--text);
		border-left-color: var(--accent);
	}
	.s-rotulo {
		font-weight: 600;
		margin-right: 4px;
	}
	.sumario .nivel-titulo a {
		color: var(--text-muted);
		margin-top: 6px;
	}
	.sumario .nivel-capitulo a {
		padding-left: 16px;
	}
	.sumario .nivel-secao a,
	.sumario .nivel-subsecao a {
		padding-left: 26px;
	}
	.sumario .nivel-secao .s-rotulo,
	.sumario .nivel-subsecao .s-rotulo {
		font-weight: 400;
	}

	.manter {
		margin: 20px 0 40px;
		border-top: 1px solid var(--border);
		padding-top: 12px;
	}
	.manter > summary {
		cursor: pointer;
		color: var(--text-muted);
		font-size: 13px;
	}
	.confirmar {
		display: flex;
		gap: 10px;
		align-items: center;
		flex-wrap: wrap;
		padding: 12px 14px;
		margin: 0 0 12px;
		border-radius: 8px;
		border: 1px solid var(--danger);
		background: var(--danger-soft);
		font-size: 13.5px;
	}
	.confirmar p {
		flex: 1 1 100%;
		margin: 0;
	}
	.arquivo {
		display: inline-flex;
		flex-direction: column;
		gap: 6px;
		font-size: 13px;
	}
	.ok {
		color: var(--good);
		font-size: 13px;
	}
</style>
