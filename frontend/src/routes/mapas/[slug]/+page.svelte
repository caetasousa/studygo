<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { tagStyle } from '$lib/format';
	import { indexar } from '$lib/mapas/arvore';
	import Mapa from '$lib/mapas/Mapa.svelte';
	import { confirmar } from '$lib/stores/confirmacao.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { MapaLido } from '$lib/types';

	/**
	 * Um mapa mental aberto: o cabeçalho com as propriedades (matérias, fonte,
	 * tamanho) e o mapa como uma página de tópicos recolhíveis. Manter o mapa
	 * (excluir) fica recolhido no fim, como na página da lei.
	 */
	const slug = $derived(page.params.slug ?? '');

	let lido = $state<MapaLido | null>(null);
	let erro = $state<string | null>(null);
	let carregando = $state(true);

	async function carregar(s: string) {
		carregando = true;
		erro = null;
		lido = null;
		try {
			const res = await api.lerMapa(s);
			if (slug === s) lido = res;
		} catch (e) {
			if (slug === s) erro = e instanceof Error ? e.message : 'Não foi possível abrir o mapa';
		} finally {
			if (slug === s) carregando = false;
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	const nos = $derived(lido ? indexar(lido.arvore) : []);

	// --- matérias ---------------------------------------------------------
	const vinculadas = $derived(mapasStore.disciplinas.filter((d) => d.mapas.some((m) => m.slug === slug)));
	const livres = $derived(mapasStore.disciplinas.filter((d) => !vinculadas.includes(d)));
	let erroVinculo = $state<string | null>(null);

	async function vincular(disciplinaId: string, ligar: boolean) {
		const concurso = concursoStore.ativoSlug;
		if (!concurso) return;
		erroVinculo = null;
		try {
			await api.vincularMapa(concurso, disciplinaId, slug, ligar);
			await mapasStore.carregar(true);
		} catch (e) {
			erroVinculo = e instanceof Error ? e.message : 'Não foi possível gravar o vínculo';
		}
	}

	const cor = (codigo: string) => planoStore.discIndex[codigo]?.cor ?? 0;

	// --- excluir -----------------------------------------------------------
	async function excluir() {
		if (!lido) return;
		const ok = await confirmar({
			titulo: `Excluir “${lido.mapa.titulo}”?`,
			texto: 'O mapa e o vínculo dele com as matérias saem do app. Para tê-lo de volta, é só importar o texto de novo.',
			rotulo: 'Excluir mapa',
			tom: 'perigo'
		});
		if (!ok) return;
		try {
			await api.excluirMapa(slug);
			await mapasStore.carregar(true);
			await goto('/mapas');
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A exclusão falhou';
		}
	}

	const nf = new Intl.NumberFormat('pt-BR');
</script>

<svelte:head>
	<title>{lido ? `${lido.mapa.titulo} — Mapas mentais` : 'Mapa mental'} — studygo</title>
</svelte:head>

<a class="voltar" href="/mapas">← Mapas mentais</a>

{#if erro}
	<div class="form-error" role="alert">{erro}</div>
{:else if carregando}
	<p class="page-sub">Carregando…</p>
{:else if lido}
	<h1 class="page-title">
		<span class="title-ic"><NavIcon name="mapa" size="md" /></span>
		<span>{lido.mapa.titulo}</span>
	</h1>

	<dl class="propriedades">
		<div class="linha">
			<dt>Matérias</dt>
			<dd>
				{#each vinculadas as d (d.disciplinaId)}
					<span class="materia">
						<span class="chip" style={tagStyle(cor(d.codigo))}>{d.codigo}</span>
						<span class="nome">{d.nome}</span>
						<button
							type="button"
							class="tirar"
							aria-label="Desvincular {d.nome}"
							title="Desvincular {d.nome}"
							onclick={() => vincular(d.disciplinaId, false)}>×</button
						>
					</span>
				{/each}
				{#if livres.length > 0}
					<select
						class="vincular"
						aria-label="Vincular a uma matéria"
						onchange={(e) => {
							const escolhida = e.currentTarget.value;
							e.currentTarget.value = '';
							if (escolhida) void vincular(escolhida, true);
						}}
					>
						<option value="">{vinculadas.length === 0 ? 'Vincular a uma matéria…' : '+ outra matéria'}</option>
						{#each livres as d (d.disciplinaId)}
							<option value={d.disciplinaId}>{d.codigo} — {d.nome}</option>
						{/each}
					</select>
				{/if}
				{#if vinculadas.length === 0 && lido.mapa.materia}
					<span class="indicada">a fonte indica: {lido.mapa.materia}</span>
				{/if}
			</dd>
		</div>
		{#if lido.mapa.fonte}
			<div class="linha">
				<dt>Fonte</dt>
				<dd class="fonte">{lido.mapa.fonte}</dd>
			</div>
		{/if}
		<div class="linha">
			<dt>Tamanho</dt>
			<dd>{lido.mapa.ramos} {lido.mapa.ramos === 1 ? 'ramo' : 'ramos'} · {nf.format(lido.mapa.itens)} itens</dd>
		</div>
	</dl>

	{#if erroVinculo}<div class="form-error" role="alert">{erroVinculo}</div>{/if}

	<div class="mapa">
		{#key slug}
			<Mapa {nos} />
		{/key}
	</div>

	<details class="manter">
		<summary>Manter este mapa</summary>
		<p class="page-sub">
			Para corrigir ou ampliar o mapa, importe o texto de novo em <a href="/mapas">Mapas mentais</a>: o mesmo
			endereço troca o conteúdo e mantém as matérias vinculadas.
		</p>
		<button type="button" class="btn danger" onclick={excluir}>Excluir mapa</button>
	</details>
{/if}

<style>
	.voltar {
		display: inline-block;
		margin-bottom: 10px;
		font-size: 13px;
		color: var(--text-muted);
		text-decoration: none;
	}
	.voltar:hover {
		color: var(--text);
	}
	.title-ic {
		display: grid;
		place-items: center;
		flex: none;
		color: var(--text-muted);
	}

	/* As propriedades no jeito do Notion: um rótulo pequeno e o valor ao lado. */
	.propriedades {
		display: grid;
		gap: 6px;
		margin: 16px 0 0;
		max-width: 860px;
	}
	.linha {
		display: grid;
		grid-template-columns: 84px minmax(0, 1fr);
		gap: 10px;
		align-items: center;
		font-size: 13.5px;
	}
	dt {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	dd {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px 8px;
		margin: 0;
		min-width: 0;
		color: var(--text);
	}
	.fonte {
		color: var(--text-muted);
		font-size: 13px;
	}
	.materia {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 3px 4px 3px 8px;
		border: 1px solid var(--border);
		border-radius: 7px;
		background: var(--bg-soft);
	}
	.chip {
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 2px 7px;
		border-radius: 5px;
	}
	.nome {
		font-size: 13px;
	}
	.tirar {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		padding: 0;
		border: 0;
		border-radius: 5px;
		background: transparent;
		color: var(--text-faint);
		font-size: 15px;
		line-height: 1;
		cursor: pointer;
	}
	.tirar:hover {
		background: var(--bg-hover);
		color: var(--danger);
	}
	.vincular {
		padding: 4px 8px;
		border: 1px dashed var(--border-strong);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 13px;
		max-width: 100%;
		cursor: pointer;
	}
	.vincular:hover {
		background: var(--bg-hover);
	}
	.indicada {
		font-size: 12.5px;
		color: var(--text-faint);
	}

	.mapa {
		margin-top: 20px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}

	.manter {
		margin: 36px 0 24px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.manter > summary {
		cursor: pointer;
		color: var(--text-muted);
		font-size: 13px;
	}
	.manter .page-sub {
		margin: 10px 0 12px;
	}

	@media (max-width: 620px) {
		.linha {
			grid-template-columns: minmax(0, 1fr);
			gap: 3px;
		}
	}

	@media (pointer: coarse) {
		/* O × de 20px é pequeno para o dedo; e campo com fonte abaixo de 16px faz
		   o iPhone dar zoom na página ao ser tocado. */
		.tirar {
			width: 32px;
			height: 32px;
		}
		.vincular {
			padding-block: 8px;
			font-size: 16px;
		}
	}
</style>
