<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { hojeISO } from '$lib/format';
	import type { PreviewTEC } from '$lib/types';

	let { onimportado }: { onimportado?: () => void } = $props();

	let csv = $state('');
	let nomeArquivo = $state('');
	let data = $state(hojeISO());
	let previa = $state<PreviewTEC | null>(null);
	let resultado = $state<PreviewTEC | null>(null);
	let erro = $state<string | null>(null);
	let ocupado = $state(false);

	async function escolher(e: Event) {
		const f = (e.target as HTMLInputElement).files?.[0];
		if (!f) return;

		erro = null;
		resultado = null;
		nomeArquivo = f.name;
		csv = await f.text();

		await analisar();
	}

	async function analisar() {
		if (!csv.trim()) return;
		ocupado = true;
		erro = null;
		try {
			previa = await planoStore.previewTec(csv);
		} catch (e) {
			previa = null;
			erro = e instanceof Error ? e.message : 'não consegui ler a planilha';
		} finally {
			ocupado = false;
		}
	}

	async function confirmar() {
		ocupado = true;
		erro = null;
		try {
			resultado = await planoStore.importarTec(csv, data);
			previa = null;
			csv = '';
			nomeArquivo = '';
			onimportado?.();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'não consegui importar';
		} finally {
			ocupado = false;
		}
	}

	function limpar() {
		previa = null;
		resultado = null;
		csv = '';
		nomeArquivo = '';
		erro = null;
	}
</script>

<div class="card">
	<div class="card-body">
		<h2 class="sec" style="margin-top:0">Importar desempenho do TEC</h2>
		<p class="page-sub" style="margin-top:0">
			No TEC, abra <b>Estatísticas</b> e use <b>Exportar para planilha</b>; salve como
			<b>CSV</b> e envie aqui. Os assuntos são casados com os temas do edital: viram o registro do
			dia por disciplina, e cada assunto fraco entra no caderno de erros. Nada é enviado ao TEC e
			sua senha não é usada.
		</p>

		{#if erro}<div class="form-error" style="margin-bottom:12px">{erro}</div>{/if}

		{#if resultado}
			<div class="callout">
				<span class="em"><NavIcon name="info" /></span>
				<div>
					Importados <b>{resultado.casados.length} assuntos</b> — {resultado.questoes} questões,
					{resultado.acertos} acertos. O registro de {data} foi preenchido por disciplina.
					<button class="btn" style="margin-top:8px" onclick={limpar}>Importar outra</button>
				</div>
			</div>
		{:else if previa}
			<div class="prev-topo">
				<b>{previa.casados.length} assuntos casaram</b>
				<span>{previa.questoes} questões · {previa.acertos} acertos</span>
			</div>

			<div class="tbl-wrap">
				<table class="tbl">
					<thead>
						<tr><th>Assunto do TEC</th><th>Vai para</th><th>Q</th><th>✓</th><th>✗</th><th>%</th></tr>
					</thead>
					<tbody>
						{#each previa.casados as c (c.assunto)}
							<tr>
								<td>{c.assunto}</td>
								<td class="destino">{c.nome}{c.tema ? ` · ${c.tema}` : ''}</td>
								<td>{c.questoes}</td>
								<td>{c.acertos}</td>
								<td>{c.erros}</td>
								<td class:fraco={c.pct < 60}>{c.pct}%</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			{#if previa.semCorrespondencia.length > 0}
				<div class="callout warn" style="margin-top:12px">
					<span class="em"><NavIcon name="alerta" /></span>
					<div>
						<b>{previa.semCorrespondencia.length} assuntos sem correspondência</b> — vão ser
						ignorados. Para trazê-los, acrescente o tema à disciplina em
						<a href="/concursos/{planoStore.plano?.concurso.slug}/editar">editar o concurso</a>.
						<ul style="margin:6px 0 0;padding-left:18px">
							{#each previa.semCorrespondencia.slice(0, 6) as c (c.assunto)}
								<li>{c.assunto} ({c.questoes}q)</li>
							{/each}
						</ul>
					</div>
				</div>
			{/if}

			<div class="form-grid" style="margin-top:14px;align-items:flex-end">
				<div class="field">
					<label for="tec-data">Lançar no dia</label>
					<input id="tec-data" type="date" bind:value={data} />
				</div>
				<button class="btn primary" disabled={ocupado} onclick={confirmar}>
					{ocupado ? 'Importando…' : `Importar ${previa.casados.length} assuntos`}
				</button>
				<button class="btn" onclick={limpar}>Cancelar</button>
			</div>
		{:else}
			<div class="form-grid" style="align-items:center">
				<label class="btn primary" style="cursor:pointer">
					{nomeArquivo || 'Escolher CSV do TEC'}
					<input type="file" accept=".csv,text/csv" onchange={escolher} hidden />
				</label>
				{#if ocupado}<span class="spinner"></span>{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.prev-topo {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
		flex-wrap: wrap;
		margin-bottom: 10px;
	}
	.prev-topo span {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
	}
	.destino {
		color: var(--text-muted);
		font-size: 12.5px;
	}
	.fraco {
		color: var(--danger);
		font-weight: 600;
	}
</style>
