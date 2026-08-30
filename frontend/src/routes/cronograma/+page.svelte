<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import DiaLog from '$lib/components/DiaLog.svelte';
	import TemaTexto from '$lib/components/TemaTexto.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, hojeISO, diffDays, nf1, rotulo, tagStyle, weekdayShort } from '$lib/format';
	import type { Dia } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const disc = $derived(planoStore.discIndex);

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
	<div class="page">
		{#each semanas as s (s.numero)}
			{#if s.mostrarFlag}
				<div
					class="phase-flag"
					style="background:{s.fase === 'reta' ? 'var(--danger)' : 'var(--good)'}"
				>
					{s.fase === 'reta'
						? 'Reta final — sem conteúdo novo: revisão dirigida, discursiva e simulados'
						: 'Ciclo de conteúdo — primeira passada no edital, com revisão semanal'}
				</div>
			{/if}
			{@const saldo = saldoSemana(s)}
			<div class="week">
				<div class="week-head">
					<h3>Semana {String(s.numero).padStart(2, '0')}</h3>
					<span class="per">
						{fc(s.dias[0].data)} – {fc(s.dias.at(-1)!.data)} · faltam
						{diffDays(s.dias[0].data, plano.config.prova)} dias
					</span>
					<span class="week-bal">
						<span class="mini-bar"
							><i style="width:{saldo.alvo ? Math.min(100, (saldo.h / saldo.alvo) * 100) : 0}%"></i></span
						>
						<span>{nf1.format(saldo.h)} / {nf1.format(saldo.alvo)} h</span>
					</span>
				</div>

				{#each s.dias as d (d.data)}
					{@const movivel = diaMovivel(d)}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div
						class="row"
						class:today={d.data === hojeISO()}
						class:done={d.registro?.concluido}
						class:esp-rev={d.itens.length === 0 && d.tipo !== 'rev'}
						class:rev-day={d.tipo === 'rev'}
						class:has-note={!!d.registro?.nota}
						class:drag-over={arrastando && arrastando !== d.data && movivel}
						ondragover={(e) => movivel && e.preventDefault()}
						ondrop={(e) => {
							e.preventDefault();
							onDrop(d);
						}}
					>
						<span class="nday">
							<b>{String(d.n).padStart(3, '0')}</b>{weekdayShort(d.data)}
							{fc(d.data)}
							{#if d.reordenado}<span class="reord" title="Reorganizado manualmente">•</span>{/if}
						</span>

						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<span
							class="items"
							draggable={movivel}
							ondragstart={() => (arrastando = d.data)}
							ondragend={() => (arrastando = null)}
						>
							{#if d.itens.length === 0}
								<span class="item">
									<span
										class="tag"
										style={d.tipo === 'rev'
											? 'background:var(--warn-soft);color:var(--warn)'
											: 'background:var(--bg-hover);color:var(--accent)'}>{rotulo(d.tipo)}</span
									>
									<span class="tema-txt">
										<TemaTexto tema={d.tema} />{#if d.tipo === 'rev' && d.meta > 0}<em
												>{d.meta} questões</em
											>{/if}
									</span>
								</span>
							{:else}
								{#each d.itens as it, i (i)}
									<span class="item">
										<span class="tag" style={tagStyle(disc[it.disciplina]?.cor ?? 0)}>{it.disciplina}</span>
										<span class="tema-txt"
											><TemaTexto tema={it.tema} />{#if it.passada === 2}<em>2ª passada</em>{/if}</span
										>
									</span>
								{/each}
							{/if}
						</span>

						<DiaLog dia={d} variant="row" />

						{#if movivel}
							<span class="row-move">
								<button
									class="mv-btn"
									disabled={!vizinho(d, -1)}
									title="Antecipar"
									aria-label="Antecipar matéria"
									onclick={() => trocar(d, -1)}>◀</button
								>
								<button
									class="mv-btn"
									disabled={!vizinho(d, 1)}
									title="Adiar"
									aria-label="Adiar matéria"
									onclick={() => trocar(d, 1)}>▶</button
								>
							</span>
						{:else}
							<span class="row-move"></span>
						{/if}
					</div>
				{/each}
			</div>
		{/each}
	</div>
{/if}
