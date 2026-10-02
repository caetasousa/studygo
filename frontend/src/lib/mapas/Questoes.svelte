<script lang="ts">
	import IconButton from '$lib/components/IconButton.svelte';
	import { api } from '$lib/api';
	import type { CorrecaoDoMapa, QuestaoDoMapa } from '$lib/types';
	import { descreverPlacar, placar, rotuloDaResposta } from './questoes';

	/**
	 * As questões de um ramo do mapa (ou todas), abertas por cima dele, como as
	 * da lei. O gabarito não está aqui antes da resposta: ele vem do servidor
	 * junto com a correção, e por isso responder é uma ida ao servidor.
	 *
	 * No celular o diálogo ocupa a tela, e cada alternativa tem altura de dedo:
	 * é onde boa parte das questões vai ser resolvida.
	 *
	 * Quando o comentário explica alternativa por alternativa, a explicação
	 * aparece debaixo de cada uma: aberta a da certa (se acertou) ou a da
	 * marcada (se errou), e as outras a um toque, para não soterrar a questão.
	 */
	let {
		titulo,
		questoes,
		onrespondida,
		onclose
	}: {
		/** O ramo, ou "Todas"; dá nome ao diálogo. */
		titulo: string;
		questoes: QuestaoDoMapa[];
		onrespondida: (id: string, correcao: CorrecaoDoMapa) => void;
		onclose: () => void;
	} = $props();

	const LETRAS = ['A', 'B', 'C', 'D', 'E'];
	const JULGAR = [
		{ valor: 'CERTO', rotulo: 'Certo' },
		{ valor: 'ERRADO', rotulo: 'Errado' }
	];

	let escolhas = $state<Record<string, string>>({});
	let refazendo = $state<Record<string, boolean>>({});
	let enviando = $state<string | null>(null);
	let erro = $state<string | null>(null);
	let soErradas = $state(false);
	/** O que a pessoa abriu ou fechou à mão, por "questão:alternativa". */
	let explicacaoAberta = $state<Record<string, boolean>>({});

	const visiveis = $derived(soErradas ? questoes.filter((q) => q.resposta && !q.resposta.acertou) : questoes);
	const p = $derived(placar(questoes));

	const opcoes = (q: QuestaoDoMapa) =>
		q.alternativas.length === 0
			? JULGAR
			: q.alternativas.map((alt, i) => ({ valor: LETRAS[i], rotulo: `${LETRAS[i]}) ${alt}` }));

	function veredito(q: QuestaoDoMapa, r: CorrecaoDoMapa): string {
		if (r.acertou) return 'Acertou';
		return q.alternativas.length === 0
			? `Errou — o item está ${rotuloDaResposta(r.gabarito)}`
			: `Errou — gabarito ${r.gabarito}`;
	}

	function abertaDeSaida(r: CorrecaoDoMapa, valor: string): boolean {
		return r.acertou ? valor === r.gabarito : valor === r.escolhida;
	}

	function esquecerExplicacoes(id: string) {
		for (const k of Object.keys(explicacaoAberta)) if (k.startsWith(`${id}:`)) delete explicacaoAberta[k];
	}

	async function responder(q: QuestaoDoMapa) {
		const escolhida = escolhas[q.id];
		if (!escolhida) return;
		enviando = q.id;
		erro = null;
		try {
			const c = await api.responderQuestaoDoMapa(q.id, escolhida);
			refazendo[q.id] = false;
			esquecerExplicacoes(q.id);
			onrespondida(q.id, c);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível gravar a resposta';
		} finally {
			enviando = null;
		}
	}

	let painel = $state<HTMLElement | null>(null);
	$effect(() => {
		painel?.focus();
	});

	function teclado(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onclose();
		}
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="dlg-fundo"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		class="dlg"
		role="dialog"
		aria-modal="true"
		aria-labelledby="questoes-mapa-titulo"
		tabindex="-1"
		bind:this={painel}
		onkeydown={teclado}
	>
		<header class="topo">
			<div class="ident">
				<h2 id="questoes-mapa-titulo">Questões — {titulo}</h2>
				<p>{descreverPlacar(p)}</p>
			</div>
			<label class="filtro">
				<input type="checkbox" bind:checked={soErradas} />
				Só o que errei
			</label>
			<IconButton icon="fechar" label="Fechar as questões" onclick={onclose} />
		</header>

		<div class="corpo">
			{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}
			{#if visiveis.length === 0}
				<p class="vazia">{soErradas ? 'Nenhum erro por aqui.' : 'Sem questões.'}</p>
			{/if}

			{#each visiveis as q (q.id)}
				{@const r = refazendo[q.id] ? null : q.resposta}
				<fieldset class="questao" class:certa={r?.acertou} class:errada={r && !r.acertou}>
					<legend>{q.enunciado}</legend>
					{#if q.origem}<p class="origem">{q.origem}</p>{/if}
					<div class="alternativas" class:julgar={q.alternativas.length === 0}>
						{#each opcoes(q) as o, i (o.valor)}
							{@const explicacao = r?.explicacoes[i]}
							{@const chave = `${q.id}:${o.valor}`}
							{@const aberta = r && (explicacaoAberta[chave] ?? abertaDeSaida(r, o.valor))}
							<div
								class="opcao"
								class:gabarito={r && r.gabarito === o.valor}
								class:marcada={r && r.escolhida === o.valor}
							>
								<label class="alt">
									<input
										type="radio"
										name="q-{q.id}"
										value={o.valor}
										disabled={!!r}
										checked={r ? r.escolhida === o.valor : escolhas[q.id] === o.valor}
										onchange={() => (escolhas[q.id] = o.valor)}
									/>
									<span>{o.rotulo}</span>
								</label>
								{#if explicacao}
									{#if aberta}
										<p class="explicacao" id="exp-{chave}">{explicacao}</p>
									{/if}
									<button
										class="ver"
										type="button"
										aria-expanded={aberta}
										aria-controls="exp-{chave}"
										onclick={() => (explicacaoAberta[chave] = !aberta)}
									>
										{aberta ? 'Ocultar explicação' : 'Ver explicação'}<span class="sr-only"> da {o.valor}</span>
									</button>
								{/if}
							</div>
						{/each}
					</div>

					{#if r}
						<p class="veredito">{veredito(q, r)}</p>
						{#if r.comentario}<p class="comentario">{r.comentario}</p>{/if}
						<button
							class="btn"
							type="button"
							onclick={() => {
								refazendo[q.id] = true;
								esquecerExplicacoes(q.id);
							}}
						>
							Responder de novo
						</button>
					{:else}
						<button
							class="btn primary"
							type="button"
							disabled={!escolhas[q.id] || enviando === q.id}
							onclick={() => responder(q)}
						>
							Responder
						</button>
					{/if}
				</fieldset>
			{/each}
		</div>
	</div>
</div>

<style>
	.dlg-fundo {
		position: fixed;
		inset: 0;
		background: rgba(20, 18, 14, 0.45);
		display: grid;
		place-items: center;
		padding: 20px;
		z-index: 70;
	}
	.dlg {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		box-shadow: var(--shadow-pop);
		width: min(720px, 100%);
		max-height: 88vh;
		display: flex;
		flex-direction: column;
	}
	.dlg:focus {
		outline: none;
	}
	.topo {
		display: flex;
		align-items: flex-start;
		flex-wrap: wrap;
		gap: 8px 12px;
		padding: 16px 16px 12px;
		border-bottom: 1px solid var(--border);
	}
	.ident {
		flex: 1;
		min-width: 0;
	}
	.topo h2 {
		margin: 0;
		font-size: 17px;
		font-weight: 700;
		overflow-wrap: anywhere;
	}
	.topo p {
		margin: 3px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}
	.filtro {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--text-muted);
		white-space: nowrap;
		padding-top: 8px;
	}
	.corpo {
		overflow-y: auto;
		padding: 12px 16px 18px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.vazia {
		color: var(--text-muted);
		font-size: 13px;
	}
	.questao {
		min-width: 0;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 12px 14px 14px;
		margin: 0;
	}
	.questao.certa {
		border-color: var(--good);
	}
	.questao.errada {
		border-color: var(--danger);
	}
	legend {
		font-weight: 600;
		font-size: 14.5px;
		line-height: 1.5;
		padding: 0 4px;
		white-space: pre-line;
	}
	.origem {
		margin: 2px 0 0;
		font-family: var(--font-mono);
		font-size: 10.5px;
		letter-spacing: 0.03em;
		color: var(--text-faint);
	}
	.alternativas {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin: 10px 0 12px;
	}
	.alternativas.julgar {
		flex-direction: row;
		gap: 8px;
	}
	.alternativas.julgar .opcao {
		flex: 1;
	}
	.alternativas.julgar .alt {
		justify-content: center;
		border: 1px solid var(--border);
	}
	.opcao {
		border-radius: 6px;
	}
	.alt {
		display: flex;
		gap: 8px;
		align-items: flex-start;
		padding: 6px 8px;
		border-radius: 6px;
		font-size: 14px;
		line-height: 1.45;
		cursor: pointer;
		overflow-wrap: anywhere;
	}
	@media (hover: hover) {
		.alt:hover {
			background: var(--bg-hover);
		}
	}
	.alt input {
		margin-top: 3px;
		flex: none;
	}
	.opcao.gabarito {
		background: var(--good-soft);
	}
	.opcao.marcada:not(.gabarito) {
		background: var(--danger-soft);
	}
	/* A explicação e o botão alinham com o texto da alternativa, não com o rádio. */
	.explicacao {
		margin: 0 8px 0 29px;
		padding: 2px 0 4px 10px;
		border-left: 2px solid var(--border);
		font-size: 13.5px;
		line-height: 1.55;
		white-space: pre-line;
		overflow-wrap: anywhere;
	}
	.gabarito .explicacao {
		border-left-color: var(--good);
	}
	.marcada:not(.gabarito) .explicacao {
		border-left-color: var(--danger);
	}
	.ver {
		display: block;
		margin: 0 0 4px 29px;
		padding: 2px 0;
		border: 0;
		background: none;
		font: inherit;
		font-size: 12.5px;
		color: var(--text-muted);
		text-decoration: underline;
		text-underline-offset: 2px;
		cursor: pointer;
	}
	@media (hover: hover) {
		.ver:hover {
			color: var(--text);
		}
	}
	.veredito {
		font-weight: 700;
		margin: 0 0 4px;
	}
	.certa .veredito {
		color: var(--good);
	}
	.errada .veredito {
		color: var(--danger);
	}
	.comentario {
		margin: 0 0 12px;
		font-size: 13.5px;
		line-height: 1.55;
		white-space: pre-line;
	}

	/* No toque, cada alternativa tem altura de dedo. */
	@media (pointer: coarse) {
		.alt {
			min-height: 44px;
			padding-block: 10px;
			align-items: center;
		}
		.alt input {
			margin-top: 0;
			width: 18px;
			height: 18px;
		}
		.btn {
			padding-block: 11px;
		}
		.ver {
			min-height: 36px;
			padding-block: 8px;
		}
		.explicacao {
			margin-left: 34px;
		}
	}

	/* No celular o diálogo é a tela: nada de moldura comendo a largura. */
	@media (max-width: 620px) {
		.dlg-fundo {
			padding: 0;
			place-items: stretch;
		}
		.dlg {
			width: 100%;
			max-height: none;
			height: 100%;
			border-radius: 0;
			border: 0;
		}
		.corpo {
			padding-inline: 12px;
		}
		.questao {
			padding-inline: 10px;
		}
	}
</style>
