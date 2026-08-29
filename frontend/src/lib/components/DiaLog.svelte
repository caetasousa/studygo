<script lang="ts">
	import { untrack } from 'svelte';
	import type { Dia, RegistroBlocoInput } from '$lib/types';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { debounce, parseNum, parseInteger } from '$lib/debounce';

	let { dia, variant = 'row' }: { dia: Dia; variant?: 'row' | 'card' } = $props();

	const disc = $derived(planoStore.discIndex);

	// One line per discipline on content days; a single "day" line on the special
	// days (simulado, revisão, véspera), which have no disciplines to split by.
	const linhas = $derived(
		dia.itens.length > 0
			? dia.itens.map((it) => ({
					codigo: it.disciplina,
					nome: disc[it.disciplina]?.nome ?? it.disciplina,
					cor: disc[it.disciplina]?.cor ?? 0
				}))
			: [{ codigo: '', nome: 'O dia', cor: -1 }]
	);

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
				<label class="bl-in">
					<span>questões</span>
					<input
						type="number"
						min="0"
						step="1"
						placeholder={linhas.length > 1 ? String(Math.round(dia.meta / linhas.length)) : String(dia.meta)}
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
			</div>
		{/each}

		{#if linhas.length > 1}
			<div class="bl total">
				<span class="bl-nome">Total do dia</span>
				<span class="bl-v">{horasTotal ?? '—'}<i>h</i></span>
				<span class="bl-v">{questoesTotal ?? '—'}<i>q</i></span>
				<span class="bl-v">{acertosTotal ?? '—'}<i>✓</i></span>
				<span class="bl-err" class:vazio={errosTotal === null}>
					{#if errosTotal !== null}<b>{errosTotal}</b> {errosTotal === 1 ? 'erro' : 'erros'}{:else}—{/if}
				</span>
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
		onclick={() => (aberto = !aberto)}
	>
		{resumo || 'lançar'}
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
	<span class="note-row">
		<input
			type="text"
			placeholder="Anotação: dúvidas, questões erradas, o que revisar…"
			value={nota}
			oninput={onNota}
		/>
	</span>
{/if}

<style>
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

	.resumo {
		grid-area: res;
		grid-column: 3 / 5;
		grid-row: 1;
		font-family: var(--font-mono);
		font-size: 11.5px;
		text-align: right;
		padding: 5px 7px;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 5px;
		color: var(--text-faint);
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
	.painel-row {
		grid-area: painel;
		grid-column: 1 / -1;
		padding: 8px 0 4px;
	}

	/* Abaixo de 900px a .row vira grid-template-areas (app.css) — as posições
	   explícitas de coluna do desktop precisam sair do caminho. */
	@media (max-width: 900px) {
		.resumo {
			grid-column: auto;
			grid-row: auto;
			text-align: left;
		}
		.painel-row {
			grid-column: auto;
		}
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
