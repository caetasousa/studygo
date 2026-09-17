<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { tagStyle } from '$lib/format';
	import ResolucaoDaQuestao from '$lib/provas/ResolucaoDaQuestao.svelte';
	import { provasApi } from '$lib/provas/api';
	import { gravarRespostas, lerRespostas } from '$lib/provas/respostas';
	import { corDaMateria, situacaoDaQuestao, type Resposta } from '$lib/provas/resolucao';
	import {
		chaveDaAvulsa,
		filtrarTreino,
		filtroDoEndereco,
		nomeDoAssunto,
		respostaDaAvulsa,
		type RespostasPorProva
	} from '$lib/provas/treino';
	import type { Anotacao, Prova, QuestaoAvulsa } from '$lib/provas/types';

	// O filtro vem no endereço e vale para a sessão inteira.
	const filtro = filtroDoEndereco(page.url.searchParams);

	// A lista é tirada uma vez, na abertura: a questão respondida agora não some
	// da sessão "não resolvidas" no meio do caminho.
	let lista = $state<QuestaoAvulsa[]>([]);
	let respostas = $state<RespostasPorProva>({});
	// Numa sessão das que errou, a questão volta em branco: mostrar a resposta
	// antiga entregaria o gabarito. Marcar de novo grava por cima.
	let emBranco = $state<Record<string, boolean>>({});
	let provas = $state<Record<string, Prova>>({});
	let anotacoes = $state<Record<string, Record<number, Anotacao>>>({});
	// Textos fechados, por prova e texto: os ids se repetem entre provas.
	let fechados = $state<Record<string, boolean>>({});
	let atual = $state(0);
	let mapaAberto = $state(false);
	let erro = $state('');
	let carregado = $state(false);
	let resolucao = $state<{ anotar: () => Promise<void> } | null>(null);

	const item = $derived<QuestaoAvulsa | undefined>(lista[atual]);
	const prova = $derived(item ? provas[item.provaId] : undefined);
	const questao = $derived(prova?.questoes.find((q) => q.numero === item?.numero));
	const apoios = $derived(prova && questao ? prova.apoios.filter((a) => questao.apoios.includes(a.id)) : []);
	const fechadosDaProva = $derived(
		Object.fromEntries(
			Object.entries(fechados)
				.filter(([k]) => item && k.startsWith(`${item.provaId}/`))
				.map(([k, v]) => [k.slice(k.indexOf('/') + 1), v])
		)
	);

	function resposta(q: QuestaoAvulsa): Resposta | undefined {
		return emBranco[chaveDaAvulsa(q)] ? undefined : respostaDaAvulsa(q, respostas);
	}

	const situacoes = $derived(lista.map((q) => situacaoDaQuestao(q, resposta(q))));
	const respondidas = $derived(situacoes.filter((s) => s !== 'aberta').length);
	const certas = $derived(situacoes.filter((s) => s === 'certa').length);
	const corrigidas = $derived(certas + situacoes.filter((s) => s === 'errada').length);

	const titulo = $derived.by(() => {
		const ms = filtro.materias;
		if (ms.length === 0) return 'Todas as matérias';
		if (ms.length === 1) return ms[0];
		return `${ms.slice(0, -1).join(', ')} e ${ms.at(-1)}`;
	});

	const detalhes = $derived.by(() => {
		const partes = [`${lista.length} ${lista.length === 1 ? 'questão' : 'questões'}`];
		if (filtro.assuntos.length) partes.push(filtro.assuntos.map(nomeDoAssunto).join(', '));
		if (filtro.ano) partes.push(`provas de ${filtro.ano}`);
		if (filtro.situacao === 'abertas') partes.push('não resolvidas');
		if (filtro.situacao === 'erradas') partes.push('as que você errou');
		if (respondidas > 0) {
			const acerto = corrigidas > 0 ? `, ${certas} certas (${Math.round((100 * certas) / corrigidas)}%)` : '';
			partes.push(`${respondidas} respondidas${acerto}`);
		}
		return partes.join(' · ');
	});

	/** O mapa agrupado por prova, na ordem da sessão. */
	const grupos = $derived.by(() => {
		const out: { provaId: string; rotulo: string; posicoes: number[] }[] = [];
		lista.forEach((q, i) => {
			const ultimo = out.at(-1);
			if (ultimo?.provaId === q.provaId) ultimo.posicoes.push(i);
			else out.push({ provaId: q.provaId, rotulo: `${q.orgao} ${q.ano} · ${q.cargo}`, posicoes: [i] });
		});
		return out;
	});

	// --- respostas ------------------------------------------------------------

	function guardar(q: QuestaoAvulsa, r: Resposta | undefined) {
		const daProva = { ...(respostas[q.provaId] ?? {}) };
		if (r) daProva[q.numero] = r;
		else delete daProva[q.numero];
		respostas[q.provaId] = daProva;
		delete emBranco[chaveDaAvulsa(q)];
		gravarRespostas(q.provaId, daProva);
	}

	function marcar(q: QuestaoAvulsa, letra: string) {
		guardar(q, { marcada: letra, conferida: false });
	}

	function responder(q: QuestaoAvulsa) {
		const r = resposta(q);
		if (r?.marcada) guardar(q, { marcada: r.marcada, conferida: true });
	}

	// --- provas, carregadas na vez da questão ----------------------------------

	const pedidas = new Map<string, Promise<void>>();

	function garantir(provaId: string): Promise<void> {
		let pedido = pedidas.get(provaId);
		if (!pedido) {
			pedido = Promise.all([provasApi.prova(provaId), provasApi.anotacoes(provaId).catch(() => [])]).then(
				([p, notas]) => {
					provas[provaId] = p;
					anotacoes[provaId] = Object.fromEntries(notas.map((a) => [a.numero, a]));
				}
			);
			// Falhou: a próxima visita à questão tenta de novo.
			pedido.catch(() => pedidas.delete(provaId));
			pedidas.set(provaId, pedido);
		}
		return pedido;
	}

	$effect(() => {
		// A prova da questão aberta e a da seguinte: passar de uma prova para a
		// outra não espera a rede.
		const agora = lista[atual];
		const depois = lista[atual + 1];
		if (agora)
			garantir(agora.provaId).catch((e) => (erro = e instanceof Error ? e.message : 'erro ao abrir a prova'));
		if (depois && depois.provaId !== agora?.provaId) garantir(depois.provaId).catch(() => {});
	});

	// --- navegação ------------------------------------------------------------

	function ir(i: number, fecharMapa = true) {
		if (!lista[i]) return;
		atual = i;
		erro = '';
		if (fecharMapa) mapaAberto = false;
		// A questão aberta vai no endereço: recarregar não perde o lugar.
		try {
			const url = new URL(page.url);
			url.searchParams.set('q', chaveDaAvulsa(lista[i]));
			replaceState(url, {});
		} catch {
			// Roteador ainda não pronto: a questão abre, só o endereço não muda.
		}
		void tick().then(() => {
			const el = document.getElementById('questao');
			const barra = document.querySelector('.barra');
			if (el && barra && el.getBoundingClientRect().top < barra.getBoundingClientRect().bottom)
				el.scrollIntoView({ block: 'start' });
		});
	}

	// Os mesmos atalhos da prova: setas navegam, A–E marcam, Enter responde,
	// N abre a anotação. Nada disso vale enquanto se digita.
	function atalhos(e: KeyboardEvent) {
		if (e.key === 'Escape' && mapaAberto) {
			mapaAberto = false;
			return;
		}
		if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || !item || !questao) return;
		const alvo = e.target as HTMLElement;
		if (alvo.closest('input, textarea, select, [contenteditable="true"]')) return;
		const r = resposta(item);
		const enterResponde = !alvo.closest('button, a, summary') || !!alvo.closest('.alternativa');
		if (e.key === 'ArrowRight') ir(atual + 1);
		else if (e.key === 'ArrowLeft') ir(atual - 1);
		else if (/^[a-e]$/i.test(e.key) && !r?.conferida) marcar(item, e.key.toUpperCase());
		else if (e.key === 'Enter' && r?.marcada && !r.conferida && enterResponde) responder(item);
		else if (e.key === 'n' || e.key === 'N') void resolucao?.anotar();
		else return;
		e.preventDefault();
	}

	function cliqueFora(e: MouseEvent) {
		if (mapaAberto && !(e.target as HTMLElement).closest('.barra')) mapaAberto = false;
	}

	onMount(() => {
		void (async () => {
			try {
				const qs = await provasApi.questoes(filtro.materias, filtro.ano);
				const lidas = Object.fromEntries([...new Set(qs.map((q) => q.provaId))].map((id) => [id, lerRespostas(id)]));
				const escolhidas = filtrarTreino(qs, filtro, lidas);
				respostas = lidas;
				if (filtro.situacao === 'erradas') emBranco = Object.fromEntries(escolhidas.map((q) => [chaveDaAvulsa(q), true]));
				lista = escolhidas;
				const pedida = escolhidas.findIndex((q) => chaveDaAvulsa(q) === page.url.searchParams.get('q'));
				atual = Math.max(0, pedida);
			} catch (e) {
				erro = e instanceof Error ? e.message : 'erro inesperado';
			} finally {
				carregado = true;
			}
		})();
	});
