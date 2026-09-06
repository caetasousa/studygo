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
		if (!previa) return;

		// Importar grava por cima do que já estiver lançado naquelas matérias:
		// é escrita em dado do estudante, e escrita em dado se pergunta antes.
		const aviso =
			`Importar ${previa.aplicadas.length} ` +
			`${previa.aplicadas.length === 1 ? 'linha' : 'linhas'}? ` +
			'O que já estiver lançado nessas matérias será substituído.';

		if (!confirm(aviso)) return;

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

<h2 class="sec">Importar de uma planilha</h2>
<p class="page-sub" style="margin-top:0">
	Envie o CSV exportado de outro plano: voltam o <b>estudo lançado</b> — tempo,
	questões, acertos e conclusão —, as <b>anotações do caderno de erros</b> e a
	<b>personalização de cada matéria</b>: a tag, o link do seu caderno (TEC,
	Qconcursos, um documento) e os ajustes de estudo.
</p>
<p class="page-sub" style="margin-top:0">
	O cronograma à frente não muda. Um dia que já passou e não tem a matéria da planilha
	é reconstruído a partir dela, que é quem sabe o que aconteceu; o que não achar onde
	entrar aparece com o motivo e não é gravado.
</p>

{#if erro}<div class="form-error" style="margin-bottom:12px">{erro}</div>{/if}

{#if resultado}
	<div class="fim">
		<div class="numeros">
			<span class="num">
				<b>{resultado.gravadas}</b>
				<i>{resultado.gravadas === 1 ? 'linha importada' : 'linhas importadas'}</i>
			</span>
			{#if resultado.criadas > 0}
				<span class="num">
					<b>{resultado.criadas}</b>
					<i>{resultado.criadas === 1 ? 'dia reconstruído' : 'dias reconstruídos'}</i>
				</span>
			{/if}
			{#if resultado.anotacoes > 0}
				<span class="num">
					<b>{resultado.anotacoes}</b>
					<i>{resultado.anotacoes === 1 ? 'anotação no caderno' : 'anotações no caderno'}</i>
				</span>
			{/if}
			{#if resultado.materias > 0}
				<span class="num">
					<b>{resultado.materias}</b>
					<i>{resultado.materias === 1 ? 'matéria restaurada' : 'matérias restauradas'}</i>
				</span>
			{/if}
			{#if resultado.recusadas.length > 0}
				<span class="num fora">
					<b>{resultado.recusadas.length}</b>
					<i>de fora</i>
				</span>
			{/if}
		</div>
		<button class="btn" onclick={limpar}>Importar outra</button>
	</div>
{:else if previa}
	<div class="numeros">
		<span class="num">
			<b>{previa.aplicadas.length}</b>
			<i>{previa.aplicadas.length === 1 ? 'linha entra' : 'linhas entram'}</i>
		</span>
		{#if previa.criadas > 0}
			<span class="num">
				<b>{previa.criadas}</b>
				<i>{previa.criadas === 1 ? 'dia reconstruído' : 'dias reconstruídos'}</i>
			</span>
		{/if}
		{#if previa.anotacoes > 0}
			<span class="num">
				<b>{previa.anotacoes}</b>
				<i>{previa.anotacoes === 1 ? 'anotação no caderno' : 'anotações no caderno'}</i>
			</span>
		{/if}
		{#if previa.materias > 0}
			<span class="num">
				<b>{previa.materias}</b>
				<i>{previa.materias === 1 ? 'matéria restaurada' : 'matérias restauradas'}</i>
			</span>
		{/if}
		{#if previa.recusadas.length > 0}
			<span class="num fora">
				<b>{previa.recusadas.length}</b>
				<i>sem lugar</i>
			</span>
		{/if}
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
	/* O resultado é uma linha de números, não um parágrafo: o que importa é
	   quanto entrou, quanto foi reconstruído e quanto ficou de fora. */
	.numeros {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 22px;
		margin-bottom: 12px;
	}
	.num {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}
	.num b {
		font-family: var(--font-mono);
		font-size: 19px;
		font-weight: 600;
		line-height: 1.1;
		color: var(--text);
	}
	.num i {
		font-style: normal;
		font-size: 11px;
		letter-spacing: 0.04em;
		color: var(--text-faint);
	}
	.num.fora b {
		color: var(--warn);
	}
	.fim {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 16px;
		flex-wrap: wrap;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 14px;
	}
	.fim .numeros {
		margin-bottom: 0;
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
