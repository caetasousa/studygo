<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, nf0, nf1 } from '$lib/format';
	import type { Estatisticas } from '$lib/types';

	let dados = $state<Estatisticas | null>(null);
	let erro = $state<string | null>(null);

	$effect(() => {
		if (planoStore.plano) {
			planoStore
				.estatisticas()
				.then((d) => (dados = d))
				.catch((e) => (erro = e instanceof Error ? e.message : 'Erro'));
		}
	});

	const maxHoras = $derived(dados ? Math.max(1, ...dados.serie.map((p) => p.horas)) : 1);
	const pctAcerto = $derived(
		dados && dados.questoesTotal > 0
			? Math.round(((dados.acertoPct ?? 0) / dados.questoesTotal) * 100)
			: null
	);
	const maxSemana = $derived(
		dados ? Math.max(1, ...dados.porSemana.map((s) => Math.max(s.horasPrevisto, s.horas))) : 1
	);

	// A fresh plan has every week at 0 h lançado, which renders dozens of identical
	// rows. Show the weeks that carry data (plus the following one) and fold the
	// untouched tail behind a toggle.
	let verTodasSemanas = $state(false);

	// Weeks up to the last one with activity, plus the next — on a plan with no
	// registros at all that is just the first week, which is the point: an empty
	// table of 20 identical rows says nothing.
	const semanasComDados = $derived.by(() => {
		const todas = dados?.porSemana ?? [];
		const ultima = todas.reduce((acc, s, i) => (s.horas > 0 || s.questoes > 0 ? i : acc), -1);
		return todas.slice(0, ultima + 2);
	});

	const semanasVisiveis = $derived(
		verTodasSemanas ? (dados?.porSemana ?? []) : semanasComDados
	);
	const semanasOcultas = $derived((dados?.porSemana.length ?? 0) - semanasVisiveis.length);
</script>

<PageHead
	icone="estatisticas"
	titulo="Estatísticas"
	sub="Sua evolução ao longo do plano: horas, acertos e sequência de dias estudados."
	mostrarProps={false}
/>

{#if erro}
	<div class="form-error">{erro}</div>
{:else if dados}
	<div class="page">
		<div class="props">
			<div class="prop">Streak <b>{dados.streak}</b> dias</div>
			<div class="prop">Horas <b>{nf1.format(dados.horasTotal)}</b></div>
			<div class="prop">Questões <b>{nf0.format(dados.questoesTotal)}</b></div>
			<div class="prop">Acerto <b>{pctAcerto !== null ? pctAcerto + '%' : '—'}</b></div>
		</div>

		<h2 class="sec">Sequência atual</h2>
		<div class="card">
			<div class="card-body" style="text-align:center">
				<div class="streak-big">{dados.streak}</div>
				<p class="page-sub" style="margin:4px auto 0">
					dia(s) seguidos concluídos até hoje. Não quebre a corrente.
				</p>
			</div>
		</div>

		<h2 class="sec">Horas por dia registrado</h2>
		<div class="card">
			<div class="card-body">
				{#if dados.serie.length === 0}
					<p class="page-sub" style="margin:0">Ainda sem registros. Comece pela página Hoje.</p>
				{:else}
					<div class="chart">
						{#each dados.serie as p (p.data)}
							<div
								class="bar"
								title="{fc(p.data)} · {nf1.format(p.horas)} h · {p.questoes} questões"
							>
								<i style="height:{(p.horas / maxHoras) * 100}%"></i>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<h2 class="sec">Semana a semana</h2>
		<div class="tbl-wrap">
			<table class="tbl">
				<thead>
					<tr><th>Semana</th><th>Previsto</th><th>Lançado</th><th>Aproveitamento</th><th>Questões</th></tr>
				</thead>
				<tbody>
					{#each semanasVisiveis as s (s.semana)}
						<tr>
							<td>Semana {String(s.semana).padStart(2, '0')}</td>
							<td>{nf1.format(s.horasPrevisto)} h</td>
							<td>{nf1.format(s.horas)} h</td>
							<td>
								<span class="mini-bar" style="display:inline-block;vertical-align:middle"
									><i
										style="width:{s.horasPrevisto
											? Math.min(100, (s.horas / s.horasPrevisto) * 100)
											: 0}%"
									></i></span
								>
								{s.horasPrevisto ? Math.round((s.horas / s.horasPrevisto) * 100) : 0}%
							</td>
							<td>{nf0.format(s.questoes)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if semanasOcultas > 0 || verTodasSemanas}
			<button class="ver-mais" onclick={() => (verTodasSemanas = !verTodasSemanas)}>
				{verTodasSemanas
					? 'mostrar só as semanas com registro'
					: `mostrar as outras ${semanasOcultas} semanas`}
			</button>
		{/if}

		<h2 class="sec">Acerto por disciplina</h2>
		<div class="tbl-wrap">
			<table class="tbl">
				<thead>
					<tr><th>Disciplina</th><th>Lançado</th><th>Desvio</th><th>Acerto</th></tr>
				</thead>
				<tbody>
					{#each dados.porDisciplina as l (l.codigo)}
						<tr>
							<td><span class="chip-dot" style="background:var(--c{l.cor}-tx)"></span>{l.nome}</td>
							<td>{nf1.format(l.horasLancado)} h</td>
							<td class="dev {l.desvio < -1 ? 'neg' : l.desvio > 1 ? 'pos' : ''}">
								{l.desvio >= 0 ? '+' : ''}{nf1.format(l.desvio)}
							</td>
							<td>{l.acertoPct !== null ? l.acertoPct + '%' : '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{:else}
	<p class="page-sub">Carregando…</p>
{/if}
