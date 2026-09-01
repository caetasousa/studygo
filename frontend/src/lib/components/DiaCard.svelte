<script lang="ts">
	import DiaLog from './DiaLog.svelte';
	import IconButton from './IconButton.svelte';
	import NavIcon from './NavIcon.svelte';
	import TemaTexto from './TemaTexto.svelte';
	import AtividadeItem from './AtividadeItem.svelte';
	import { hojeISO, weekdayShort } from '$lib/format';
	import { blocoDaAtividade, planoStore } from '$lib/stores/plano.svelte';
	import { atividadeFeita } from '$lib/estudo';
	import AtividadeForm from './AtividadeForm.svelte';
	import RevisaoForm from './RevisaoForm.svelte';
	import type { Dia, ItemDia } from '$lib/types';

	const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

	/**
	 * One day of the schedule, as a card.
	 *
	 * The date is a fixed plaque on the left, the day's own actions get their own
	 * band, and the activities own the body underneath.
	 *
	 * A rest day (no items, not a review) collapses to a single quiet line
	 * instead of a full card — it carries no work to show.
	 *
	 * Rearranging is per-activity and explicit: each activity carries a "levar
	 * para o topo" button (see AtividadeItem). Drag was removed — it leaked
	 * corner cases that made the whole schedule feel flaky.
	 */
	let {
		dia,
		movivel,
		onMoverParaTopo
	}: {
		dia: Dia;
		movivel: boolean;
		/** Moves one activity to the first slot of its own day. */
		onMoverParaTopo: (id: string) => void;
	} = $props();

	const hoje = $derived(dia.data === hojeISO());
	const revisao = $derived(dia.tipo === 'rev');
	// A day the engine left empty and that is not a review is a rest day: it has
	// nothing to log and nothing to move, so it does not earn a card.
	const descanso = $derived(dia.itens.length === 0 && !revisao);

	const diaNum = $derived(Number(dia.data.slice(8, 10)));
	const mes = $derived(MESES[Number(dia.data.slice(5, 7)) - 1]);

	/** Whether one scheduled ACTIVITY is already marked done. */
	// True once the day has any per-activity record at all: from then on, the
	// absence of a block for one activity MEANS that activity is not done.
	const temRegistroPorAtividade = $derived(
		(dia.registro?.blocos ?? []).some((b) => !!b.atividadeId)
	);

	function feita(codigo: string, atividadeId: string): boolean {
		const b = blocoDaAtividade(dia.registro, { id: atividadeId, disciplina: codigo }, dia.itens);

		return atividadeFeita(b, temRegistroPorAtividade, dia.registro?.concluido);
	}

	// Only used by the special days, which still log at day level.
	let logAberto = $state(false);

	/**
	 * The day's state is DERIVED from its activities, never set directly: it is
	 * done when every activity in it is.
	 */
	const concluidoDerivado = $derived(
		dia.itens.length > 0
			? dia.itens.every((it) => feita(it.disciplina, it.id))
			: (dia.registro?.concluido ?? false)
	);

	/**
	 * A day can be pushed forward while it still has work and nothing recorded.
	 * Once something is logged the day is history, not schedule.
	 */
	const podeAdiar = $derived(
		dia.itens.length > 0 && !concluidoDerivado && !dia.registro?.horas
	);

	let adiando = $state(false);

	async function adiar() {
		if (adiando) return;

		adiando = true;
		// A refusal (dia já concluído, nada para onde empurrar…) used to be
		// discarded here — the button just did nothing, with no explanation.
		const erro = await planoStore.adiarDia(dia.data);
		if (erro) planoStore.erro = erro;
		adiando = false;
	}

	/**
	 * Brings a future activity forward to today. A refusal used to be
	 * discarded silently — the icon just did nothing, most often because today
	 * had already been fully checked off and the day itself was still (wrongly)
	 * treated as locked against new arrivals.
	 */
	async function antecipar(id: string) {
		const erro = await planoStore.anteciparAtividade(id, hojeISO());
		if (erro) planoStore.erro = erro;
	}

	/** Which activity's form is open, by activity id. */
	let editando = $state<string | null>(null);
	let salvando = $state(false);
	let erroForm = $state<string | null>(null);
	// The trigger to return focus to when the dialog closes.
	let gatilho: HTMLElement | null = null;

	const nomeDe = (codigo: string) => planoStore.discIndex[codigo]?.nome ?? codigo;

	// One review tail per day, so a boolean is enough — unlike the subject
	// activities above, which need an id to tell several open forms apart.
	let editandoRevisao = $state(false);
	let salvandoRevisao = $state(false);
	let erroRevisaoForm = $state<string | null>(null);
	let gatilhoRevisao: HTMLElement | null = null;

	async function salvarRevisao(v: { questoes: number | null; acertos: number | null; observacao: string }) {
		if (salvandoRevisao) return;

		salvandoRevisao = true;
		erroRevisaoForm = null;

		const msg = await planoStore.registrarRevisao(dia.data, v);

		salvandoRevisao = false;

		if (msg) {
			erroRevisaoForm = msg;

			return;
		}

		fecharRevisaoForm();
	}

	function fecharRevisaoForm() {
		editandoRevisao = false;
		erroRevisaoForm = null;
		salvandoRevisao = false;
		gatilhoRevisao?.focus();
		gatilhoRevisao = null;
	}

	/**
	 * Saves one activity. `salvando` gates the button so a double click cannot
	 * fire two requests, and a failure keeps the form open with the message
	 * rather than closing over a change that did not land.
	 */
	async function salvarAtividade(
		it: ItemDia,
		v: {
			horas: number | null;
			questoes: number | null;
			acertos: number | null;
			concluido: boolean;
			nota: string;
		}
	) {
		if (salvando) return;

		salvando = true;
		erroForm = null;

		// Finishing a topic scheduled for a later day records it on TODAY — the day
		// it was actually studied — and the backend then brings the activity here.
		// Recording it on the day it was planned for would say the study happened
		// in the future and leave nothing to move.
		const hoje = hojeISO();
		const quando = v.concluido && dia.data > hoje ? hoje : dia.data;

		const msg = await planoStore.salvarAtividade(quando, it.id, it.disciplina, v);

		salvando = false;

		if (msg) {
			erroForm = msg;

			return;
		}

		fecharForm();
	}

	/** Closes without persisting anything and returns focus to the icon. */
	function fecharForm() {
		editando = null;
		erroForm = null;
		salvando = false;
		gatilho?.focus();
		gatilho = null;
	}

	/**
	 * The day's review block, when the plan has one.
	 *
	 * The engine returns every block of the day; the subjects are already drawn
	 * as activities above, so only the review one is left to show here.
	 */
	/**
	 * True for the block that closes the day.
	 *
	 * Matched by prefix, not by exact title: the block names the subject it
	 * revises ("Revisão — Banco de Dados"), and an exact match silently dropped
	 * every day but the first — where it happens to read just "Revisão" because
	 * there is nothing studied yet to revise. It also shifted the content
	 * durations, since the review block was then counted as a subject block.
	 */
	const ehRevisao = (titulo: string) => titulo.startsWith('Revisão');

	const blocoRevisao = $derived(dia.blocos.find((b) => ehRevisao(b.titulo)) ?? null);

	/**
	 * The subject being revised, from the backend's own field — not the
	 * block's title. Reverse-engineering it out of "Revisão — <nome>" broke
	 * the moment two disciplines shared a display name, and the caderno link
	 * disappeared right along with it. dia.revisao is set from the plan's
	 * second study day onward, once the queue has something to name.
	 */
	const disciplinaRevisada = $derived(dia.revisao?.disciplina ?? '');
	const materiaRevisada = $derived(
		disciplinaRevisada ? (planoStore.discIndex[disciplinaRevisada]?.nome ?? disciplinaRevisada) : ''
	);
	// The subject's external error notebook, when the user set one on the
	// discipline — the review block links straight to it.
	const cadernoExterno = $derived(
		disciplinaRevisada ? (planoStore.discIndex[disciplinaRevisada]?.cadernoUrl ?? '') : ''
	);

	/**
	 * Minutes per activity, by position.
	 *
	 * The engine emits one content block per item of the day, in the same order,
	 * then the review block. Pairing by index is how the two line up — the blocks
	 * carry no activity id of their own.
	 */
	const minutosPorItem = $derived(dia.blocos.filter((b) => !ehRevisao(b.titulo)).map((b) => b.minutos));

	// The header band says what the day holds, so the count is worth stating.
	const resumoItens = $derived(
		dia.itens.length === 1 ? '1 atividade' : `${dia.itens.length} atividades`
	);
