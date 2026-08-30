<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import DiaLog from '$lib/components/DiaLog.svelte';
	import RevisoesDoDia from '$lib/components/RevisoesDoDia.svelte';
	import TemaTexto from '$lib/components/TemaTexto.svelte';
	import { linkQuestoes } from '$lib/tec';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, fl, hojeISO, diffDays, nf1, rotulo, tagStyle } from '$lib/format';
	import type { Dia } from '$lib/types';

	const plano = $derived(planoStore.plano);
	const disc = $derived(planoStore.discIndex);
	const idx = $derived(plano?.hojeIndex ?? null);
	const diaAtual = $derived(idx !== null && plano ? plano.dias[idx] : null);
	const ehHoje = $derived(diaAtual?.data === hojeISO());

	const proximos = $derived(idx !== null && plano ? plano.dias.slice(idx + 1, idx + 7) : []);

	const deficit = $derived.by(() => {
		if (!plano || plano.props.horasTotal === 0) return [];
		return [...plano.balanceamento].sort((a, b) => a.desvio - b.desvio).slice(0, 4);
	});

	const proxMarco = $derived.by(() => {
		if (!plano) return null;
		const h = hojeISO();
		return plano.marcos.find((m) => (m.dataFim ?? m.dataInicio) >= h) ?? null;
	});

	/** Linhas de um dia na lista "Próximos dias": uma por bloco, com a
	 *  disciplina separada do tema para o texto não virar um parágrafo corrido. */
	function linhasDoDia(d: Dia): { rotulo: string; tema: string }[] {
		if (d.itens.length === 0) return [{ rotulo: rotulo(d.tipo), tema: d.tema }];
		return d.itens.map((it) => ({ rotulo: it.disciplina, tema: it.tema }));
	}
</script>

<PageHead
	icone="hoje"
	titulo="Hoje"
	sub="O que estudar agora, com o tempo de cada matéria calculado pelo peso dela na prova."
/>

{#if planoStore.carregando && !plano}
	<p class="page-sub">Carregando…</p>
{:else if plano}
	<div class="page hoje-grid">
		<div>
			<div class="card">
				{#if diaAtual}
					{@const cor = diaAtual.itens.length ? disc[diaAtual.itens[0].disciplina]?.cor ?? 0 : null}
					{@const ehRev = diaAtual.tipo === 'rev'}
					<div class="card-top">
						<span
							class="pill"
							style={ehRev
								? 'background:var(--warn)'
								: cor !== null
									? `background:var(--c${cor}-tx)`
									: ''}
						>
							{ehHoje ? 'Dia ' : 'Próximo · dia '}{String(diaAtual.n).padStart(3, '0')}
						</span>
						<span>
							{fl(diaAtual.data)} · {diaAtual.fase === 'reta' ? 'Reta final' : 'Ciclo de conteúdo'} ·
							semana {diaAtual.semana} · {nf1.format(plano.config.horasDia)} h · faltam
							{diffDays(diaAtual.data, plano.config.prova)} dias
						</span>
					</div>
					<div class="card-body">
						{#if diaAtual.itens.length === 0}
							<div class="disc-label" style="color:{ehRev ? 'var(--warn)' : 'var(--accent)'}">
								{rotulo(diaAtual.tipo)}
							</div>
							<h2 class="tema-grande">{diaAtual.tema}</h2>
							{#if ehRev && diaAtual.meta > 0}
								<p class="rev-meta">Meta do dia: <b>{diaAtual.meta} questões</b></p>
							{/if}
						{:else}
							{#each diaAtual.itens as it, i (i)}
								<div class="disc-label" style="color:var(--c{disc[it.disciplina]?.cor ?? 0}-tx)">
									{i === 0 ? '1º bloco' : '2º bloco'} · {disc[it.disciplina]?.nome ?? it.disciplina}{it.passada ===
									2
										? ' — 2ª passada'
										: ''}
								</div>
								<h2 class="tema-grande">{it.tema}</h2>
								{@const tec = linkQuestoes(
									plano.concurso.disciplinas.find((d) => d.codigo === it.disciplina),
									it.tema
								)}
								{#if tec}
									<p class="tec-link">
										<a class="btn" href={tec} target="_blank" rel="noopener noreferrer">
											Abrir no TEC ↗
										</a>
									</p>
								{/if}
							{/each}
						{/if}

						<div class="blocos">
							{#each diaAtual.blocos as b (b.titulo)}
								<div class="bloco">
									<span class="t">{b.minutos} min</span>
									<span class="d"><strong>{b.titulo}</strong><span>{b.detalhe}</span></span>
								</div>
							{/each}
						</div>

						{#if ehHoje}
							<DiaLog dia={diaAtual} variant="card" />
						{/if}
					</div>
				{:else}
					<div class="card-top"><span class="pill muted">Ciclo encerrado</span></div>
					<div class="card-body"><h2 class="tema-grande">O plano chegou ao fim.</h2></div>
				{/if}
			</div>
		</div>

		<div class="side-cards">
			<RevisoesDoDia revisoes={diaAtual?.revisoes ?? []} />

			<div class="card">
				<div class="card-top">Próximos dias</div>
				<div class="card-body">
					<ul class="prox-list">
						{#each proximos as d (d.data)}
							<li>
								<span class="dt">{fc(d.data)}</span>
								<span class="prox-blocos">
									{#each linhasDoDia(d) as l, i (i)}
										<span class="prox-bloco">
											<span class="prox-rot">{l.rotulo}</span>
											<span class="prox-tema"><TemaTexto tema={l.tema} limite={2} /></span>
										</span>
									{/each}
								</span>
							</li>
						{/each}
					</ul>
				</div>
			</div>

			<div class="card">
				<div class="card-top">Onde você está devendo</div>
				<div class="card-body">
					{#if deficit.length === 0}
						Lance suas primeiras horas para o painel comparar o tempo aplicado com o peso de cada
						matéria.
					{:else}
						{#each deficit as x (x.codigo)}
							<div>
								<span class="chip-dot" style="background:var(--c{x.cor}-tx)"></span>{x.nome}<br />
								peso {nf1.format(x.pctIdeal)}% · seu tempo {nf1.format(
									x.pctIdeal + x.desvio
								)}% ·
								<b style="color:{x.desvio < 0 ? 'var(--danger)' : 'var(--good)'}">
									{x.desvio >= 0 ? '+' : ''}{nf1.format(x.desvio)} p.p.
								</b>
							</div>
							<div style="height:8px"></div>
						{/each}
					{/if}
				</div>
			</div>

			<div class="card">
				<div class="card-top">Próxima data do edital</div>
				<div class="card-body">
					{#if proxMarco}
						<div>
							<b>{fc(proxMarco.dataInicio)}{proxMarco.dataFim ? ' a ' + fc(proxMarco.dataFim) : ''}</b>
							· em {diffDays(hojeISO(), proxMarco.dataInicio)} dia(s)<br />
							{proxMarco.titulo}
						</div>
					{:else}
						Todas as datas do cronograma já passaram.
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.tec-link {
		margin: -4px 0 14px;
	}
	.rev-meta {
		margin: -6px 0 12px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.rev-meta b {
		color: var(--warn);
	}
	.prox-blocos {
		display: flex;
		flex-direction: column;
		gap: 5px;
		min-width: 0;
	}
	.prox-bloco {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}
	.prox-rot {
		font-family: var(--font-mono);
		font-size: 9.5px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--text-faint);
		font-weight: 600;
	}
	.prox-tema {
		color: var(--text);
		line-height: 1.35;
	}
</style>
