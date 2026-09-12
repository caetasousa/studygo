<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import DiaCard from '$lib/components/DiaCard.svelte';
	import EmentaModal from '$lib/components/EmentaModal.svelte';
	import TemaTexto from '$lib/components/TemaTexto.svelte';
	import { linkQuestoes } from '$lib/tec';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, fmtDuracao, hojeISO, diffDays, nf1 } from '$lib/format';

	/**
	 * A tela Hoje.
	 *
	 * O dia é desenhado pelo MESMO DiaCard do cronograma. Antes esta página
	 * tinha uma terceira representação do mesmo dia — título grande, lista de
	 * blocos e uma grade de lançamento —, o que empilhava o mesmo conteúdo três
	 * vezes e ainda deixava de fora o que só o cronograma sabia fazer: mover a
	 * atividade, adiar o dia, registrar a revisão, abrir a ementa. Uma
	 * representação só resolve as duas queixas de uma vez.
	 *
	 * O que é exclusivo daqui fica em volta: o link do caderno de questões do
	 * assunto do dia, e o contexto curto à direita.
	 */
	const plano = $derived(planoStore.plano);
	const idx = $derived(plano?.hojeIndex ?? null);
	const diaAtual = $derived(idx !== null && plano ? plano.dias[idx] : null);
	const ehHoje = $derived(diaAtual?.data === hojeISO());

	/** Três dias, não seis: o resto do plano é a tela do cronograma. */
	const proximos = $derived(idx !== null && plano ? plano.dias.slice(idx + 1, idx + 4) : []);

	/** As três matérias mais atrasadas em relação ao peso delas na prova. */
	const deficit = $derived.by(() => {
		if (!plano || plano.props.horasTotal === 0) return [];
		return [...plano.balanceamento].sort((a, b) => a.desvio - b.desvio).slice(0, 3);
	});

	const proxMarco = $derived.by(() => {
		if (!plano) return null;
		const h = hojeISO();
		return plano.marcos.find((m) => (m.dataFim ?? m.dataInicio) >= h) ?? null;
	});

	/**
	 * O ritmo do dia numa linha: quantos blocos, de quanto, e a meta de questões.
	 *
	 * Substitui a lista de blocos, que repetia a mesma instrução uma vez por
	 * matéria — três linhas idênticas dizendo "teoria com resumo + 8 questões".
	 */
	const ritmo = $derived.by(() => {
		if (!diaAtual) return '';

		const conteudo = diaAtual.blocos.filter((b) => !b.titulo.startsWith('Revisão'));
		const revisao = diaAtual.blocos.find((b) => b.titulo.startsWith('Revisão'));
		const partes: string[] = [];

		if (conteudo.length > 0) {
			const min = conteudo[0].minutos;
			const iguais = conteudo.every((b) => b.minutos === min);
			partes.push(
				iguais
					? `${conteudo.length} ${conteudo.length === 1 ? 'bloco' : 'blocos'} de ${fmtDuracao(min)}`
					: `${conteudo.length} blocos`
			);
		}

		if (revisao) partes.push(`${fmtDuracao(revisao.minutos)} de revisão`);
		if (diaAtual.meta > 0) partes.push(`meta de ${diaAtual.meta} questões`);

		return partes.join(' · ');
	});

	/** O caderno de questões da banca já filtrado pelo assunto de cada linha. */
	function tecDe(codigo: string, tema: string): string {
		return linkQuestoes(
			plano?.concurso.disciplinas.find((d) => d.codigo === codigo),
			tema
		);
	}

	// --- o conteúdo programático da matéria ---------------------------------
	let ementa = $state<{ codigo: string; tema: string } | null>(null);
	let gatilho: HTMLElement | null = null;

	function abrirMateria(codigo: string, tema: string) {
		gatilho = document.activeElement as HTMLElement | null;
		ementa = { codigo, tema };
	}

	function fecharEmenta() {
		ementa = null;
		gatilho?.focus();
		gatilho = null;
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
		<div class="coluna">
			{#if planoStore.erro}
				<div class="form-error" role="status" aria-live="polite">{planoStore.erro}</div>
			{/if}

			{#if diaAtual}
				<!-- UMA linha de contexto. O cartão abaixo já traz a data, o número
				     do dia e quantas atividades ele tem; repetir isso aqui em cima
				     era metade da poluição da tela antiga. -->
				<p class="contexto">
					{#if !ehHoje}<b>Próximo dia de estudo</b> ·{/if}
					{diaAtual.fase === 'reta' ? 'Reta final' : 'Ciclo de conteúdo'} · semana
					{diaAtual.semana}{#if ritmo} · {ritmo}{/if}
				</p>

				<!-- Sem as setas de subir e descer: aqui você está estudando, não
				     remanejando. Um clique sem querer numa delas reordena o plano
				     inteiro, e o lugar de rearrumar é o cronograma. -->
				<DiaCard
					dia={diaAtual}
					movivel={false}
					onAbrirMateria={abrirMateria}
					{tecDe}
				/>
			{:else}
				<div class="card">
					<div class="card-top"><span class="pill muted">Ciclo encerrado</span></div>
					<div class="card-body"><h2 class="tema-grande">O plano chegou ao fim.</h2></div>
				</div>
			{/if}
		</div>

		<div class="side-cards">
			<div class="card">
				<div class="card-top">
					Depois de hoje
					<a class="ver-tudo" href="/cronograma">ver tudo</a>
				</div>
				<div class="card-body">
					<ul class="prox-list">
						{#each proximos as d (d.data)}
							<li>
								<span class="dt">{fc(d.data)}</span>
								<span class="prox-tema">
									{#if d.itens.length === 0}
										<TemaTexto tema={d.tema} limite={1} />
									{:else}
										{d.itens.map((it) => it.disciplina).join(' · ')}
									{/if}
								</span>
							</li>
						{/each}
					</ul>
				</div>
			</div>

			<div class="card">
				<div class="card-top">
					Onde você está devendo
					<a class="ver-tudo" href="/balanceamento">ver tudo</a>
				</div>
				<div class="card-body">
					{#if deficit.length === 0}
						Lance suas primeiras horas para o painel comparar o tempo aplicado com o peso de cada
						matéria.
					{:else}
						<ul class="deficit">
							{#each deficit as x (x.codigo)}
								<li>
									<span class="chip-dot" style="background:var(--c{x.cor}-tx)"></span>
									<span class="dv-nome">{x.nome}</span>
									<b class="dv-num" class:ruim={x.desvio < 0}>
										{x.desvio >= 0 ? '+' : ''}{nf1.format(x.desvio)} p.p.
									</b>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			</div>

			{#if proxMarco}
				<div class="card">
					<div class="card-top">Próxima data do edital</div>
					<div class="card-body marco">
						<b>{fc(proxMarco.dataInicio)}{proxMarco.dataFim ? ' a ' + fc(proxMarco.dataFim) : ''}</b>
						<span>em {diffDays(hojeISO(), proxMarco.dataInicio)} dias · {proxMarco.titulo}</span>
					</div>
				</div>
			{/if}
		</div>
	</div>

	{#if ementa}
		<EmentaModal codigo={ementa.codigo} tema={ementa.tema} onclose={fecharEmenta} />
	{/if}
{/if}

<style>
	.coluna {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-width: 0;
	}
	/* O ritmo e a fase do dia numa linha só, no lugar da lista de blocos que
	   repetia a mesma instrução uma vez por matéria. */
	.contexto {
		margin: 0 0 2px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.contexto b {
		color: var(--warn);
		font-weight: 600;
	}
	.prox-list .prox-tema {
		color: var(--text);
		line-height: 1.35;
		min-width: 0;
	}
	.deficit {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.deficit li {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 8px;
		padding: 4px 0;
		font-size: 13px;
	}
	.dv-nome {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dv-num {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--good);
		font-variant-numeric: tabular-nums;
	}
	.dv-num.ruim {
		color: var(--danger);
	}
	.ver-tudo {
		margin-left: auto;
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text-faint);
		white-space: nowrap;
	}
	.ver-tudo:hover {
		color: var(--accent);
	}
	.marco {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-size: 13px;
	}
	.marco span {
		color: var(--text-muted);
	}
</style>
