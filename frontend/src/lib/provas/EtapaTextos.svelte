<script lang="ts">
	import { untrack } from 'svelte';
	import BarraDeDestaque from './BarraDeDestaque.svelte';
	import Campo from './Campo.svelte';
	import CampoDeTexto from './CampoDeTexto.svelte';
	import ListaDeFiguras, { figurasDoCampo } from './ListaDeFiguras.svelte';
	import PreviaDoOriginal from './PreviaDoOriginal.svelte';
	import Recorte from './Recorte.svelte';
	import {
		aplicarRecorteNoApoio,
		escreverNumeros,
		lerNumeros,
		novoApoio,
		numerosDoAviso,
		regiaoDoApoio,
		regiaoDoCaderno,
		regiaoDoTrechoDeApoio,
		removerApoio,
		retanguloInicial,
		rotuloDaRegiao,
		rotuloDoApoio,
		trechoDeApoioInicial,
		vincularApoio
	} from './revisao';
	import type { Origem, Rascunho } from './types';

	let {
		rascunho = $bindable(),
		indice = $bindable(0),
		importacao,
		versao,
		regioes,
		onalterar,
		onabrirQuestao,
		onreler
	}: {
		rascunho: Rascunho;
		/** O texto aberto, em `rascunho.apoios`. */
		indice?: number;
		importacao: string;
		versao: number;
		regioes: Origem[];
		onalterar: () => void;
		/** Leva à questão, na etapa de questões. */
		onabrirQuestao: (numero: number) => void;
		/** Põe na fila a leitura do trecho marcado em volta do texto; o erro volta ao recorte. */
		onreler: (apoio: string, origem: Origem) => Promise<void>;
	} = $props();

	let marcar = $state<((marca: string) => void) | null>(null);
	let painel = $state<'texto' | 'regiao' | 'trecho'>('texto');
	/** A faixa do caderno em que o trecho do texto é marcado. */
	let trechoIdx = $state(0);
	const faixas = $derived(regioes.flatMap((r, i) => (regiaoDoCaderno(r) ? [i] : [])));
	let zoom = $state(100);
	let regiaoIdx = $state(0);
	/** A figura do texto sendo recortada de novo ("p:id:índice"). */
	let ajustando = $state<string | null>(null);

	const apoio = $derived(rascunho.apoios[indice]);
	const apoioId = $derived(apoio?.id);
	const ordenadas = $derived(apoio ? [...apoio.questoes].sort((a, b) => a - b) : []);
	/** O que a frase do caderno cita: a faixa que o texto deveria ter. */
	const doCaderno = $derived(apoio ? numerosDoAviso(apoio.aviso) : []);
	const figuras = $derived(apoio ? figurasDoCampo(apoio.blocos, `p:${apoio.id}`, 'Texto') : []);
	const existentes = $derived(new Set(rascunho.questoes.map((q) => q.numero)));

	// A faixa digitada vale só ao ligar; volta ao que está ligado quando o
	// texto muda ou quando a ligação é aplicada.
	let faixa = $state('');
	$effect(() => {
		faixa = escreverNumeros(apoio?.questoes ?? []);
	});
	const digitados = $derived(lerNumeros(faixa));
	const mudou = $derived(digitados.join() !== ordenadas.join());
	const inexistentes = $derived(digitados.filter((n) => !existentes.has(n)));

	// Trocar de texto leva o painel ao lugar dele no caderno. Só a troca: digitar
	// no texto não pode mexer no painel.
	$effect(() => {
		void apoioId;
		untrack(() => {
			regiaoIdx = regiaoDoApoio(regioes, apoio);
			trechoIdx = regiaoDoTrechoDeApoio(regioes, apoio);
			painel = apoio?.origens.length ? 'texto' : 'regiao';
			ajustando = null;
		});
	});

	function ligar(numeros: number[]) {
		if (!apoio) return;
		vincularApoio(rascunho, apoio, numeros);
		onalterar();
	}

	function adicionar() {
		rascunho.apoios = [...rascunho.apoios, novoApoio(rascunho.apoios)];
		indice = rascunho.apoios.length - 1;
		onalterar();
	}

	function remover() {
		if (!apoio || !confirm(`Remover o ${rotuloDoApoio(apoio).toLowerCase()}? Ele sai de todas as questões ligadas.`))
			return;
		removerApoio(rascunho, apoio.id);
		indice = Math.max(0, indice - 1);
		onalterar();
	}

	function conferirEProximo() {
		if (!apoio) return;
		apoio.revisado = true;
		onalterar();
		const n = rascunho.apoios.length;
		for (let passo = 1; passo < n; passo++) {
			const i = (indice + passo) % n;
			if (!rascunho.apoios[i].revisado) {
				indice = i;
				return;
			}
		}
	}

	function ajustarFigura(chave: string) {
		const origem = figuras.find((f) => f.chave === chave)?.bloco.origem;
		const i = origem ? regioes.findIndex((r) => r.regiao === origem.regiao) : -1;
		if (i >= 0) regiaoIdx = i;
		ajustando = chave;
	}
