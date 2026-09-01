<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import PanoramaPlano from '$lib/components/PanoramaPlano.svelte';
	import DiaCard from '$lib/components/DiaCard.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, nf1 } from '$lib/format';
	import type { Dia } from '$lib/types';

	const plano = $derived(planoStore.plano);

	interface Semana {
		numero: number;
		fase: string;
		dias: Dia[];
		mostrarFlag: boolean;
	}

	const semanas = $derived.by<Semana[]>(() => {
		if (!plano) return [];
		const out: Semana[] = [];
		let faseAnterior: string | null = null;
		for (const d of plano.dias) {
			let s = out.at(-1);
			if (!s || s.numero !== d.semana) {
				s = { numero: d.semana, fase: d.fase, dias: [], mostrarFlag: d.fase !== faseAnterior };
				faseAnterior = d.fase;
				out.push(s);
			}
			s.dias.push(d);
		}
		return out;
	});

	function saldoSemana(s: Semana): { h: number; alvo: number } {
		const h = s.dias.reduce((a, d) => a + (d.registro?.horas ?? 0), 0);
		return { h, alvo: s.dias.length * (plano?.config.horasDia ?? 0) };
	}

	/**
	 * A day is movable when it has at least one activity. Rearranging is
	 * per-activity: each row carries an up and a down button that step it
	 * one slot at a time, crossing the day boundary when it reaches the top
	 * or bottom.
	 */
	function diaMovivel(d: Dia): boolean {
		return d.itens.length > 0;
	}

	/** Days that receive activities: study, weekly review, guided review. */
	function ehDiaUtil(d: Dia): boolean {
		return d.tipo === 'est' || d.tipo === 'rev' || d.tipo === 'revd';
	}

	/** The (day, index) an activity currently sits on, or null. */
	function posicaoAtual(id: string): { dia: Dia; indice: number } | null {
		for (const d of plano?.dias ?? []) {
			const i = d.itens.findIndex((x) => x.id === id);
			if (i >= 0) return { dia: d, indice: i };
		}
		return null;
	}

	/** The nearest neighbour of `d` in the plan that accepts activities. */
	function diaVizinho(d: Dia, passo: -1 | 1): Dia | null {
		const dias = plano?.dias ?? [];
		const i = dias.findIndex((x) => x.data === d.data);
		if (i < 0) return null;
		for (let j = i + passo; j >= 0 && j < dias.length; j += passo) {
			if (ehDiaUtil(dias[j])) return dias[j];
		}
		return null;
	}

	/**
	 * Steps one activity by one slot. Inside the day it swaps with the
	 * neighbour (backend TrocarAtividades). At the top or bottom of a day it
	 * crosses to the nearest useful day — up goes to the BOTTOM of the day
	 * before, down to the TOP of the day after — with trocar=false so the
	 * source day loses one and the target day gains one.
	 */
	async function moverPorPasso(id: string, passo: -1 | 1) {
		const p = posicaoAtual(id);
		if (!p) return;

		const dentro = p.indice + passo;
		if (dentro >= 0 && dentro < p.dia.itens.length) {
			await planoStore.moverAtividade(id, p.dia.data, dentro, true);
			return;
		}

		const vizinho = diaVizinho(p.dia, passo);
		if (!vizinho) return; // no useful day on that side; button was hidden anyway

		// Up → land at the END of the previous useful day; down → land at slot 0.
		const alvoPos = passo === -1 ? vizinho.itens.length : 0;
		await planoStore.moverAtividade(id, vizinho.data, alvoPos, false);
	}

	const moverAcima = (id: string) => moverPorPasso(id, -1);
	const moverAbaixo = (id: string) => moverPorPasso(id, 1);
</script>

<PageHead
	icone="cronograma"
	titulo="Cronograma"
	sub="Todas as semanas do plano, do início até a véspera da prova."
/>

{#if plano}
	<PanoramaPlano {plano} />

	<div class="page">
		{#if planoStore.erro}
			<div class="mov-aviso" role="status" aria-live="polite">
				<span>{planoStore.erro}</span>
				<button type="button" class="btn" onclick={() => (planoStore.erro = null)}>
					Dispensar
				</button>
			</div>
		{/if}

		{#each semanas as s (s.numero)}
			{@const saldo = saldoSemana(s)}
			<section class="semana">
				<!-- The week's own header. The phase used to need a separate full-width
				     banner above the card; it is a chip on this line now. -->
				<header class="sem-head">
					<h2>Semana {String(s.numero).padStart(2, '0')}</h2>
					<span class="per">
						{fc(s.dias[0].data)} – {fc(s.dias.at(-1)!.data)}
					</span>
					{#if s.mostrarFlag}
						<span class="fase" class:reta={s.fase === 'reta'}>
							{s.fase === 'reta' ? 'Reta final' : 'Ciclo de conteúdo'}
						</span>
					{/if}
					<span class="sem-bal">
						<span class="mini-bar"
							><i style="width:{saldo.alvo ? Math.min(100, (saldo.h / saldo.alvo) * 100) : 0}%"
							></i></span
						>
						<span>{nf1.format(saldo.h)} / {nf1.format(saldo.alvo)} h</span>
					</span>
				</header>

				{#if s.mostrarFlag && s.fase === 'reta'}
					<p class="fase-nota">
						Sem conteúdo novo: revisão dirigida, discursiva e simulados até a prova.
					</p>
				{/if}

				<div class="dias">
					{#each s.dias as d (d.data)}
						<DiaCard
							dia={d}
							movivel={diaMovivel(d)}
							temAntes={diaVizinho(d, -1) !== null}
							temDepois={diaVizinho(d, 1) !== null}
							onMoverAcima={moverAcima}
							onMoverAbaixo={moverAbaixo}
						/>
					{/each}
				</div>
			</section>
		{/each}
	</div>
{/if}

<style>
	.semana + .semana {
		margin-top: 26px;
	}
	.sem-head {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex-wrap: wrap;
		margin-bottom: 12px;
	}
	.sem-head h2 {
		margin: 0;
		font-size: 19px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.per {
		font-family: var(--font-mono);
		font-size: 12.5px;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
	}
	.fase {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		font-weight: 600;
		padding: 4px 9px;
		border-radius: 5px;
		background: var(--good-soft);
		color: var(--good);
	}
	.fase.reta {
		background: var(--danger-soft);
		color: var(--danger);
	}
	.fase-nota {
		margin: -4px 0 12px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.sem-bal {
		margin-left: auto;
		display: flex;
		align-items: center;
		gap: 9px;
		/* the hour readout should not jitter as it updates */
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
	}
	.mini-bar {
		width: 90px;
		height: 6px;
		border-radius: 3px;
		background: var(--bg-hover);
		overflow: hidden;
		display: block;
	}
	.mini-bar i {
		display: block;
		height: 100%;
		background: var(--accent);
		width: 0;
	}
	.dias {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	@media (max-width: 620px) {
		.sem-bal {
			margin-left: 0;
			width: 100%;
		}
	}
</style>
