<script lang="ts">
	import { fetchAutenticado } from '$lib/api';

	let { id, alt }: { id: string; alt: string } = $props();

	let url = $state('');
	let erro = $state('');

	// Os arquivos exigem o token, então <img src> direto não serve: baixa pelo
	// fetch autenticado e mostra por uma URL de blob, liberada ao trocar de id.
	$effect(() => {
		const atual = id;
		let ativo = true;
		let blob = '';
		url = '';
		erro = '';

		fetchAutenticado(`/api/provas/arquivos/${atual}`)
			.then(async (r) => {
				if (!r.ok) throw new Error('imagem indisponível');
				const conteudo = await r.blob();
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

{#if url}
	<img src={url} {alt} />
{:else}
	<span class="espera" role="status">{erro || 'Carregando imagem…'}</span>
{/if}

<style>
	img {
		display: block;
		max-width: 100%;
		height: auto;
		/* O recorte é de papel branco: no tema escuro, sem fundo, o traço preto some. */
		background: #fff;
		border-radius: 4px;
	}
	.espera {
		font-size: 12.5px;
		color: var(--text-faint);
	}
</style>
