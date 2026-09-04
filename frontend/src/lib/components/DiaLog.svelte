<script lang="ts">
	import { untrack } from 'svelte';
	import type { Dia } from '$lib/types';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { debounce, parseNum, parseInteger } from '$lib/debounce';
	import IconButton from './IconButton.svelte';

	let {
		dia,
		variant = 'row',
		aberto = $bindable(false)
	}: {
		dia: Dia;
		variant?: 'row' | 'card';
		/** Bindable so an activity's own "registrar" icon can open this panel. */
		aberto?: boolean;
	} = $props();

	const disc = $derived(planoStore.discIndex);

	// One line per discipline on content days; a single "day" line on the special
	// days (simulado, revisão, véspera), which have no disciplines to split by.
	// Which fields a line shows depends on how that matéria is studied: a
	// theory-only session has no questions to report, so asking for them is noise.
	const modos = $derived(planoStore.plano?.config.modos ?? {});

	// Question fields still appear on a day whose activity is questions by nature
	// (revisão por questões, simulado), whatever the matéria's own mode says.
	const diaDeQuestoes = $derived(dia.tipo === 'rev' || dia.tipo === 'sim');

	// Uma linha por ATIVIDADE — nunca por disciplina: um dia que agenda a mesma
	// matéria duas vezes tem duas linhas independentes, e a chave é o id.
	const linhas = $derived(
		dia.itens.map((it) => ({
			id: it.id,
			codigo: it.disciplina,
			nome: it.disciplina ? (disc[it.disciplina]?.nome ?? it.disciplina) : 'O dia',
			cor: it.disciplina ? (disc[it.disciplina]?.cor ?? 0) : -1,
			questoes: diaDeQuestoes || (modos[it.disciplina] ?? 'completo') !== 'teoria'
		}))
	);

	const algumaComQuestoes = $derived(linhas.some((l) => l.questoes));

	interface Campos {
		horas: number | null;
		questoes: number | null;
		acertos: number | null;
	}

	// Tudo indexado pelo id da ATIVIDADE.
	let valores = $state<Record<string, Campos>>({});
	let feitos = $state<Record<string, boolean>>({});
	let nota = $state('');

	// A conclusão do dia é DERIVADA das atividades — o servidor a calcula, e aqui
	// ela só é exibida.
	const concluido = $derived(linhas.length > 0 && linhas.every((l) => feitos[l.id]));

	// Re-sync from the server only when the component switches to a different
	// day — never mid-edit, so a debounced save in flight can't clobber newer
	// keystrokes.
	let carregadoDe = $state('');
	$effect(() => {
		if (dia.data === carregadoDe) return;
		carregadoDe = dia.data;
		untrack(() => {
			const novo: Record<string, Campos> = {};
			const novosFeitos: Record<string, boolean> = {};

			for (const it of dia.itens) {
				novo[it.id] = {
					horas: it.horas,
					questoes: it.questoes,
					acertos: it.acertos
				};
				novosFeitos[it.id] = it.concluido;
			}

			valores = novo;
			feitos = novosFeitos;
			nota = dia.nota;
			aberto = false;
		});
	});

	function campo(id: string): Campos {
		return valores[id] ?? { horas: null, questoes: null, acertos: null };
	}

	function soma(chave: keyof Campos): number | null {
		let total: number | null = null;
		for (const l of linhas) {
			const v = campo(l.id)[chave];
			if (v !== null) total = (total ?? 0) + v;
		}
		return total;
	}

	const horasTotal = $derived(soma('horas'));
	const questoesTotal = $derived(soma('questoes'));
	const acertosTotal = $derived(soma('acertos'));
	const errosTotal = $derived(
		questoesTotal !== null && acertosTotal !== null
			? Math.max(0, questoesTotal - acertosTotal)
			: null
	);

	function errosDe(c: Campos): number | null {
		return c.questoes !== null && c.acertos !== null
			? Math.max(0, c.questoes - c.acertos)
			: null;
	}

	// Só as atividades realmente alteradas são enviadas: a API é por atividade,
	// então reenviar o dia inteiro a cada tecla seria trabalho (e risco) à toa.
	let sujas = $state<Set<string>>(new Set());

	const salvar = debounce(() => {
		const pendentes = [...sujas];
		sujas = new Set();

		for (const id of pendentes) {
			const l = linhas.find((x) => x.id === id);
			if (!l) continue;

			void planoStore.salvarAtividade(id, {
				...campo(id),
				concluido: feitos[id] ?? false,
				nota: ''
			});
		}
	}, 450);

	// A nota é do DIA, não de uma atividade, e por isso vai por outra rota.
	const salvarNota = debounce(() => {
		void planoStore.registrarDia(dia.data, {
			nota,
			questoes: dia.revisao?.questoes ?? null,
			acertos: dia.revisao?.acertos ?? null,
			observacao: dia.revisao?.observacao ?? ''
		});
	}, 450);

	function setCampo(id: string, chave: keyof Campos, bruto: string) {
		const v = chave === 'horas' ? parseNum(bruto) : parseInteger(bruto);
		valores[id] = { ...campo(id), [chave]: v };
		// Lançar horas numa matéria implica que você a estudou.
		if (chave === 'horas' && v && !feitos[id]) feitos[id] = true;
		sujas.add(id);
		salvar();
	}

	// What ticking a discipline filled in on its own, so unticking can take back
	// exactly that and leave anything typed by hand alone.
	let autoPreenchido = $state<Record<string, boolean>>({});

	/**
	 * Toggles one discipline. Ticking it fills in the default hours when none
	 * were given; unticking puts it back the way it was, rather than leaving
	 * hours behind that the user never typed.
	 */
	function alternarAtividade(id: string, marcado: boolean) {
		feitos[id] = marcado;

		if (marcado) {
			if (campo(id).horas === null) {
				const padrao = planoStore.plano?.config.horasDia ?? null;
				if (padrao !== null && linhas.length > 0) {
					const fatia = Math.round((padrao / linhas.length) * 100) / 100;
					valores[id] = { ...campo(id), horas: fatia };
					autoPreenchido[id] = true;
				}
			}
		} else if (autoPreenchido[id]) {
			valores[id] = { ...campo(id), horas: null };
			autoPreenchido[id] = false;
		}

		sujas.add(id);
		salvar();
	}

	// O checkbox do dia inteiro liga todas as atividades de uma vez, para que as
	// duas visões nunca discordem.
	function onConcluido(e: Event) {
		const marcado = (e.target as HTMLInputElement).checked;
		for (const l of linhas) alternarAtividade(l.id, marcado);
	}

	function onNota(e: Event) {
		nota = (e.target as HTMLInputElement).value;
		salvarNota();
	}

	const resumo = $derived(
		[
			horasTotal !== null ? `${horasTotal}h` : null,
			questoesTotal !== null ? `${questoesTotal}q` : null,
			acertosTotal !== null ? `${acertosTotal}✓` : null,
			errosTotal ? `${errosTotal}✗` : null
		]
			.filter(Boolean)
			.join(' · ')
	);
