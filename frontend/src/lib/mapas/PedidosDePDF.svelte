<script lang="ts">
	import { onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { confirmar } from '$lib/stores/confirmacao.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import type { PedidoDeMapa, SituacaoDoPedido } from '$lib/types';

	/**
	 * O PDF da aula que vira mapa. A tela envia o PDF; o backend o entrega ao
	 * edital-processor, que escreve o mapa com o Claude Code e a skill
	 * mapa-mental, e o mapa volta importado na conta. Aqui se acompanha a fila: o
	 * que está sendo feito, o que ficou pronto, o relatório do processador e o que
	 * falhou.
	 */
	let { aoFicarPronto }: { aoFicarPronto: () => void } = $props();

	let pedidos = $state<PedidoDeMapa[]>([]);
	/** O que falhou no envio, ao pôr de volta na fila ou ao excluir: fica até a próxima ação. */
	let erro = $state<string | null>(null);
	/** A fila que não carregou: some na próxima carga que der certo. */
	let erroDaFila = $state<string | null>(null);
	let enviando = $state<string | null>(null);
	let disciplina = $state('');

	const rotulo: Record<SituacaoDoPedido, string> = {
		na_fila: 'Na fila',
		processando: 'Processando',
		pronto: 'Pronto',
		falhou: 'Falhou'
	};

	const andando = $derived(pedidos.some((p) => p.situacao === 'na_fila' || p.situacao === 'processando'));

	let espera: ReturnType<typeof setTimeout> | null = null;

	let carregouUmaVez = false;

	async function carregar() {
		const prontosAntes = new Set(pedidos.filter((p) => p.situacao === 'pronto').map((p) => p.id));
		try {
			pedidos = (await api.pedidosDeMapa()).pedidos;
			erroDaFila = null;
		} catch (e) {
			erroDaFila = e instanceof Error ? e.message : 'Não foi possível carregar a fila de PDFs';
		}

		// Um pedido que ficou pronto trouxe um mapa novo para a lista da página.
		if (carregouUmaVez && pedidos.some((p) => p.situacao === 'pronto' && !prontosAntes.has(p.id))) {
			aoFicarPronto();
		}
		carregouUmaVez = true;

		agendar();
	}

	// Enquanto há pedido andando, a fila é consultada de tempos em tempos: o
	// processador trabalha fora daqui e não avisa.
	function agendar() {
		if (espera) clearTimeout(espera);
		espera = andando ? setTimeout(carregar, 10_000) : null;
	}

	onDestroy(() => {
		if (espera) clearTimeout(espera);
	});

	void carregar();

	async function aoEscolher(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivos = [...(e.currentTarget.files ?? [])];
		e.currentTarget.value = '';
		erro = null;

		// Um pedido por PDF, um de cada vez: o PDF é grande, e o que falha diz qual foi.
		for (const pdf of arquivos) {
			enviando = pdf.name;
			try {
				await api.pedirMapa(pdf, concursoStore.ativoSlug, disciplina || null);
			} catch (err) {
				erro = `${pdf.name}: ${err instanceof Error ? err.message : 'o envio falhou'}`;
				break;
			}
		}

		enviando = null;
		await carregar();
	}

	async function reenfileirar(p: PedidoDeMapa) {
		erro = null;
		try {
			await api.reenfileirarPedido(p.id);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível pôr o pedido na fila';
		}
		await carregar();
	}

	async function excluir(p: PedidoDeMapa) {
		const ok = await confirmar({
			titulo: `Excluir o pedido de “${p.arquivo}”?`,
			texto:
				p.situacao === 'pronto'
					? 'Sai só o pedido da lista; o mapa que ele gerou continua.'
					: 'O PDF sai do servidor e o mapa não será feito.',
			rotulo: 'Excluir',
			tom: 'perigo'
		});
		if (!ok) return;

		erro = null;
		try {
			await api.excluirPedido(p.id);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível excluir o pedido';
		}
		await carregar();
	}

	const quando = new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short' });
</script>

<details class="pedir" open>
	<summary>Criar mapa a partir do PDF da aula</summary>
	<div class="painel">
		<p class="ajuda">
			Assim que o PDF chega, o processador do servidor escreve o mapa e as questões com o Claude e os importa
			sozinho — uma aula leva de dez minutos a meia hora. Ele usa a sua assinatura do Claude, conectada em
			<a href="/config">Configurações</a>.
		</p>
		<label class="campo">
			<span>Matéria do mapa</span>
			<select bind:value={disciplina} aria-label="Matéria do mapa" disabled={enviando !== null}>
				<option value="">Sem matéria (vincular depois)</option>
				{#each mapasStore.disciplinas as d (d.disciplinaId)}
					<option value={d.disciplinaId}>{d.codigo} — {d.nome}</option>
				{/each}
			</select>
		</label>
		<label class="campo">
			<span>PDF da aula (um ou vários, até 40 MB cada)</span>
			<input
				type="file"
				accept=".pdf,application/pdf"
				multiple
				aria-label="PDF da aula"
				disabled={enviando !== null}
				onchange={aoEscolher}
			/>
		</label>
		{#if enviando}<p class="ajuda" role="status">Enviando {enviando}…</p>{/if}

	</div>
</details>

{#if erro ?? erroDaFila}<div class="form-error" role="alert">{erro ?? erroDaFila}</div>{/if}

{#if pedidos.length > 0}
	<section class="fila" aria-labelledby="fila-titulo">
		<h2 id="fila-titulo">PDFs na fila</h2>
		<ul>
			{#each pedidos as p (p.id)}
				<li class="pedido" data-situacao={p.situacao}>
					<div class="linha">
						<span class="situacao s-{p.situacao}">{rotulo[p.situacao]}</span>
						<span class="arquivo">{p.arquivo}</span>
						{#if p.materia}<span class="materia">{p.materia}</span>{/if}
						<span class="data">{quando.format(new Date(p.atualizadoEm))}</span>
					</div>
					{#if p.relatorio}
						<details class="relatorio" open={p.situacao === 'falhou'}>
							<summary>{p.situacao === 'falhou' ? 'Por que falhou' : 'Relatório do processador'}</summary>
							<pre>{p.relatorio}</pre>
						</details>
					{/if}
					<div class="acoes">
						{#if p.situacao === 'pronto' && p.mapa}
							<a class="btn" href="/mapas/{p.mapa}">Abrir o mapa</a>
						{/if}
						{#if (p.situacao === 'falhou' || p.situacao === 'processando') && p.temPdf}
							<button type="button" class="btn" onclick={() => reenfileirar(p)}>Pôr na fila de novo</button>
						{:else if p.situacao === 'falhou'}
							<span class="sem-pdf">O PDF foi apagado depois de uma semana: para tentar de novo, envie-o outra vez.</span>
						{/if}
						<button type="button" class="btn danger" onclick={() => excluir(p)}>Excluir</button>
					</div>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.pedir {
		margin-bottom: 8px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--bg-card);
	}
	.pedir summary {
		padding: 11px 14px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
	}
	.pedir summary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
		border-radius: 10px;
	}
	.painel {
		display: grid;
		gap: 12px;
		padding: 0 14px 14px;
	}
	.ajuda {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: var(--text-muted);
		max-width: 76ch;
	}
	.campo {
		display: grid;
		gap: 5px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	select {
		box-sizing: border-box;
		max-width: 100%;
		min-height: 36px;
		padding: 6px 9px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg);
		color: var(--text);
		font-size: 13.5px;
	}
	@media (pointer: coarse) {
		select {
			font-size: 16px;
			min-height: 44px;
		}
	}

	.fila {
		margin: 18px 0 8px;
	}
	.fila h2 {
		margin: 0 0 10px;
		font-size: 15px;
		font-weight: 700;
	}
	.fila ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 8px;
	}
	.pedido {
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 9px;
		background: var(--bg-card);
		min-width: 0;
	}
	.linha {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 6px 10px;
		min-width: 0;
	}
	.situacao {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		padding: 2px 7px;
		border-radius: 5px;
		background: var(--bg-soft);
		color: var(--text-muted);
	}
	.s-pronto {
		color: var(--good);
		background: var(--good-soft);
	}
	.s-falhou {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.s-processando {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.s-na_fila {
		color: var(--warn);
		background: var(--warn-soft);
	}
	.arquivo {
		font-weight: 600;
		font-size: 14px;
		overflow-wrap: anywhere;
		min-width: 0;
	}
	.materia,
	.data {
		font-size: 12px;
		color: var(--text-faint);
	}
	.relatorio {
		margin-top: 8px;
		font-size: 13px;
	}
	.relatorio summary {
		cursor: pointer;
		color: var(--text-muted);
	}
	.relatorio pre {
		margin: 6px 0 0;
		padding: 8px 10px;
		border-radius: 7px;
		background: var(--bg-soft);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		font-family: inherit;
		font-size: 13px;
		line-height: 1.5;
	}
	.sem-pdf {
		align-self: center;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.acoes {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin-top: 8px;
	}
	@media (pointer: coarse) {
		.acoes :global(.btn) {
			min-height: 44px;
		}
	}
</style>
