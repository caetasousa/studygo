<script lang="ts">
	import { untrack } from 'svelte';
	import { fetchAutenticado } from '$lib/api';
	import { provasApi } from './api';
	import type { Origem } from './types';

	let {
		importacao,
		versao,
		origem,
		rotulo,
		zoom = 100
	}: {
		importacao: string;
		versao: number;
		/** O retângulo do caderno a mostrar: a área da questão ou a região. */
		origem: Origem;
		rotulo: string;
		zoom?: number;
	} = $props();

	let url = $state('');
	let erro = $state('');

	// O recorte tem id derivado do retângulo: voltar à questão devolve o mesmo
	// arquivo. A versão é lida sem rastrear: salvar não pede outra imagem.
	$effect(() => {
		const o = origem;
		const v = untrack(() => versao);
		let ativo = true;
		let blob = '';
		url = '';
		erro = '';

		provasApi
			.recortar(importacao, v, o)
			.then(async ({ arquivo }) => {
				const resp = await fetchAutenticado(`/api/provas/arquivos/${arquivo}`);
				if (!resp.ok) throw new Error('original indisponível');
				const conteudo = await resp.blob();
				if (!ativo) return;
				blob = URL.createObjectURL(conteudo);
				url = blob;
			})
			.catch((e: Error) => {
				if (ativo) erro = e.message;
			});

		return () => {
			ativo = false;
			if (blob) URL.revokeObjectURL(blob);
		};
	});
</script>

<figure>
	<figcaption>{rotulo}</figcaption>
	<div class="janela">
		{#if url}
			<img src={url} alt="Trecho do caderno original: {rotulo}" style:width="{zoom}%" />
		{:else}
			<p class="espera" role="status">{erro || 'Carregando o original…'}</p>
		{/if}
	</div>
</figure>

<style>
	figure {
		margin: 0;
		display: grid;
		gap: 6px;
	}
	figcaption {
		font-size: 11.5px;
		color: var(--text-faint);
	}
	.janela {
		overflow: auto;
		max-height: 70vh;
		background: #fff;
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	img {
		display: block;
		max-width: none;
	}
	.espera {
		margin: 0;
		padding: 16px;
		color: #555;
		font-size: 13px;
	}
</style>
