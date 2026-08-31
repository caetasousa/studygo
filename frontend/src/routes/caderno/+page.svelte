<script lang="ts">
	import IconButton from '$lib/components/IconButton.svelte';
	import { semNumeroInicial } from '$lib/estudo';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import DossieModal from '$lib/components/DossieModal.svelte';
	import ImportarTEC from '$lib/components/ImportarTEC.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc } from '$lib/format';
	import type { Caderno } from '$lib/types';

	let dados = $state<Caderno | null>(null);
	let erro = $state<string | null>(null);

	let novoTexto = $state('');
	let novaDisc = $state('');
	let enviando = $state(false);

	let dossieCodigo = $state('');
	let dossieAberto = $state<string | null>(null);

	const disciplinas = $derived(planoStore.plano?.concurso.disciplinas ?? []);

	async function carregar() {
		try {
			dados = await planoStore.caderno();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Erro';
		}
	}

	$effect(() => {
		if (planoStore.plano && !dados) void carregar();
	});

	async function adicionar(e: SubmitEvent) {
		e.preventDefault();
		if (!novoTexto.trim()) return;
		enviando = true;
		try {
			dados = await planoStore.criarAnotacao({
				texto: novoTexto.trim(),
				disciplina: novaDisc || null,
				resolvido: false
			});
			novoTexto = '';
			novaDisc = '';
		} catch (err) {
			erro = err instanceof Error ? err.message : 'Erro';
		} finally {
			enviando = false;
		}
	}

	async function alternar(a: Caderno['anotacoes'][number]) {
		dados = await planoStore.atualizarAnotacao(a.id, {
			texto: a.texto,
			disciplina: a.disciplina,
			data: a.data,
			tema: a.tema,
			url: a.url,
			resolvido: !a.resolvido
		});
	}

	async function remover(id: string) {
		dados = await planoStore.removerAnotacao(id);
	}

	function nomeDisc(codigo: string | null): string {
		if (!codigo) return '';
		return disciplinas.find((d) => d.codigo === codigo)?.nome ?? codigo;
	}
</script>

<PageHead
	icone="caderno"
	titulo="Caderno de erros"
	sub="Tudo o que você marcou para revisar: anotações livres, notas dos dias e as baterias com aproveitamento baixo."
	mostrarProps={false}
/>

