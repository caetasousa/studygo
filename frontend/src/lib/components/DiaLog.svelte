<script lang="ts">
	import { untrack } from 'svelte';
	import type { Dia } from '$lib/types';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { debounce, parseNum, parseInteger } from '$lib/debounce';

	let { dia, variant = 'row' }: { dia: Dia; variant?: 'row' | 'card' } = $props();

	let horas = $state<number | null>(null);
	let questoes = $state<number | null>(null);
	let acertos = $state<number | null>(null);
	let concluido = $state(false);
	let nota = $state('');

	// Re-sync from the server only when the component switches to a different
	// day — never mid-edit, so a debounced save in flight can't clobber newer
	// keystrokes.
	let carregadoDe = $state('');
	$effect(() => {
		if (dia.data === carregadoDe) return;
		carregadoDe = dia.data;
		untrack(() => {
			horas = dia.registro?.horas ?? null;
			questoes = dia.registro?.questoes ?? null;
			acertos = dia.registro?.acertos ?? null;
			concluido = dia.registro?.concluido ?? false;
			nota = dia.registro?.nota ?? '';
		});
	});

	const salvar = debounce(() => {
		void planoStore.registrarDia(dia.data, { horas, questoes, acertos, concluido, nota });
	}, 450);

	function onHoras(e: Event) {
		horas = parseNum((e.target as HTMLInputElement).value);
		if (horas && !concluido) concluido = true;
		salvar();
	}
	function onQuestoes(e: Event) {
		questoes = parseInteger((e.target as HTMLInputElement).value);
		salvar();
	}
	function onAcertos(e: Event) {
		acertos = parseInteger((e.target as HTMLInputElement).value);
		salvar();
	}
	function onConcluido(e: Event) {
		concluido = (e.target as HTMLInputElement).checked;
		if (concluido && horas === null) horas = planoStore.plano?.config.horasDia ?? null;
		salvar();
	}
	function onNota(e: Event) {
		nota = (e.target as HTMLInputElement).value;
		salvar();
	}
</script>

{#if variant === 'card'}
	<div class="lanc">
		<div class="field">
			<label for="in-h">Horas estudadas</label>
			<input id="in-h" type="number" min="0" max="24" step="0.25" placeholder="0,00" value={horas ?? ''} oninput={onHoras} />
		</div>
		<div class="field">
			<label for="in-q">Questões feitas</label>
			<input id="in-q" type="number" min="0" step="1" placeholder={String(dia.meta)} value={questoes ?? ''} oninput={onQuestoes} />
		</div>
		<div class="field">
			<label for="in-a">Acertos</label>
			<input id="in-a" type="number" min="0" step="1" placeholder="0" value={acertos ?? ''} oninput={onAcertos} />
		</div>
		<div class="field">
			<label for="in-ok">Concluído</label>
			<input id="in-ok" type="checkbox" class="checkbox" checked={concluido} onchange={onConcluido} />
		</div>
	</div>
{:else}
	<span class="cell-in h">
		<input
			type="number"
			min="0"
			max="24"
			step="0.25"
			placeholder="0,00"
			aria-label="Horas"
			class:filled={!!horas}
			value={horas ?? ''}
			oninput={onHoras}
		/>h
	</span>
	<span class="cell-in q">
		<input
			type="number"
			min="0"
			step="1"
			placeholder={String(dia.meta)}
			aria-label="Questões"
			class:filled={!!questoes}
			value={questoes ?? ''}
			oninput={onQuestoes}
		/>q
	</span>
	<input
		type="checkbox"
		class="checkbox rowchk"
		aria-label="Concluído"
		checked={concluido}
		onchange={onConcluido}
	/>
	<span class="note-row">
		<input
			type="text"
			placeholder="Anotação: dúvidas, questões erradas, o que revisar…"
			value={nota}
			oninput={onNota}
		/>
	</span>
{/if}
