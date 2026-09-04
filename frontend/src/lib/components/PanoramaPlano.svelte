<script lang="ts">
	import { nf0, nf1 } from '$lib/format';
	import type { PlanoResposta } from '$lib/types';

	/**
	 * The whole plan as one strip of weekly bars, so "where am I, and how have I
	 * been doing" is answered before scrolling through months of days.
	 *
	 * Each bar is a week: its height is that week's target load, its fill the
	 * share actually logged. Everything is derived from the days the API already
	 * sends — no new endpoint.
	 */
	let { plano }: { plano: PlanoResposta } = $props();

	interface Barra {
		numero: number;
		alvo: number;
		feito: number;
		pct: number;
		reta: boolean;
		atual: boolean;
		futura: boolean;
	}

	const semanaAtual = $derived(
		plano.hojeIndex !== null ? (plano.dias[plano.hojeIndex]?.semana ?? null) : null
	);

	const barras = $derived.by<Barra[]>(() => {
		const porSemana = new Map<number, { alvo: number; feito: number; reta: boolean; fim: string }>();
		for (const d of plano.dias) {
			// Rest days carry no target, so they neither help nor hurt the ratio.
			const conta = d.itens.length > 0 || d.tipo === 'rev';
			const s = porSemana.get(d.semana) ?? {
				alvo: 0,
				feito: 0,
				reta: d.fase === 'reta',
				fim: d.data
			};
			if (conta) s.alvo += plano.config.horasDia;
			s.feito += d.horas ?? 0;
			s.reta = s.reta || d.fase === 'reta';
			if (d.data > s.fim) s.fim = d.data;
			porSemana.set(d.semana, s);
		}

		return [...porSemana.entries()]
			.sort((a, b) => a[0] - b[0])
			.map(([numero, s]) => ({
				numero,
				alvo: s.alvo,
				feito: s.feito,
				pct: s.alvo > 0 ? Math.min(100, (s.feito / s.alvo) * 100) : 0,
				reta: s.reta,
				atual: numero === semanaAtual,
				futura: semanaAtual !== null && numero > semanaAtual
			}));
	});

	const totalSemanas = $derived(barras.length);

	// A bar's colour states how the week went, not what kind of week it was —
	// which is the question the strip exists to answer.
	function cor(b: Barra): string {
		if (b.atual) return 'var(--accent)';
		if (b.futura) return b.reta ? 'var(--danger-soft)' : 'var(--bg-hover)';
		if (b.alvo === 0) return 'var(--bg-hover)';
		if (b.pct >= 85) return 'var(--good)';
		if (b.pct >= 50) return 'var(--warn)';
		return 'var(--danger)';
	}

	// Past weeks show how much got done; future ones are drawn at full height as
	// a plan yet to be filled.
	function altura(b: Barra): number {
		if (b.futura) return 100;
		return Math.max(6, b.pct);
	}
</script>

<section class="panorama" aria-label="Panorama do plano">
	<header>
		<h2>Panorama do plano</h2>
		<span class="ctx">
			{#if semanaAtual !== null}
				semana {semanaAtual} de {totalSemanas} · faltam {plano.props.faltamDias} dias
			{:else}
				{totalSemanas} semanas · faltam {plano.props.faltamDias} dias
			{/if}
		</span>
	</header>

	<div class="barras">
		{#each barras as b (b.numero)}
			<div
				class="col"
				title="Semana {String(b.numero).padStart(2, '0')} — {nf1.format(b.feito)} de {nf1.format(
					b.alvo
				)} h"
			>
				<span class="bar" class:atual={b.atual} style="height:{altura(b)}%; background:{cor(b)}"
				></span>
			</div>
		{/each}
	</div>

	<footer>
		<span class="leg"><i style="background:var(--good)"></i>meta cumprida</span>
		<span class="leg"><i style="background:var(--warn)"></i>parcial</span>
		<span class="leg"><i style="background:var(--danger)"></i>abaixo da metade</span>
		<span class="leg"><i style="background:var(--bg-hover)"></i>a fazer</span>
		<span class="tot">
			{nf1.format(plano.props.horasTotal)} / {nf0.format(plano.props.horasAlvo)} h · {plano.props
				.progresso}% do plano
		</span>
	</footer>
</section>

<style>
	.panorama {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 16px 18px;
		margin-top: 22px;
	}
	header {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex-wrap: wrap;
		margin-bottom: 13px;
	}
	h2 {
		margin: 0;
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.09em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.ctx {
		margin-left: auto;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
	}
	.barras {
		display: flex;
		gap: 3px;
		align-items: flex-end;
		height: 44px;
	}
	.col {
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: flex-end;
		height: 100%;
		min-width: 0;
	}
	.bar {
		display: block;
		border-radius: 2px;
		width: 100%;
	}
	.bar.atual {
		box-shadow: 0 0 0 2px var(--accent-soft);
	}
	footer {
		display: flex;
		gap: 16px;
		flex-wrap: wrap;
		align-items: center;
		margin-top: 11px;
		font-size: 11.5px;
		color: var(--text-muted);
	}
	.leg {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.leg i {
		width: 9px;
		height: 9px;
		border-radius: 2px;
		display: block;
		flex: none;
	}
	.tot {
		margin-left: auto;
		font-family: var(--font-mono);
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}

	@media (max-width: 620px) {
		.tot {
			margin-left: 0;
		}
	}
</style>
