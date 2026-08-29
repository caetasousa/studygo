<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { nf1, nf0 } from '$lib/format';
	import { debounce, parseInteger } from '$lib/debounce';
	import type { LinhaBalanceamento } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const esp = $derived(plano?.balanceamento.filter((l) => l.bloco === 'esp') ?? []);
	const ger = $derived(plano?.balanceamento.filter((l) => l.bloco === 'ger') ?? []);

	let pendente: Record<string, number> = {};
	const salvar = debounce(() => {
		if (Object.keys(pendente).length === 0) return;
		void planoStore.salvarConfig({ questoes: { ...pendente } });
		pendente = {};
	}, 500);

	function onQuestoes(codigo: string, e: Event) {
		const v = parseInteger((e.target as HTMLInputElement).value);
		if (v === null || v < 0) return;
		pendente[codigo] = v;
		salvar();
	}

	function total(linhas: LinhaBalanceamento[], f: (l: LinhaBalanceamento) => number): number {
		return linhas.reduce((a, l) => a + f(l), 0);
	}

	const hBloco = $derived((plano?.config.horasDia ?? 0) / 2);

	// Orçamento de questões: o motor divide o tempo em proporção estrita, então
	// aumentar uma matéria tira tempo de todas as outras. Isto é o que avisa.
	const orcamento = $derived.by(() => {
		const linhas = plano?.balanceamento ?? [];
		const distribuido = total(linhas, (l) => l.questoes);
		const edital = total(linhas, (l) => l.questoesEdital);
		const sobra = distribuido - edital;
		return {
			distribuido,
			edital,
			sobra,
			pct: edital > 0 ? Math.min(100, (distribuido / edital) * 100) : 0,
			nivel: sobra === 0 ? 'ok' : Math.abs(sobra) * 4 > edital ? 'danger' : 'warn'
		};
	});

	const resumo = $derived.by(() => {
		if (!plano) return [];
		const grupos: { nome: string; tipo: string }[] = [
			{ nome: 'Revisão semanal do ciclo', tipo: 'rev' },
			{ nome: 'Simulados completos', tipo: 'sim' },
			{ nome: 'Prova discursiva', tipo: 'disc' },
			{ nome: 'Véspera', tipo: 'vespera' }
		];
		return grupos.map((g) => {
			const ds = plano.dias.filter((d) => d.tipo === g.tipo);
			return {
				nome: g.nome,
				dias: ds.length,
				previsto: ds.length * plano.config.horasDia,
				lancado: ds.reduce((a, d) => a + (d.registro?.horas ?? 0), 0),
				questoes: ds.reduce((a, d) => a + (d.registro?.questoes ?? 0), 0)
			};
		});
	});
</script>

<PageHead
	emoji="⚖️"
	titulo="Balanceamento"
	sub="Como as horas se dividem entre as disciplinas, e o quanto você está aplicando de fato."
/>

