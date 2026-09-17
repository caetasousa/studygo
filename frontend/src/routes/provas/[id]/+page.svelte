<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { page } from '$app/state';
	import { goto, replaceState } from '$app/navigation';
	import { tagStyle } from '$lib/format';
	import Anotacao from '$lib/provas/Anotacao.svelte';
	import ResolucaoDaQuestao from '$lib/provas/ResolucaoDaQuestao.svelte';
	import { provasApi } from '$lib/provas/api';
	import { gravarRespostas, lerRespostas } from '$lib/provas/respostas';
	import { corDaMateria, placar, situacaoDaQuestao, type Respostas } from '$lib/provas/resolucao';
	import type { Anotacao as AnotacaoDaQuestao, Prova, Questao } from '$lib/provas/types';

	let prova = $state<Prova | null>(null);
	let curador = $state(false);
	let erro = $state('');
	let ocupado = $state(false);
	let materia = $state('');
	let mapaAberto = $state(false);
	let anotacoes = $state<Record<number, AnotacaoDaQuestao>>({});
	let nota = $state<{ editar: () => Promise<void> } | null>(null);
	let resolucao = $state<{ anotar: () => Promise<void> } | null>(null);
	let menuCuradoria = $state<HTMLDetailsElement | null>(null);
	// Textos que o estudante fechou: o texto das questões 1–10 fechado na 3
	// continua fechado na 4.
	let fechados = $state<Record<string, boolean>>({});

	const id = $derived(page.params.id ?? '');

	// Resolver mostra a questão, com a anotação fechada até responder — ela
	// costuma dizer qual é a resposta. Estudar é rever o que o estudante anotou:
	// só existe com anotação, anda só pelas questões anotadas e mostra só a nota,
	// sem repetir a questão.
	let modo = $state<'resolver' | 'estudar'>('resolver');

	// A questão aberta vai no endereço (?q=7): recarregar ou mandar o link não
	// perde o lugar.
	let atual = $state(Number(page.url.searchParams.get('q')) || 0);

	// As respostas ficam no navegador, por prova: recarregar a página não perde
	// a resolução. Lidas já na criação do estado: o efeito que grava roda antes
	// de um onMount, e gravaria o vazio por cima do que estava salvo.
	let respostas = $state<Respostas>(lerRespostas(page.params.id ?? ''));

	$effect(() => gravarRespostas(id, respostas));

	const resultado = $derived(prova ? placar(prova.questoes, respostas) : null);

	/** Matérias da prova, com quantas questões cada uma tem. */
	const materias = $derived.by(() => {
		const contagem = new Map<string, number>();
		for (const q of prova?.questoes ?? []) {
			if (q.disciplina) contagem.set(q.disciplina, (contagem.get(q.disciplina) ?? 0) + 1);
		}
		return [...contagem].sort(([a], [b]) => a.localeCompare(b, 'pt-BR'));
	});

	const totalAnotadas = $derived(Object.keys(anotacoes).length);
	const estudando = $derived(modo === 'estudar' && totalAnotadas > 0);
	const lista = $derived(
		prova
			? prova.questoes.filter((q) => (!materia || q.disciplina === materia) && (!estudando || anotacoes[q.numero]))
			: []
	);
	const posicao = $derived(Math.max(0, lista.findIndex((q) => q.numero === atual)));
	const questao = $derived<Questao | undefined>(lista[posicao]);
	const apoios = $derived(prova && questao ? prova.apoios.filter((a) => questao.apoios.includes(a.id)) : []);

	function estudar() {
		modo = 'estudar';
		if (!anotacoes[atual]) {
			const primeira = prova?.questoes.find((q) => anotacoes[q.numero] && (!materia || q.disciplina === materia));
			if (primeira) ir(primeira.numero);
		}
	}

	function ir(numero: number, fecharMapa = true) {
		atual = numero;
		if (fecharMapa) mapaAberto = false;
		try {
			const url = new URL(page.url);
			url.searchParams.set('q', String(numero));
			replaceState(url, {});
		} catch {
			// Roteador ainda não pronto: a questão abre, só o endereço não muda.
		}
		// Quem trocou de questão lá do pé da página volta ao título dela.
		void tick().then(() => {
			const el = document.getElementById('questao');
			const barra = document.querySelector('.barra');
			if (el && barra && el.getBoundingClientRect().top < barra.getBoundingClientRect().bottom)
				el.scrollIntoView({ block: 'start' });
		});
	}

	function passo(delta: number) {
		const q = lista[posicao + delta];
		if (q) ir(q.numero);
	}

	function marcar(numero: number, letra: string) {
		respostas[numero] = { marcada: letra, conferida: false };
	}

	function responder(numero: number) {
		const r = respostas[numero];
		if (r) r.conferida = true;
	}

	function refazer(numero: number) {
		delete respostas[numero];
	}

	async function anotar() {
		if (estudando) await nota?.editar();
		else await resolucao?.anotar();
	}

	// Atalhos como num leitor de prova: setas navegam, A–E marcam, Enter
	// responde, N abre a anotação. Nada disso vale enquanto se digita.
	function atalhos(e: KeyboardEvent) {
		if (e.key === 'Escape' && mapaAberto) {
			mapaAberto = false;
			return;
		}
		if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || !questao) return;
		const alvo = e.target as HTMLElement;
		if (alvo.closest('input, textarea, select, [contenteditable="true"]')) return;
		const r = respostas[questao.numero];
		// Enter sobre a alternativa que acabou de clicar responde, em vez de
		// clicá-la de novo; sobre os outros botões, é o clique deles.
		const enterResponde = !alvo.closest('button, a, summary') || !!alvo.closest('.alternativa');
		if (e.key === 'ArrowRight') passo(1);
		else if (e.key === 'ArrowLeft') passo(-1);
		else if (!estudando && /^[a-e]$/i.test(e.key) && !r?.conferida) marcar(questao.numero, e.key.toUpperCase());
		else if (!estudando && e.key === 'Enter' && r?.marcada && !r.conferida && enterResponde) responder(questao.numero);
		else if (e.key === 'n' || e.key === 'N') void anotar();
		else return;
		e.preventDefault();
	}

	// Clique fora fecha o mapa e o menu, como qualquer menu suspenso.
	function cliqueFora(e: MouseEvent) {
		const alvo = e.target as HTMLElement;
		if (mapaAberto && !alvo.closest('.barra')) mapaAberto = false;
		if (menuCuradoria?.open && !alvo.closest('.menu')) menuCuradoria.open = false;
	}

	function trocarMateria(m: string) {
		materia = m;
		if (!m || prova?.questoes.find((q) => q.numero === atual)?.disciplina === m) return;
		const primeira = prova?.questoes.find((q) => q.disciplina === m && (!estudando || anotacoes[q.numero]));
		if (primeira) ir(primeira.numero, false);
	}

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

	onMount(() => {
		void executar(async () => {
			const [p, souCurador, notas] = await Promise.all([
				provasApi.prova(id),
				provasApi.souCurador(),
				provasApi.anotacoes(id).catch(() => [])
			]);
			prova = p;
			curador = souCurador;
			anotacoes = Object.fromEntries(notas.map((a) => [a.numero, a]));
			if (!p.questoes.some((q) => q.numero === atual)) atual = p.questoes[0]?.numero ?? 0;
		});
	});
