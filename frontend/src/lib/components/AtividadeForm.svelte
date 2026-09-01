<script lang="ts">
	import { untrack } from 'svelte';
	import { fl } from '$lib/format';
	import { valoresIniciais, valoresInvalidos } from '$lib/estudo';
	import type { ItemDia, RegistroBloco } from '$lib/types';

	/**
	 * Logs ONE scheduled activity — not the day.
	 *
	 * The day-level form asked for every subject at once, so recording one meant
	 * looking past the others. This shows only the activity you opened it from,
	 * and saves only that activity: the other subjects of the day are neither
	 * displayed nor written.
	 *
	 * Edits are local until Salvar. Cancelling (button, Escape, backdrop, close)
	 * discards them without touching the store or the API, so an abandoned edit
	 * can never leak into the schedule.
	 */
	let {
		item,
		data,
		nome,
		registro,
		cadernoUrl = '',
		salvando = false,
		erro = null,
		onSalvar,
		onSalvarCaderno,
		onCancelar
	}: {
		item: ItemDia;
		/** ISO date of the activity, shown so the form says what it is editing. */
		data: string;
		/** full discipline name, for the title */
		nome: string;
		/** what is already recorded for THIS activity, if anything */
		registro: RegistroBloco | null;
		/** the discipline's current error-notebook link (discipline-wide, not per-activity) */
		cadernoUrl?: string;
		salvando?: boolean;
		erro?: string | null;
		onSalvar: (v: {
			horas: number | null;
			questoes: number | null;
			acertos: number | null;
			concluido: boolean;
			nota: string;
		}) => void;
		/** persists a changed caderno link for the whole discipline; returns an error message or null */
		onSalvarCaderno?: (url: string) => Promise<string | null>;
		onCancelar: () => void;
	} = $props();

	// The pristine copy, snapshotted once on open — `untrack` states that the
	// capture is the point: later store updates must NOT rewrite the form under
	// someone who is typing in it. Everything below edits `form`; Cancelar simply
	// throws it away, which is why no restore logic is needed anywhere else.
	const original = untrack(() => valoresIniciais(registro));

	let form = $state({ ...original });

	// The caderno link is the discipline's, not this activity's — snapshotted the
	// same way, saved separately (different endpoint) only when it actually changed.
	const cadernoOriginal = untrack(() => cadernoUrl);
	let cadernoForm = $state(cadernoOriginal);
	let erroCaderno = $state<string | null>(null);

	const erros = $derived(
		form.questoes !== null && form.acertos !== null
			? Math.max(0, form.questoes - form.acertos)
			: null
	);

	// Acertos above questões is the one input that produces nonsense downstream.
	const invalido = $derived(valoresInvalidos(form));

	function num(bruto: string): number | null {
		const t = bruto.trim();
		if (t === '') return null;
		const v = Number(t.replace(',', '.'));
		return Number.isFinite(v) && v >= 0 ? v : null;
	}

	function inteiro(bruto: string): number | null {
		const v = num(bruto);
		return v === null ? null : Math.round(v);
	}

	let salvandoCaderno = $state(false);

	async function salvar() {
		if (salvando || salvandoCaderno || invalido) return;

		// The discipline-wide caderno link first, so a failure there is shown
		// before the activity record closes the dialog.
		const url = cadernoForm.trim();
		if (onSalvarCaderno && url !== cadernoOriginal.trim()) {
			salvandoCaderno = true;
			erroCaderno = await onSalvarCaderno(url);
			salvandoCaderno = false;
			if (erroCaderno) return;
		}

		onSalvar({ ...form, nota: form.nota.trim() });
	}

	// --- focus management -------------------------------------------------
	let painel = $state<HTMLDivElement | null>(null);
	let primeiro = $state<HTMLInputElement | null>(null);

	$effect(() => {
		primeiro?.focus();
	});

	/** Keeps Tab inside the dialog while it is open. */
	function prender(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onCancelar();
			return;
		}

		if (e.key !== 'Tab' || !painel) return;

		const foco = painel.querySelectorAll<HTMLElement>(
			'button:not(:disabled), input:not(:disabled), textarea:not(:disabled), [href]'
		);
		if (foco.length === 0) return;

		const primeiroEl = foco[0];
		const ultimoEl = foco[foco.length - 1];

		if (e.shiftKey && document.activeElement === primeiroEl) {
			e.preventDefault();
			ultimoEl.focus();
		} else if (!e.shiftKey && document.activeElement === ultimoEl) {
			e.preventDefault();
			primeiroEl.focus();
		}
	}

	// Derived, not captured: the dialog remounts per activity, and $derived keeps
	// the ids correct if the props ever change under it.
	const tituloID = $derived(`atv-form-${item.id || data}`);
	const descID = $derived(`${tituloID}-desc`);
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="dlg-fundo"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onCancelar();
	}}
