<script lang="ts">
	import { onMount } from 'svelte';
	import { goto, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { browser } from '$app/environment';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { tagStyle } from '$lib/format';
	import { chave } from '$lib/storageKey';
	import { provasApi } from '$lib/provas/api';
	import { corDaMateria } from '$lib/provas/resolucao';
	import { lerRespostas } from '$lib/provas/respostas';
	import {
		ajustarAoCatalogo,
		enderecoDoTreino,
		filtrarTreino,
		lerFiltro,
		opcoesDoTreino,
		type FiltroDoTreino,
		type RespostasPorProva,
		type SituacaoDoFiltro
	} from '$lib/provas/treino';
	import type { ImportacaoResumo, ProvaResumo, QuestaoAvulsa } from '$lib/provas/types';

	type Aba = 'provas' | 'materia' | 'curadoria';

	const ABAS: Aba[] = ['provas', 'materia', 'curadoria'];

	const SITUACOES: [SituacaoDoFiltro, string][] = [
		['todas', 'Todas'],
		['abertas', 'Não resolvidas'],
		['erradas', 'Que errei']
	];

	const ESTADO: Record<ImportacaoResumo['estado'], string> = {
		na_fila: 'na fila',
		processando: 'processando',
		em_revisao: 'em revisão',
		falhou: 'falhou',
		publicada: 'publicada',
		cancelada: 'cancelada'
	};

	// O filtro e a aba ficam no navegador: quem treina Português volta amanhã e
	// encontra Português marcado.
	const CHAVE_FILTRO = chave('.questoes.filtro');
	const CHAVE_ABA = chave('.questoes.aba');

	function ler(k: string): string | null {
		if (!browser) return null;
		try {
			return localStorage.getItem(k);
		} catch {
			return null;
		}
	}

	function gravar(k: string, v: string) {
		try {
			localStorage.setItem(k, v);
		} catch {
			// Sem armazenamento: o filtro vale só enquanto a tela está aberta.
		}
	}

	function abaValida(a: string | null): Aba | null {
		return ABAS.find((x) => x === a) ?? null;
	}

	// O endereço manda (a curadoria volta para ?aba=curadoria); sem ele, a última aba usada.
	let aba = $state<Aba>(abaValida(page.url.searchParams.get('aba')) ?? abaValida(ler(CHAVE_ABA)) ?? 'provas');
	let filtro = $state<FiltroDoTreino>(lerFiltro(ler(CHAVE_FILTRO)));

	let provas = $state<ProvaResumo[]>([]);
	let avulsas = $state<QuestaoAvulsa[]>([]);
	let respostas = $state<RespostasPorProva>({});
	let curador = $state(false);
	let importacoes = $state<ImportacaoResumo[]>([]);
	let erro = $state('');
	let ocupado = $state(false);
	let carregado = $state(false);
	let arquivoProva = $state<HTMLInputElement | null>(null);
	let arquivoGabarito = $state<HTMLInputElement | null>(null);
	// O seletor do navegador corta o nome; o curador precisa dele inteiro para
	// saber que prova está mandando.
	let nomeProva = $state('');
	let nomeGabarito = $state('');

	$effect(() => {
		// Antes de carregar, o filtro ainda não foi conferido com o catálogo.
		if (carregado) gravar(CHAVE_FILTRO, JSON.stringify(filtro));
	});

	function trocarAba(nova: Aba) {
		aba = nova;
		gravar(CHAVE_ABA, nova);
		try {
			const url = new URL(page.url);
			url.searchParams.set('aba', nova);
			replaceState(url, {});
		} catch {
			// Roteador ainda não pronto: a aba troca, só o endereço não.
		}
	}

	// --- provas -------------------------------------------------------------

	/** As provas em grupos por ano, do mais recente, como uma galeria agrupada. */
	const porAno = $derived.by(() => {
		const grupos = new Map<number, ProvaResumo[]>();
		for (const p of provas) grupos.set(p.ano, [...(grupos.get(p.ano) ?? []), p]);
		for (const lista of grupos.values())
			lista.sort((a, b) => a.orgao.localeCompare(b.orgao, 'pt-BR') || a.cargo.localeCompare(b.cargo, 'pt-BR'));
		return [...grupos].sort(([a], [b]) => b - a);
	});

	function feitas(p: ProvaResumo): number {
		return Object.values(respostas[p.id] ?? {}).filter((r) => r.conferida).length;
	}

	// --- por matéria --------------------------------------------------------

	const opcoes = $derived(opcoesDoTreino(avulsas, filtro, respostas));
	const escolhidas = $derived(filtrarTreino(avulsas, filtro, respostas));
	const deQuantasProvas = $derived(new Set(escolhidas.map((q) => q.provaId)).size);

	function alternarMateria(m: string) {
		filtro.materias = filtro.materias.includes(m) ? filtro.materias.filter((x) => x !== m) : [...filtro.materias, m];
	}

	/** "Língua Portuguesa e Redes" — o que o botão vai resolver, numa frase. */
	const descricao = $derived.by(() => {
		const ms = filtro.materias;
		const materias =
			ms.length === 0
				? 'Todas as matérias'
				: ms.length <= 2
					? ms.join(' e ')
					: `${ms.slice(0, 2).join(', ')} e mais ${ms.length - 2}`;
		const partes = [materias];
		if (filtro.ano) partes.push(`provas de ${filtro.ano}`);
		if (filtro.situacao === 'abertas') partes.push('só as não resolvidas');
		if (filtro.situacao === 'erradas') partes.push('só as que você errou');
		if (escolhidas.length) partes.push(deQuantasProvas === 1 ? 'de 1 prova' : `de ${deQuantasProvas} provas`);
		return partes.join(' · ');
	});

	// --- curadoria ----------------------------------------------------------

	async function executar(fn: () => Promise<void>) {
		ocupado = true;
		erro = '';
		try {
			await fn();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'erro inesperado';
		} finally {
			ocupado = false;
		}
	}

	async function importar() {
		const prova = arquivoProva?.files?.[0];
		if (!prova) throw new Error('escolha o PDF da prova');
		const nova = await provasApi.importar(prova, arquivoGabarito?.files?.[0] ?? null);
		// O mesmo caderno devolve a importação que já existe; a nova nasce na
		// fila, na primeira versão.
		const existente = nova.versao > 1 || nova.estado !== 'na_fila';
		await goto(`/provas/importacoes/${nova.id}${existente ? '?existente=1' : ''}`);
	}

	onMount(() => {
		void executar(async () => {
			const [ps, qs, souCurador] = await Promise.all([
				provasApi.todasAsProvas(),
				provasApi.questoes(),
				provasApi.souCurador()
			]);
			provas = ps;
			avulsas = qs;
			curador = souCurador;
			respostas = Object.fromEntries(ps.map((p) => [p.id, lerRespostas(p.id)]));
			filtro = ajustarAoCatalogo(filtro, qs);
			if (aba === 'curadoria' && !curador) aba = 'provas';
			carregado = true;
			if (curador) importacoes = await provasApi.importacoes();
		});
	});
</script>

<svelte:head><title>Questões</title></svelte:head>

<PageHead
	icone="questoes"
	titulo="Questões"
	sub="Resolva uma prova inteira, como no dia da prova, ou treine uma matéria com as questões de todas as provas."
	mostrarProps={false}
/>

<div class="page">
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	<div class="abas" role="tablist" aria-label="Como resolver">
		<button type="button" role="tab" aria-selected={aba === 'provas'} onclick={() => trocarAba('provas')}>
			<NavIcon name="prova" size="sm" /> Provas {#if carregado}<span class="n">{provas.length}</span>{/if}
		</button>
		<button type="button" role="tab" aria-selected={aba === 'materia'} onclick={() => trocarAba('materia')}>
			<NavIcon name="conteudo" size="sm" /> Por matéria {#if carregado}<span class="n">{avulsas.length}</span>{/if}
		</button>
		{#if curador}
			<button type="button" role="tab" aria-selected={aba === 'curadoria'} onclick={() => trocarAba('curadoria')}>
				<NavIcon name="config" size="sm" /> Curadoria
			</button>
		{/if}
	</div>

	{#if !carregado}
		{#if !erro}<p class="vazio">Carregando as questões…</p>{/if}
	{:else if aba === 'provas'}
		<p class="dica">A prova como caiu: as questões na ordem do caderno, com o texto de apoio e o gabarito oficial.</p>
		{#each porAno as [ano, lista] (ano)}
			<section>
				<h2 class="grupo">{ano} <span class="n">{lista.length}</span></h2>
				<div class="galeria">
					{#each lista as p (p.id)}
						{@const feito = feitas(p)}
						<a class="prova" href="/provas/{p.id}">
							<span class="orgao">{p.orgao}</span>
							<span class="cargo" title={p.cargoNome}>{p.cargoNome || `Cargo ${p.cargo}`}</span>
							<span class="meta">
								{p.banca} · {p.cargo} · {p.total} questões
								{#if p.gabaritoTipo === 'preliminar'}· <span class="preliminar">gabarito preliminar</span>{/if}
							</span>
							<span class="andamento">
								<span class="trilho" aria-hidden="true"><i style="width:{(100 * feito) / Math.max(1, p.total)}%"></i></span>
								{#if feito === 0}
									Não começada
								{:else if feito >= p.total}
									Resolvida inteira
								{:else}
									{feito} de {p.total} resolvidas
								{/if}
							</span>
						</a>
					{/each}
				</div>
			</section>
		{:else}
			<p class="vazio">Nenhuma prova publicada ainda.</p>
		{/each}
	{:else if aba === 'materia'}
		<p class="dica">
			As questões de todas as provas, juntas por matéria. A questão que caiu igual em dois cargos aparece uma vez só.
		</p>

		<div class="propriedades">
			<div class="propriedade">
				<span class="rotulo"><NavIcon name="conteudo" size="sm" /> Matérias</span>
				<div class="valor" class:escolhendo={filtro.materias.length > 0}>
					{#each opcoes.materias as [m, n] (m)}
						{@const marcada = filtro.materias.includes(m)}
						<button
							type="button"
							class="etiqueta"
							style={tagStyle(corDaMateria(m))}
							aria-pressed={marcada}
							onclick={() => alternarMateria(m)}
						>
							{#if marcada}<span class="check" aria-hidden="true">✓</span>{/if}{m}
							<span class="n">{n}</span>
						</button>
					{/each}
					{#if filtro.materias.length}
						<button type="button" class="limpar" onclick={() => (filtro.materias = [])}>Limpar</button>
					{/if}
				</div>
			</div>

			{#if opcoes.anos.length > 1}
				<div class="propriedade">
					<span class="rotulo"><NavIcon name="datas" size="sm" /> Ano</span>
					<div class="valor">
						<button type="button" class="opcao" aria-pressed={!filtro.ano} onclick={() => (filtro.ano = 0)}>
							Todos
						</button>
						{#each opcoes.anos as [a, n] (a)}
							<button type="button" class="opcao" aria-pressed={filtro.ano === a} onclick={() => (filtro.ano = a)}>
								{a} <span class="n">{n}</span>
							</button>
						{/each}
					</div>
				</div>
			{/if}

			<div class="propriedade">
				<span class="rotulo"><NavIcon name="acerto" size="sm" /> Situação</span>
				<div class="valor">
					{#each SITUACOES as [s, rotulo] (s)}
						<button
							type="button"
							class="opcao"
							aria-pressed={filtro.situacao === s}
							onclick={() => (filtro.situacao = s)}
						>
							{rotulo} <span class="n">{opcoes.situacoes[s]}</span>
						</button>
					{/each}
				</div>
			</div>
		</div>

		<div class="resumo">
			<div>
				<strong>{escolhidas.length} {escolhidas.length === 1 ? 'questão' : 'questões'}</strong>
				<span>{descricao}</span>
			</div>
			{#if escolhidas.length}
				<a class="nbtn primario" href={enderecoDoTreino(filtro)}>Resolver →</a>
			{:else}
				<button type="button" class="nbtn primario" disabled>Resolver →</button>
			{/if}
		</div>
		{#if filtro.situacao === 'erradas' && escolhidas.length}
			<p class="dica rodape">As que você errou voltam em branco, para resolver de novo sem ver o gabarito.</p>
		{/if}
	{:else}
		<section>
			<h2 class="grupo">Importar prova da FCC</h2>
			<p class="dica">
				PDFs de até 25 MiB cada. A extração roda em segundo plano; a prova só entra no catálogo depois de revisada.
			</p>
			<div class="importar">
				<label class="arquivo">
					<span>Caderno de prova</span>
					<input
						type="file"
						accept="application/pdf"
						bind:this={arquivoProva}
						onchange={(e) => (nomeProva = e.currentTarget.files?.[0]?.name ?? '')}
					/>
					{#if nomeProva}<span class="nome-arquivo">{nomeProva}</span>{/if}
				</label>
				<label class="arquivo">
					<span>Gabarito oficial <em>opcional</em></span>
					<input
						type="file"
						accept="application/pdf"
						bind:this={arquivoGabarito}
						onchange={(e) => (nomeGabarito = e.currentTarget.files?.[0]?.name ?? '')}
					/>
					{#if nomeGabarito}<span class="nome-arquivo">{nomeGabarito}</span>{/if}
				</label>
				<button class="nbtn primario" type="button" disabled={ocupado} onclick={() => executar(importar)}>
					Importar
				</button>
			</div>
		</section>

		<section>
			<h2 class="grupo">Importações recentes</h2>
			<div class="linhas">
				{#each importacoes as i (i.id)}
					<a class="linha" href="/provas/importacoes/{i.id}">
						<span class="titulo">
							{i.orgao || 'Prova sem identificação'}
							{i.ano || ''}
							<span class="sub">{i.cargoNome || `cargo ${i.cargo || '—'}`}</span>
						</span>
						{#if i.estado === 'na_fila' || i.estado === 'processando'}
							<span class="sub">etapa {i.etapa} de {i.totalEtapas}</span>
						{/if}
						<span class="estado {i.estado}">{ESTADO[i.estado]}</span>
						{#if i.nomeDocumento}
							<span class="nome-arquivo">
								{i.nomeDocumento}{#if i.nomeGabarito}<br />gabarito: {i.nomeGabarito}{/if}
							</span>
						{/if}
						{#if i.erro}<span class="erro">{i.erro}</span>{/if}
					</a>
				{:else}
					<p class="vazio">Nenhuma importação ainda.</p>
				{/each}
			</div>
		</section>
	{/if}
</div>

<style>
	/* As abas de visão do Notion: texto, um traço embaixo da que está aberta. */
	.abas {
		display: flex;
		gap: 2px;
		margin-bottom: 14px;
		overflow-x: auto;
		border-bottom: 1px solid var(--border);
	}
	.abas button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 10px 9px;
		margin-bottom: -1px;
		font: inherit;
		font-size: 14px;
		white-space: nowrap;
		color: var(--text-muted);
		background: none;
		border: 0;
		border-bottom: 2px solid transparent;
		cursor: pointer;
	}
	.abas button:hover {
		color: var(--text);
	}
	.abas button[aria-selected='true'] {
		color: var(--text);
		font-weight: 600;
		border-bottom-color: var(--text);
	}
	.n {
		font-size: 12px;
		font-weight: 400;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}
	.dica {
		margin: 0 0 18px;
		font-size: 13.5px;
		color: var(--text-muted);
	}
	.dica.rodape {
		margin: 10px 2px 0;
		font-size: 13px;
		color: var(--text-faint);
	}
	.vazio {
		color: var(--text-faint);
		font-size: 14px;
	}

	/* --- provas: galeria agrupada por ano --- */
	.grupo {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 26px 0 10px;
		font-size: 14px;
		font-weight: 600;
		color: var(--text-muted);
	}
	section:first-of-type .grupo {
		margin-top: 0;
	}
	.galeria {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
		gap: 12px;
	}
	.prova {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 14px 16px 12px;
		color: inherit;
		text-decoration: none;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		transition:
			background 0.12s,
			border-color 0.12s;
	}
	.prova:hover {
		background: var(--bg-soft);
		border-color: var(--border-strong);
	}
	.orgao {
		font-size: 17px;
		font-weight: 700;
	}
	/* Nome de cargo da FCC é comprido: três linhas bastam para distinguir. */
	.cargo {
		display: -webkit-box;
		overflow: hidden;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		font-size: 14px;
		line-height: 1.4;
	}
	.meta {
		font-size: 12.5px;
		color: var(--text-faint);
	}
	.preliminar {
		color: var(--warn);
	}
	.andamento {
		display: grid;
		gap: 6px;
		margin-top: auto;
		padding-top: 10px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.trilho {
		height: 4px;
		overflow: hidden;
		border-radius: 2px;
		background: var(--bg-hover);
	}
	.trilho i {
		display: block;
		height: 100%;
		background: var(--good);
	}

	/* --- por matéria: as propriedades de uma página do Notion --- */
	.propriedades {
		display: grid;
		gap: 4px;
		margin-bottom: 20px;
	}
	.propriedade {
		display: grid;
		grid-template-columns: 130px minmax(0, 1fr);
		gap: 10px;
		align-items: start;
	}
	.rotulo {
		display: flex;
		align-items: center;
		gap: 7px;
		height: 34px;
		font-size: 14px;
		color: var(--text-muted);
	}
	.valor {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
		min-height: 34px;
		padding: 4px 0;
	}
	/* A etiqueta colorida do Notion: cada matéria com a sua cor, a mesma da prova. */
	.etiqueta {
		display: inline-flex;
		align-items: baseline;
		gap: 5px;
		max-width: 100%;
		padding: 2px 8px;
		font: inherit;
		font-size: 13.5px;
		line-height: 1.5;
		text-align: left;
		border: 0;
		border-radius: 4px;
		cursor: pointer;
		transition: opacity 0.1s;
	}
	.etiqueta .n {
		color: inherit;
		opacity: 0.65;
	}
	.etiqueta[aria-pressed='true'] {
		box-shadow: inset 0 0 0 1.5px currentColor;
		font-weight: 600;
	}
	.escolhendo .etiqueta:not([aria-pressed='true']) {
		opacity: 0.5;
	}
	.etiqueta:hover {
		opacity: 1 !important;
	}
	.check {
		font-size: 12px;
	}
	.limpar {
		padding: 2px 6px;
		font: inherit;
		font-size: 13px;
		color: var(--text-faint);
		background: none;
		border: 0;
		border-radius: 4px;
		cursor: pointer;
	}
	.limpar:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	/* Escolha de uma opção só: texto simples, a escolhida com fundo. */
	.opcao {
		padding: 3px 10px;
		font: inherit;
		font-size: 14px;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.opcao:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	.opcao[aria-pressed='true'] {
		color: var(--text);
		font-weight: 600;
		background: var(--bg-hover);
	}
	.resumo {
		display: flex;
		flex-wrap: wrap;
		gap: 12px 20px;
		align-items: center;
		justify-content: space-between;
		padding: 14px 16px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.resumo div {
		display: grid;
		gap: 2px;
		min-width: 0;
	}
	.resumo strong {
		font-size: 18px;
		font-weight: 700;
	}
	.resumo span {
		font-size: 13.5px;
		color: var(--text-muted);
	}

	/* --- curadoria --- */
	.importar {
		display: flex;
		flex-wrap: wrap;
		gap: 12px 16px;
		align-items: flex-end;
	}
	.arquivo {
		display: grid;
		gap: 5px;
		font-size: 13.5px;
		color: var(--text-muted);
	}
	.arquivo em {
		font-style: normal;
		color: var(--text-faint);
	}
	.arquivo input {
		max-width: 100%;
		font: inherit;
		font-size: 13px;
		color: var(--text-muted);
	}
	.arquivo input::file-selector-button {
		height: 30px;
		padding: 0 12px;
		margin-right: 10px;
		font: inherit;
		font-size: 13.5px;
		color: var(--text);
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
	}
	.linhas {
		border-top: 1px solid var(--border);
	}
	.linha {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 12px;
		align-items: center;
		padding: 9px 6px;
		font-size: 14px;
		color: inherit;
		text-decoration: none;
		border-bottom: 1px solid var(--border);
	}
	.linha:hover {
		background: var(--bg-hover);
	}
	.titulo {
		flex: 1 1 280px;
		min-width: 0;
		font-weight: 600;
	}
	.sub {
		font-weight: 400;
		font-size: 13px;
		color: var(--text-muted);
	}
	.titulo .sub {
		margin-left: 6px;
	}
	.estado {
		padding: 1px 8px;
		font-size: 12.5px;
		border-radius: 4px;
		color: var(--text-muted);
		background: var(--bg-hover);
	}
	.estado.em_revisao {
		color: var(--c0-tx);
		background: var(--c0-bg);
	}
	.estado.publicada {
		color: var(--good);
		background: var(--good-soft);
	}
	.estado.falhou {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.estado.na_fila,
	.estado.processando {
		color: var(--warn);
		background: var(--warn-soft);
	}
	.erro {
		flex-basis: 100%;
		font-size: 12.5px;
		color: var(--danger);
	}
	/* O nome inteiro, quebrando onde precisar: é por ele que o curador sabe
	   qual prova mandou. */
	.nome-arquivo {
		flex-basis: 100%;
		max-width: 44ch;
		font-size: 12.5px;
		color: var(--text-muted);
		overflow-wrap: anywhere;
	}
	.linha .nome-arquivo {
		max-width: none;
	}

	@media (max-width: 560px) {
		.propriedade {
			grid-template-columns: 1fr;
			gap: 0;
		}
		.rotulo {
			height: auto;
			padding-top: 6px;
		}
	}
</style>
