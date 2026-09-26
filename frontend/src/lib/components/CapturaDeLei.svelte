<script lang="ts">
	import { onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import type { CapturaDeLei, PublicacaoDeLei } from '$lib/types';

	/**
	 * Captura de uma lei pela fonte oficial: o link vai para o processador, a
	 * prévia volta, e a pessoa decide se publica.
	 *
	 * A prévia não traz o texto: traz o que precisa de olho humano. Bloqueio
	 * impede a publicação; aviso (o Gemini discordou da regra, a numeração
	 * salta) só sai do caminho marcado como revisado. Quem confere de novo é o
	 * servidor — o botão desabilitado aqui é conveniência, não a regra.
	 *
	 * Com `atual`, é a atualização do texto daquela lei: o link e os nomes vêm
	 * preenchidos, e as questões dela continuam.
	 */
	interface Props {
		atual?: { slug: string; nome: string; curto: string; fonte: string; reconhecer: string[] };
		aoPublicar: (r: PublicacaoDeLei) => void;
	}

	let { atual, aoPublicar }: Props = $props();

	const ETAPAS: Record<string, string> = {
		'na fila': 'Na fila',
		baixando: 'Baixando da fonte',
		organizando: 'Separando o texto vigente, a redação anterior e as notas',
		classificando: 'Conferindo a estrutura com o Gemini',
		verificando: 'Conferindo o texto com o original'
	};

	// svelte-ignore state_referenced_locally
	let link = $state(atual?.fonte ?? '');
	// svelte-ignore state_referenced_locally
	let nome = $state(atual?.nome ?? '');
	// svelte-ignore state_referenced_locally
	let curto = $state(atual?.curto ?? '');
	// svelte-ignore state_referenced_locally
	let reconhecer = $state((atual?.reconhecer ?? []).join(', '));

	let captura = $state<CapturaDeLei | null>(null);
	let aceitos = $state<string[]>([]);
	let erro = $state<string | null>(null);
	let enviando = $state(false);
	let espera: ReturnType<typeof setTimeout> | undefined;

	onDestroy(() => clearTimeout(espera));

	const resultado = $derived(captura?.estado === 'pronta' ? captura.resultado : null);
	const pendentes = $derived((resultado?.avisos ?? []).filter((a) => !aceitos.includes(a.id)).length);
	const podePublicar = $derived(
		!!resultado?.publicavel && pendentes === 0 && nome.trim() !== '' && curto.trim() !== '' && !enviando
	);

	async function consultar(id: string) {
		try {
			captura = await api.capturaDeLei(id);
			if (captura.estado === 'rodando') espera = setTimeout(() => consultar(id), 1500);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível consultar a captura';
			captura = null;
		}
	}

	async function capturar(e: SubmitEvent) {
		e.preventDefault();
		clearTimeout(espera);
		erro = null;
		captura = null;
		aceitos = [];
		enviando = true;
		try {
			const { id } = await api.capturarLei(link.trim());
			await consultar(id);
		} catch (err) {
			erro = err instanceof Error ? err.message : 'Não foi possível capturar a lei';
		} finally {
			enviando = false;
		}
	}

	async function publicar() {
		if (!captura) return;
		erro = null;
		enviando = true;
		try {
			const r = await api.publicarLei(captura.id, {
				slug: atual?.slug,
				nome: nome.trim(),
				curto: curto.trim(),
				reconhecer: reconhecer
					.split(',')
					.map((x) => x.trim())
					.filter(Boolean),
				aceitos
			});
			captura = null;
			if (!atual) {
				link = nome = curto = reconhecer = '';
			}
			aoPublicar(r);
		} catch (err) {
			erro = err instanceof Error ? err.message : 'A publicação falhou';
		} finally {
			enviando = false;
		}
	}

	function marcar(id: string, marcado: boolean) {
		aceitos = marcado ? [...aceitos, id] : aceitos.filter((x) => x !== id);
	}
</script>

<form class="captura" onsubmit={capturar}>
	<div class="field link">
		<label for="link-lei">Link da lei na fonte oficial</label>
		<input
			id="link-lei"
			type="text"
			inputmode="url"
			placeholder="https://www.planalto.gov.br/ccivil_03/…"
			bind:value={link}
			disabled={enviando || captura?.estado === 'rodando'}
		/>
	</div>
	<button class="btn" type="submit" disabled={!link.trim() || enviando || captura?.estado === 'rodando'}>
		{atual ? 'Capturar de novo' : 'Capturar'}
	</button>
</form>
<p class="dica">
	Planalto, Casa Civil de Goiás (o link da página da lei) ou outro site .gov.br, .leg.br ou .jus.br.
</p>

{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

{#if captura?.estado === 'rodando'}
	<p class="andamento" role="status">
		Capturando… {ETAPAS[captura.etapa] ?? captura.etapa}{captura.progresso.total > 1
			? ` (${captura.progresso.feitos} de ${captura.progresso.total})`
			: ''}. A Constituição leva uns quatro minutos; pode esperar aqui.
	</p>
{:else if captura?.estado === 'falhou'}
	<div class="form-error" role="alert">{captura.erro}</div>
{:else if resultado}
	<section class="previa" aria-labelledby="previa-t">
		<h3 id="previa-t">Prévia da captura</h3>
		<p class="resumo">
			{resultado.artigos} artigos, {resultado.dispositivos} dispositivos{resultado.resumo.revogados
				? `, ${resultado.resumo.revogados} revogados`
				: ''} ·
			<a href={resultado.fonte} target="_blank" rel="noopener noreferrer">fonte</a> ·
			{resultado.gemini ? 'estrutura conferida pelo Gemini' : 'sem conferência do Gemini'}
		</p>

		{#if resultado.bloqueios.length > 0}
			<div class="form-error" role="alert">
				<strong>Esta captura não pode ser publicada:</strong>
				<ul>
					{#each resultado.bloqueios as b (b)}<li>{b}</li>{/each}
				</ul>
			</div>
		{:else}
			{#if resultado.avisos.length > 0}
				<fieldset class="avisos">
					<legend>Revise antes de publicar ({pendentes} de {resultado.avisos.length} pendentes)</legend>
					{#each resultado.avisos as a (a.id)}
						<label class="aviso">
							<input
								type="checkbox"
								checked={aceitos.includes(a.id)}
								onchange={(e) => marcar(a.id, e.currentTarget.checked)}
							/>
							<span>
								Revisei: {a.texto}
								{#if a.trecho}<q>{a.trecho}</q>{/if}
							</span>
						</label>
					{/each}
				</fieldset>
			{/if}

			{#if resultado.sumario.length > 0}
				<details>
					<summary>Estrutura ({resultado.sumario.length} divisões)</summary>
					<ul class="sumario">
						{#each resultado.sumario as s (s.ref)}
							<li class="t-{s.tipo}">{s.rotulo}{s.nome ? ` — ${s.nome}` : ''}</li>
						{/each}
					</ul>
				</details>
			{/if}
			{#if resultado.resumo.descartados.length + resultado.resumo.riscados.length + resultado.resumo.juncoes.length > 0}
				<details>
					<summary>O que a captura deixou de fora ou emendou</summary>
					<ul class="miudo">
						{#each resultado.resumo.descartados as d (d)}<li>Cabeçalho do site, descartado: {d}</li>{/each}
						{#each resultado.resumo.riscados as r (r)}<li>Riscado no meio do texto vigente: {r}</li>{/each}
						{#each resultado.resumo.juncoes as j (j)}<li>Hifenização desfeita (PDF): {j}</li>{/each}
					</ul>
				</details>
			{/if}

			<div class="form-grid campos">
				<div class="field largo">
					<label for="nome-lei">Nome da lei</label>
					<input id="nome-lei" type="text" bind:value={nome} placeholder="Lei Geral de Proteção de Dados — Lei nº 13.709/2018" />
				</div>
				<div class="field">
					<label for="curto-lei">Nome curto</label>
					<input id="curto-lei" type="text" bind:value={curto} placeholder="LGPD" />
				</div>
				<div class="field largo">
					<label for="reconhecer-lei">Sugerir para a matéria cujo tópico cite</label>
					<input id="reconhecer-lei" type="text" bind:value={reconhecer} placeholder="vazio = o número da lei no nome (13.709)" />
				</div>
			</div>
			<button class="btn primary" type="button" disabled={!podePublicar} onclick={publicar}>
				{atual ? 'Publicar o texto novo' : 'Publicar lei'}
			</button>
		{/if}
	</section>
{/if}

<style>
	.captura {
		display: flex;
		gap: 10px;
		align-items: flex-end;
		flex-wrap: wrap;
	}
	.link {
		flex: 1;
		min-width: min(100%, 320px);
	}
	.link input {
		width: 100%;
	}
	.dica,
	.resumo,
	.andamento {
		color: var(--text-muted);
		font-size: 12.5px;
		margin: 6px 0 0;
	}
	.andamento {
		margin-top: 12px;
	}
	.previa {
		margin-top: 16px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.previa h3 {
		font-size: 14px;
		margin: 0;
	}
	.avisos {
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px 12px 10px;
		margin: 12px 0;
	}
	.avisos legend {
		font-size: 12px;
		font-weight: 600;
		color: var(--warn);
		padding: 0 4px;
	}
	.aviso {
		display: flex;
		gap: 8px;
		align-items: flex-start;
		font-size: 13px;
		padding: 4px 0;
	}
	.aviso q {
		display: block;
		color: var(--text-muted);
		font-family: var(--font-serif, serif);
		margin-top: 2px;
	}
	details {
		margin: 8px 0;
		font-size: 13px;
	}
	summary {
		cursor: pointer;
		color: var(--text-muted);
	}
	.sumario,
	.miudo {
		margin: 6px 0 0;
		padding-left: 18px;
		max-height: 260px;
		overflow: auto;
	}
	.sumario .t-capitulo {
		margin-left: 10px;
	}
	.sumario .t-secao,
	.sumario .t-subsecao {
		margin-left: 20px;
	}
	.miudo {
		color: var(--text-muted);
		font-size: 12px;
	}
	.campos {
		margin: 12px 0;
	}
	.largo {
		flex: 1;
		min-width: min(100%, 280px);
	}
	.largo input {
		width: 100%;
	}
	.form-error ul {
		margin: 6px 0 0;
		padding-left: 18px;
	}
</style>
