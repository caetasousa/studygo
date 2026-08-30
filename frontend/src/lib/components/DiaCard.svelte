<script lang="ts">
	import DiaLog from './DiaLog.svelte';
	import IconButton from './IconButton.svelte';
	import TemaTexto from './TemaTexto.svelte';
	import AtividadeItem from './AtividadeItem.svelte';
	import { hojeISO, weekdayShort } from '$lib/format';
	import type { Dia } from '$lib/types';

	const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

	/**
	 * One day of the schedule, as a card.
	 *
	 * The date is a fixed plaque on the left, the day's own actions get their own
	 * band, and the activities own the body underneath. Before this, all three
	 * competed for the same line and the topic text lost.
	 *
	 * A rest day (no items, not a review) collapses to a single quiet line
	 * instead of a full card — it carries no work to show.
	 */
	let {
		dia,
		movivel,
		datasDisponiveis,
		temAnterior,
		temProximo,
		onMover,
		onTrocar,
		onArrastarDia,
		onLargarDia,
		onSoltarDia,
		onArrastarAtv,
		onSoltarAtv,
		arrastandoDia
	}: {
		dia: Dia;
		movivel: boolean;
		datasDisponiveis: string[];
		temAnterior: boolean;
		temProximo: boolean;
		onMover: (id: string, data: string, posicao: number) => void;
		onTrocar: (dir: -1 | 1) => void;
		onArrastarDia: () => void;
		/** drag finished without a drop target accepting it */
		onLargarDia: () => void;
		onSoltarDia: () => void;
		onArrastarAtv: (id: string) => void;
		onSoltarAtv: (posicao: number) => void;
		/** date being dragged, so this card can show itself as a drop target */
		arrastandoDia: string | null;
	} = $props();

	const hoje = $derived(dia.data === hojeISO());
	const revisao = $derived(dia.tipo === 'rev');
	// A day the engine left empty and that is not a review is a rest day: it has
	// nothing to log and nothing to move, so it does not earn a card.
	const descanso = $derived(dia.itens.length === 0 && !revisao);
	const alvoDrop = $derived(!!arrastandoDia && arrastandoDia !== dia.data && movivel);

	const diaNum = $derived(Number(dia.data.slice(8, 10)));
	const mes = $derived(MESES[Number(dia.data.slice(5, 7)) - 1]);

	// The header band says what the day holds, so the count is worth stating.
	const resumoItens = $derived(
		dia.itens.length === 1 ? '1 atividade' : `${dia.itens.length} atividades`
	);
</script>