{#if plano}
	<div class="page">
		<div class="callout">
			<span class="em">⚖️</span>
			<div>
				<b>Como o tempo é dividido.</b> Cada questão de Conhecimentos Gerais vale 1 ponto; cada questão
				de Específicos vale 2. O número de questões por disciplina é uma <b>estimativa editável</b> —
				altere os campos e todo o cronograma se refaz.
			</div>
		</div>

		{#if orcamento.edital > 0}
			<div class="orc {orcamento.nivel}">
				<div class="orc-topo">
					<b>
						{orcamento.distribuido} de {orcamento.edital} questões distribuídas
					</b>
					<span>
						{#if orcamento.sobra === 0}
							bate com o edital
						{:else if orcamento.sobra > 0}
							{orcamento.sobra} a mais — tire de alguma matéria
						{:else}
							faltam {-orcamento.sobra} — distribua em alguma matéria
						{/if}
					</span>
				</div>
				<div class="orc-bar"><i style="width:{orcamento.pct}%"></i></div>
			</div>
		{/if}

		{#snippet tabela(linhas: LinhaBalanceamento[])}
			<div class="tbl-wrap">
				<table class="tbl">
					<thead>
						<tr>
							<th>Disciplina</th><th>Questões</th><th>vs edital</th><th>Peso</th><th>Pontos</th><th>% ideal</th>
							<th>Bl. conteúdo</th><th>Bl. reta final</th><th>Previsto</th><th>Lançado</th>
							<th>Desvio</th><th>Acerto</th>
						</tr>
					</thead>
					<tbody>
						{#each linhas as l (l.codigo)}
							<tr>
								<td>
									<span class="chip-dot" style="background:var(--c{l.cor}-tx)"></span>{l.nome}
								</td>
								<td>
									<input
										type="number"
										min="0"
										max="60"
										step="1"
										value={l.questoes}
										oninput={(e) => onQuestoes(l.codigo, e)}
									/>
								</td>
								<td class="delta {l.delta > 0 ? 'pos' : l.delta < 0 ? 'neg' : ''}">
									{l.delta === 0 ? '=' : (l.delta > 0 ? '+' : '') + l.delta}
								</td>
								<td>{l.peso}</td>
								<td><b>{l.pontos}</b></td>
								<td>{nf1.format(l.pctIdeal)}%</td>
								<td>{l.blocosConteudo}</td>
								<td>{l.blocosReta}</td>
								<td>{nf1.format(l.horasPrevisto)} h</td>
								<td>{nf1.format(l.horasLancado)} h</td>
								<td class="dev {l.desvio < -1 ? 'neg' : l.desvio > 1 ? 'pos' : ''}">
									{plano.props.horasTotal ? (l.desvio >= 0 ? '+' : '') + nf1.format(l.desvio) : '—'}
								</td>
								<td>{l.acertoPct !== null ? l.acertoPct + '%' : '—'}</td>
							</tr>
						{/each}
					</tbody>
					<tfoot>
						<tr>
							<td>Total</td>
							<td>{total(linhas, (l) => l.questoes)}</td>
							<td>{total(linhas, (l) => l.questoesEdital)} no edital</td>
							<td>—</td>
							<td>{total(linhas, (l) => l.pontos)}</td>
							<td></td>
							<td>{total(linhas, (l) => l.blocosConteudo)}</td>
							<td>{total(linhas, (l) => l.blocosReta)}</td>
							<td>
								{nf1.format(
									total(linhas, (l) => l.blocosConteudo + l.blocosReta) * hBloco
								)} h
							</td>
							<td>{nf1.format(total(linhas, (l) => l.horasLancado))} h</td>
							<td></td><td></td>
						</tr>
					</tfoot>
				</table>
			</div>
		{/snippet}

		<h2 class="sec">Conhecimentos específicos — peso 2 por questão</h2>
		{@render tabela(esp)}

		<h2 class="sec">Conhecimentos gerais — peso 1 por questão</h2>
		{@render tabela(ger)}

		<h2 class="sec">Simulados e prova discursiva</h2>
		<div class="tbl-wrap">
			<table class="tbl">
				<thead>
					<tr><th>Bloco</th><th>Dias</th><th>Previsto</th><th>Lançado</th><th>Questões</th></tr>
				</thead>
				<tbody>
					{#each resumo as r (r.nome)}
						<tr>
							<td>{r.nome}</td>
							<td>{r.dias}</td>
							<td>{nf1.format(r.previsto)} h</td>
							<td>{nf1.format(r.lancado)} h</td>
							<td>{nf0.format(r.questoes)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.orc {
		border: 1px solid var(--border);
		border-left: 3px solid var(--good);
		border-radius: 8px;
		background: var(--bg-card);
		padding: 12px 14px;
		margin-bottom: 18px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.orc.warn {
		border-left-color: var(--warn);
		background: var(--warn-soft);
	}
	.orc.danger {
		border-left-color: var(--danger);
		background: var(--danger-soft);
	}
	.orc-topo {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
		flex-wrap: wrap;
	}
	.orc-topo b {
		font-size: 14px;
	}
	.orc-topo span {
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.orc-bar {
		height: 5px;
		border-radius: 3px;
		background: var(--bg-soft);
		overflow: hidden;
	}
	.orc-bar i {
		display: block;
		height: 100%;
		background: var(--good);
	}
	.orc.warn .orc-bar i {
		background: var(--warn);
	}
	.orc.danger .orc-bar i {
		background: var(--danger);
	}

	.delta {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-faint);
	}
	.delta.pos {
		color: var(--warn);
		font-weight: 600;
	}
	.delta.neg {
		color: var(--accent);
		font-weight: 600;
	}
</style>
