<script lang="ts">
	import { untrack } from 'svelte';
	import type { Dia, RegistroBlocoInput } from '$lib/types';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { debounce, parseNum, parseInteger } from '$lib/debounce';

	let { dia, variant = 'row' }: { dia: Dia; variant?: 'row' | 'card' } = $props();

	const disc = $derived(planoStore.discIndex);

	// One line per discipline on content days; a single "day" line on the special
	// days (simulado, revisão, véspera), which have no disciplines to split by.
	// Which fields a line shows depends on how that matéria is studied: a
	// theory-only session has no questions to report, so asking for them is noise.
	const modos = $derived(planoStore.plano?.config.modos ?? {});

	// Question fields still appear on a day whose activity is questions by nature
	// (revisão por questões, simulado), whatever the matéria's own mode says.
	const diaDeQuestoes = $derived(dia.tipo === 'rev' || dia.tipo === 'sim');

	const linhas = $derived(
		dia.itens.length > 0
			? dia.itens.map((it) => ({
					codigo: it.disciplina,
					nome: disc[it.disciplina]?.nome ?? it.disciplina,
					cor: disc[it.disciplina]?.cor ?? 0,
					questoes: diaDeQuestoes || (modos[it.disciplina] ?? 'completo') !== 'teoria'
				}))
			: [{ codigo: '', nome: 'O dia', cor: -1, questoes: true }]
	);

	const algumaComQuestoes = $derived(linhas.some((l) => l.questoes));

	interface Campos {
		horas: number | null;
		questoes: number | null;
		acertos: number | null;
	}

	let valores = $state<Record<string, Campos>>({});
	let concluido = $state(false);
	let nota = $state('');
	let aberto = $state(false);

	// Re-sync from the server only when the component switches to a different
	// day — never mid-edit, so a debounced save in flight can't clobber newer
	// keystrokes.
	let carregadoDe = $state('');
	$effect(() => {
		if (dia.data === carregadoDe) return;
		carregadoDe = dia.data;
		untrack(() => {
			const reg = dia.registro;
			const novo: Record<string, Campos> = {};

			if (dia.itens.length === 0) {
				novo[''] = {
					horas: reg?.horas ?? null,
					questoes: reg?.questoes ?? null,
					acertos: reg?.acertos ?? null
				};
			} else {
				for (const it of dia.itens) {
					const b = reg?.blocos?.find((x) => x.disciplina === it.disciplina);
					novo[it.disciplina] = {
						horas: b?.horas ?? null,
						questoes: b?.questoes ?? null,
						acertos: b?.acertos ?? null
					};
				}
			}

			valores = novo;
			concluido = reg?.concluido ?? false;
			nota = reg?.nota ?? '';
			aberto = false;
		});
	});

	function campo(codigo: string): Campos {
		return valores[codigo] ?? { horas: null, questoes: null, acertos: null };
	}

	function soma(chave: keyof Campos): number | null {
		let total: number | null = null;
		for (const l of linhas) {
			const v = campo(l.codigo)[chave];
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

	const salvar = debounce(() => {
		const porDisciplina = dia.itens.length > 0;

		const blocos: RegistroBlocoInput[] = porDisciplina
			? linhas
					.map((l) => ({ disciplina: l.codigo, ...campo(l.codigo), nota: '' }))
					.filter((b) => b.horas !== null || b.questoes !== null || b.acertos !== null)
			: [];

		const dia0 = campo('');

		void planoStore.registrarDia(dia.data, {
			// Com blocos, o servidor recalcula os totais do dia a partir deles.
			horas: porDisciplina ? horasTotal : dia0.horas,
			questoes: porDisciplina ? questoesTotal : dia0.questoes,
			acertos: porDisciplina ? acertosTotal : dia0.acertos,
			concluido,
			nota,
			blocos
		});
	}, 450);

	function setCampo(codigo: string, chave: keyof Campos, bruto: string) {
		const v = chave === 'horas' ? parseNum(bruto) : parseInteger(bruto);
		valores[codigo] = { ...campo(codigo), [chave]: v };
		if (chave === 'horas' && v && !concluido) concluido = true;
		salvar();
	}

	function onConcluido(e: Event) {
		concluido = (e.target as HTMLInputElement).checked;
		if (concluido && horasTotal === null) {
			const padrao = planoStore.plano?.config.horasDia ?? null;
			if (padrao !== null && linhas.length > 0) {
				const fatia = Math.round((padrao / linhas.length) * 100) / 100;
				for (const l of linhas) valores[l.codigo] = { ...campo(l.codigo), horas: fatia };
			}
		}
		salvar();
	}

	function onNota(e: Event) {
		nota = (e.target as HTMLInputElement).value;
		salvar();
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
		{#each linhas as l (l.codigo)}
			{@const c = campo(l.codigo)}
			{@const err = errosDe(c)}
			<div class="bl">
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
						oninput={(e) => setCampo(l.codigo, 'horas', e.currentTarget.value)}
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
							oninput={(e) => setCampo(l.codigo, 'questoes', e.currentTarget.value)}
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
							oninput={(e) => setCampo(l.codigo, 'acertos', e.currentTarget.value)}
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
	<button
		type="button"
		class="resumo"
		class:filled={resumo !== ''}
		aria-expanded={aberto}
		title={resumo ? 'Editar o que você estudou neste dia' : 'Registrar o que você estudou neste dia'}
		onclick={() => (aberto = !aberto)}
	>
		{resumo || 'registrar'}
	</button>
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
		grid-template-columns: minmax(0, 1fr) 84px 84px 84px 74px;
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

	/* "Registrar" is the main action of a day, so it looks like a button instead
	   of faint monospaced text that reads as a label. */
	.resumo {
		font-family: inherit;
		font-size: 12.5px;
		font-weight: 500;
		text-align: center;
		padding: 6px 12px;
		min-height: 32px;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 7px;
		color: var(--text-muted);
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.resumo:hover,
	.resumo:focus-visible {
		background: var(--bg-soft);
		border-color: var(--border);
		color: var(--text);
	}
	.resumo.filled {
		color: var(--good);
		font-weight: 600;
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
