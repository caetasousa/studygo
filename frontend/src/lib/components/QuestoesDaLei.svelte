<script lang="ts">
	import IconButton from './IconButton.svelte';
	import { api } from '$lib/api';
	import { partirNoTrecho, placar } from '$lib/leis';
	import type { CorrecaoDeQuestao, Dispositivo, QuestaoDeLei } from '$lib/types';

	/**
	 * As questões de um artigo (ou de uma unidade inteira), abertas por cima do
	 * texto da lei.
	 *
	 * O gabarito não está aqui antes da resposta: ele vem do servidor junto com
	 * a correção, e é por isso que responder é uma ida ao servidor e não uma
	 * comparação local.
	 */
	let {
		titulo,
		questoes,
		porRef,
		onrespondida,
		onclose
	}: {
		/** "Art. 71" ou o título da unidade; dá nome ao diálogo. */
		titulo: string;
		questoes: QuestaoDeLei[];
		porRef: Map<string, Dispositivo>;
		onrespondida: (id: string, correcao: CorrecaoDeQuestao) => void;
		onclose: () => void;
	} = $props();

	const LETRAS = ['A', 'B', 'C', 'D', 'E'];

	let escolhas = $state<Record<string, string>>({});
	let refazendo = $state<Record<string, boolean>>({});
	let enviando = $state<string | null>(null);
	let erro = $state<string | null>(null);
	let soErradas = $state(false);

	const visiveis = $derived(
		soErradas ? questoes.filter((q) => q.resposta && !q.resposta.acertou) : questoes
	);
	const p = $derived(placar(questoes));

	async function responder(q: QuestaoDeLei) {
		const letra = escolhas[q.id];
		if (!letra) return;
		enviando = q.id;
		erro = null;
		try {
			const c = await api.responderQuestao(q.id, letra);
			refazendo[q.id] = false;
			onrespondida(q.id, c);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível gravar a resposta';
		} finally {
			enviando = null;
		}
	}

	/** O trecho que justifica a resposta, no texto do dispositivo citado. */
	function contexto(q: QuestaoDeLei, trecho: string): [string, string, string] {
		for (const ref of q.dispositivos) {
			const texto = porRef.get(ref)?.texto ?? '';
			const partes = partirNoTrecho(texto, trecho);
			if (partes) return partes;
		}
		return ['', trecho, ''];
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
		aria-labelledby="questoes-titulo"
		tabindex="-1"
		bind:this={painel}
		onkeydown={teclado}
	>
		<header class="topo">
			<div class="ident">
				<h2 id="questoes-titulo">Questões — {titulo}</h2>
				<p>
					{p.total}
					{p.total === 1 ? 'questão' : 'questões'} · {p.respondidas} respondidas · {p.certas} certas
				</p>
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
					<div class="alternativas">
						{#each q.alternativas as alt, i (i)}
							{@const letra = LETRAS[i]}
							<label
								class="alt"
								class:gabarito={r && r.gabarito === letra}
								class:marcada={r && r.escolhida === letra}
							>
								<input
									type="radio"
									name="q-{q.id}"
									value={letra}
									disabled={!!r}
									checked={r ? r.escolhida === letra : escolhas[q.id] === letra}
									onchange={() => (escolhas[q.id] = letra)}
								/>
								<span>{letra}) {alt}</span>
							</label>
						{/each}
					</div>

					{#if r}
						{@const [antes, grifo, depois] = contexto(q, r.trecho)}
						<p class="veredito">{r.acertou ? 'Certo' : `Errado — gabarito ${r.gabarito}`}</p>
						<p class="comentario">{r.comentario}</p>
						<blockquote class="trecho">{antes}<mark>{grifo}</mark>{depois}</blockquote>
						<button class="btn" type="button" onclick={() => (refazendo[q.id] = true)}>
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
		gap: 12px;
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
		font-size: 14px;
		line-height: 1.45;
		padding: 0 4px;
	}
	.alternativas {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin: 8px 0 12px;
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
	}
	.alt:hover {
		background: var(--bg-hover);
	}
	.alt input {
		margin-top: 3px;
	}
	.alt.gabarito {
		background: var(--good-soft);
	}
	.alt.marcada:not(.gabarito) {
		background: var(--danger-soft);
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
		margin: 0 0 8px;
		font-size: 13.5px;
		line-height: 1.5;
	}
	.trecho {
		margin: 0 0 12px;
		padding: 8px 12px;
		border-left: 3px solid var(--accent);
		background: var(--bg-soft);
		font-size: 13.5px;
		line-height: 1.55;
	}
	mark {
		background: var(--warn-soft);
		color: inherit;
		padding: 0 1px;
	}
</style>
