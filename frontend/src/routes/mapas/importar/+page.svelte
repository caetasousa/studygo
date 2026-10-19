<script lang="ts">
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { salvarArquivo } from '$lib/download';
	import PageHead from '$lib/components/PageHead.svelte';
	import PedidosDePDF from '$lib/mapas/PedidosDePDF.svelte';
	import ImportarExportacao from '$lib/mapas/ImportarExportacao.svelte';
	import MapasProntos from '$lib/mapas/MapasProntos.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import type { MapaImportado } from '$lib/types';

	/**
	 * Tudo o que entra e sai dos mapas: os mapas que vêm com o app, o PDF da
	 * aula (ou o assunto sem aula), que o processador transforma em mapa, o
	 * texto de um mapa já pronto, o .zip de uma exportação e a exportação de
	 * todos. Fica fora da lista, que é para ler: no celular,
	 * estes formulários vinham telas antes do primeiro mapa.
	 *
	 * Importar de novo o mesmo mapa troca o conteúdo e mantém os vínculos.
	 */
	let erro = $state<string | null>(null);

	// `?materia=GEO&assunto=…`, que a lista do que falta virar mapa manda.
	const materia = $derived(page.url.searchParams.get('materia') ?? '');
	const assunto = $derived(page.url.searchParams.get('assunto') ?? '');

	/** A lista e o cronograma passam a ver o mapa que acabou de entrar. */
	const recarregar = () => void mapasStore.carregar(true);

	// --- exportar todos ----------------------------------------------------------
	let exportando = $state(false);

	async function exportarTodos() {
		erro = null;
		exportando = true;
		try {
			salvarArquivo(await api.exportarMapas(), `mapas-${new Date().toISOString().slice(0, 10)}.zip`);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível exportar os mapas';
		} finally {
			exportando = false;
		}
	}

	// --- importar o texto ----------------------------------------------------------
	let restaurar = $state<ReturnType<typeof ImportarExportacao>>();
	let texto = $state('');
	let importando = $state(false);
	let resultado = $state<MapaImportado | null>(null);

	const nf = new Intl.NumberFormat('pt-BR');

	async function importar(conteudo: string) {
		erro = null;
		resultado = null;
		importando = true;
		try {
			resultado = await api.importarMapa(conteudo, concursoStore.ativoSlug);
			texto = '';
			recarregar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A importação falhou';
		} finally {
			importando = false;
		}
	}

	/** Escolher o arquivo já importa: importar de novo é seguro, e é um passo a menos. */
	async function aoEscolherArquivo(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!arquivo) return;

		// O PDF da aula vai para a fila, não aqui: lido como texto, ele virava um
		// "mapa" ilegível ou um 413 sem explicação.
		if (arquivo.type === 'application/pdf' || arquivo.name.toLowerCase().endsWith('.pdf')) {
			resultado = null;
			erro = `“${arquivo.name}” é o PDF da aula: envie-o em “Criar um mapa”, logo acima, e ele vira mapa sozinho. Aqui entra só o texto de um mapa já pronto (.md).`;
			return;
		}

		// O .zip exportado tem o seu lugar, que traz também as questões, as
		// imagens, os vínculos e as respostas.
		if (arquivo.name.toLowerCase().endsWith('.zip')) {
			resultado = null;
			await restaurar?.importar(arquivo);
			return;
		}

		await importar(await arquivo.text());
	}
</script>

<svelte:head>
	<title>Importar e exportar mapas — studygo</title>
</svelte:head>

<PageHead
	icone="mapa"
	titulo="Importar e exportar"
	sub="Os mapas mentais entram por aqui — pelo PDF da aula, pelo assunto, pelo texto ou pela cópia exportada — e saem num .zip."
	mostrarProps={false}
/>

<div class="page">
	<p class="voltar"><a href="/mapas">← Mapas mentais</a></p>

	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if resultado}
		<div class="callout aviso-ok" role="status">
			<div>
				<b>{resultado.mapa.titulo}</b>
				{resultado.novo ? 'importado' : 'atualizado'}: {resultado.mapa.ramos} ramos, {nf.format(resultado.mapa.itens)} itens.
				{#if resultado.vinculadas.length > 0}
					Vinculado a {resultado.vinculadas.map((v) => `${v.codigo} — ${v.nome}`).join(', ')}.
				{:else}
					Nenhuma matéria do concurso aberto bate com ele: vincule na página do mapa.
				{/if}
				<a href="/mapas/{resultado.mapa.slug}">Abrir o mapa</a>
			</div>
		</div>
	{/if}

	<MapasProntos aoAdicionar={recarregar} />

	<PedidosDePDF aoFicarPronto={recarregar} {materia} {assunto} />

	<section class="importar" aria-labelledby="importar-titulo">
		<h2 id="importar-titulo">Importar mapa</h2>
		<p class="ajuda">
			Um outline de texto: <code># Título</code>, metadados opcionais (<code>materia:</code>,
			<code>fonte:</code>) e os itens como <code>- texto</code>, com 2 espaços por nível. Importar de novo o mesmo
			mapa troca o conteúdo e mantém os vínculos.
		</p>
		<label class="arquivo">
			<span>Arquivo do mapa (.md)</span>
			<input
				type="file"
				accept=".md,.markdown,.txt,text/markdown,text/plain"
				aria-label="Arquivo do mapa (.md)"
				disabled={importando}
				onchange={aoEscolherArquivo}
			/>
		</label>
		<label class="colar">
			<span>Ou cole o texto do mapa</span>
			<textarea rows="6" spellcheck="false" aria-label="Texto do mapa" bind:value={texto}></textarea>
		</label>
		<button
			type="button"
			class="btn primary"
			disabled={importando || texto.trim() === ''}
			onclick={() => importar(texto)}
		>
			{importando ? 'Importando…' : 'Importar'}
		</button>
	</section>

	<ImportarExportacao bind:this={restaurar} aoTerminar={recarregar} />

	<p class="exportar">
		<button type="button" class="btn" onclick={exportarTodos} disabled={exportando}>
			{exportando ? 'Exportando…' : '⬇ Exportar todos os mapas'}
		</button>
		<span
			>Um .zip com o texto, as questões, as imagens, os vínculos e as respostas de cada mapa: a cópia de segurança do
			que está aqui. Ele volta em “Importar uma exportação (.zip)”.</span
		>
	</p>
</div>

<style>
	.voltar {
		margin: 0 0 14px;
		font-size: 13.5px;
	}
	.aviso-ok {
		margin-bottom: 14px;
	}
	.aviso-ok a {
		margin-left: 6px;
		font-weight: 600;
	}

	.importar {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 12px;
		margin-bottom: 8px;
		padding: 11px 14px 14px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--bg-card);
	}
	.importar h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
	}
	.ajuda {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: var(--text-muted);
		max-width: 76ch;
	}
	.ajuda code {
		font-family: var(--font-mono);
		font-size: 12px;
		padding: 1px 5px;
		border-radius: 4px;
		background: var(--bg-soft);
	}
	.arquivo,
	.colar {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 5px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	textarea {
		box-sizing: border-box;
		width: 100%;
		padding: 9px 11px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 12px;
		line-height: 1.5;
		resize: vertical;
	}
	textarea:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.importar .btn {
		justify-self: start;
	}
	@media (pointer: coarse) {
		textarea {
			font-size: 16px;
		}
	}

	.exportar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px;
		margin: 28px 0 0;
		font-size: 12.5px;
		color: var(--text-muted);
	}
</style>