</script>

<svelte:window onkeydown={atalhos} onclick={cliqueFora} />

<svelte:head><title>{item ? `${titulo} · ${atual + 1} de ${lista.length}` : 'Questões'}</title></svelte:head>

<div class="pagina">
	<div class="topo"><a class="voltar" href="/questoes?aba=materia">← Questões</a></div>

	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if item}
		<header class="cabeca">
			<h1>{titulo}</h1>
			<p class="meta">{detalhes}</p>
		</header>

		<div class="barra">
			<div class="controles">
				<nav class="passos" aria-label="Navegar entre as questões">
					<button type="button" class="seta" disabled={atual === 0} aria-label="Questão anterior" onclick={() => ir(atual - 1)}
						>‹</button
					>
					<button
						type="button"
						class="contador"
						aria-expanded={mapaAberto}
						title="Todas as questões da sessão · atalhos: ← → navegam, A–E marcam, Enter responde, N anota"
						onclick={() => (mapaAberto = !mapaAberto)}
					>
						{atual + 1} <span class="de">de {lista.length}</span> ▾
					</button>
					<button
						type="button"
						class="seta"
						disabled={atual >= lista.length - 1}
						aria-label="Próxima questão"
						onclick={() => ir(atual + 1)}>›</button
					>
				</nav>
			</div>

			{#if mapaAberto}
				<div class="mapa" role="dialog" aria-label="Todas as questões da sessão">
					{#each grupos as g (g.provaId)}
						<div class="grupo">
							<p class="grupo-rotulo">{g.rotulo}</p>
							<div class="numeros">
								{#each g.posicoes as i (i)}
									{@const q = lista[i]}
									<button
										type="button"
										class="num {situacoes[i]}"
										class:atual={i === atual}
										aria-current={i === atual ? 'true' : undefined}
										title="Questão {q.numero} · {q.disciplina}{q.assunto ? ` › ${q.assunto}` : ''}{anotacoes[q.provaId]?.[q.numero] ? ' · anotada' : ''}"
										onclick={() => ir(i)}
									>
										{q.numero}
										{#if anotacoes[q.provaId]?.[q.numero]}<i class="ponto" aria-label="anotada"></i>{/if}
									</button>
								{/each}
							</div>
						</div>
					{/each}
					<p class="legenda">O número é o da questão na prova · verde: acertou · vermelho: errou · ponto: tem anotação</p>
				</div>
			{/if}
		</div>

		<article class="questao" id="questao">
			<h2>
				Questão {item.numero}
				<span class="materia" style={tagStyle(corDaMateria(item.disciplina))}>{item.disciplina}</span>
				{#if item.assunto}<span class="assunto">{item.assunto}</span>{/if}
			</h2>
			<p class="origem">
				<a href="/provas/{item.provaId}?q={item.numero}" title="Abrir a prova inteira nesta questão">
					{item.orgao}
					{item.ano} · {item.cargoNome || item.cargo}
				</a>
			</p>

			{#if prova && questao}
				{#key chaveDaAvulsa(item)}
					<ResolucaoDaQuestao
						bind:this={resolucao}
						provaId={item.provaId}
						{questao}
						{apoios}
						resposta={resposta(item)}
						nota={anotacoes[item.provaId]?.[item.numero]}
						fechados={fechadosDaProva}
						onalternarapoio={(apoio, aberto) => (fechados[`${item.provaId}/${apoio}`] = !aberto)}
						onmarcar={(letra) => marcar(item, letra)}
						onresponder={() => responder(item)}
						onrefazer={() => guardar(item, undefined)}
						onanotado={(a) => {
							if (!anotacoes[item.provaId]) anotacoes[item.provaId] = {};
							if (a.texto) anotacoes[item.provaId][a.numero] = a;
							else delete anotacoes[item.provaId][a.numero];
						}}
					/>
				{/key}
			{:else if prova}
				<p class="dim">Esta questão saiu da prova numa revisão. Siga para a próxima.</p>
			{:else if !erro}
				<p class="dim">Carregando a questão…</p>
			{/if}

			<footer class="rodape">
				<button type="button" class="vizinha" disabled={atual === 0} onclick={() => ir(atual - 1)}>← Anterior</button>
				<button type="button" class="vizinha" disabled={atual >= lista.length - 1} onclick={() => ir(atual + 1)}>
					Próxima →
				</button>
			</footer>
		</article>
	{:else if carregado && !erro}
		<div class="nada">
			<p>Nenhuma questão com esse filtro{filtro.situacao === 'erradas' ? ' — você não errou nenhuma' : ''}.</p>
			<a class="nbtn" href="/questoes?aba=materia">Mudar o filtro</a>
		</div>
	{:else if !carregado}
		<p class="dim">Montando a sessão…</p>
	{/if}
</div>

<style>
	/* A mesma coluna de leitura da prova inteira: a questão é o que importa. */
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
	.cabeca {
		padding: 18px 0 4px;
	}
	h1 {
		margin: 0;
		font-size: 28px;
		font-weight: 700;
		line-height: 1.2;
	}
	.meta {
		margin: 6px 0 0;
		font-size: 13px;
		color: var(--text-faint);
	}
	.dim {
		color: var(--text-faint);
		font-size: 14px;
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
		align-items: center;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
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

	.mapa {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		right: 0;
		display: grid;
		gap: 14px;
		max-height: min(70vh, 540px);
		overflow: auto;
		padding: 12px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow-pop);
	}
	.grupo-rotulo {
		margin: 0 0 6px;
		font-size: 12.5px;
		font-weight: 600;
		color: var(--text-muted);
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
		margin: 0;
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
	/* De onde a questão vem: discreto, e leva à prova inteira nela. */
	.origem {
		margin: 2px 0 14px;
		font-size: 13px;
	}
	.origem a {
		color: var(--text-faint);
		text-decoration: none;
	}
	.origem a:hover {
		color: var(--text-muted);
		text-decoration: underline;
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
	.nada {
		display: grid;
		gap: 12px;
		justify-items: start;
		padding-top: 24px;
		color: var(--text-muted);
	}
	.nada p {
		margin: 0;
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