</script>

{#snippet painel()}
	<div class="blocos">
		{#each linhas as l (l.id)}
			{@const c = campo(l.id)}
			{@const err = errosDe(c)}
			<div class="bl" class:feito={feitos[l.id]}>
				<label class="bl-ok" title="Marcar {l.nome} como concluída">
					<input
						type="checkbox"
						class="checkbox"
						checked={feitos[l.id] ?? false}
						onchange={(e) => alternarAtividade(l.id, e.currentTarget.checked)}
					/>
					<span class="sr-only">Concluí {l.nome}</span>
				</label>
				<span class="bl-nome">
					{#if l.cor >= 0}
						<span class="chip-dot" style="background:var(--c{l.cor}-tx)"></span>
					{/if}
					{l.nome}
				</span>
				<label class="bl-in">
					<span>horas</span>
					<input
						type="number"
						min="0"
						max="24"
						step="0.25"
						placeholder="0,00"
						value={c.horas ?? ''}
						oninput={(e) => setCampo(l.id, 'horas', e.currentTarget.value)}
					/>
				</label>
				{#if l.questoes}
					<label class="bl-in">
						<span>questões</span>
						<input
							type="number"
							min="0"
							step="1"
							placeholder={linhas.length > 1
								? String(Math.round(dia.meta / linhas.length))
								: String(dia.meta)}
							value={c.questoes ?? ''}
							oninput={(e) => setCampo(l.id, 'questoes', e.currentTarget.value)}
						/>
					</label>
					<label class="bl-in">
						<span>acertos</span>
						<input
							type="number"
							min="0"
							step="1"
							placeholder="0"
							max={c.questoes ?? undefined}
							value={c.acertos ?? ''}
							oninput={(e) => setCampo(l.id, 'acertos', e.currentTarget.value)}
						/>
					</label>
					<span class="bl-err" class:vazio={err === null}>
						{#if err !== null}
							<b>{err}</b> {err === 1 ? 'erro' : 'erros'}
						{:else}
							—
						{/if}
					</span>
				{:else}
					<span class="bl-so-teoria">só teoria</span>
				{/if}
			</div>
		{/each}

		{#if linhas.length > 1}
			<div class="bl total">
				<span aria-hidden="true"></span>
				<span class="bl-nome">Total do dia</span>
				<span class="bl-v">{horasTotal ?? '—'}<i>h</i></span>
				{#if algumaComQuestoes}
					<span class="bl-v">{questoesTotal ?? '—'}<i>q</i></span>
					<span class="bl-v">{acertosTotal ?? '—'}<i>✓</i></span>
					<span class="bl-err" class:vazio={errosTotal === null}>
						{#if errosTotal !== null}<b>{errosTotal}</b>
							{errosTotal === 1 ? 'erro' : 'erros'}{:else}—{/if}
					</span>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}

{#if variant === 'card'}
	{@render painel()}
	<div class="lanc-fim">
		<label class="ok-lbl">
			<input type="checkbox" class="checkbox" checked={concluido} onchange={onConcluido} />
			Dia concluído
		</label>
	</div>
{:else}
	<!-- Same register icon the activities use, so the control means one thing
	     across the schedule. What was recorded shows beside it when there is
	     something to show, instead of the word "registrar". -->
	{#if resumo !== ''}
		<span class="resumo filled">{resumo}</span>
	{/if}
	<IconButton
		icon="registrar"
		label={resumo ? 'Editar o que você estudou neste dia' : 'Registrar o que você estudou neste dia'}
		onclick={() => (aberto = !aberto)}
	/>
	<input
		type="checkbox"
		class="checkbox rowchk"
		aria-label="Concluído"
		checked={concluido}
		onchange={onConcluido}
	/>
	{#if aberto}
		<div class="painel-row">{@render painel()}</div>
	{/if}
	<span class="note-row" class:tem-nota={nota !== '' || aberto}>
		<input
			type="text"
			placeholder="Anotação: dúvidas, questões erradas, o que revisar…"
			value={nota}
			oninput={onNota}
		/>
	</span>
{/if}

<style>
	.bl-ok {
		display: grid;
		place-items: center;
		flex: none;
		cursor: pointer;
	}
	/* A finished discipline stays legible but visibly settled. */
	.bl.feito .bl-nome {
		color: var(--text-faint);
	}

	/* The note lives under the day's controls: hidden until the day carries one or
	   the panel is open, so an empty schedule is not a wall of empty inputs.
	   This used to be driven by the page's `.row` wrapper, which no longer exists —
	   the reveal belongs to the component that owns the input. */
	.note-row {
		display: none;
		flex-basis: 100%;
		padding-top: 4px;
	}
	.note-row.tem-nota,
	.note-row:focus-within {
		display: block;
	}
	.note-row input {
		width: 100%;
		background: transparent;
		border: 0;
		border-bottom: 1px dotted var(--border-strong);
		border-radius: 0;
		font-family: var(--font-ui);
		font-size: 12.5px;
		color: var(--text-muted);
		padding: 3px 2px;
	}
	.blocos {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.bl {
		display: grid;
		/* check | discipline | horas | questões | acertos | erros */
		grid-template-columns: 22px minmax(0, 1fr) 84px 84px 84px 74px;
		align-items: end;
		gap: 8px;
	}
	.bl-nome {
		font-size: 13px;
		font-weight: 500;
		line-height: 1.3;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		padding-bottom: 6px;
	}
	.bl-in {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.bl-in span {
		font-family: var(--font-mono);
		font-size: 9.5px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.bl-in input {
		width: 100%;
		text-align: right;
		padding: 5px 6px;
		font-size: 12.5px;
	}
	.bl-err {
		font-size: 11.5px;
		color: var(--danger);
		text-align: right;
		padding-bottom: 6px;
		white-space: nowrap;
	}
	.bl-so-teoria {
		grid-column: span 3;
		font-size: 11px;
		color: var(--text-faint);
		font-style: italic;
		align-self: center;
	}
	.bl-err b {
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.bl-err.vazio {
		color: var(--text-faint);
	}
	.bl.total {
		border-top: 1px solid var(--border);
		padding-top: 6px;
		margin-top: 2px;
		align-items: center;
	}
	.bl.total .bl-nome {
		color: var(--text-muted);
		padding-bottom: 0;
	}
	.bl.total .bl-err {
		padding-bottom: 0;
	}
	.bl-v {
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 600;
		text-align: right;
		padding-right: 6px;
	}
	.bl-v i {
		font-style: normal;
		font-size: 10px;
		color: var(--text-faint);
		margin-left: 2px;
	}

	.lanc-fim {
		margin-top: 12px;
		padding-top: 10px;
		border-top: 1px solid var(--border);
	}
	.ok-lbl {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13.5px;
		cursor: pointer;
	}

	/* What was recorded, shown beside the icon. It is a readout now, not the
	   control — the icon is the control. */
	.resumo {
		font-family: var(--font-mono);
		font-size: 11.5px;
		font-variant-numeric: tabular-nums;
		color: var(--good);
		font-weight: 600;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* The day is a flex block now, so the log panel simply takes the full width
	   under the activities instead of being placed into a grid area. */
	.painel-row {
		width: 100%;
		padding: 8px 0 4px;
	}

	@media (max-width: 720px) {
		.bl {
			grid-template-columns: minmax(0, 1fr) 1fr 1fr;
			gap: 6px;
		}
		.bl-nome {
			grid-column: 1 / -1;
			padding-bottom: 0;
		}
		.bl-err {
			text-align: left;
			padding-bottom: 0;
		}
	}
</style>
