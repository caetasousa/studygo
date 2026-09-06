<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import Ajuste from '$lib/components/Ajuste.svelte';
	import ImportarCSV from '$lib/components/ImportarCSV.svelte';
	import { planoStore, applyTheme, ehTema, type Tema } from '$lib/stores/plano.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { DIAS_CURTOS, DIAS_SEMANA, ORDEM_DIAS, nf1 } from '$lib/format';
	import type { ConfigInput, Modo, Simulados } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const cfg = $derived(plano?.config);
	const disciplinas = $derived(plano?.concurso.disciplinas ?? []);

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
		void auth.definirTema(t);
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

	// Intervalos de revisão editados como rascunho local, enviados ao sair do campo.
	// Turning the block off must not forget how long it was, or turning it back
	// on would silently reset to the default.
	let ultimaDuracao = $state(20);

	$effect(() => {
		if (cfg && cfg.minutosRevisao > 0) ultimaDuracao = cfg.minutosRevisao;
	});

	const mostrarDesc = $derived(
		cfg && cfg.minutosRevisao > 0
			? `Ligado: cada dia de estudo do cronograma termina com um bloco de ${cfg.minutosRevisao} min de revisão.`
			: 'Desligado: o cronograma mostra só os blocos de matéria.'
	);

	let compactando = $state(false);

	async function compactar() {
		if (compactando) return;

		compactando = true;
		await planoStore.compactarPlano();
		compactando = false;
	}

	// Reorganizar a partir de uma data. Serve a quem registrou dias já vividos e
	// quer o motor rearrumando dali para frente, em vez de arrastar matéria por
	// matéria. Começa no início do plano, que é o caso de quem está pondo o
	// histórico em dia.
	let reorganizarData = $state('');
	let reorganizando = $state(false);

	$effect(() => {
		if (!reorganizarData && cfg?.inicio) reorganizarData = cfg.inicio;
	});

	async function reorganizar() {
		if (reorganizando || !reorganizarData) return;

		// Reorganizar devolve a sequência ao motor: a ordem montada à mão daquela
		// data em diante é descartada. Desfazer trabalho se pergunta antes.
		if (
			!confirm(
				`Refazer o cronograma a partir de ${reorganizarData.split('-').reverse().join('/')}? ` +
					'A ordem que você arrumou à mão daí para frente será substituída pela do motor.'
			)
		) {
			return;
		}

		reorganizando = true;
		await planoStore.reorganizarDesde(reorganizarData);
		reorganizando = false;
	}

	const semanalDesc = $derived(
		cfg?.revisaoSemanal
			? 'Ligado: um dia inteiro por semana sai do conteúdo e vira revisão. São ~11 dias de matéria nova a menos num ciclo.'
			: 'Desligado: a semana inteira é conteúdo, e a revisão acontece no bloco diário acima.'
	);





	let baixando = $state(false);
	let erroCsv = $state<string | null>(null);
	// A exportação passa pelo cliente da API, e não por um fetch próprio: o
	// access token expira em minutos, e sem a renovação o download vinha a ser o
	// corpo do 401 — um arquivo com {"erro":"não autenticado"} dentro.
	async function baixarCsv() {
		if (baixando) return;

		baixando = true;
		erroCsv = null;

		try {
			const csv = await planoStore.exportarCsv();
			const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `plano-${concursoStore.ativoSlug ?? 'concurso'}.csv`;
			a.click();
			URL.revokeObjectURL(url);
		} catch (e) {
			erroCsv = e instanceof Error ? e.message : 'não consegui exportar agora';
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
					Todo dia de estudo termina com um bloco de revisão, que cobra questões
					dos assuntos que você errou — o seu caderno de erros, por matéria.
				</p>

				<Ajuste
					titulo="Duração do bloco de revisão"
					descricao="Zero remove o bloco: o dia passa a ser só conteúdo."
					para="pf-rev"
					unidade="minutos"
				>
					{#snippet controle()}
						<input
							id="pf-rev"
							type="number"
							min="0"
							max="120"
							step="5"
							value={cfg.minutosRevisao}
							onchange={(e) =>
								salvar({ minutosRevisao: parseInt((e.target as HTMLInputElement).value, 10) })}
							style="width:100px"
						/>
					{/snippet}
				</Ajuste>

				<Ajuste
					titulo="Mostrar o bloco de revisão no cronograma"
					descricao={mostrarDesc}
					para="pf-revmostrar"
				>
					{#snippet controle()}
						<input
							id="pf-revmostrar"
							type="checkbox"
							class="checkbox"
							checked={cfg.minutosRevisao > 0}
							onchange={(e) =>
								salvar({ minutosRevisao: e.currentTarget.checked ? ultimaDuracao : 0 })}
						/>
					{/snippet}
				</Ajuste>

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
					dificuldade — ela aparece em mais dias. O tamanho do bloco continua o que você definiu
					em “Minutos por bloco”.
				</p>
				<div class="modos">
					{#each disciplinas as d (d.codigo)}
						<div class="modo-linha">
							<span class="modo-nome">
								<span class="chip-dot" style="background:var(--c{d.cor}-tx)"></span>
								<span class="modo-nome-txt">{d.nome}</span>
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

				<h2 class="sec">Reordenação manual</h2>
				<p class="page-sub" style="margin-top:0">
					No Cronograma, cada matéria é movida sozinha: as setas <b>↑ ↓</b> trocam a
					matéria de lugar dentro do dia e, na borda dele, a levam para o dia
					vizinho. O dia inteiro não é movido — só as matérias dentro dele. Uma
					matéria já concluída não se move, e dia fixo não recebe conteúdo.
				</p>
				<p class="page-sub" style="margin-top:0">
					Adiantar uma matéria já fecha sozinho o vão que ela deixa: o que vinha
					depois sobe uma vaga, na mesma ordem. <b>Compactar</b> faz isso no
					cronograma inteiro, de hoje em diante — é para arrumar vãos antigos, e
					também mantém a sua ordem. O tempo livre passa a se acumular no fim, onde
					cabe mais conteúdo antes da prova.
				</p>
				<div class="form-grid">
					<button class="btn" onclick={compactar} disabled={compactando}>
						{compactando ? 'Compactando…' : '⇡ Compactar o cronograma'}
					</button>
					<button class="btn" disabled={!plano.temMovimentacaoManual} onclick={restaurar}>
						↺ Restaurar ordem automática
					</button>
					<span class="page-sub" style="margin:0">
						{plano.temMovimentacaoManual
							? 'Há matérias que você reposicionou à mão.'
							: 'Nenhuma troca manual ainda.'}
					</span>
				</div>

				<p class="page-sub" style="margin:14px 0 8px">
					Registrou dias que já passaram e quer o cronograma <b>refeito</b> dali para
					a frente? Escolha a data: o que tem registro fica onde está, e o resto é
					redistribuído pelos dias que sobram até a prova. Aqui quem decide a
					sequência é o motor — se você arrumou a ordem à mão, ela é refeita.
					Para só fechar vãos sem perder a sua ordem, use Compactar.
				</p>
				<div class="form-grid">
					<input
						type="date"
						aria-label="Reorganizar a partir de"
						bind:value={reorganizarData}
						min={cfg.inicio}
						max={cfg.prova}
					/>
					<button
						class="btn"
						onclick={reorganizar}
						disabled={reorganizando || !reorganizarData}
					>
						{reorganizando ? 'Reorganizando…' : '⇅ Reorganizar dali em diante'}
					</button>
				</div>

				<h2 class="sec">Este concurso</h2>
				<div class="form-grid">
					<a class="btn" href="/concursos/{plano.concurso.slug}/editar"
						>✏️ Editar disciplinas e datas</a
					>
					<a class="btn" href="/concursos">Trocar de concurso</a>
				</div>

				<h2 class="sec">Dados</h2>
				{#if erroCsv}<div class="form-error" style="margin-bottom:12px">{erroCsv}</div>{/if}
				<div class="form-grid">
					<button class="btn" onclick={baixarCsv} disabled={baixando}>
						{baixando ? 'Exportando…' : '⬇ Exportar CSV'}
					</button>
					<button class="btn danger" onclick={limpar}>Limpar registros</button>
				</div>
				<p class="page-sub" style="margin-top:14px">
					Seus dados ficam salvos no servidor, ligados à sua conta ({auth.usuario?.email}). O CSV é
					uma cópia de segurança que você pode abrir no Excel — e é por ele que o histórico volta.
				</p>

				<ImportarCSV />
			</div>
		</div>
	</div>
{/if}

<style>
	.modos-t {
		font-size: 13px;
		font-weight: 600;
		margin: 20px 0 8px;
	}
	/* Cada matéria é um bloco de duas linhas: o NOME ocupa a largura toda em
	   cima, os dois grupos de botões vêm embaixo.
	   
	   Antes o nome dividia a linha com os oito botões e sobravam ~200px para
	   ele — "Engenharia de Software Assistida por Inteligência Artificial"
	   virava "E...", e o estudante não sabia em que matéria estava clicando.
	   
	   O nome quebra a linha sozinho (`flex-basis: 100%`) em vez de esticar
	   colunas de grid: assim um nome comprido não empurra o grupo de reforço
	   para a direita só naquela matéria. Como os rótulos dos botões são os
	   mesmos em toda linha, os dois grupos ficam alinhados entre as matérias. */
	.modos {
		display: flex;
		flex-direction: column;
	}
	.modo-linha {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 5px 10px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
	}
	.modo-linha:last-child {
		border-bottom: none;
	}
	.modo-nome {
		flex: 0 0 100%;
		display: flex;
		align-items: baseline;
		gap: 6px;
		min-width: 0;
		font-size: 13.5px;
	}
	.modo-nome-txt {
		min-width: 0;
		font-weight: 500;
	}
	.reforco button[aria-pressed='true'] {
		background: var(--warn-soft);
		color: var(--warn);
	}
	.modo-linha .day-sel button {
		white-space: nowrap;
	}

	/* Sem largura nem para um grupo inteiro, os botões dele também quebram. */
	@media (max-width: 560px) {
		.modo-linha .day-sel {
			flex-wrap: wrap;
		}
	}
</style>
