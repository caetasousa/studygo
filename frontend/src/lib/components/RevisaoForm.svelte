<script lang="ts">
	import { untrack } from 'svelte';
	import { fl } from '$lib/format';
	import type { Revisao } from '$lib/types';

	/**
	 * Logs one day's review tail: the battery (questões/acertos) and an
	 * observação, which goes into the notebook under the discipline the review
	 * covered that day — the same place the rest of the app writes to.
	 *
	 * Edits are local until Salvar, exactly like AtividadeForm: cancelling
	 * discards them without touching the store.
	 */
	let {
		data,
		nome,
		revisao,
		salvando = false,
		erro = null,
		onSalvar,
		onCancelar
	}: {
		/** ISO date of the review. */
		data: string;
		/** full discipline name, for the title. */
		nome: string;
		/** what is already saved for this review, if anything. */
		revisao: Revisao;
		salvando?: boolean;
		erro?: string | null;
		onSalvar: (v: { questoes: number | null; acertos: number | null; observacao: string }) => void;
		onCancelar: () => void;
	} = $props();

	const original = untrack(() => ({
		questoes: revisao.questoes,
		acertos: revisao.acertos,
		observacao: revisao.observacao
	}));

	let form = $state({ ...original });

	const erros = $derived(
		form.questoes !== null && form.acertos !== null
			? Math.max(0, form.questoes - form.acertos)
			: null
	);

	const invalido = $derived(
		form.questoes !== null && form.acertos !== null && form.acertos > form.questoes
	);

	function inteiro(bruto: string): number | null {
		const t = bruto.trim();
		if (t === '') return null;
		const v = Math.round(Number(t.replace(',', '.')));
		return Number.isFinite(v) && v >= 0 ? v : null;
	}

	function salvar() {
		if (salvando || invalido) return;
		onSalvar({ ...form, observacao: form.observacao.trim() });
	}

	let painel = $state<HTMLDivElement | null>(null);
	let primeiro = $state<HTMLInputElement | null>(null);

	$effect(() => {
		primeiro?.focus();
	});

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

	const tituloID = $derived(`rev-form-${data}`);
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
		class="dlg rev-dlg"
		role="dialog"
		aria-modal="true"
		aria-labelledby={tituloID}
		aria-describedby={descID}
		tabindex="-1"
		bind:this={painel}
		onkeydown={prender}
	>
		<h2 id={tituloID} class="sec" style="margin-top:0">Registrar revisão — {nome}</h2>
		<p id={descID} class="page-sub" style="margin-top:0">{fl(data)}</p>

		<div class="campos">
			<label class="campo">
				<span>Questões</span>
				<input
					type="number"
					min="0"
					step="1"
					inputmode="numeric"
					bind:this={primeiro}
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

		<label class="campo nota">
			<span>Observação</span>
			<textarea
				rows="3"
				placeholder="O que ainda precisa de atenção nesta matéria…"
				bind:value={form.observacao}
			></textarea>
		</label>
		<p class="page-sub obs-nota">
			Vira uma anotação no caderno de erros desta disciplina — apagar o texto remove a anotação.
		</p>

		{#if erro}
			<p class="aviso erro" role="alert">{erro}</p>
		{/if}

		<div class="dlg-acoes">
			<button type="button" class="btn" onclick={onCancelar} disabled={salvando}>Cancelar</button>
			<button type="button" class="btn primario" onclick={salvar} disabled={salvando || invalido}>
				{salvando ? 'Salvando…' : 'Salvar'}
			</button>
		</div>
	</div>
</div>

<style>
	/* Same dialog shell as AtividadeForm — scoped per component, so restated
	   rather than shared, the same tradeoff already made there. */
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
	.rev-dlg {
		width: min(460px, 100%);
	}
	.campos {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr)) auto;
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
	.campo input,
	.campo textarea {
		width: 100%;
		font: inherit;
	}
	.campo textarea {
		resize: vertical;
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
	.obs-nota {
		margin: 4px 0 0;
		font-size: 11.5px;
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