</script>

{#if rascunho.apoios.length === 0}
	<section class="card">
		<div class="card-top">Textos de apoio</div>
		<div class="card-body vazio">
			<p>A extração não achou texto compartilhado nesta prova.</p>
			<p class="ajuda">
				Se alguma questão começa com "Considere o texto…" ou "De acordo com o texto…", adicione o texto aqui
				e ligue às questões de uma vez.
			</p>
			<button class="btn" type="button" onclick={adicionar}>Adicionar texto de apoio</button>
		</div>
	</section>
{:else}
	<p class="intro">
		Texto de apoio é o que várias questões usam — em Português, quase sempre o texto das questões 1 a
		10. Confira cada um com o original e diga a quais questões ele serve: o aluno o vê dentro de cada
		uma delas.
	</p>

	<nav class="lista" aria-label="Textos de apoio">
		{#each rascunho.apoios as a, i (a.id)}
			<button
				type="button"
				class:atual={i === indice}
				class:ok={a.revisado && a.questoes.length > 0}
				class:sem={a.questoes.length === 0}
				aria-current={i === indice ? 'true' : undefined}
				onclick={() => (indice = i)}
			>
				<b>Texto {i + 1}</b>
				<span>
					{a.questoes.length ? `questões ${escreverNumeros(a.questoes)}` : 'sem questões'}
					{a.revisado && a.questoes.length ? '· conferido' : ''}
				</span>
			</button>
		{/each}
		<button type="button" class="novo" onclick={adicionar}>+ Adicionar texto</button>
	</nav>

	{#if apoio}
		<div class="duas-colunas">
			<section class="card">
				<div class="card-top">
					<span class="pill" class:muted={!apoio.revisado}>Texto {indice + 1} de {rascunho.apoios.length}</span>
					<span>{rotuloDoApoio(apoio)}</span>
					{#if apoio.revisado}<span class="dim">conferido</span>{/if}
					{#if apoio.igualA}
						<span class="pill reaproveitado" title="O texto é o já publicado nessa prova, guardado uma vez só; editar cria uma versão só desta"
							>igual ao de {apoio.igualA}</span
						>
					{/if}
				</div>
				<div class="card-body corpo">
					<section class="passo">
						<h3>1. A quais questões ele serve</h3>
						{#if apoio.aviso}
							<figure class="caderno">
								<figcaption>O que o caderno diz</figcaption>
								<blockquote>{apoio.aviso}</blockquote>
							</figure>
						{:else}
							<p class="ajuda">
								A extração não achou a frase "Considere o texto… questões de X a Y" deste texto. Veja no
								original, ao lado, quais questões ele indica.
							</p>
						{/if}

						<Campo
							rotulo="Questões ligadas"
							ajuda="Os números das questões que usam o texto, do jeito que o caderno escreve: 1-10, ou 1-5, 8. Ao ligar, o texto aparece em todas elas de uma vez e sai das que não estão na lista."
						>
							{#snippet children({ id, ajuda })}
								<div class="ligar">
									<input
										{id}
										aria-describedby={ajuda}
										type="text"
										inputmode="numeric"
										placeholder="ex.: 1-10"
										bind:value={faixa}
										onkeydown={(e) => {
											if (e.key === 'Enter' && mudou) ligar(digitados);
										}}
									/>
									<button class="btn primary" type="button" disabled={!mudou} onclick={() => ligar(digitados)}>
										Ligar às questões
									</button>
								</div>
							{/snippet}
						</Campo>

						{#if doCaderno.length > 0 && doCaderno.join() !== ordenadas.join()}
							<div class="callout warn sugestao">
								<span>
									O caderno diz questões <b>{escreverNumeros(doCaderno)}</b>, e o texto está ligado a
									<b>{escreverNumeros(ordenadas) || 'nenhuma'}</b>.
								</span>
								<button class="btn" type="button" onclick={() => ligar(doCaderno)}>
									Ligar às questões {escreverNumeros(doCaderno)}
								</button>
							</div>
						{/if}
						{#if inexistentes.length > 0}
							<p class="callout warn">
								<span>As questões {escreverNumeros(inexistentes)} não existem neste rascunho.</span>
							</p>
						{/if}

						{#if ordenadas.length > 0}
							<div class="ligadas">
								<span class="ajuda">Aparece nas questões (clique para abrir):</span>
								{#each ordenadas as n (n)}
									<button class="chip" type="button" title="Abrir a questão {n}" onclick={() => onabrirQuestao(n)}>
										{n}
									</button>
								{/each}
							</div>
						{:else}
							<p class="callout warn">
								<span>
									Nenhuma questão usa este texto. Sem ligar, ele não aparece para o aluno — e a prova não
									pode ser publicada.
								</span>
							</p>
						{/if}
					</section>

					<section class="passo">
						<h3>2. O texto</h3>
						<p class="ajuda">
							É o que o aluno lê ao abrir o texto numa questão. Deixe só o texto e a fonte no fim; apague
							cabeçalho, instruções e começo de questão que tenham vindo junto. Compare com o original ao
							lado.
						</p>
						<BarraDeDestaque {marcar} />
						{#key apoio.id}
							<CampoDeTexto
								bind:blocos={apoio.blocos}
								rotulo={rotuloDoApoio(apoio)}
								linhas={10}
								onalterar={() => {
									apoio.revisado = false;
									onalterar();
								}}
								onfoco={(m) => (marcar = m)}
							/>
						{/key}
						{#if figuras.length > 0}
							<ListaDeFiguras {figuras} onajustarFigura={ajustarFigura} {onalterar} />
						{/if}
					</section>

					<section class="passo conferencia">
						<label class="opcao">
							<input type="checkbox" class="checkbox" bind:checked={apoio.revisado} onchange={onalterar} />
							<span>
								<b>Conferi o texto e as questões ligadas</b>
								<span class="ajuda">O texto bate com o original e está nas questões certas.</span>
							</span>
						</label>
						<button class="btn primary" type="button" onclick={conferirEProximo}>
							{rascunho.apoios.length > 1 ? 'Conferir e ir ao próximo texto' : 'Conferir'}
						</button>
					</section>

					<details class="mais">
						<summary>Mais opções</summary>
						<p class="ajuda">Remover apaga o texto e o tira de todas as questões ligadas a ele.</p>
						<button class="btn danger" type="button" onclick={remover}>Remover este texto</button>
					</details>
				</div>
			</section>

			<aside class="card lateral">
				<div class="card-top">
					{ajustando ? 'Recortar a figura do texto' : painel === 'trecho' ? 'Ler de novo · texto' : 'O texto no caderno original'}
				</div>
				<div class="card-body lateral-corpo">
					{#if ajustando && regioes[regiaoIdx]}
						<p class="ajuda">Desenhe o retângulo sobre a figura e aplique. O recorte sai do PDF original.</p>
						<Recorte
							{importacao}
							{versao}
							regiao={regioes[regiaoIdx]}
							inicial={retanguloInicial(regioes[regiaoIdx], figuras.find((f) => f.chave === ajustando)?.bloco)}
							onaplicar={(arquivo, origem) => {
								aplicarRecorteNoApoio(rascunho, ajustando!, arquivo, origem);
								ajustando = null;
								onalterar();
							}}
						/>
						<button class="btn" type="button" onclick={() => (ajustando = null)}>Voltar ao texto</button>
					{:else}
						<div class="abas" role="group" aria-label="O que mostrar">
							<button
								class="btn"
								class:primary={painel === 'texto'}
								type="button"
								disabled={!apoio.origens.length}
								title={apoio.origens.length ? '' : 'Esta importação não guardou onde o texto está'}
								onclick={() => (painel = 'texto')}
							>
								Só o texto
							</button>
							<button class="btn" class:primary={painel === 'regiao'} type="button" onclick={() => (painel = 'regiao')}>
								Região inteira
							</button>
							<button
								class="btn"
								class:primary={painel === 'trecho'}
								type="button"
								title="Para o texto que a extração leu mal: marque no caderno o texto inteiro, e a IA lê só esse trecho"
								onclick={() => (painel = 'trecho')}
							>
								Ler de novo
							</button>
						</div>
						{#if painel !== 'trecho'}
							<label class="zoom">
								Zoom
								<input type="range" min="60" max="220" step="10" bind:value={zoom} />
								<span>{zoom}%</span>
							</label>
						{/if}
						{#if painel === 'trecho'}
							<p class="ajuda">
								Desenhe o retângulo em volta do texto inteiro — do título, ou da frase "Considere o texto…",
								até a fonte. A IA lê só esse trecho e troca o texto; as questões ligadas ficam. Se ela se
								recusar a transcrever a obra, o texto vem do próprio PDF ou do OCR, e um aviso pede para
								conferir.
							</p>
							<Campo rotulo="Onde o texto está" ajuda="A faixa do caderno em que o texto está.">
								{#snippet children({ id, ajuda })}
									<select {id} aria-describedby={ajuda} bind:value={trechoIdx}>
										{#each faixas as i (i)}
											<option value={i}>{rotuloDaRegiao(regioes[i], i, regioes.length)}</option>
										{/each}
									</select>
								{/snippet}
							</Campo>
							{#if regioes[trechoIdx]}
								{#key trechoIdx}
									<Recorte
										{importacao}
										{versao}
										regiao={regioes[trechoIdx]}
										inicial={trechoDeApoioInicial(regioes[trechoIdx], apoio)}
										rotulo="Ler este trecho do texto"
										onmarcar={(origem) => onreler(apoio.id, origem)}
									/>
								{/key}
							{/if}
						{:else if painel === 'texto' && apoio.origens[0]}
							<PreviaDoOriginal
								{importacao}
								{versao}
								origem={apoio.origens[0]}
								{zoom}
								rotulo="Página {apoio.origens[0].pagina}"
							/>
						{:else}
							<Campo rotulo="Região do caderno" ajuda="O caderno é lido em faixas; escolha a que tem o texto.">
								{#snippet children({ id, ajuda })}
									<select {id} aria-describedby={ajuda} bind:value={regiaoIdx}>
										{#each regioes as r, i (i)}
											<option value={i}>{rotuloDaRegiao(r, i, regioes.length)}</option>
										{/each}
									</select>
								{/snippet}
							</Campo>
							{#if regioes[regiaoIdx]}
								<PreviaDoOriginal
									{importacao}
									{versao}
									origem={regioes[regiaoIdx]}
									{zoom}
									rotulo="Página {regioes[regiaoIdx].pagina} · região {regiaoIdx + 1}"
								/>
							{/if}
						{/if}
					{/if}
				</div>
			</aside>
		</div>
	{/if}
{/if}

<style>
	.intro {
		margin: 0 0 12px;
		font-size: 13.5px;
		color: var(--text-muted);
		max-width: 80ch;
	}
	.lista {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 12px;
	}
	.lista button {
		display: grid;
		gap: 2px;
		padding: 7px 12px;
		text-align: left;
		font: inherit;
		font-size: 12.5px;
		color: var(--text-muted);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 8px;
		cursor: pointer;
	}
	.lista button b {
		color: var(--text);
		font-size: 13px;
	}
	.lista button.ok {
		background: var(--good-soft);
		border-color: transparent;
	}
	.lista button.sem {
		border-color: var(--warn);
	}
	.lista button.atual {
		border-color: var(--accent);
		box-shadow: inset 0 0 0 1px var(--accent);
	}
	.lista .novo {
		align-content: center;
		border-style: dashed;
	}
	.duas-colunas {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 16px;
		align-items: start;
	}
	.corpo {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 18px;
	}
	.passo {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 8px;
	}
	h3 {
		margin: 0;
		font-size: 14px;
		font-weight: 700;
	}
	.ajuda {
		margin: 0;
		font-size: 12.5px;
		line-height: 1.45;
		color: var(--text-muted);
	}
	.caderno {
		margin: 0;
		padding: 10px 12px;
		border-left: 3px solid var(--accent);
		background: var(--accent-soft);
		border-radius: 0 6px 6px 0;
	}
	.caderno figcaption {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-muted);
	}
	.caderno blockquote {
		margin: 4px 0 0;
		font-size: 14px;
	}
	.ligar {
		display: flex;
		gap: 8px;
	}
	.ligar input {
		flex: 1;
		min-width: 0;
	}
	.sugestao {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 12px;
		align-items: center;
		margin: 0;
		font-size: 13px;
	}
	.ligadas {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
	}
	.chip {
		min-width: 32px;
		height: 26px;
		padding: 0 6px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--accent);
		background: var(--accent-soft);
		border: 0;
		border-radius: 6px;
		cursor: pointer;
	}
	.conferencia {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 16px;
		align-items: center;
		justify-content: space-between;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.opcao {
		display: flex;
		gap: 10px;
		align-items: flex-start;
		font-size: 13px;
	}
	.opcao > span {
		display: grid;
		gap: 2px;
	}
	.mais summary {
		cursor: pointer;
		font-size: 13px;
		color: var(--text-muted);
	}
	.mais {
		display: grid;
		gap: 8px;
	}
	.mais[open] {
		padding-bottom: 4px;
	}
	.lateral {
		position: sticky;
		top: 64px;
	}
	.lateral-corpo {
		display: grid;
		gap: 10px;
	}
	.abas {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.zoom {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.zoom span {
		font-family: var(--font-mono);
		min-width: 4ch;
	}
	.vazio {
		display: grid;
		gap: 8px;
		justify-items: start;
	}
	.vazio p {
		margin: 0;
	}
	.callout {
		margin: 0;
		font-size: 13px;
	}
	.dim {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	@media (max-width: 1000px) {
		.duas-colunas {
			grid-template-columns: minmax(0, 1fr);
		}
		.lateral {
			position: static;
		}
	}
	.reaproveitado {
		color: var(--accent);
		background: var(--accent-soft);
	}
</style>
