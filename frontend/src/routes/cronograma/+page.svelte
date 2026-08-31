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
	 * The only thing that blocks a move is having been marked done — moving a
	 * concluded activity would rewrite history. Everything else is movable.
	 */
	function diaMovivel(d: Dia): boolean {
		return d.itens.length > 0;
	}

	// Rearranging is per-activity: a whole day is never swapped with another, so
	// there is no day-level drag or swap here — only AtividadeItem moves.
	let arrastandoAtv = $state<string | null>(null);
	// Slot a touch drag is hovering, so the target row can show "trocar" without
	// the dragover events that touch never fires.
	let sobrevoo = $state<{ data: string; posicao: number } | null>(null);

	// Days that can receive an activity: the ones the engine filled with content.
	const datasDisponiveis = $derived(
		(plano?.dias ?? []).filter((d) => d.itens.length > 0).map((d) => d.data)
	);

	/** The date an activity currently sits on, or null if it is not in the plan. */
	function dataAtual(id: string): string | null {
		for (const d of plano?.dias ?? []) {
			if (d.itens.some((x) => x.id === id)) return d.data;
		}

		return null;
	}

	/**
	 * Moves one activity. When the target slot in another day is already taken,
	 * the two subjects swap places instead of the target day growing — which is
	 * what "move this to the 2nd" means when the 2nd is already full.
	 */
	async function mover(id: string, data: string, posicao: number) {
		const origem = dataAtual(id);
		const destino = plano?.dias.find((d) => d.data === data);

		// An occupied slot swaps the two activities; an empty one is a plain move.
		// Same-day reordering always inserts, since there is no second day to
		// exchange with.
		const ocupado = !!destino && posicao < destino.itens.length;
		const trocar = ocupado && origem !== data;

		// A move that worked needs no announcement — the board shows it. Only a
		// refusal does, and planoStore.erro carries that.
		await planoStore.moverAtividade(id, data, posicao, trocar);
	}

	function soltarAtv(data: string, posicao: number) {
		if (!arrastandoAtv) return;
		const id = arrastandoAtv;
		arrastandoAtv = null;
		void mover(id, data, posicao);
	}

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
							{datasDisponiveis}
							onMover={mover}
							arrastandoAlgo={arrastandoAtv !== null}
							onArrastarAtv={(id) => (arrastandoAtv = id)}
							onLargarAtv={() => {
								arrastandoAtv = null;
								sobrevoo = null;
							}}
							onSobrevoar={(alvo) => (sobrevoo = alvo)}
							{sobrevoo}
							onSoltarAtv={(pos) => soltarAtv(d.data, pos)}
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