</script>

{#if descanso}
	<div class="folga">
		<span class="folga-dt">{weekdayShort(dia.data)} {String(diaNum).padStart(2, '0')}/{dia.data.slice(5, 7)}</span>
		<!-- A day the engine gave a theme says what it is. One left empty by a
		     rearrangement has none, and used to render as a bare dashed line with
		     nothing in it, which reads as a bug rather than as free time. -->
		<span class="folga-tx">
			{#if dia.tema}<TemaTexto tema={dia.tema} />{:else}nada agendado neste dia{/if}
		</span>
	</div>
{:else}
	<div class="dia" class:hoje class:revisao class:concluido={dia.registro?.concluido}>
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
					dia {dia.n}{#if dia.itens.length > 0} · {resumoItens}{/if}
				</span>
				{#if dia.itens.length === 0}
					<!-- Special days (simulado, revisão geral) have no subjects to split
					     by, so the day itself stays the unit and keeps its own control. -->
					<span class="acoes">
						<DiaLog {dia} variant="row" bind:aberto={logAberto} />
					</span>
				{:else if concluidoDerivado}
					<!-- Days with subjects carry no global control: the state is derived
					     from the activities, and this only reports it. -->
					<span class="selo-feito">Dia concluído</span>
				{/if}
			</div>

			<div class="conteudo">
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
								podeMover={movivel}
								{onMoverParaTopo}
								minutos={minutosPorItem[i] ?? null}
								concluida={feita(it.disciplina, it.id)}
								onAntecipar={(id) => void antecipar(id)}
								onRegistrar={(el) => {
									gatilho = el;
									erroForm = null;
									editando = it.id;
								}}
							/>
						{/each}

						<!-- The review row lives inside the same list as the subjects: a
						     sibling grid with its own margins never lines up, however
						     carefully its columns are copied. -->
						{#if blocoRevisao}
							<div class="atv revisao-bloco">
								<span class="alca vazia" aria-hidden="true"></span>
								<span class="min">{blocoRevisao.minutos} min</span>
								<span class="chip rev-selo">REV</span>
								<span class="txt">
									<span class="tema">{materiaRevisada || 'Revisão'}</span>
									{#if dia.revisao?.questoes != null}
										<span class="rev-resultado" title="Acertos já registrados nesta revisão">
											{dia.revisao.acertos ?? 0}/{dia.revisao.questoes}
										</span>
									{/if}
								</span>
								<span class="acoes">
									{#if dia.revisao}
										<!-- From the plan's second study day onward: the queue has
										     nothing to name before then, so there is nothing to log
										     or link to yet (see plano.FilaRevisao). -->
										<IconButton
											icon="registrar"
											label="Registrar revisão de {materiaRevisada}"
											onclick={(e) => {
												gatilhoRevisao = e.currentTarget as HTMLElement;
												erroRevisaoForm = null;
												editandoRevisao = true;
											}}
										/>
										<a
											class="rev-link"
											href="/caderno#{disciplinaRevisada}"
											title="Ver os erros de {materiaRevisada} no app"
											aria-label="Ver os erros de {materiaRevisada} no app"
										>
											<NavIcon name="caderno" size="sm" />
										</a>
										{#if cadernoExterno}
											<a
												class="rev-link"
												href={cadernoExterno}
												target="_blank"
												rel="noopener noreferrer"
												title="Abrir o caderno de erros de {materiaRevisada}"
												aria-label="Abrir o caderno de erros de {materiaRevisada} (link externo)"
											>
												<NavIcon name="link" size="sm" />
											</a>
										{/if}
									{/if}
								</span>
							</div>
						{/if}

					</div>
				{/if}

			</div>
		</div>
	</div>

	{#each dia.itens as it (it.id || it.disciplina)}
		{#if editando === it.id && it.id}
			<AtividadeForm
				item={it}
				data={dia.data}
				nome={nomeDe(it.disciplina)}
				registro={blocoDaAtividade(dia.registro, it)}
				cadernoUrl={planoStore.discIndex[it.disciplina]?.cadernoUrl ?? ''}
				{salvando}
				erro={erroForm}
				onSalvar={(v) => void salvarAtividade(it, v)}
				onSalvarCaderno={(url) => planoStore.atualizarCadernoDisciplina(it.disciplina, url)}
				onCancelar={fecharForm}
			/>
		{/if}
	{/each}

	{#if editandoRevisao && dia.revisao}
		<RevisaoForm
			data={dia.data}
			nome={materiaRevisada}
			revisao={dia.revisao}
			salvando={salvandoRevisao}
			erro={erroRevisaoForm}
			onSalvar={(v) => void salvarRevisao(v)}
			onCancelar={fecharRevisaoForm}
		/>
	{/if}
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
	.atvs {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	/* Set apart from the subjects above without competing with them: this is what
	   closes the day, not another thing to move. */
	/* Same column structure as an activity row — handle | minutes | code |
	   subject | action — so the review lines up with the subjects above it
	   instead of reading as a footnote. */
	/* The activity row's styles are scoped to AtividadeItem, so they cannot be
	   inherited here — the grid is restated with the same values. Any change to
	   AtividadeItem's own grid has to be mirrored, which is the cost of the two
	   rows living in different components. */
	.revisao-bloco {
		display: grid;
		grid-template-columns: 18px auto auto minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 6px 10px;
		padding: 7px 8px;
		border-radius: 8px;
		border-top: 1px dashed var(--border);
		margin-top: 2px;
	}
	.revisao-bloco .alca {
		width: 18px;
		height: 18px;
	}
	.revisao-bloco .min {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	/* Stands in for the discipline chip, in the review colour. */
	.revisao-bloco .chip {
		min-width: 52px;
		box-sizing: border-box;
		text-align: center;
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		padding: 3px 7px;
		border-radius: 5px;
		background: var(--warn-soft);
		color: var(--warn);
		white-space: nowrap;
	}
	.revisao-bloco .txt {
		min-width: 0;
	}
	.revisao-bloco .tema {
		font-size: 14px;
		line-height: 1.5;
		color: var(--text);
	}
	.rev-resultado {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		margin-left: 8px;
		font-variant-numeric: tabular-nums;
	}
	.revisao-bloco .acoes {
		display: flex;
		align-items: center;
		gap: 2px;
		align-self: center;
	}
	.rev-link {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		border-radius: 8px;
		color: var(--text-muted);
	}
	.rev-link:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.rev-link:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}

	.selo-feito {
		margin-left: auto;
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		font-weight: 600;
		padding: 3px 8px;
		border-radius: 5px;
		background: var(--good-soft);
		color: var(--good);
	}
	/* Line the special day's text up with the topic column of an ordinary day:
	   8px of row padding + the 18px handle + its 10px gap. Without this, a
	   review day's text sits 28px to the left of every other day's. */
	.especial {
		padding: 8px 8px 8px 36px;
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
		/* the handle is hidden on touch, so the indent it paid for goes too */
		.especial {
			padding-left: 8px;
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
		/* Mirror AtividadeItem's narrow grid (see the note by .revisao-bloco): the
		   duration, seal and actions sit on the top line, the subject flows full
		   width beneath — otherwise the review's title wrapped one word per line
		   in a squeezed middle column. */
		.revisao-bloco {
			grid-template-columns: auto auto 1fr auto;
			grid-template-areas:
				'min chip . acoes'
				'txt txt txt txt';
			align-items: center;
			gap: 4px 8px;
		}
		.revisao-bloco .min {
			grid-area: min;
		}
		.revisao-bloco .chip {
			grid-area: chip;
		}
		.revisao-bloco .txt {
			grid-area: txt;
		}
		.revisao-bloco .acoes {
			grid-area: acoes;
			justify-self: end;
		}
	}
</style>
