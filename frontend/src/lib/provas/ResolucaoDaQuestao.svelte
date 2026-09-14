<script lang="ts">
	import { tick } from 'svelte';
	import Anotacao from './Anotacao.svelte';
	import QuestaoResolucao from './QuestaoResolucao.svelte';
	import TextoDeApoio from './TextoDeApoio.svelte';
	import type { Resposta } from './resolucao';
	import type { Anotacao as AnotacaoDaQuestao, Apoio, Questao } from './types';

	// A questão como o estudante a resolve: os textos que ela usa, as
	// alternativas e a anotação dele. Serve à prova inteira e ao treino por
	// matéria; quem a usa monta uma por questão ({#key}), e o que é da tela —
	// título, navegação — fica com quem a usa.
	let {
		provaId,
		questao,
		apoios,
		resposta,
		nota,
		fechados,
		onalternarapoio,
		onmarcar,
		onresponder,
		onrefazer,
		onanotado
	}: {
		provaId: string;
		questao: Questao;
		apoios: Apoio[];
		resposta: Resposta | undefined;
		nota: AnotacaoDaQuestao | undefined;
		/** Textos que o estudante fechou: o texto das questões 1–10 fechado na 3 continua fechado na 4. */
		fechados: Record<string, boolean>;
		onalternarapoio: (id: string, aberto: boolean) => void;
		onmarcar: (letra: string) => void;
		onresponder: () => void;
		onrefazer: () => void;
		onanotado: (a: AnotacaoDaQuestao) => void;
	} = $props();

	// A anotação costuma dizer qual é a resposta: fica fechada até o estudante
	// responder, a não ser que ele peça para ver.
	let revelada = $state(false);
	let editor = $state<{ editar: () => Promise<void> } | null>(null);
	const fechada = $derived(!resposta?.conferida && !!nota && !revelada);

	/** Abre a anotação para escrever — o atalho N. */
	export async function anotar() {
		revelada = true;
		await tick();
		await editor?.editar();
	}
</script>

{#each apoios as apoio (apoio.id)}
	<div class="apoio">
		<TextoDeApoio {apoio} aberto={!fechados[apoio.id]} onalternar={(a) => onalternarapoio(apoio.id, a)} />
	</div>
{/each}

<QuestaoResolucao {questao} {resposta} {onmarcar} {onresponder} {onrefazer} />

<section class="anotacoes" aria-label="Anotações da questão {questao.numero}">
	{#if fechada}
		<button type="button" class="nota-fechada" onclick={() => (revelada = true)}>
			<span class="seta-toggle" aria-hidden="true"></span>
			Anotações <span class="dim">· abrem quando você responder; clique para ver agora</span>
		</button>
	{:else}
		<h3>Anotações</h3>
		<Anotacao
			bind:this={editor}
			{provaId}
			numero={questao.numero}
			inicial={nota?.texto ?? ''}
			onsalvo={(a) => {
				onanotado(a);
				// Quem está escrevendo não perde a nota de vista ao ela passar a existir.
				revelada = true;
			}}
		/>
	{/if}
</section>

<style>
	.apoio {
		margin-bottom: 12px;
	}
	.anotacoes {
		margin-top: 30px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}
	h3 {
		margin: 0 0 6px;
		font-size: 15px;
		font-weight: 600;
		color: var(--text-muted);
	}
	.nota-fechada {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 3px 6px 3px 2px;
		margin-left: -2px;
		font: inherit;
		font-size: 15px;
		font-weight: 600;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
		text-align: left;
	}
	.nota-fechada:hover {
		background: var(--bg-hover);
	}
	.dim {
		font-weight: 400;
		font-size: 13px;
		color: var(--text-faint);
	}
	.seta-toggle {
		width: 0;
		height: 0;
		border-top: 5px solid transparent;
		border-bottom: 5px solid transparent;
		border-left: 7px solid var(--text-muted);
	}
</style>
