<script lang="ts">
	import { baixarImagem, imagensDoMapa } from './imagens';

	/**
	 * A imagem de um item do mapa — o fluxo, o diagrama — com a legenda embaixo.
	 * A imagem que ainda não foi enviada vira um aviso com o nome do arquivo, e
	 * o mapa segue legível: é o texto dele que se estuda.
	 *
	 * Tocar na imagem a abre em tela cheia, na própria página: um fluxo de BPMN
	 * não se lê na largura do celular, e o endereço data: não abre em outra aba.
	 */
	let { legenda, nome, ampliavel = true }: { legenda: string; nome: string; ampliavel?: boolean } = $props();

	const contexto = imagensDoMapa();
	const enviada = $derived(contexto?.enviadas.includes(nome) ?? false);

	let url = $state<string | null>(null);
	let falhou = $state(false);
	let dialogo = $state<HTMLDialogElement | null>(null);

	$effect(() => {
		if (!contexto || !enviada) return;

		let vivo = true;
		url = null;
		falhou = false;

		baixarImagem(contexto.slug, nome, contexto.versao).then(
			(u) => {
				if (vivo) url = u;
			},
			() => {
				if (vivo) falhou = true;
			}
		);

		return () => {
			vivo = false;
		};
	});
</script>

<span class="figura">
	{#if url}
		{#if ampliavel}
			<button type="button" class="ampliar" aria-label="Ampliar: {legenda}" onclick={() => dialogo?.showModal()}>
				<img src={url} alt={legenda} />
			</button>
			<dialog bind:this={dialogo} class="tela-cheia" aria-label={legenda} onclick={() => dialogo?.close()}>
				<img src={url} alt={legenda} />
				<span class="legenda">{legenda} · toque para fechar</span>
			</dialog>
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
	.ampliar {
		display: block;
		padding: 0;
		border: 0;
		background: none;
		line-height: 0;
		cursor: zoom-in;
	}
	img {
		display: block;
		max-width: 100%;
		height: auto;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: #fff;
	}
	.tela-cheia {
		width: 100vw;
		max-width: 100vw;
		height: 100dvh;
		max-height: 100dvh;
		margin: 0;
		padding: 12px;
		border: 0;
		background: rgb(0 0 0 / 0.92);
		overflow: auto;
		cursor: zoom-out;
	}
	.tela-cheia img {
		max-width: none;
		width: max(100%, 900px);
		margin: auto;
	}
	.tela-cheia .legenda {
		display: block;
		margin-top: 8px;
		color: #ddd;
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