>
	<div
		class="dlg atv-dlg"
		role="dialog"
		aria-modal="true"
		aria-labelledby={tituloID}
		aria-describedby={descID}
		tabindex="-1"
		bind:this={painel}
		onkeydown={prender}
	>
		<h2 id={tituloID} class="sec" style="margin-top:0">Registrar estudo — {nome}</h2>
		<p id={descID} class="page-sub" style="margin-top:0">
			{fl(data)}{#if item.tema}
				· {item.tema}{/if}
		</p>

		<div class="campos">
			<label class="campo">
				<span>Horas estudadas</span>
				<input
					type="number"
					min="0"
					max="24"
					step="0.25"
					inputmode="decimal"
					bind:this={primeiro}
					value={form.horas ?? ''}
					oninput={(e) => (form.horas = num(e.currentTarget.value))}
				/>
			</label>

			<label class="campo">
				<span>Questões</span>
				<input
					type="number"
					min="0"
					step="1"
					inputmode="numeric"
					value={form.questoes ?? ''}
					oninput={(e) => (form.questoes = inteiro(e.currentTarget.value))}
				/>
			</label>

			<label class="campo">
				<span>Acertos</span>
				<input
					type="number"
					min="0"
					step="1"
					inputmode="numeric"
					aria-invalid={invalido}
					value={form.acertos ?? ''}
					oninput={(e) => (form.acertos = inteiro(e.currentTarget.value))}
				/>
			</label>

			<span class="campo-err" class:vazio={erros === null}>
				{#if erros !== null}<b>{erros}</b> {erros === 1 ? 'erro' : 'erros'}{:else}—{/if}
			</span>
		</div>

		{#if invalido}
			<p class="aviso" role="alert">Acertos não pode ser maior que o número de questões.</p>
		{/if}

		<label class="ok-lbl">
			<input type="checkbox" class="checkbox" bind:checked={form.concluido} />
			Concluí esta matéria
		</label>

		<label class="campo nota">
			<span>Observação</span>
			<input
				type="text"
				placeholder="Dúvidas, questões erradas, o que revisar…"
				bind:value={form.nota}
			/>
		</label>

		{#if onSalvarCaderno}
			<label class="campo nota">
				<span>Caderno de erros — link</span>
				<input
					type="url"
					inputmode="url"
					placeholder="https://www.tecconcursos.com.br/questoes/caderno/…"
					bind:value={cadernoForm}
				/>
			</label>
			<p class="dica-caderno">
				Vale para {nome} em todo o cronograma. Aparece como atalho no bloco de revisão do dia.
			</p>
		{/if}

		{#if erroCaderno}
			<p class="aviso erro" role="alert">{erroCaderno}</p>
		{/if}

		{#if erro}
			<p class="aviso erro" role="alert">{erro}</p>
		{/if}

		<div class="dlg-acoes">
			<button type="button" class="btn" onclick={onCancelar} disabled={salvando || salvandoCaderno}>
				Cancelar
			</button>
			<button
				type="button"
				class="btn primario"
				onclick={salvar}
				disabled={salvando || salvandoCaderno || invalido}
			>
				{salvando || salvandoCaderno ? 'Salvando…' : 'Salvar'}
			</button>
		</div>
	</div>
</div>

<style>
	/* Same dialog shell as the concurso form's "dividir em tópicos" — those
	   styles are scoped to that component, so the shape is restated rather than
	   reached for. */
	.dlg-fundo {
		position: fixed;
		inset: 0;
		background: rgba(20, 18, 14, 0.45);
		display: grid;
		place-items: center;
		padding: 20px;
		z-index: 60;
	}
	.dlg {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 20px;
		max-height: 84vh;
		overflow-y: auto;
		box-shadow: var(--shadow-pop);
	}
	.atv-dlg {
		width: min(460px, 100%);
	}
	.campos {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
		gap: 10px;
		align-items: end;
		margin-top: 14px;
	}
	.campo {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	.campo span {
		font-size: 10.5px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-faint);
		font-weight: 600;
	}
	.campo input {
		width: 100%;
	}
	.campo-err {
		font-family: var(--font-mono);
		font-size: 11.5px;
		color: var(--danger);
		white-space: nowrap;
		padding-bottom: 8px;
	}
	.campo-err.vazio {
		color: var(--text-faint);
	}
	.nota {
		margin-top: 12px;
	}
	.dica-caderno {
		margin: 4px 0 0;
		font-size: 11.5px;
		color: var(--text-faint);
	}
	.ok-lbl {
		display: flex;
		align-items: center;
		gap: 9px;
		margin-top: 14px;
		font-size: 13.5px;
		cursor: pointer;
	}
	.aviso {
		margin: 10px 0 0;
		font-size: 12.5px;
		color: var(--warn);
	}
	.aviso.erro {
		color: var(--danger);
	}
	.dlg-acoes {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 18px;
	}
	.btn.primario {
		border-color: var(--accent);
		color: var(--accent);
	}
	.btn.primario:hover:not(:disabled) {
		background: var(--accent-soft);
	}

	@media (max-width: 620px) {
		.campos {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
</style>