{#if erro}<div class="form-error">{erro}</div>{/if}

{#if dossieAberto}
	<DossieModal codigo={dossieAberto} onclose={() => (dossieAberto = null)} />
{/if}

{#if dados}
	<div class="page">

		<ImportarTEC onimportado={carregar} />

		{#if disciplinas.length > 0}
			<div class="card">
				<div class="card-body">
					<h2 class="sec" style="margin-top:0">Estudar no NotebookLM</h2>
					<p class="page-sub" style="margin-top:0">
						Gera um dossiê da disciplina (ementa + leis cadastradas + suas anotações) pronto para colar
						como fonte no NotebookLM e pedir um guia de estudos ou áudio.
					</p>
					<div class="form-grid">
						<div class="field">
							<label for="dossie-disc">Disciplina</label>
							<select id="dossie-disc" bind:value={dossieCodigo}>
								<option value="">escolha…</option>
								{#each disciplinas as d (d.codigo)}
									<option value={d.codigo}>{d.nome}</option>
								{/each}
							</select>
						</div>
						<button
							class="btn primary"
							disabled={!dossieCodigo}
							onclick={() => (dossieAberto = dossieCodigo)}
						>
							Preparar dossiê
						</button>
					</div>
				</div>
			</div>
		{/if}

		<div class="card">
			<div class="card-body">
				<form class="form-grid" onsubmit={adicionar} style="align-items:flex-end">
					<div class="field" style="flex:1 1 320px">
						<label for="an-texto">Nova anotação</label>
						<input
							id="an-texto"
							type="text"
							bind:value={novoTexto}
							placeholder="Ex.: revisar regência de assistir"
							style="width:100%"
						/>
					</div>
					<div class="field">
						<label for="an-disc">Disciplina</label>
						<select id="an-disc" bind:value={novaDisc}>
							<option value="">—</option>
							{#each disciplinas as d (d.codigo)}
								<option value={d.codigo}>{d.nome}</option>
							{/each}
						</select>
					</div>
					<button class="btn primary" type="submit" disabled={enviando}>Adicionar</button>
				</form>
			</div>
		</div>

		<!-- The notebook proper: what went wrong, per subject, accumulating. This is
		     exactly what the daily review tail drills, so the screen and the
		     schedule are showing the same thing. -->
		<h2 class="sec">Caderno por matéria</h2>
		{#if dados.porDisciplina.length === 0}
			<p class="page-sub" style="margin-top:0">
				Ainda sem erros registrados. Assim que uma bateria ficar abaixo de {planoStore.plano?.config.limiarFraco ?? 70}%,
				o assunto entra aqui e passa a voltar na revisão diária.
			</p>
		{:else}
			<p class="page-sub" style="margin-top:0">
				Os assuntos em que você foi mal, acumulados. A revisão no fim de cada dia
				puxa daqui, começando pelos que você mais errou.
			</p>
			<div class="cad-grid">
				{#each dados.porDisciplina as d (d.disciplina)}
					<div class="card cad-disc">
						<div class="card-top">
							<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>
							<b>{d.nome}</b>
							<span class="cad-n">{d.temas.length} {d.temas.length === 1 ? 'assunto' : 'assuntos'}</span>
						</div>
						<div class="card-body cad-body">
							{#each d.temas as t (t.tema)}
								<div class="cad-item">
									<span class="cad-tema">{semNumeroInicial(t.tema)}</span>
									<span class="cad-pct" class:critico={t.aproveitamento < 50}>
										{t.aproveitamento}%
									</span>
									<span class="cad-meta">
										{t.acertos}/{t.questoes}
										{#if t.erros > 1}· {t.erros} vezes{/if}
									</span>
								</div>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		{/if}

		<h2 class="sec">Anotações ({dados.anotacoes.length})</h2>
		{#if dados.anotacoes.length === 0}
			<p class="page-sub" style="margin-top:0">Nenhuma anotação ainda.</p>
		{:else}
			<div class="card">
				{#each dados.anotacoes as a (a.id)}
					<div class="marco" style="grid-template-columns:30px minmax(0,1fr) 140px 40px">
						<input
							type="checkbox"
							class="checkbox"
							checked={a.resolvido}
							onchange={() => alternar(a)}
							aria-label="Resolvido"
						/>
						<span class="tx" style:opacity={a.resolvido ? 0.5 : 1}>
							{#if a.tema}<b class="tema">{a.tema}</b>{/if}{a.texto}
							{#if a.origem !== 'manual'}
								<span class="origem">{a.origem}</span>
							{/if}
							{#if a.url}
								<a class="fonte" href={a.url} target="_blank" rel="noopener noreferrer">questões ↗</a>
							{/if}
						</span>
						<span class="fa">{nomeDisc(a.disciplina)}</span>
						<IconButton
							icon="fechar"
							label="Remover anotação"
							tom="perigo"
							onclick={() => remover(a.id)}
						/>
					</div>
				{/each}
			</div>
		{/if}

		<h2 class="sec">Notas lançadas nos dias ({dados.diasComNota.length})</h2>
		{#if dados.diasComNota.length === 0}
			<p class="page-sub" style="margin-top:0">
				As anotações que você escrever direto no Cronograma aparecem aqui.
			</p>
		{:else}
			<div class="card">
				{#each dados.diasComNota as d (d.data)}
					<div class="marco" style="grid-template-columns:80px minmax(0,1fr) 120px">
						<span class="dt">dia {String(d.n).padStart(3, '0')}</span>
						<span class="tx">{d.nota}</span>
						<span class="fa">{fc(d.data)} · {d.disciplinas.join(' · ')}</span>
					</div>
				{/each}
			</div>
		{/if}

		<h2 class="sec">Baterias com aproveitamento abaixo de 70% ({dados.diasFracos.length})</h2>
		{#if dados.diasFracos.length === 0}
			<p class="page-sub" style="margin-top:0">Nenhuma até agora — ou você ainda não registrou acertos.</p>
		{:else}
			<div class="tbl-wrap">
				<table class="tbl">
					<thead><tr><th>Dia</th><th>Data</th><th>Questões</th><th>Acertos</th><th>Aproveitamento</th></tr></thead>
					<tbody>
						{#each dados.diasFracos as d (d.data)}
							<tr>
								<td>dia {String(d.n).padStart(3, '0')}</td>
								<td>{fc(d.data)}</td>
								<td>{d.questoes}</td>
								<td>{d.acertos}</td>
								<td class="dev neg">{d.pct}%</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
{:else if !erro}
	<p class="page-sub">Carregando…</p>
{/if}

<style>
	.cad-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 12px;
	}
	.cad-disc .card-top {
		gap: 8px;
	}
	.cad-n {
		margin-left: auto;
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
	}
	.cad-body {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 8px 10px;
	}
	/* Topic | hit rate | attempts — the rate is the point, so it gets the
	   fixed column and the mono figures. */
	.cad-item {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto auto;
		align-items: baseline;
		gap: 10px;
		padding: 6px 4px;
		border-bottom: 1px dotted var(--border);
	}
	.cad-item:last-child {
		border-bottom: 0;
	}
	.cad-tema {
		font-size: 13px;
		line-height: 1.4;
		min-width: 0;
	}
	.cad-pct {
		font-family: var(--font-mono);
		font-size: 12.5px;
		font-weight: 600;
		color: var(--warn);
		font-variant-numeric: tabular-nums;
	}
	.cad-pct.critico {
		color: var(--danger);
	}
	.cad-meta {
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--text-faint);
		white-space: nowrap;
	}

	.tema {
		font-weight: 600;
		margin-right: 6px;
	}
	.origem {
		font-family: var(--font-mono);
		font-size: 9.5px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		color: var(--text-faint);
		padding: 1px 5px;
		border-radius: 4px;
		margin-left: 6px;
		white-space: nowrap;
	}
	.fonte {
		font-size: 11.5px;
		margin-left: 6px;
		white-space: nowrap;
	}
</style>