</script>

<svelte:window onkeydown={atalhos} onclick={cliqueFora} />

{#snippet filtroDeMateria()}
	<div class="materias" class:filtrando={!!materia} role="group" aria-label="Filtrar por matéria">
		<button type="button" class="chip todas" aria-pressed={!materia} onclick={() => trocarMateria('')}>
			Todas <span class="n">{prova?.questoes.length}</span>
		</button>
		{#each materias as [m, n] (m)}
			<button
				type="button"
				class="chip"
				style={tagStyle(corDaMateria(m))}
				aria-pressed={materia === m}
				title={materia === m ? 'Mostrar todas de novo' : `Só as questões de ${m}`}
				onclick={() => trocarMateria(materia === m ? '' : m)}>{m} <span class="n">{n}</span></button
			>
		{/each}
	</div>
{/snippet}

<svelte:head><title>{prova ? `${prova.orgao} ${prova.ano} · Questão ${questao?.numero ?? ''}` : 'Prova'}</title></svelte:head>

<div class="pagina">
	<div class="topo">
		<a class="voltar" href="/questoes">← Questões</a>
		{#if curador && prova}
			<details class="menu" bind:this={menuCuradoria}>
				<summary aria-label="Ações da curadoria" title="Ações da curadoria">•••</summary>
				<div class="menu-corpo">
					<button
						type="button"
						disabled={ocupado}
						onclick={() =>
							executar(async () => {
								const nova = await provasApi.revisar(prova!.id);
								await goto(`/provas/importacoes/${nova.id}`);
							})}
					>
						Abrir revisão
						<small>Corrigir texto, figura ou gabarito desta prova</small>
					</button>
					<button
						type="button"
						disabled={ocupado}
						onclick={() => {
							if (
								confirm(
									'Extrair o caderno de novo, do zero? Vira uma revisão nova (uns 3 minutos de IA); a prova publicada continua no ar até você publicar a nova.'
								)
							)
								void executar(async () => {
									const nova = await provasApi.reextrair(prova!.id);
									await goto(`/provas/importacoes/${nova.id}`);
								});
						}}
					>
						Extrair de novo
						<small>Refaz a leitura do PDF com o extrator atual</small>
					</button>
					<button
						type="button"
						class="perigo"
						disabled={ocupado}
						onclick={() => {
							if (confirm('Tirar esta prova do catálogo? Ela deixa de aparecer para todos.'))
								void executar(async () => {
									await provasApi.retirar(prova!.id);
									await goto('/questoes');
								});
						}}
					>
						Tirar do catálogo
						<small>Some para os alunos; as anotações ficam guardadas</small>
					</button>
				</div>
			</details>
		{/if}
	</div>

	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if prova}
		<header class="cabeca">
			<h1>{prova.orgao} {prova.ano}</h1>
			{#if prova.cargoNome || prova.cargo}<p class="cargo">{prova.cargoNome || prova.cargo}</p>{/if}
			<p class="meta">
				{prova.banca} · caderno {prova.caderno} · {prova.questoes.length} questões ·
				{#if prova.gabaritoTipo === 'preliminar'}gabarito preliminar{:else if prova.gabaritoTipo}gabarito definitivo{:else}sem gabarito{/if}
				{#if resultado && resultado.respondidas > 0}
					· {resultado.respondidas} respondidas{#if resultado.corrigiveis > 0}, {resultado.acertos} certas ({Math.round(
							(100 * resultado.acertos) / resultado.corrigiveis
						)}%){/if}
					<button
						type="button"
						class="link"
						onclick={() => {
							if (confirm('Apagar todas as respostas desta prova? As anotações ficam.')) respostas = {};
						}}>recomeçar</button
					>
				{/if}
			</p>
			{#if materias.length > 1}{@render filtroDeMateria()}{/if}
		</header>

		<div class="barra">
			<div class="controles">
				{#if totalAnotadas > 0}
					<div class="modos" role="group" aria-label="Modo">
						<button type="button" aria-pressed={!estudando} onclick={() => (modo = 'resolver')}>Resolver</button>
						<button
							type="button"
							aria-pressed={estudando}
							title="Rever só as suas anotações, questão por questão"
							onclick={estudar}>Estudar <span class="n">{totalAnotadas}</span></button
						>
					</div>
				{/if}
				{#if materia}
					<button type="button" class="filtro-ativo" title="Mostrar todas as matérias" onclick={() => trocarMateria('')}>
						{materia} ✕
					</button>
				{/if}
				<nav class="passos" aria-label="Navegar entre as questões">
					<button type="button" class="seta" disabled={posicao === 0} aria-label="Questão anterior" onclick={() => passo(-1)}
						>‹</button
					>
					<button
						type="button"
						class="contador"
						aria-expanded={mapaAberto}
						title="Todas as questões e o filtro por matéria · atalhos: ← → navegam, A–E marcam, Enter responde, N anota"
						onclick={() => (mapaAberto = !mapaAberto)}
					>
						{questao?.numero ?? '—'}
						<span class="de"
							>{estudando ? `· ${posicao + 1} de ${lista.length} anotadas` : `de ${prova.questoes.length}`}</span
						>
						▾
					</button>
					<button
						type="button"
						class="seta"
						disabled={posicao >= lista.length - 1}
						aria-label="Próxima questão"
						onclick={() => passo(1)}>›</button
					>
				</nav>
			</div>

			{#if mapaAberto}
				<div class="mapa" role="dialog" aria-label="Todas as questões">
					{#if materias.length > 1}{@render filtroDeMateria()}{/if}
					<div class="numeros">
						{#each lista as q (q.numero)}
							<button
								type="button"
								class="num {situacaoDaQuestao(q, respostas[q.numero])}"
								class:atual={q.numero === questao?.numero}
								aria-current={q.numero === questao?.numero ? 'true' : undefined}
								title="Questão {q.numero}{q.disciplina ? ` · ${q.disciplina}` : ''}{anotacoes[q.numero] ? ' · anotada' : ''}"
								onclick={() => ir(q.numero)}
							>
								{q.numero}
								{#if anotacoes[q.numero]}<i class="ponto" aria-label="anotada"></i>{/if}
							</button>
						{/each}
					</div>
					<p class="legenda">Verde: acertou · vermelho: errou · ponto: tem anotação</p>
				</div>
			{/if}
		</div>

		{#if questao && estudando}
			<article class="questao estudo" id="questao">
				<h2>
					Questão {questao.numero}
					{#if questao.disciplina}
						<span class="materia" style={tagStyle(corDaMateria(questao.disciplina))}>{questao.disciplina}</span>
					{/if}
					{#if questao.assunto}<span class="assunto">{questao.assunto}</span>{/if}
					<button type="button" class="link ver" onclick={() => (modo = 'resolver')}>ver a questão</button>
				</h2>
				{#key questao.numero}
					<Anotacao
						bind:this={nota}
						provaId={prova.id}
						numero={questao.numero}
						inicial={anotacoes[questao.numero]?.texto ?? ''}
						onsalvo={(a) => {
							if (a.texto) {
								anotacoes[a.numero] = a;
								return;
							}
							// Apagou a nota: a questão sai do estudo, que segue na próxima anotada.
							const seguinte = lista[posicao + 1] ?? lista[posicao - 1];
							delete anotacoes[a.numero];
							if (seguinte) ir(seguinte.numero);
						}}
					/>
				{/key}
				<footer class="rodape">
					<button type="button" class="vizinha" disabled={posicao === 0} onclick={() => passo(-1)}>
						← {lista[posicao - 1] ? `Questão ${lista[posicao - 1].numero}` : 'Anterior'}
					</button>
					<button type="button" class="vizinha" disabled={posicao >= lista.length - 1} onclick={() => passo(1)}>
						{lista[posicao + 1] ? `Questão ${lista[posicao + 1].numero}` : 'Próxima'} →
					</button>
				</footer>
			</article>
		{:else if questao}
			<article class="questao" id="questao">
				<h2>
					Questão {questao.numero}
					{#if questao.disciplina}
						<span class="materia" style={tagStyle(corDaMateria(questao.disciplina))}>{questao.disciplina}</span>
					{/if}
					{#if questao.assunto}<span class="assunto">{questao.assunto}</span>{/if}
				</h2>

				{#key questao.numero}
					<ResolucaoDaQuestao
						bind:this={resolucao}
						provaId={prova.id}
						{questao}
						{apoios}
						resposta={respostas[questao.numero]}
						nota={anotacoes[questao.numero]}
						{fechados}
						onalternarapoio={(apoio, aberto) => (fechados[apoio] = !aberto)}
						onmarcar={(letra) => marcar(questao.numero, letra)}
						onresponder={() => responder(questao.numero)}
						onrefazer={() => refazer(questao.numero)}
						onanotado={(a) => {
							if (a.texto) anotacoes[a.numero] = a;
							else delete anotacoes[a.numero];
						}}
					/>
				{/key}

				<footer class="rodape">
					<button type="button" class="vizinha" disabled={posicao === 0} onclick={() => passo(-1)}>
						← {lista[posicao - 1] ? `Questão ${lista[posicao - 1].numero}` : 'Anterior'}
					</button>
					<button type="button" class="vizinha" disabled={posicao >= lista.length - 1} onclick={() => passo(1)}>
						{lista[posicao + 1] ? `Questão ${lista[posicao + 1].numero}` : 'Próxima'} →
					</button>
				</footer>
			</article>
		{:else if prova.questoes.length === 0}
			<!-- Sem gabarito, a questão não aparece: responder sem poder conferir não é treinar. -->
			<p class="callout warn">
				<span>
					Esta prova ainda está sem gabarito, e as questões só aparecem com ele. Quem cuida da curadoria
					precisa enviar o gabarito na revisão da prova.
				</span>
			</p>
		{:else}
			<p class="dim">Nenhuma questão com esse filtro.</p>
		{/if}
	{:else if !erro}
		<p class="dim">Carregando a prova…</p>
	{/if}
</div>

<style>
	/* Uma coluna de leitura, sem molduras: a questão é o que importa. */
	.pagina {
		max-width: 760px;
		margin: 0 auto;
		padding: 8px 4px 64px;
		font-size: 16px;
		line-height: 1.6;
		color: var(--text);
	}
	.topo {
		display: flex;
		align-items: center;
		justify-content: space-between;
		min-height: 32px;
	}
	.voltar {
		font-size: 14px;
		color: var(--text-muted);
		text-decoration: none;
	}
	.voltar:hover {
		color: var(--text);
	}
	.menu {
		position: relative;
	}
	.menu summary {
		list-style: none;
		padding: 2px 10px;
		border-radius: 5px;
		color: var(--text-muted);
		cursor: pointer;
		letter-spacing: 1px;
	}
	.menu summary::-webkit-details-marker {
		display: none;
	}
	.menu summary:hover,
	.menu[open] summary {
		background: var(--bg-hover);
	}
	.menu-corpo {
		position: absolute;
		right: 0;
		top: calc(100% + 4px);
		z-index: 10;
		display: grid;
		width: 280px;
		padding: 6px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow-pop);
	}
	.menu-corpo button {
		display: grid;
		gap: 1px;
		padding: 7px 10px;
		text-align: left;
		font: inherit;
		font-size: 14px;
		color: var(--text);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.menu-corpo button:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	.menu-corpo small {
		font-size: 12px;
		color: var(--text-faint);
	}
	.menu-corpo .perigo {
		color: var(--danger);
	}

	.cabeca {
		padding: 18px 0 4px;
	}
	h1 {
		margin: 0;
		font-size: 28px;
		font-weight: 700;
		line-height: 1.2;
	}
	.cargo {
		margin: 4px 0 0;
		font-size: 15px;
		color: var(--text-muted);
	}
	.meta {
		margin: 6px 0 0;
		font-size: 13px;
		color: var(--text-faint);
	}
	.link {
		font: inherit;
		color: var(--text-muted);
		background: none;
		border: 0;
		padding: 0;
		text-decoration: underline;
		cursor: pointer;
	}
	.dim {
		color: var(--text-faint);
		font-size: 13px;
	}

	.barra {
		position: sticky;
		top: 0;
		z-index: 5;
		margin-top: 14px;
		background: var(--bg);
	}
	.controles {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
	}
	.modos {
		display: inline-flex;
		gap: 2px;
	}
	.modos button,
	.filtro-ativo {
		padding: 3px 10px;
		font: inherit;
		font-size: 13.5px;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.modos button:hover,
	.filtro-ativo:hover {
		background: var(--bg-hover);
	}
	.modos .n {
		font-size: 12px;
		color: var(--text-faint);
	}
	.modos button[aria-pressed='true'] {
		color: var(--text);
		background: var(--bg-hover);
		font-weight: 600;
	}
	.filtro-ativo {
		max-width: 40%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--accent);
	}
	.passos {
		display: inline-flex;
		align-items: center;
		margin-left: auto;
	}
	.passos button {
		height: 30px;
		font: inherit;
		color: var(--text);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.passos button:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	.passos button:disabled {
		opacity: 0.3;
		cursor: default;
	}
	.seta {
		width: 30px;
		font-size: 20px !important;
		line-height: 1;
	}
	.contador {
		padding: 0 8px;
		font-size: 14px !important;
		font-weight: 600;
	}
	.contador .de {
		font-weight: 400;
		color: var(--text-faint);
	}

	/* Suspenso sob a barra: abre de onde se estiver na página. */
	.mapa {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		right: 0;
		display: grid;
		gap: 12px;
		max-height: min(70vh, 540px);
		overflow: auto;
		padding: 12px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow-pop);
	}
	.materias {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
	/* As matérias como etiquetas coloridas, cada uma com a sua cor: clicar filtra. */
	.cabeca .materias {
		margin-top: 12px;
	}
	.chip {
		max-width: 100%;
		padding: 1px 8px;
		font: inherit;
		font-size: 13px;
		line-height: 1.6;
		text-align: left;
		white-space: normal;
		border: 0;
		border-radius: 4px;
		cursor: pointer;
	}
	.chip.todas {
		color: var(--text-muted);
		background: var(--bg-hover);
	}
	.chip .n {
		opacity: 0.65;
		margin-left: 2px;
	}
	.chip[aria-pressed='true'] {
		box-shadow: inset 0 0 0 1.5px currentColor;
	}
	.materias.filtrando .chip:not([aria-pressed='true']) {
		opacity: 0.5;
	}
	.chip:hover {
		opacity: 1 !important;
	}
	.numeros {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(38px, 1fr));
		gap: 4px;
	}
	.num {
		position: relative;
		height: 32px;
		font: inherit;
		font-size: 13px;
		color: var(--text-muted);
		background: var(--bg-hover);
		border: 1px solid transparent;
		border-radius: 6px;
		cursor: pointer;
	}
	.num:hover {
		color: var(--text);
	}
	.num.certa {
		color: var(--good);
		background: var(--good-soft);
	}
	.num.errada {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.num.atual {
		border-color: var(--accent);
		color: var(--text);
		font-weight: 700;
	}
	.ponto {
		position: absolute;
		top: 4px;
		right: 4px;
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--accent);
	}
	.legenda {
		margin: 0;
		font-size: 12px;
		color: var(--text-faint);
	}

	.questao {
		padding-top: 22px;
		scroll-margin-top: 48px;
	}
	h2 {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 10px;
		align-items: center;
		margin: 0 0 14px;
		font-size: 20px;
		font-weight: 700;
	}
	.materia {
		padding: 0 7px;
		border-radius: 4px;
		font-size: 12.5px;
		font-weight: 500;
		line-height: 1.7;
	}
	.assunto {
		font-size: 13px;
		font-weight: 400;
		color: var(--text-muted);
	}
	h2 .ver {
		margin-left: auto;
		font-size: 13px;
		font-weight: 400;
	}

	.rodape {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		margin-top: 32px;
	}
	.vizinha {
		padding: 6px 10px;
		font: inherit;
		font-size: 14px;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 6px;
		cursor: pointer;
	}
	.vizinha:hover:not(:disabled) {
		color: var(--text);
		background: var(--bg-hover);
	}
	.vizinha:disabled {
		visibility: hidden;
	}

	/* Abaixo de 900px o botão do menu (☰) fica fixo no canto: a barra presa
	   no topo abre espaço para ele. */
	@media (max-width: 900px) {
		.controles {
			padding-left: 32px;
		}
	}
	@media (max-width: 560px) {
		h1 {
			font-size: 24px;
		}
	}
</style>
