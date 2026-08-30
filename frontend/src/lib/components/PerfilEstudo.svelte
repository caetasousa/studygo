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
			sub: '2 blocos por dia, teoria e questões, simulado toda semana da reta final, revisão 1 · 7 · 30',
			valores: {
				simulados: 'semanal',
				discursiva: true,
				intervalos: [1, 7, 30],
				pctQuestoes: 0.5,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 10,
				blocosPorDia: 2,
				pctRevisao: 0.16
			}
		},
		{
			id: 'questoes',
			nome: 'Só questões',
			sub: '3 blocos por dia, a teoria vem da correção, sem simulado, revisão 1 · 5 · 15 · 45',
			valores: {
				simulados: 'nunca',
				discursiva: false,
				intervalos: [1, 5, 15, 45],
				pctQuestoes: 0.8,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 15,
				blocosPorDia: 3,
				pctRevisao: 0.2
			}
		},
		{
			id: 'teoria',
			nome: 'Teoria primeiro',
			sub: '2 blocos por dia, mais tempo de leitura e resumo, simulado quinzenal, revisão 2 · 10 · 30',
			valores: {
				simulados: 'quinzenal',
				discursiva: true,
				intervalos: [2, 10, 30],
				pctQuestoes: 0.3,
				revisaoPorQuestoes: true,
				questoesPorRevisao: 8,
				blocosPorDia: 2,
				pctRevisao: 0.12
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
			p.valores.blocosPorDia === atual.blocosPorDia &&
			p.valores.pctRevisao === atual.pctRevisao &&
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

	// O que o motor usa quando o plano não tem ciclo próprio — mostrado como
	// ponto de partida editável, para o ciclo não ficar invisível como antes.
	const CICLO_PADRAO = [
		{ titulo: 'Revisão ativa da semana + caderno de erros', questoes: 30 },
		{ titulo: 'Bateria mista de questões no peso da prova', questoes: 60 },
		{ titulo: 'Treino da prova discursiva, com autocorreção', questoes: 0 },
		{ titulo: 'Simulado parcial cronometrado + correção comentada', questoes: 45 }
	];

	let intervalosTexto = $state('');
	let editandoIntervalos = $state(false);

	const intervalosView = $derived(
		editandoIntervalos ? intervalosTexto : (perfil?.intervalos ?? []).join(', ')
	);

	const BLOCOS = [1, 2, 3, 4, 5, 6];

	const REFORCOS: { v: number; r: string }[] = [
		{ v: 0.5, r: 'menos' },
		{ v: 1, r: 'normal' },
		{ v: 1.5, r: '+50%' },
		{ v: 2, r: 'dobro' },
		{ v: 3, r: 'triplo' }
	];

	// O ciclo semanal é editado como rascunho local e enviado ao sair do campo,
	// para não regerar o plano a cada tecla.
	let cicloDraft = $state<{ titulo: string; questoes: number }[] | null>(null);

	const ciclo = $derived(
		cicloDraft ??
			(perfil?.cicloRevisao.length
				? perfil.cicloRevisao.map((c) => ({ ...c }))
				: CICLO_PADRAO.map((c) => ({ ...c })))
	);

	function editarCiclo(i: number, campo: 'titulo' | 'questoes', valor: string) {
		const copia = ciclo.map((c) => ({ ...c }));
		if (campo === 'titulo') copia[i].titulo = valor;
		else copia[i].questoes = Math.max(0, parseInt(valor, 10) || 0);
		cicloDraft = copia;
	}

	function commitCiclo() {
		if (!cicloDraft) return;
		const limpo = cicloDraft.filter((c) => c.titulo.trim());
		cicloDraft = null;
		salvar({ cicloRevisao: limpo });
	}

	function addSemana() {
		cicloDraft = [...ciclo.map((c) => ({ ...c })), { titulo: '', questoes: 30 }];
	}

	function rmSemana(i: number) {
		const copia = ciclo.map((c) => ({ ...c }));
		copia.splice(i, 1);
		cicloDraft = copia;
		commitCiclo();
	}

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
		O método é seu. Quantas matérias por dia, o tempo de cada bloco, simulado, discursiva, os
		intervalos de revisão e o peso de cada matéria mudam o cronograma na hora. Os presets são só
		um ponto de partida — todo controle abaixo continua editável.
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

		<div class="field" style="flex:1 1 240px">
			<!-- svelte-ignore a11y_label_has_associated_control -->
			<label>
				Matérias por dia — {perfil.minutosPorBloco} min cada
			</label>
			<div class="day-sel">
				{#each BLOCOS as n (n)}
					<button
						type="button"
						aria-pressed={perfil.blocosPorDia === n}
						style="width:auto;padding:0 12px"
						onclick={() => salvar({ blocosPorDia: n })}>{n}</button
					>
				{/each}
			</div>
		</div>

		<div class="field">
			<label for="pf-prev">Tempo da revisão — {Math.round(perfil.pctRevisao * 100)}% do dia</label>
			<input
				id="pf-prev"
				type="range"
				min="0"
				max="40"
				step="4"
				value={Math.round(perfil.pctRevisao * 100)}
				onchange={(e) => salvar({ pctRevisao: parseInt(e.currentTarget.value, 10) / 100 })}
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
	<p class="page-sub" style="margin:0 0 10px;font-size:12px">
		O reforço dá mais peso a uma matéria em que você está com dificuldade: ela aparece mais vezes
		no cronograma e ganha um bloco mais longo quando aparece.
	</p>
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
				<div class="day-sel reforco">
					{#each REFORCOS as r (r.v)}
						<button
							type="button"
							aria-pressed={(perfil.reforcos[d.codigo] ?? 1) === r.v}
							style="width:auto;padding:0 9px"
							title="Peso {r.v}×"
							onclick={() => salvar({ reforcos: { [d.codigo]: r.v } })}>{r.r}</button
						>
					{/each}
				</div>
			</div>
		{/each}
	</div>

	<h3 class="modos-t">Ciclo de revisão semanal</h3>
	<p class="page-sub" style="margin:0 0 10px;font-size:12px">
		Um dia de cada semana da fase de conteúdo, em rodízio. É independente dos simulados da reta
		final — desligar simulado acima não mexe aqui.
	</p>
	<div class="ciclo">
		{#each ciclo as c, i (i)}
			<div class="ciclo-linha">
				<span class="ciclo-n">sem. {i + 1}</span>
				<input
					type="text"
					value={c.titulo}
					placeholder="o que fazer nesta semana"
					oninput={(e) => editarCiclo(i, 'titulo', e.currentTarget.value)}
					onblur={commitCiclo}
				/>
				<input
					type="number"
					min="0"
					max="300"
					class="ciclo-q"
					value={c.questoes}
					oninput={(e) => editarCiclo(i, 'questoes', e.currentTarget.value)}
					onblur={commitCiclo}
				/>
				<button
					type="button"
					class="mv-btn"
					aria-label="Remover semana"
					disabled={ciclo.length <= 1}
					onclick={() => rmSemana(i)}>✕</button
				>
			</div>
		{/each}
		<button type="button" class="btn" style="margin-top:8px" onclick={addSemana}>+ semana</button>
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
	.reforco button[aria-pressed='true'] {
		background: var(--warn-soft);
		color: var(--warn);
	}
	.ciclo {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.ciclo-linha {
		display: grid;
		grid-template-columns: 58px minmax(0, 1fr) 72px 30px;
		align-items: center;
		gap: 8px;
	}
	.ciclo-n {
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--text-faint);
	}
	.ciclo-linha input {
		width: 100%;
	}
	.ciclo-q {
		text-align: right;
	}
	@media (max-width: 560px) {
		.ciclo-linha {
			grid-template-columns: minmax(0, 1fr) 68px 30px;
		}
		.ciclo-n {
			grid-column: 1 / -1;
		}
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
