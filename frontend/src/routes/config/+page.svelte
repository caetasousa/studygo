<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import Ajuste from '$lib/components/Ajuste.svelte';
	import { planoStore, applyTheme, ehTema, type Tema } from '$lib/stores/plano.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { DIAS_CURTOS, DIAS_SEMANA, ORDEM_DIAS, nf1 } from '$lib/format';
	import type { ConfigInput, Modo, Simulados } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const cfg = $derived(plano?.config);
	const disciplinas = $derived(plano?.concurso.disciplinas ?? []);
	const balanceamento = $derived(plano?.balanceamento ?? []);

	function salvar(patch: ConfigInput) {
		void planoStore.salvarConfig(patch);
	}

	// ---- aparência ----
	// The config screen is the single place the theme is chosen; the sidebar no
	// longer carries a second, competing control.
	const TEMA_OPCOES: { v: Tema; r: string }[] = [
		{ v: 'light', r: 'Claro' },
		{ v: 'dark', r: 'Escuro' },
		{ v: 'system', r: 'Do sistema' }
	];

	const temaAtual = $derived<Tema>(ehTema(cfg?.temaUi) ? cfg.temaUi : 'dark');
	const temaDesc = $derived(
		temaAtual === 'system'
			? 'Seguindo o tema do seu sistema operacional — muda sozinho quando ele muda.'
			: 'Fixo em ' + (temaAtual === 'light' ? 'claro' : 'escuro') + ', em qualquer aparelho.'
	);

	function setTema(t: Tema) {
		applyTheme(t); // instant feedback; the save below persists it
		salvar({ temaUi: t });
	}

	function toggleDia(wd: number) {
		if (!cfg) return;
		const atual = cfg.diasEstudo;
		let novo: number[];
		if (atual.includes(wd)) {
			if (atual.length <= 2) return;
			novo = atual.filter((d) => d !== wd);
		} else {
			novo = [...atual, wd];
		}
		salvar({ diasEstudo: novo });
	}

	// ---- ritmo de estudo (antes: componente "Perfil de estudo") ----

	const BLOCOS = [1, 2, 3, 4, 5, 6];

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

	const REFORCOS: { v: number; r: string }[] = [
		{ v: 0.5, r: 'menos' },
		{ v: 1, r: 'normal' },
		{ v: 1.5, r: '+50%' },
		{ v: 2, r: 'dobro' },
		{ v: 3, r: 'triplo' }
	];

	// O motor usa este ciclo quando o plano não tem um próprio — mostrado como
	// ponto de partida editável.
	const CICLO_PADRAO = [
		{ titulo: 'Revisão ativa da semana + caderno de erros', questoes: 30 },
		{ titulo: 'Bateria mista de questões no peso da prova', questoes: 60 },
		{ titulo: 'Treino da prova discursiva, com autocorreção', questoes: 0 },
		{ titulo: 'Simulado parcial cronometrado + correção comentada', questoes: 45 }
	];

	// Intervalos de revisão editados como rascunho local, enviados ao sair do campo.
	let intervalosTexto = $state('');
	let editandoIntervalos = $state(false);
	const intervalosView = $derived(
		editandoIntervalos ? intervalosTexto : (cfg?.intervalos ?? []).join(', ')
	);

	let intervalosErro = $state<string | null>(null);

	const metodoDesc = $derived(
		cfg?.revisaoPorQuestoes
			? 'Questões: a revisão é uma bateria do assunto, sem consultar antes — você só confere o resumo no que errar.'
			: 'Releitura: você reconstrói o assunto de memória e confere o resumo depois.'
	);

	const pctDesc = $derived(
		cfg ? `${Math.round(cfg.pctRevisao * 100)}% do dia` : ''
	);

	const semanalDesc = $derived(
		cfg?.revisaoSemanal
			? 'Ligado: um dia inteiro por semana sai do conteúdo e vira revisão. São ~11 dias de matéria nova a menos num ciclo.'
			: 'Desligado: a semana inteira é conteúdo, e a revisão acontece na fatia diária acima.'
	);

	/**
	 * The day the current settings actually produce, in proportion.
	 *
	 * The settings page used to describe the schedule in prose while the schedule
	 * itself was a screen away — so a slider moved with no visible consequence.
	 * This mirrors the engine's own split: content blocks take (1 - pctRevisao)
	 * of the day, the review tail takes the rest.
	 */
	const previaBlocos = $derived.by(() => {
		if (!cfg) return [];

		const totalMin = Math.round(cfg.horasDia * 60);
		const revMin = Math.round((totalMin * cfg.pctRevisao) / 5) * 5;
		const conteudoMin = totalMin - revMin;
		const porBloco = cfg.blocosPorDia > 0 ? Math.round(conteudoMin / cfg.blocosPorDia / 5) * 5 : 0;

		const out = [];
		for (let i = 0; i < cfg.blocosPorDia; i++) {
			out.push({ rotulo: `${i + 1}º bloco`, minutos: porBloco, revisao: false });
		}

		if (revMin > 0) out.push({ rotulo: 'Revisão', minutos: revMin, revisao: true });

		return out;
	});

	const previaTexto = $derived.by(() => {
		if (!cfg) return '';

		const rev = previaBlocos.find((b) => b.revisao);
		if (!rev) {
			return 'Sem fatia de revisão: o dia inteiro é conteúdo novo.';
		}

		const como = cfg.revisaoPorQuestoes
			? `${cfg.questoesPorRevisao} questões de cada assunto que voltar`
			: 'releitura ativa de cada assunto que voltar';

		return `${rev.minutos} min no fim do dia — ${como}, puxados do caderno de erros das matérias daquele dia.`;
	});

	// Worked example of what the current intervals actually do, so the effect on
	// the schedule is visible without having to generate it.
	const intervalosDesc = $derived.by(() => {
		const dias = cfg?.intervalos ?? [];
		if (dias.length === 0) return 'Sem revisões automáticas.';
		const lista = dias.join(', ');
		return `Estudou hoje? O tópico volta ${dias.length === 1 ? 'uma vez' : dias.length + ' vezes'}: ${lista} ${dias.length === 1 ? 'dia' : 'dias'} depois.`;
	});

	function commitIntervalos() {
		editandoIntervalos = false;
		const bruto = intervalosTexto.trim();
		if (!bruto) {
			intervalosErro = null;
			return;
		}
		const pedacos = bruto.split(/[,\s]+/).filter(Boolean);
		const nums = pedacos.map((x) => parseInt(x, 10));
		if (nums.some((n) => !Number.isFinite(n) || n <= 0)) {
			intervalosErro = 'Use apenas números de dias maiores que zero, separados por vírgula.';
			return;
		}
		intervalosErro = null;
		// Ascending and de-duplicated: "7, 1, 7" is the same schedule as "1, 7".
		const limpo = [...new Set(nums)].sort((a, b) => a - b);
		if (limpo.length > 0) salvar({ intervalos: limpo });
	}

	// Ciclo semanal como rascunho local — enviado ao sair do campo, para não
	// regerar o plano a cada tecla.
	let cicloDraft = $state<{ titulo: string; questoes: number }[] | null>(null);
	const ciclo = $derived(
		cicloDraft ??
			(cfg?.cicloRevisao.length
				? cfg.cicloRevisao.map((c) => ({ ...c }))
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

	// Quantas vezes mais uma matéria aparece que uma básica (peso 1, reforço 1).
	function frequenciaRelativa(codigo: string): number {
		if (!cfg) return 1;
		const l = balanceamento.find((x) => x.codigo === codigo);
		const peso = l?.peso ?? 1;
		const reforco = cfg.reforcos[codigo] ?? 1;
		return peso * reforco;
	}

	let baixando = $state(false);
	async function baixarCsv() {
		baixando = true;
		try {
			const res = await fetch(planoStore.csvUrl(), {
				headers: { Authorization: `Bearer ${auth.accessToken}` }
			});
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `plano-${concursoStore.ativoSlug ?? 'concurso'}.csv`;
			a.click();
			URL.revokeObjectURL(url);
		} finally {
			baixando = false;
		}
	}

	async function limpar() {
		if (
			!confirm(
				'Isso apaga todas as horas, questões, anotações de dias e marcações de prazo. Continuar?'
			)
		)
			return;
		await planoStore.limparRegistros();
	}

	async function restaurar() {
		if (
			!confirm('Isso desfaz as trocas manuais de matérias e volta à ordem calculada pelo peso. Continuar?')
		)
			return;
		await planoStore.restaurarOrdem();
	}
</script>

<PageHead
	icone="config"
	titulo="Configurações"
	sub="Datas, ritmo de estudo e seus dados."
	mostrarProps={false}
/>

{#if plano && cfg}
	<div class="page">
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Datas</h2>
				<div class="form-grid">
					<div class="field">
						<label for="inicio">Início</label>
						<input
							id="inicio"
							type="date"
							value={cfg.inicio}
							onchange={(e) => salvar({ inicio: (e.target as HTMLInputElement).value })}
						/>
					</div>
					<div class="field">
						<label for="prova">Dia da prova</label>
						<input
							id="prova"
							type="date"
							value={cfg.prova}
							onchange={(e) => salvar({ prova: (e.target as HTMLInputElement).value })}
						/>
					</div>
					<div class="field">
						<!-- svelte-ignore a11y_label_has_associated_control -->
						<label>Dias de estudo</label>
						<div class="day-sel">
							{#each ORDEM_DIAS as wd (wd)}
								<button
									type="button"
									class:rev={cfg.revisaoSemanal && wd === cfg.diaRevisao}
									aria-pressed={cfg.diasEstudo.includes(wd)}
									title={DIAS_SEMANA[wd]}
									onclick={() => toggleDia(wd)}
								>
									{DIAS_CURTOS[wd]}
								</button>
							{/each}
						</div>
					</div>
					<div class="field">
						<label for="reta">Reta final (dias)</label>
						<input
							id="reta"
							type="number"
							min="7"
							max="120"
							step="7"
							value={cfg.retaFinalDias}
							onchange={(e) =>
								salvar({ retaFinalDias: parseInt((e.target as HTMLInputElement).value, 10) })}
						/>
					</div>
				</div>
				<p class="page-sub" style="margin:8px 0 0">
					O <b>dia da revisão</b> é um dia de estudo dedicado à <b>resolução de questões</b> no peso
					da prova. Ele segue o dia que você escolher aqui — mude para domingo, sábado ou qualquer
					outro e o cronograma acompanha, com cor própria.
				</p>
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Aparência</h2>
				<Ajuste
					titulo="Tema"
					descricao={temaDesc}
				>
					{#snippet controle()}
						<div class="day-sel" role="group" aria-label="Tema da interface">
							{#each TEMA_OPCOES as t (t.v)}
								<button
									type="button"
									aria-pressed={temaAtual === t.v}
									style="width:auto;padding:0 12px"
									onclick={() => setTema(t.v)}>{t.r}</button
								>
							{/each}
						</div>
					{/snippet}
				</Ajuste>
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Ritmo de estudo</h2>
				<p class="page-sub" style="margin:0 0 4px">
					Quanto você estuda por dia. Alterar estes valores regenera o cronograma —
					seus registros de horas e questões são preservados.
				</p>

				<Ajuste
					titulo="Minutos por bloco"
					descricao="Duração de um bloco de estudo. Define o tamanho de cada atividade do cronograma."
					para="minbloco"
					unidade="min"
				>
					{#snippet controle()}
						<input
							id="minbloco"
							type="number"
							min="15"
							max="240"
							step="5"
							value={cfg.minutosBloco}
							onchange={(e) =>
								salvar({ minutosBloco: parseInt((e.target as HTMLInputElement).value, 10) })}
						/>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Blocos por dia"
					descricao="Quantos blocos cabem num dia de estudo."
					valor="{nf1.format(cfg.horasDia)} h por dia"
				>
					{#snippet controle()}
						<div class="day-sel" role="group" aria-label="Blocos por dia">
							{#each BLOCOS as n (n)}
								<button
									type="button"
									aria-pressed={cfg.blocosPorDia === n}
									style="width:auto;padding:0 12px"
									onclick={() => salvar({ blocosPorDia: n })}>{n}</button
								>
							{/each}
						</div>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Simulados completos"
					descricao="Com que frequência o cronograma reserva um dia inteiro para um simulado."
				>
					{#snippet controle()}
						<div class="day-sel" role="group" aria-label="Frequência de simulados">
							{#each SIMULADOS as s (s.v)}
								<button
									type="button"
									aria-pressed={cfg.simulados === s.v}
									style="width:auto;padding:0 10px"
									onclick={() => salvar({ simulados: s.v })}>{s.r}</button
								>
							{/each}
						</div>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Treinar a prova discursiva"
					descricao="Reserva blocos para redação ou estudo de caso, quando o edital cobra."
					para="pf-disc"
				>
					{#snippet controle()}
						<input
							id="pf-disc"
							type="checkbox"
							class="checkbox"
							checked={cfg.discursiva}
							onchange={(e) => salvar({ discursiva: e.currentTarget.checked })}
						/>
					{/snippet}
				</Ajuste>
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Revisão</h2>
				<p class="page-sub" style="margin:0 0 4px">
					A revisão é diária: uma fatia no fim de cada dia de estudo, depois dos
					blocos de conteúdo. O que ela cobra depende do método abaixo.
				</p>

				<Ajuste
					titulo="Método de revisão"
					descricao={metodoDesc}
				>
					{#snippet controle()}
						<div class="metodo-sel">
							<button
								type="button"
								aria-pressed={cfg.revisaoPorQuestoes}
								onclick={() => salvar({ revisaoPorQuestoes: true })}
							>
								Questões
							</button>
							<button
								type="button"
								aria-pressed={!cfg.revisaoPorQuestoes}
								onclick={() => salvar({ revisaoPorQuestoes: false })}
							>
								Releitura
							</button>
						</div>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Tempo reservado para revisão"
					descricao="Fatia do fim de cada dia. O resto vai para o conteúdo novo."
					para="pf-prev"
					valor={pctDesc}
				>
					{#snippet controle()}
						<input
							id="pf-prev"
							type="range"
							min="0"
							max="40"
							step="4"
							value={Math.round(cfg.pctRevisao * 100)}
							onchange={(e) =>
								salvar({ pctRevisao: parseInt((e.target as HTMLInputElement).value, 10) / 100 })}
						/>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Questões por tema revisado"
					descricao="Quantas questões a revisão pede de cada assunto que volta."
					para="pf-qrev"
					unidade="questões"
				>
					{#snippet controle()}
						<input
							id="pf-qrev"
							type="number"
							min="1"
							max="50"
							value={cfg.questoesPorRevisao}
							onchange={(e) =>
								salvar({ questoesPorRevisao: parseInt((e.target as HTMLInputElement).value, 10) })}
							style="width:90px"
						/>
					{/snippet}
				</Ajuste>

				<!-- What the setting actually produces, so the effect is visible here
				     instead of only after opening the schedule. -->
				<div class="previa">
					<span class="previa-tit">No cronograma, um dia fica assim:</span>
					<div class="previa-barra">
						{#each previaBlocos as b (b.rotulo)}
							<span
								class="previa-fatia"
								class:rev={b.revisao}
								style="flex:{b.minutos}"
								title="{b.rotulo} — {b.minutos} min"
							>
								{b.minutos}min
							</span>
						{/each}
					</div>
					<span class="previa-leg">{previaTexto}</span>
				</div>

				<h2 class="sec">Quando um assunto volta</h2>
				<p class="page-sub" style="margin:0 0 4px">
					A revisão puxa primeiro o seu <b>caderno de erros</b> — os assuntos em que
					você foi mal, acumulados por matéria. Sem erros pendentes daquela matéria,
					ela cai nos intervalos abaixo.
				</p>

				<Ajuste
					titulo="Criar revisões após"
					descricao={intervalosDesc}
					para="pf-int"
					unidade="dias"
				>
					{#snippet controle()}
						<input
							id="pf-int"
							type="text"
							inputmode="numeric"
							placeholder="1, 7, 30"
							aria-describedby="pf-int-erro"
							value={intervalosView}
							oninput={(e) => {
								editandoIntervalos = true;
								intervalosTexto = e.currentTarget.value;
							}}
							onblur={commitIntervalos}
							style="width:150px"
						/>
					{/snippet}
				</Ajuste>
				{#if intervalosErro}
					<p id="pf-int-erro" class="ajuste-erro" role="alert">{intervalosErro}</p>
				{/if}

				<Ajuste
					titulo="Reservar um dia da semana só para revisão"
					descricao={semanalDesc}
					para="pf-revsem"
				>
					{#snippet controle()}
						<input
							id="pf-revsem"
							type="checkbox"
							class="checkbox"
							checked={cfg.revisaoSemanal}
							onchange={(e) => salvar({ revisaoSemanal: e.currentTarget.checked })}
						/>
					{/snippet}
				</Ajuste>

				{#if cfg.revisaoSemanal}
					<Ajuste titulo="Dia da revisão" descricao="Qual dia da semana deixa de ser conteúdo." para="pf-diarev">
						{#snippet controle()}
							<select
								id="pf-diarev"
								value={cfg.diaRevisao}
								onchange={(e) =>
									salvar({ diaRevisao: parseInt((e.target as HTMLSelectElement).value, 10) })}
							>
								{#each ORDEM_DIAS as wd (wd)}
									<option value={wd}>{DIAS_SEMANA[wd]}</option>
								{/each}
							</select>
						{/snippet}
					</Ajuste>
				{/if}
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Questões</h2>

				<Ajuste
					titulo="Questões por revisão"
					descricao="Quantidade usada ao criar automaticamente uma atividade de revisão por questões. Você pode alterar em cada atividade."
					para="pf-qrev"
					unidade="questões"
				>
					{#snippet controle()}
						<input
							id="pf-qrev"
							type="number"
							min="1"
							max="200"
							value={cfg.questoesPorRevisao}
							onchange={(e) =>
								salvar({ questoesPorRevisao: parseInt(e.currentTarget.value, 10) })}
						/>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Parte do bloco dedicada a questões"
					descricao="Num bloco de teoria + questões, quanto do tempo vai para resolver questões."
					para="pf-pct"
					valor="{Math.round(cfg.pctQuestoes * 100)}% do bloco"
				>
					{#snippet controle()}
						<input
							id="pf-pct"
							type="range"
							min="10"
							max="90"
							step="10"
							value={Math.round(cfg.pctQuestoes * 100)}
							onchange={(e) => salvar({ pctQuestoes: parseInt(e.currentTarget.value, 10) / 100 })}
						/>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Aproveitamento fraco abaixo de"
					descricao="Abaixo desta taxa de acerto a matéria é sinalizada como ponto fraco nas estatísticas."
					para="pf-limiar"
					unidade="%"
				>
					{#snippet controle()}
						<input
							id="pf-limiar"
							type="number"
							min="1"
							max="100"
							value={cfg.limiarFraco}
							onchange={(e) => salvar({ limiarFraco: parseInt(e.currentTarget.value, 10) })}
						/>
					{/snippet}
				</Ajuste>
			</div>
		</div>

		<div class="card">
			<div class="card-body">

				<h3 class="modos-t">Peso e método por matéria</h3>
				<p class="page-sub" style="margin:0 0 10px;font-size:12px">
					Matérias de peso maior (as <b>específicas</b> valem 2, as básicas 1) já aparecem mais vezes
					no cronograma. O <b>reforço</b> multiplica ainda mais uma matéria em que você está com
					dificuldade — ela aparece mais e ganha um bloco mais longo.
				</p>
				<div class="modos">
					{#each disciplinas as d (d.codigo)}
						{@const rel = frequenciaRelativa(d.codigo)}
						<div class="modo-linha">
							<span class="modo-nome">
								<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>{d.nome}
								<em class="peso-tag">
									peso {d.peso}{#if rel !== 1} · ~{nf1.format(rel)}× as básicas{/if}
								</em>
							</span>
							<div class="day-sel">
								{#each MODOS as m (m.v)}
									<button
										type="button"
										aria-pressed={(cfg.modos[d.codigo] ?? 'completo') === m.v}
										style="width:auto;padding:0 10px"
										onclick={() => salvar({ modos: { [d.codigo]: m.v } })}>{m.r}</button
									>
								{/each}
							</div>
							<div class="day-sel reforco">
								{#each REFORCOS as r (r.v)}
									<button
										type="button"
										aria-pressed={(cfg.reforcos[d.codigo] ?? 1) === r.v}
										style="width:auto;padding:0 9px"
										title="Reforço {r.v}×"
										onclick={() => salvar({ reforcos: { [d.codigo]: r.v } })}>{r.r}</button
									>
								{/each}
							</div>
						</div>
					{/each}
				</div>

				<h3 class="modos-t">Ciclo de revisão semanal</h3>
				<p class="page-sub" style="margin:0 0 10px;font-size:12px">
					Um dia de cada semana da fase de conteúdo, em rodízio — sempre focado em resolver
					questões. É independente dos simulados da reta final.
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
								onclick={() => rmSemana(i)}>remover</button
							>
						</div>
					{/each}
					<button type="button" class="btn" style="margin-top:8px" onclick={addSemana}
						>+ semana</button
					>
				</div>

				<h2 class="sec">Reordenação manual</h2>
				<p class="page-sub" style="margin-top:0">
					No Cronograma, cada matéria é movida sozinha: use as setas para mudá-la de
					posição no dia, o menu <b>…</b> para enviá-la a outra data, ou arraste-a. O
					dia inteiro não é movido — só as matérias dentro dele. Dias concluídos e
					dias fixos não podem ser alterados.
				</p>
				<div class="form-grid">
					<button class="btn" disabled={plano.reordenados.length === 0} onclick={restaurar}>
						↺ Restaurar ordem automática
					</button>
					<span class="page-sub" style="margin:0">
						{plano.reordenados.length
							? `${plano.reordenados.length} dia(s) reorganizado(s) manualmente.`
							: 'Nenhuma troca manual ainda.'}
					</span>
				</div>

				<h2 class="sec">Este concurso</h2>
				<div class="form-grid">
					<a class="btn" href="/concursos/{plano.concurso.slug}/editar"
						>✏️ Editar disciplinas e datas</a
					>
					<a class="btn" href="/concursos">Trocar de concurso</a>
				</div>

				<h2 class="sec">Dados</h2>
				<div class="form-grid">
					<button class="btn" onclick={baixarCsv} disabled={baixando}>⬇ Exportar CSV</button>
					<button class="btn danger" onclick={limpar}>Limpar registros</button>
				</div>
				<p class="page-sub" style="margin-top:14px">
					Seus dados ficam salvos no servidor, ligados à sua conta ({auth.usuario?.email}). O CSV é
					uma cópia de segurança que você pode abrir no Excel.
				</p>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Two exclusive choices read better as a pair of buttons than as a checkbox
	   whose off-state has to be inferred from the label. */
	.metodo-sel {
		display: flex;
		gap: 4px;
	}
	.metodo-sel button {
		font-size: 12.5px;
		padding: 7px 14px;
		border: 1px solid var(--border);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}
	.metodo-sel button[aria-pressed='true'] {
		background: var(--accent-soft);
		border-color: var(--accent);
		color: var(--accent-strong);
		font-weight: 600;
	}

	/* The settings only mattered once you opened the schedule; this shows the day
	   they produce, here, in proportion. */
	.previa {
		margin: 14px 0 4px;
		padding: 12px 14px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.previa-tit {
		display: block;
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		font-weight: 600;
		color: var(--text-faint);
		margin-bottom: 8px;
	}
	.previa-barra {
		display: flex;
		gap: 3px;
		height: 30px;
	}
	.previa-fatia {
		display: grid;
		place-items: center;
		border-radius: 5px;
		background: var(--accent-soft);
		color: var(--accent-strong);
		font-family: var(--font-mono);
		font-size: 10.5px;
		min-width: 0;
		overflow: hidden;
		white-space: nowrap;
	}
	.previa-fatia.rev {
		background: var(--warn-soft);
		color: var(--warn);
	}
	.previa-leg {
		display: block;
		margin-top: 8px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.ajuste-erro {
		margin: -6px 0 10px;
		font-size: 12px;
		color: var(--danger);
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
		flex: 1 1 200px;
		min-width: 0;
		font-size: 13.5px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.peso-tag {
		font-style: normal;
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--text-faint);
		margin-left: 6px;
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
</style>
