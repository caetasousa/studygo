<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fmtDuracao } from '$lib/format';
	import type { ImportacaoCSV } from '$lib/types';

	/**
	 * Traz de volta os REGISTROS de uma planilha do plano: tempo, questões,
	 * acertos e conclusão.
	 *
	 * O cronograma não se mexe. Cada linha procura a atividade que já existe
	 * naquele dia e naquela matéria — é por isso que a importação serve para
	 * trazer o histórico de outra instalação sem reescrever o plano desta.
	 *
	 * Dois passos, como o assistente de edital: a prévia é a MESMA chamada com
	 * `confirmar: false`, então ela não tem como discordar do que acontece
	 * depois.
	 */
	let { onimportado }: { onimportado?: () => void } = $props();

	let csv = $state('');
	let nomeArquivo = $state('');
	let previa = $state<ImportacaoCSV | null>(null);
	let resultado = $state<ImportacaoCSV | null>(null);
	let erro = $state<string | null>(null);
	let ocupado = $state(false);

	const MAX_LINHAS_MOSTRADAS = 8;

	async function escolher(e: Event) {
		const alvo = e.target as HTMLInputElement;
		const f = alvo.files?.[0];
		if (!f) return;

		erro = null;
		resultado = null;
		nomeArquivo = f.name;
		csv = await f.text();
		// Escolher o MESMO arquivo de novo tem de disparar o evento outra vez.
		alvo.value = '';

		await analisar();
	}

	async function analisar() {
		if (!csv.trim()) return;

		ocupado = true;
		erro = null;

		try {
			previa = await planoStore.importarCsv(csv, false);
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
			resultado = await planoStore.importarCsv(csv, true);
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

	function tempo(min: number | null): string {
		return min === null ? '—' : fmtDuracao(min);
	}
</script>

<h2 class="sec">Importar registros de uma planilha</h2>
<p class="page-sub" style="margin-top:0">
	Exporte o CSV do outro plano e envie aqui: volta só o que foi <b>estudado</b> — tempo,
	questões, acertos e conclusão. Cada linha procura a matéria daquele dia; quando o dia
	já passou e a matéria não está lá, a atividade é <b>reconstruída</b> a partir da
	planilha, porque é ela que sabe o que aconteceu. O cronograma futuro não é tocado, e
	o que não achar onde entrar é listado com o motivo, não gravado.
</p>

{#if erro}<div class="form-error" style="margin-bottom:12px">{erro}</div>{/if}

{#if resultado}
	<div class="callout">
		<span class="em"><NavIcon name="info" /></span>
		<div>
			<b>{resultado.gravadas}</b>
			{resultado.gravadas === 1 ? 'linha importada' : 'linhas importadas'}{#if resultado.criadas > 0}, {resultado.criadas}
				com a atividade reconstruída no dia{/if}.
			{#if resultado.recusadas.length > 0}
				{resultado.recusadas.length} ficaram de fora.
			{/if}
			<button class="btn" style="margin-top:8px" onclick={limpar}>Importar outra</button>
		</div>
	</div>
{:else if previa}
	<div class="prev-topo">
		<b>
			{previa.aplicadas.length}
			{previa.aplicadas.length === 1 ? 'linha entra' : 'linhas entram'}
		</b>
		<span>
			{#if previa.criadas > 0}
				{previa.criadas}
				{previa.criadas === 1 ? 'reconstrói o dia' : 'reconstroem o dia'}{#if previa.recusadas.length > 0}
					·
				{/if}
			{/if}
			{#if previa.recusadas.length > 0}
				{previa.recusadas.length} sem lugar
			{/if}
		</span>
	</div>

	{#if previa.aplicadas.length > 0}
		<div class="tbl-wrap">
			<table class="tbl">
				<thead>
					<tr><th>Data</th><th>Matéria</th><th>Tempo</th><th>Q</th><th>✓</th></tr>
				</thead>
				<tbody>
					{#each previa.aplicadas.slice(0, MAX_LINHAS_MOSTRADAS) as l (l.linha)}
						<tr>
							<td>{l.data.split('-').reverse().slice(0, 2).join('/')}</td>
							<td class="materia">
								{l.disciplina}{l.tema ? ` · ${l.tema}` : ''}
								{#if l.criada}<span class="nova" title="A atividade não existe nesse dia e vai ser criada">nova</span>{/if}
							</td>
							<td>{tempo(l.minutos)}</td>
							<td>{l.questoes ?? '—'}</td>
							<td>{l.acertos ?? '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if previa.aplicadas.length > MAX_LINHAS_MOSTRADAS}
			<p class="page-sub" style="margin:6px 0 0">
				…e mais {previa.aplicadas.length - MAX_LINHAS_MOSTRADAS}.
			</p>
		{/if}
	{/if}

	{#if previa.recusadas.length > 0}
		<div class="callout warn" style="margin-top:12px">
			<span class="em"><NavIcon name="alerta" /></span>
			<div>
				<b>{previa.recusadas.length} linhas sem lugar</b> — vão ser ignoradas.
				<ul style="margin:6px 0 0;padding-left:18px">
					{#each previa.recusadas.slice(0, 6) as l (l.linha)}
						<li>linha {l.linha}: {l.motivo}</li>
					{/each}
				</ul>
			</div>
		</div>
	{/if}

	<div class="form-grid" style="margin-top:14px;align-items:flex-end">
		<button
			class="btn primario"
			disabled={ocupado || previa.aplicadas.length === 0}
			onclick={confirmar}
		>
			{ocupado ? 'Importando…' : `Importar ${previa.aplicadas.length} linhas`}
		</button>
		<button class="btn" onclick={limpar}>Cancelar</button>
	</div>
{:else}
	<div class="form-grid" style="align-items:center">
		<label class="btn" style="cursor:pointer">
			{nomeArquivo || '⬆ Escolher CSV do plano'}
			<input type="file" accept=".csv,text/csv" onchange={escolher} hidden />
		</label>
		{#if ocupado}<span class="page-sub">lendo…</span>{/if}
	</div>
{/if}

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
	.materia {
		color: var(--text-muted);
		font-size: 12.5px;
	}
	.nova {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--warn);
		margin-left: 6px;
	}
	.btn.primario {
		border-color: var(--accent);
		color: var(--accent);
	}
	.btn.primario:hover:not(:disabled) {
		background: var(--accent-soft);
	}
</style>
