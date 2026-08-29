<script lang="ts">
	import { planoStore } from '$lib/stores/plano.svelte';
	import type { Modo, Perfil, PerfilInput, Simulados } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const perfil = $derived(plano?.config.perfil);
	const disciplinas = $derived(plano?.concurso.disciplinas ?? []);

	function salvar(patch: PerfilInput) {
		void planoStore.salvarConfig({ perfil: patch });
	}

	interface Preset {
		id: string;
		nome: string;
		sub: string;
		valores: PerfilInput;
	}

	const presets: Preset[] = [
		{
			id: 'raiz',
			nome: 'Concurseiro raiz',
			sub: 'teoria e questões, simulado toda semana da reta final, revisão 1 · 7 · 30',
			valores: {
				simulados: 'semanal',
				discursiva: true,
				intervalos: [1, 7, 30],
				pctQuestoes: 0.5,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 10
			}
		},
		{
			id: 'questoes',
			nome: 'Só questões',
			sub: 'a teoria vem da correção, sem simulado, revisão 1 · 5 · 15 · 45',
			valores: {
				simulados: 'nunca',
				discursiva: false,
				intervalos: [1, 5, 15, 45],
				pctQuestoes: 0.8,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 15
			}
		},
		{
			id: 'teoria',
			nome: 'Teoria primeiro',
			sub: 'mais tempo de leitura e resumo, simulado quinzenal, revisão 2 · 10 · 30',
			valores: {
				simulados: 'quinzenal',
				discursiva: true,
				intervalos: [2, 10, 30],
				pctQuestoes: 0.3,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 8
			}
		}
	];

	// Um preset está "ativo" quando todos os seus valores batem com o perfil atual —
	// mexer em qualquer controle abaixo simplesmente o desmarca.
	function presetAtivo(p: Preset, atual: Perfil): boolean {
		return (
			p.valores.simulados === atual.simulados &&
			p.valores.discursiva === atual.discursiva &&
			p.valores.pctQuestoes === atual.pctQuestoes &&
			p.valores.questoesPorRevisao === atual.questoesPorRevisao &&
			(p.valores.intervalos ?? []).join(',') === atual.intervalos.join(',')
		);
	}

	const SIMULADOS: { v: Simulados; r: string }[] = [
		{ v: 'nunca', r: 'nunca' },
		{ v: 'quinzenal', r: 'a cada 2 semanas' },
		{ v: 'semanal', r: 'toda semana' }
	];

	const MODOS: { v: Modo; r: string }[] = [
		{ v: 'completo', r: 'teoria + questões' },
		{ v: 'questoes', r: 'só questões' },
		{ v: 'teoria', r: 'só teoria' }
	];

	let intervalosTexto = $state('');
	let editandoIntervalos = $state(false);

	const intervalosView = $derived(
		editandoIntervalos ? intervalosTexto : (perfil?.intervalos ?? []).join(', ')
	);

	function commitIntervalos() {
		editandoIntervalos = false;
		const nums = intervalosTexto
			.split(/[,\s]+/)
			.map((x) => parseInt(x, 10))
			.filter((n) => Number.isFinite(n) && n > 0);
		if (nums.length > 0) salvar({ intervalos: nums });
	}
</script>

