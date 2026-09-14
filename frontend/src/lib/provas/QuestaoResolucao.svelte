<script lang="ts">
	import Blocos from './Blocos.svelte';
	import { estadoDaAlternativa, type Resposta } from './resolucao';
	import type { Questao } from './types';

	let {
		questao,
		resposta,
		onmarcar,
		onresponder,
		onrefazer
	}: {
		questao: Questao;
		resposta: Resposta | undefined;
		onmarcar: (letra: string) => void;
		onresponder: () => void;
		onrefazer: () => void;
	} = $props();

	const conferida = $derived(!!resposta?.conferida);
	const acertou = $derived(conferida && !!questao.resposta && resposta?.marcada === questao.resposta);

	function estado(letra: string) {
		return estadoDaAlternativa(questao, letra, resposta);
	}
</script>

<div class="enunciado"><Blocos blocos={questao.blocos} /></div>

<div class="alternativas" role="radiogroup" aria-label="Alternativas da questão {questao.numero}">
	{#each questao.alternativas as alt (alt.letra)}
		{@const e = estado(alt.letra)}
		<button
			type="button"
			class="alternativa {e}"
			role="radio"
			aria-checked={resposta?.marcada === alt.letra}
			disabled={conferida}
			title={conferida ? '' : `Marcar ${alt.letra} (tecla ${alt.letra})`}
			onclick={() => onmarcar(alt.letra)}
		>
			<span class="letra">{alt.letra}</span>
			<span class="conteudo"><Blocos blocos={alt.blocos} interativo={false} /></span>
			{#if e === 'correta'}<span class="sinal" aria-label="gabarito">✓</span>
			{:else if e === 'errada'}<span class="sinal" aria-label="sua resposta">✕</span>{/if}
		</button>
	{/each}
</div>

{#if !conferida}
	<div class="acoes">
		<button class="nbtn primario" type="button" disabled={!resposta?.marcada} onclick={onresponder}>Responder</button>
		<span class="dica">{resposta?.marcada ? 'Enter responde' : 'Marque uma alternativa (A–E)'}</span>
	</div>
{:else}
	<div class="acoes">
		{#if !questao.resposta}
			<p class="aviso neutro">
				<span class="emoji" aria-hidden="true">ℹ️</span>
				<span>
					{questao.situacao ? `Sem resposta no gabarito: ${questao.situacao}.` : 'Esta prova não tem gabarito para conferir.'}
				</span>
			</p>
		{:else if acertou}
			<p class="aviso certo">
				<span class="emoji" aria-hidden="true">✅</span>
				<span>Você acertou. Gabarito: <b>{questao.resposta}</b>.</span>
			</p>
		{:else}
			<p class="aviso errado">
				<span class="emoji" aria-hidden="true">❌</span>
				<span>Você marcou <b>{resposta?.marcada}</b>; o gabarito é <b>{questao.resposta}</b>.</span>
			</p>
		{/if}
		<button class="nbtn" type="button" onclick={onrefazer}>Refazer</button>
	</div>
{/if}

<style>
	.enunciado {
		margin-bottom: 14px;
	}
	.alternativas {
		display: grid;
		gap: 2px;
		margin: 0 -8px;
	}
	.alternativa {
		display: grid;
		grid-template-columns: 26px minmax(0, 1fr) auto;
		gap: 12px;
		align-items: start;
		width: 100%;
		padding: 8px;
		text-align: left;
		font: inherit;
		color: inherit;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 6px;
		cursor: pointer;
		transition: background 0.1s;
	}
	.alternativa:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	.alternativa:focus-visible {
		outline: 2px solid var(--accent);
	}
	.alternativa:disabled {
		cursor: default;
	}
	.letra {
		display: grid;
		place-items: center;
		width: 24px;
		height: 24px;
		margin-top: 1px;
		border-radius: 5px;
		font-size: 12.5px;
		font-weight: 600;
		color: var(--text-muted);
		background: var(--bg-hover);
	}
	.conteudo :global(p) {
		margin: 0;
	}
	.sinal {
		font-weight: 700;
		padding-top: 2px;
	}
	.marcada {
		background: var(--accent-soft);
		border-color: color-mix(in srgb, var(--accent) 40%, transparent);
	}
	.marcada .letra {
		color: var(--bg);
		background: var(--accent);
	}
	.correta {
		background: var(--good-soft);
	}
	.correta .letra,
	.correta .sinal {
		color: var(--good);
	}
	.correta .letra {
		color: var(--bg);
		background: var(--good);
	}
	.errada {
		background: var(--danger-soft);
	}
	.errada .letra {
		color: var(--bg);
		background: var(--danger);
	}
	.errada .sinal {
		color: var(--danger);
	}
	.neutra {
		background: var(--bg-hover);
	}
	.acoes {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 14px;
		align-items: center;
		margin-top: 16px;
	}
	.dica {
		font-size: 13px;
		color: var(--text-faint);
	}
	/* O callout do Notion: fundo suave, emoji na frente. */
	.aviso {
		display: flex;
		gap: 10px;
		align-items: flex-start;
		flex: 1 1 280px;
		margin: 16px 0 0;
		padding: 12px 14px;
		border-radius: 6px;
		font-size: 15px;
	}
	.acoes .aviso {
		margin: 0;
	}
	.aviso.certo {
		background: var(--good-soft);
	}
	.aviso.errado {
		background: var(--danger-soft);
	}
	.aviso.neutro {
		background: var(--bg-hover);
	}
	.emoji {
		line-height: 1.4;
	}
</style>
