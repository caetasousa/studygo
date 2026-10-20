<script lang="ts">
	import { api } from '$lib/api';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { MapaPronto, PacoteImportado } from '$lib/types';

	/**
	 * Os mapas que vêm com o app — as provas de português da FCC, uma por ramo,
	 * com o comentário de cada alternativa. Um clique põe o mapa e as questões
	 * na conta; o clique de novo atualiza, sem perder as respostas nem o
	 * vínculo. Com o concurso aberto, o mapa vai para a matéria que ele indica.
	 */
	let { aoAdicionar }: { aoAdicionar: () => void } = $props();

	let prontos = $state<MapaPronto[]>([]);
	let erro = $state<string | null>(null);
	let adicionando = $state<string | null>(null);
	let feito = $state<PacoteImportado | null>(null);

	const nf = new Intl.NumberFormat('pt-BR');

	async function carregar() {
		try {
			prontos = await api.mapasProntos(concursoStore.ativoSlug);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível listar os mapas prontos';
		}
	}

	void carregar();

	async function adicionar(p: MapaPronto) {
		erro = null;
		feito = null;
		adicionando = p.slug;
		try {
			feito = await api.adicionarMapaPronto(p.slug, concursoStore.ativoSlug);
			await carregar();
			aoAdicionar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível adicionar o mapa';
		} finally {
			adicionando = null;
		}
	}
</script>

{#if prontos.length > 0 || erro}
	<section class="prontos" aria-labelledby="prontos-titulo">
		<h2 id="prontos-titulo">Mapas prontos</h2>
		<p class="ajuda">
			Vêm com o app: é só adicionar à conta. Adicionar de novo traz a versão mais nova e mantém as suas respostas.
		</p>

		{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

		<ul aria-label="Mapas prontos">
			{#each prontos as p (p.slug)}
				<li>
					<div class="ident">
						<b>{p.titulo}</b>
						<span>{nf.format(p.ramos)} provas · {nf.format(p.questoes)} questões comentadas</span>
					</div>
					<div class="acoes">
						{#if p.naConta}<a href="/mapas/{p.slug}">Abrir</a>{/if}
						<button
							type="button"
							class="btn"
							class:primary={!p.naConta}
							disabled={adicionando !== null}
							onclick={() => adicionar(p)}
						>
							{#if adicionando === p.slug}
								Adicionando…
							{:else}
								{p.naConta ? 'Atualizar' : 'Adicionar à minha conta'}
							{/if}
						</button>
					</div>
				</li>
			{/each}
		</ul>

		{#if feito}
			<p class="feito" role="status">
				<a href="/mapas/{feito.mapa.slug}"><b>{feito.mapa.titulo}</b></a>
				{feito.novo ? 'adicionado' : 'atualizado'}: {nf.format(feito.questoes)} questões.
				{#if feito.vinculadas.length > 0}
					Vinculado a {feito.vinculadas.map((v) => `${v.codigo} — ${v.nome}`).join(', ')}.
				{/if}
			</p>
		{/if}
	</section>
{/if}

<style>
	.prontos {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 10px;
		margin-bottom: 8px;
		padding: 11px 14px 14px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--bg-card);
	}
	h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
	}
	.ajuda {
		margin: 0;
		font-size: 13px;
		color: var(--text-muted);
	}
	ul {
		display: grid;
		gap: 8px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 8px 12px;
	}
	.ident {
		display: grid;
		gap: 2px;
		min-width: 0;
		font-size: 13.5px;
		overflow-wrap: anywhere;
	}
	.ident span {
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.acoes {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 13px;
	}
	.feito {
		margin: 0;
		font-size: 13px;
	}
</style>
