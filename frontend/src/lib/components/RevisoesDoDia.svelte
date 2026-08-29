<script lang="ts">
	import { planoStore } from '$lib/stores/plano.svelte';
	import { linkQuestoes } from '$lib/tec';
	import type { RevisaoDia } from '$lib/types';

	let { revisoes }: { revisoes: RevisaoDia[] } = $props();

	const disc = $derived(planoStore.discIndex);
	const perfil = $derived(planoStore.plano?.config.perfil);
	const disciplinas = $derived(planoStore.plano?.concurso.disciplinas ?? []);

	// A revisão em aberto: qual está sendo lançada e com quais números.
	let lancando = $state<string | null>(null);
	let questoes = $state<number | null>(null);
	let acertos = $state<number | null>(null);
	let salvando = $state(false);

	function abrir(r: RevisaoDia) {
		lancando = r.id;
		questoes = r.questoes;
		acertos = null;
	}

	async function confirmar(r: RevisaoDia) {
		if (questoes === null || acertos === null) return;
		salvando = true;
		try {
			await planoStore.registrarRevisao(r.id, questoes, Math.min(acertos, questoes));
			lancando = null;
		} finally {
			salvando = false;
		}
	}

	function pct(): number | null {
		if (questoes === null || acertos === null || questoes === 0) return null;
		return Math.round((Math.min(acertos, questoes) / questoes) * 100);
	}

	function faixa(p: number | null): string {
		if (p === null) return '';
		if (p >= 80) return 'boa';
		if (p >= 60) return 'media';
		return 'fraca';
	}

	function proximo(p: number | null, etapa: number): string {
		if (p === null || !perfil) return '';
		const prox = p >= 80 ? etapa + 1 : p >= 60 ? etapa : Math.max(0, etapa - 1);
		if (prox >= perfil.intervalos.length) return 'sai da fila — consolidado';
		return `volta em ${perfil.intervalos[prox]} ${perfil.intervalos[prox] === 1 ? 'dia' : 'dias'}`;
	}

	function link(r: RevisaoDia): string {
		return linkQuestoes(disciplinas.find((d) => d.codigo === r.disciplina), r.tema);
	}
</script>

<div class="card">
	<div class="card-top">
		🔁 Revisão espaçada
		{#if revisoes.length}<span class="cont">{revisoes.length}</span>{/if}
	</div>
	<div class="card-body">
		{#if revisoes.length === 0}
			Nada vencendo hoje. Conclua um dia e seus temas entram na fila — o primeiro retorno é em
			{perfil?.intervalos[0] ?? 1}
			{(perfil?.intervalos[0] ?? 1) === 1 ? 'dia' : 'dias'}.
		{:else}
			<p class="como">
				Resolva as questões <b>sem consultar antes</b>. Só depois confira o resumo no que errar — é
				a tentativa de lembrar que fixa, não a releitura.
			</p>

			{#each revisoes as r (r.id)}
				<div class="rev" class:atrasada={r.atraso > 0}>
					<div class="rev-topo">
						<span class="rev-disc" style="color:var(--c{disc[r.disciplina]?.cor ?? 0}-tx)">
							{disc[r.disciplina]?.nome ?? r.disciplina}
						</span>
						<span class="rev-etapa">
							{#if r.atraso > 0}
								<b class="atraso">{r.atraso}d atrasada</b>
							{:else}
								D+{r.intervalo}
							{/if}
						</span>
					</div>
					<div class="rev-tema">{r.tema}</div>

					{#if lancando === r.id}
						<div class="rev-form">
							<label>
								<span>questões</span>
								<input
									type="number"
									min="1"
									max="200"
									value={questoes ?? ''}
									oninput={(e) => (questoes = parseInt(e.currentTarget.value, 10) || null)}
								/>
							</label>
							<label>
								<span>acertos</span>
								<input
									type="number"
									min="0"
									max={questoes ?? 200}
									value={acertos ?? ''}
									oninput={(e) => {
										const v = parseInt(e.currentTarget.value, 10);
										acertos = Number.isFinite(v) ? v : null;
									}}
								/>
							</label>
							<button
								class="btn primary"
								disabled={salvando || questoes === null || acertos === null}
								onclick={() => confirmar(r)}
							>
								{salvando ? 'Salvando…' : 'Lançar'}
							</button>
						</div>
						{#if pct() !== null}
							<div class="previa {faixa(pct())}">
								{pct()}% — {proximo(pct(), r.etapa)}
							</div>
						{/if}
					{:else}
						<div class="rev-acoes">
							<button class="btn" onclick={() => abrir(r)}>
								Lancei {r.questoes} questões
							</button>
							{#if link(r)}
								<a class="btn" href={link(r)} target="_blank" rel="noopener noreferrer">
									Abrir no TEC ↗
								</a>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>
</div>

<style>
	.cont {
		font-family: var(--font-mono);
		font-size: 10px;
		font-weight: 600;
		background: var(--accent-soft);
		color: var(--accent-strong);
		padding: 1px 6px;
		border-radius: 10px;
		margin-left: 6px;
	}
	.como {
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
		margin: 0 0 12px;
		padding-bottom: 10px;
		border-bottom: 1px solid var(--border);
	}
	.rev + .rev {
		margin-top: 12px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.rev-topo {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}
	.rev-disc {
		font-size: 12px;
		font-weight: 600;
	}
	.rev-etapa {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--text-faint);
		white-space: nowrap;
	}
	.atraso {
		color: var(--danger);
	}
	.rev-tema {
		font-size: 13px;
		line-height: 1.4;
		color: var(--text);
		margin: 3px 0 8px;
	}
	.rev.atrasada .rev-tema {
		font-weight: 500;
	}
	.rev-acoes {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}
	.rev-form {
		display: flex;
		align-items: flex-end;
		gap: 8px;
		flex-wrap: wrap;
	}
	.rev-form label {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1 1 70px;
	}
	.rev-form label span {
		font-family: var(--font-mono);
		font-size: 9.5px;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.rev-form input {
		width: 100%;
		text-align: right;
		padding: 5px 6px;
		font-size: 12.5px;
	}
	.previa {
		margin-top: 8px;
		font-family: var(--font-mono);
		font-size: 11.5px;
		padding: 5px 8px;
		border-radius: 5px;
	}
	.previa.boa {
		background: var(--good-soft);
		color: var(--good);
	}
	.previa.media {
		background: var(--warn-soft);
		color: var(--warn);
	}
	.previa.fraca {
		background: var(--danger-soft);
		color: var(--danger);
	}
</style>