{#if descanso}
	<div class="folga">
		<span class="folga-dt">{weekdayShort(dia.data)} {String(diaNum).padStart(2, '0')}/{dia.data.slice(5, 7)}</span>
		<span class="folga-tx"><TemaTexto tema={dia.tema} /></span>
	</div>
{:else}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="dia"
		class:hoje
		class:revisao
		class:concluido={dia.registro?.concluido}
		class:alvo={alvoDrop}
		ondragover={(e) => movivel && e.preventDefault()}
		ondrop={(e) => {
			e.preventDefault();
			onSoltarDia();
		}}
	>
		<!-- The date plaque: weekday, number, month — read top to bottom, fixed
		     width, so every card's content starts at the same x. -->
		<div class="placa">
			<span class="placa-wd">{weekdayShort(dia.data)}{#if hoje} · hoje{/if}</span>
			<b class="placa-dia">{String(diaNum).padStart(2, '0')}</b>
			<span class="placa-mes">{mes}</span>
		</div>

		<div class="corpo">
			<div class="faixa">
				<span class="faixa-n">
					dia {dia.n}{#if !revisao} · {resumoItens}{/if}
					{#if dia.reordenado}
						<span class="reord" title="Reorganizado manualmente">•</span>
					{/if}
				</span>
				{#if revisao}
					<span class="selo">Revisão semanal</span>
				{/if}
				<span class="acoes">
					<DiaLog {dia} variant="row" />
					{#if movivel}
						<IconButton
							icon="anterior"
							label="Trocar este dia com o anterior"
							disabled={!temAnterior}
							onclick={() => onTrocar(-1)}
						/>
						<IconButton
							icon="proximo"
							label="Trocar este dia com o próximo"
							disabled={!temProximo}
							onclick={() => onTrocar(1)}
						/>
					{/if}
				</span>
			</div>

			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="conteudo"
				draggable={movivel}
				ondragstart={onArrastarDia}
				ondragend={onLargarDia}
			>
				{#if dia.itens.length === 0}
					<div class="especial">
						<span class="tema-txt">
							<TemaTexto tema={dia.tema} />{#if revisao && dia.meta > 0}<em>{dia.meta} questões</em
								>{/if}
						</span>
					</div>
				{:else}
					<div class="atvs" role="list">
						{#each dia.itens as it, i (it.id || i)}
							<AtividadeItem
								item={it}
								data={dia.data}
								indice={i}
								total={dia.itens.length}
								podeMover={movivel}
								{datasDisponiveis}
								{onMover}
								onArrastar={onArrastarAtv}
								onSoltar={onSoltarAtv}
							/>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}

<style>
	.dia {
		display: grid;
		grid-template-columns: 78px minmax(0, 1fr);
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
	}
	.dia.hoje {
		border-color: var(--accent);
	}
	.dia.alvo {
		outline: 2px dashed var(--accent);
		outline-offset: -2px;
	}

	/* ---- date plaque ---- */
	.placa {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1px;
		padding: 16px 8px;
		background: var(--bg-soft);
		border-right: 1px solid var(--border);
	}
	/* Colour marks the day's nature here, on the plaque, instead of flooding the
	   whole row — which is what made a week of reviews read as one amber block. */
	.dia.hoje .placa {
		background: var(--accent-soft);
	}
	.dia.revisao .placa {
		background: var(--warn-soft);
	}
	.placa-wd {
		font-family: var(--font-mono);
		font-size: 10.5px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		font-weight: 600;
		color: var(--text-muted);
		text-align: center;
	}
	.dia.hoje .placa-wd {
		color: var(--accent);
	}
	.dia.revisao .placa-wd {
		color: var(--warn);
	}
	.placa-dia {
		font-size: 27px;
		font-weight: 800;
		line-height: 1.05;
		color: var(--text);
		font-variant-numeric: tabular-nums;
	}
	.placa-mes {
		font-size: 12px;
		color: var(--text-muted);
	}

	/* ---- body ---- */
	.corpo {
		min-width: 0;
	}
	.faixa {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border);
	}
	.dia.hoje .faixa {
		background: var(--bg-soft);
	}
	.faixa-n {
		font-family: var(--font-mono);
		font-size: 11px;
		letter-spacing: 0.04em;
		color: var(--text-faint);
	}
	.reord {
		color: var(--accent);
		font-size: 14px;
		line-height: 1;
	}
	.selo {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		font-weight: 600;
		padding: 3px 8px;
		border-radius: 5px;
		background: var(--warn-soft);
		color: var(--warn);
	}
	.acoes {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-left: auto;
		flex-wrap: wrap;
	}
	.conteudo {
		padding: 4px 6px;
	}
	.conteudo[draggable='true'] {
		cursor: grab;
	}
	.conteudo[draggable='true']:active {
		cursor: grabbing;
	}
	.atvs {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.especial {
		padding: 8px;
	}
	.tema-txt {
		font-size: 14px;
		line-height: 1.5;
	}
	.tema-txt em {
		font-style: normal;
		font-family: var(--font-mono);
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--accent);
		margin-left: 8px;
	}
	.dia.concluido .tema-txt {
		color: var(--text-faint);
		text-decoration: line-through;
		text-decoration-color: var(--border-strong);
	}

	/* ---- rest day ---- */
	.folga {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 9px 16px;
		border: 1px dashed var(--border);
		border-radius: 10px;
		color: var(--text-faint);
	}
	.folga-dt {
		font-family: var(--font-mono);
		font-size: 12px;
		letter-spacing: 0.03em;
		font-variant-numeric: tabular-nums;
	}
	.folga-tx {
		font-size: 13px;
	}

	/* On a phone the plaque becomes a header strip: a 78px column would leave the
	   topic text a sliver. */
	@media (max-width: 620px) {
		.dia {
			grid-template-columns: 1fr;
		}
		.placa {
			flex-direction: row;
			align-items: baseline;
			justify-content: flex-start;
			gap: 7px;
			padding: 9px 12px;
			border-right: 0;
			border-bottom: 1px solid var(--border);
		}
		.placa-dia {
			font-size: 19px;
		}
	}
</style>
