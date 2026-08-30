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

	function diaMovivel(d: Dia): boolean {
		return d.itens.length > 0 && !d.registro?.concluido;
	}

	function vizinho(d: Dia, dir: -1 | 1): Dia | null {
		if (!plano) return null;
		const i = plano.dias.findIndex((x) => x.data === d.data);
		for (let j = i + dir; j >= 0 && j < plano.dias.length; j += dir) {
			if (diaMovivel(plano.dias[j])) return plano.dias[j];
		}
		return null;
	}

	function trocar(d: Dia, dir: -1 | 1) {
		const alvo = vizinho(d, dir);
		if (alvo) void planoStore.reordenar(d.data, alvo.data);
	}

	let arrastando = $state<string | null>(null);

	// --- individual activity moves ---
	let arrastandoAtv = $state<string | null>(null);
	let ultimoMovimento = $state<{ id: string; data: string; posicao: number } | null>(null);
	let aviso = $state<string | null>(null);

	// Days that can receive an activity: the ones the engine filled with content.
	const datasDisponiveis = $derived(
		(plano?.dias ?? []).filter((d) => d.itens.length > 0).map((d) => d.data)
	);

	function posicaoAtual(id: string): { data: string; posicao: number } | null {
		for (const d of plano?.dias ?? []) {
			const i = d.itens.findIndex((x) => x.id === id);
			if (i >= 0) return { data: d.data, posicao: i };
		}
		return null;
	}

	async function mover(id: string, data: string, posicao: number) {
		const antes = posicaoAtual(id);
		const ok = await planoStore.moverAtividade(id, data, posicao);
		if (ok && antes) {
			ultimoMovimento = { id, ...antes };
			aviso = 'Atividade movida.';
		} else if (!ok) {
			// planoStore.erro already carries the reason; the plan itself is untouched.
			aviso = null;
		}
	}

	async function desfazer() {
		if (!ultimoMovimento) return;
		const { id, data, posicao } = ultimoMovimento;
		ultimoMovimento = null;
		aviso = null;
		await planoStore.moverAtividade(id, data, posicao);
	}

	function soltarAtv(data: string, posicao: number) {
		if (!arrastandoAtv) return;
		const id = arrastandoAtv;
		arrastandoAtv = null;
		void mover(id, data, posicao);
	}

	function onDrop(destino: Dia) {
		if (arrastando && arrastando !== destino.data && diaMovivel(destino)) {
			void planoStore.reordenar(arrastando, destino.data);
		}
		arrastando = null;
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
		{#if aviso || planoStore.erro}
			<div class="mov-aviso" role="status" aria-live="polite">
				<span>{planoStore.erro ?? aviso}</span>
				{#if ultimoMovimento && !planoStore.erro}
					<button type="button" class="btn" onclick={desfazer}>Desfazer</button>
				{/if}
				<button
					type="button"
					class="btn"
					onclick={() => {
						aviso = null;
						planoStore.erro = null;
					}}>Dispensar</button
				>
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
							temAnterior={!!vizinho(d, -1)}
							temProximo={!!vizinho(d, 1)}
							arrastandoDia={arrastando}
							onMover={mover}
							onTrocar={(dir) => trocar(d, dir)}
							onArrastarDia={() => (arrastando = d.data)}
							onLargarDia={() => (arrastando = null)}
							onSoltarDia={() => onDrop(d)}
							onArrastarAtv={(id) => (arrastandoAtv = id)}
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