{#if plano && perfil}
	<h2 class="sec">Perfil de estudo</h2>
	<p class="page-sub" style="margin-top:0">
		O método é seu. Simulado, discursiva, os intervalos de revisão e como cada matéria é estudada
		mudam o cronograma na hora.
	</p>

	<div class="presets">
		{#each presets as p (p.id)}
			<button
				type="button"
				class="preset"
				class:on={presetAtivo(p, perfil)}
				onclick={() => salvar(p.valores)}
			>
				<b>{p.nome}</b>
				<span>{p.sub}</span>
			</button>
		{/each}
	</div>

	<div class="form-grid" style="margin-top:16px">
		<div class="field">
			<!-- svelte-ignore a11y_label_has_associated_control -->
			<label>Simulados completos</label>
			<div class="day-sel">
				{#each SIMULADOS as s (s.v)}
					<button
						type="button"
						aria-pressed={perfil.simulados === s.v}
						style="width:auto;padding:0 10px"
						onclick={() => salvar({ simulados: s.v })}>{s.r}</button
					>
				{/each}
			</div>
		</div>

		<div class="field">
			<label for="pf-disc">Prova discursiva</label>
			<input
				id="pf-disc"
				type="checkbox"
				class="checkbox"
				checked={perfil.discursiva}
				onchange={(e) => salvar({ discursiva: e.currentTarget.checked })}
			/>
		</div>

		<div class="field" style="flex:1 1 220px">
			<label for="pf-int">Revisão espaçada — dias, separados por vírgula</label>
			<input
				id="pf-int"
				type="text"
				inputmode="numeric"
				placeholder="1, 7, 30"
				value={intervalosView}
				oninput={(e) => {
					editandoIntervalos = true;
					intervalosTexto = e.currentTarget.value;
				}}
				onblur={commitIntervalos}
				style="width:100%"
			/>
		</div>

		<div class="field">
			<label for="pf-qrev">Questões por revisão</label>
			<input
				id="pf-qrev"
				type="number"
				min="1"
				max="200"
				value={perfil.questoesPorRevisao}
				onchange={(e) => salvar({ questoesPorRevisao: parseInt(e.currentTarget.value, 10) })}
			/>
		</div>

		<div class="field">
			<label for="pf-pct">Questões no bloco — {Math.round(perfil.pctQuestoes * 100)}%</label>
			<input
				id="pf-pct"
				type="range"
				min="10"
				max="90"
				step="10"
				value={Math.round(perfil.pctQuestoes * 100)}
				onchange={(e) => salvar({ pctQuestoes: parseInt(e.currentTarget.value, 10) / 100 })}
			/>
		</div>

		<div class="field">
			<label for="pf-limiar">Aproveitamento fraco abaixo de</label>
			<input
				id="pf-limiar"
				type="number"
				min="1"
				max="100"
				value={perfil.limiarFraco}
				onchange={(e) => salvar({ limiarFraco: parseInt(e.currentTarget.value, 10) })}
			/>
		</div>
	</div>

	<h3 class="modos-t">Como estudar cada matéria</h3>
	<div class="modos">
		{#each disciplinas as d (d.codigo)}
			<div class="modo-linha">
				<span class="modo-nome">
					<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>{d.nome}
				</span>
				<div class="day-sel">
					{#each MODOS as m (m.v)}
						<button
							type="button"
							aria-pressed={(perfil.modos[d.codigo] ?? 'completo') === m.v}
							style="width:auto;padding:0 10px"
							onclick={() => salvar({ modos: { [d.codigo]: m.v } })}>{m.r}</button
						>
					{/each}
				</div>
			</div>
		{/each}
	</div>
{/if}

<style>
	.presets {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 8px;
	}
	.preset {
		display: flex;
		flex-direction: column;
		gap: 3px;
		text-align: left;
		padding: 11px 13px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-card);
		cursor: pointer;
	}
	.preset:hover {
		border-color: var(--border-strong);
		background: var(--bg-hover);
	}
	.preset.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.preset b {
		font-size: 13.5px;
	}
	.preset span {
		font-size: 11.5px;
		line-height: 1.4;
		color: var(--text-muted);
	}
	.preset.on b {
		color: var(--accent-strong);
	}

	.modos-t {
		font-size: 13px;
		font-weight: 600;
		margin: 20px 0 8px;
	}
	.modos {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.modo-linha {
		display: flex;
		align-items: center;
		gap: 12px;
		flex-wrap: wrap;
	}
	.modo-nome {
		flex: 1 1 180px;
		min-width: 0;
		font-size: 13.5px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
