<script lang="ts">
	import { baixarImagem, imagensDoMapa } from './imagens';

	/**
	 * A imagem de um item do mapa — o fluxo, o diagrama — com a legenda embaixo.
	 * A imagem que ainda não foi enviada vira um aviso com o nome do arquivo, e
	 * o mapa segue legível: é o texto dele que se estuda.
	 */
	let { legenda, nome, ampliavel = true }: { legenda: string; nome: string; ampliavel?: boolean } = $props();

	const contexto = imagensDoMapa();
	const enviada = $derived(contexto?.enviadas.includes(nome) ?? false);

	let url = $state<string | null>(null);
	let falhou = $state(false);

	$effect(() => {
		if (!contexto || !enviada) return;

		let vivo = true;
		let criado: string | null = null;
		url = null;
		falhou = false;

		baixarImagem(contexto.slug, nome, contexto.versao).then(
			(blob) => {
				if (!vivo) return;
				criado = URL.createObjectURL(blob);
				url = criado;
			},
			() => {
				if (vivo) falhou = true;
			}
		);

		return () => {
			vivo = false;
			if (criado) URL.revokeObjectURL(criado);
		};
	});
</script>

<span class="figura">
	{#if url}
		{#if ampliavel}
			<a href={url} target="_blank" rel="noopener" title="Abrir a imagem inteira"><img src={url} alt={legenda} /></a>
		{:else}
			<img src={url} alt={legenda} />
		{/if}
	{:else if !enviada || falhou}
		<span class="falta">Imagem {falhou ? 'indisponível' : 'não enviada'}: <code>{nome}</code></span>
	{:else}
		<span class="carregando" aria-hidden="true"></span>
	{/if}
	<span class="legenda">{legenda}</span>
</span>

<style>
	.figura {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 4px 0;
	}
	a {
		display: block;
		line-height: 0;
	}
	img {
		display: block;
		max-width: 100%;
		height: auto;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: #fff;
	}
	.falta {
		padding: 10px 12px;
		border: 1px dashed var(--border);
		border-radius: 6px;
		color: var(--text-muted);
		font-size: 13px;
		font-weight: 400;
	}
	.carregando {
		height: 120px;
		border-radius: 6px;
		background: var(--bg-soft);
	}
	.legenda {
		color: var(--text-muted);
		font-size: 13px;
		font-weight: 400;
	}
</style>
