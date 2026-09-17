<script lang="ts">
	import BarraDeDestaque from './BarraDeDestaque.svelte';
	import Blocos from './Blocos.svelte';
	import Campo from './Campo.svelte';
	import CampoDeTexto from './CampoDeTexto.svelte';
	import ListaDeFiguras, { figurasDoCampo } from './ListaDeFiguras.svelte';
	import TextoDeApoio from './TextoDeApoio.svelte';
	import {
		LETRAS,
		citaTexto,
		comecoDoApoio,
		definirResposta,
		incompleta,
		nomesJaUsados,
		novoBloco,
		problemasDaQuestao,
		rotuloDoApoio,
		vincularApoio
	} from './revisao';
	import type { QuestaoAvulsa, Rascunho } from './types';

	let {
		rascunho = $bindable(),
		indice,
		onalterar,
		onremover,
		onajustarFigura,
		onabrirTexto,
		onproxima,
		catalogo = []
	}: {
		rascunho: Rascunho;
		indice: number;
		/** As questões já publicadas, de onde vêm os nomes de matéria e assunto sugeridos. */
		catalogo?: Pick<QuestaoAvulsa, 'disciplina' | 'assunto'>[];
		/** Algo mudou. `desfazer` diz se a mudança derruba a conferência. */
		onalterar: (desfazer: boolean) => void;
		onremover: () => void;
		/** Abre o recorte da figura (pelo destino) no painel do original. */
		onajustarFigura: (destino: string) => void;
		/** Leva ao texto de apoio, na etapa de textos. */
		onabrirTexto: (apoioId: string) => void;
		/** Vai para a próxima questão a conferir. */
		onproxima: () => void;
	} = $props();

	// Conferir é ler a questão pronta ao lado do original; editar é exceção.
	// O editor fica fechado até o curador pedir.
	let editando = $state(false);
	// A barra de destaque age no último campo em foco.
	let marcar = $state<((marca: string) => void) | null>(null);

	const q = $derived(rascunho.questoes[indice]);
	const faltaLetra = $derived(LETRAS.find((l) => !q.alternativas.some((a) => a.letra === l)));

	const usados = $derived(nomesJaUsados(q.disciplina, rascunho.questoes, catalogo));

	const figuras = $derived([
		...figurasDoCampo(q.blocos, 'q', 'Enunciado'),
		...q.alternativas.flatMap((a, i) => figurasDoCampo(a.blocos, `a:${i}`, `Alternativa ${a.letra}`))
	]);

	const ligados = $derived(rascunho.apoios.filter((a) => q.apoios.includes(a.id)));
	const livres = $derived(rascunho.apoios.filter((a) => !q.apoios.includes(a.id)));
	const semTexto = $derived(ligados.length === 0 && citaTexto(q));
	const problemas = $derived(problemasDaQuestao(q, rascunho));

	function alterar() {
		q.revisada = false;
		onalterar(true);
	}

	function alternarApoio(id: string, ligado: boolean) {
		const apoio = rascunho.apoios.find((a) => a.id === id);
		if (!apoio) return;
		const numeros = ligado ? [...apoio.questoes, q.numero] : apoio.questoes.filter((n) => n !== q.numero);
		vincularApoio(rascunho, apoio, numeros);
		onalterar(true);
	}

	function conferirEProxima() {
		q.revisada = true;
		onalterar(false);
		onproxima();
	}
</script>

{#snippet ligarTexto(rotulo: string)}
	<select
		class="ligar"
		aria-label={rotulo}
		onchange={(e) => {
			if (e.currentTarget.value) alternarApoio(e.currentTarget.value, true);
			e.currentTarget.value = '';
		}}
	>
		<option value="">{rotulo}</option>
		{#each livres as a (a.id)}
			<option value={a.id}>{rotuloDoApoio(a)} — {comecoDoApoio(a, 45)}</option>
		{/each}
	</select>
{/snippet}

<div class="questao">
	<div class="classificacao">
		<Campo
			rotulo="Matéria"
			ajuda="É por ela que o aluno filtra a prova. Ao digitar, aparecem as matérias já usadas — use o mesmo nome para agrupar."
		>
			{#snippet children({ id, ajuda })}
				<input
					{id}
					aria-describedby={ajuda}
					type="text"
					list="materias-usadas"
					bind:value={q.disciplina}
					oninput={alterar}
				/>
				<datalist id="materias-usadas">
					{#each usados.materias as m (m)}<option value={m}></option>{/each}
				</datalist>
			{/snippet}
		</Campo>
		<!-- Classificar não mexe no que a questão diz: não desfaz a conferência. -->
		<Campo
			rotulo="Assunto"
			ajuda="O tema dentro da matéria, para o treino. Aparecem os assuntos que essa matéria já tem nas provas publicadas."
		>
			{#snippet children({ id, ajuda })}
				<input
					{id}
					aria-describedby={ajuda}
					type="text"
					list="assuntos-usados"
					bind:value={q.assunto}
					oninput={() => onalterar(false)}
				/>
				<datalist id="assuntos-usados">
					{#each usados.assuntos as a (a)}<option value={a}></option>{/each}
				</datalist>
			{/snippet}
		</Campo>
	</div>

	{#each ligados as apoio (apoio.id)}
		<TextoDeApoio {apoio}>
			{#snippet acoes()}
				<button class="btn" type="button" onclick={() => onabrirTexto(apoio.id)}>
					Conferir ou corrigir o texto
				</button>
			{/snippet}
		</TextoDeApoio>
	{/each}
	{#if problemas.length > 0}
		<div class="callout warn problemas">
			<b>O que corrigir nesta questão</b>
			<ul>
				{#each problemas as p (p)}<li>{p}</li>{/each}
			</ul>
			{#if incompleta(q)}
				<span class="ajuda">
					Compare com o original ao lado. Dá para pedir à IA que releia (botão acima do mapa) ou
					corrigir em "Editar texto e alternativas".
				</span>
			{/if}
			{#if semTexto && livres.length > 0}{@render ligarTexto('Ligar a um texto…')}{/if}
		</div>
	{/if}

	<div class="previa" aria-label="A questão como o aluno vai ver">
		<span class="rotulo-previa">Como o aluno vai ver</span>
		<Blocos blocos={q.blocos} />
		<ol>
			{#each q.alternativas as alt, i (i)}
				<li><b>{alt.letra}</b><div><Blocos blocos={alt.blocos} /></div></li>
			{/each}
		</ol>
	</div>

	{#if figuras.length > 0}
		<ListaDeFiguras {figuras} {onajustarFigura} onalterar={() => onalterar(false)} />
	{/if}

	<div class="conferencia">
		<Campo rotulo="Resposta oficial" ajuda="Vem do gabarito. Só mude se o gabarito foi lido errado.">
			{#snippet children({ id, ajuda })}
				<select
					{id}
					aria-describedby={ajuda}
					value={q.resposta}
					onchange={(e) => {
						definirResposta(rascunho, q, e.currentTarget.value);
						onalterar(true);
					}}
				>
					<option value="">Sem resposta</option>
					{#each LETRAS as l (l)}<option value={l}>{l}</option>{/each}
				</select>
			{/snippet}
		</Campo>
		<div class="marcas">
			<label class="opcao">
				<input type="checkbox" class="checkbox" bind:checked={q.completa} onchange={alterar} />
				<span><b>Completa</b><span class="ajuda">Desmarque se a questão está cortada ou faltando um pedaço.</span></span>
			</label>
			<label class="opcao">
				<input type="checkbox" class="checkbox" bind:checked={q.revisada} onchange={() => onalterar(false)} />
				<span>
					<b>Conferi com o original</b>
					<span class="ajuda">Enunciado, alternativas e figuras batem com o caderno ao lado.</span>
				</span>
			</label>
		</div>
		<button class="btn primary" type="button" onclick={conferirEProxima}>Conferir e ir à próxima</button>
	</div>

	<button class="btn editar" type="button" aria-expanded={editando} onclick={() => (editando = !editando)}>
		{editando ? 'Fechar edição' : 'Editar texto e alternativas'}
	</button>

	{#if editando}
		<div class="edicao">
			<BarraDeDestaque {marcar} />

			<div class="grupo">
				<span class="rotulo">Enunciado</span>
				<p class="ajuda">O texto antes das alternativas, igual ao caderno. A prévia acima mostra o resultado.</p>
				<CampoDeTexto
					bind:blocos={q.blocos}
					rotulo="Enunciado"
					linhas={4}
					onalterar={alterar}
					onfoco={(m) => (marcar = m)}
				/>
			</div>

			<div class="grupo">
				<span class="rotulo">Alternativas</span>
				<p class="ajuda">Uma por letra, só o texto dela — sem o "(A)" do começo. O × tira a alternativa.</p>
				{#each q.alternativas as alt, i (i)}
					<div class="alternativa">
						<b>{alt.letra}</b>
						<CampoDeTexto
							bind:blocos={alt.blocos}
							rotulo="Alternativa {alt.letra}"
							linhas={1}
							onalterar={alterar}
							onfoco={(m) => (marcar = m)}
						/>
						<button
							class="remover"
							type="button"
							title="Remover a alternativa {alt.letra}"
							aria-label="Remover a alternativa {alt.letra}"
							onclick={() => {
								q.alternativas = q.alternativas.filter((_, k) => k !== i);
								alterar();
							}}
						>
							×
						</button>
					</div>
				{/each}
				{#if faltaLetra}
					<button
						class="btn"
						type="button"
						onclick={() => {
							q.alternativas = [...q.alternativas, { letra: faltaLetra!, blocos: [novoBloco()] }];
							alterar();
						}}
					>
						Adicionar alternativa {faltaLetra}
					</button>
				{/if}
			</div>

			<div class="grupo">
				<span class="rotulo">Textos de apoio desta questão</span>
				<p class="ajuda">
					O texto que a questão cita ("De acordo com o texto…"). Para ligar um texto a várias questões de
					uma vez, use a etapa Textos de apoio.
				</p>
				{#each ligados as a (a.id)}
					<div class="ligado">
						<span><b>{rotuloDoApoio(a)}</b> <span class="dim">{comecoDoApoio(a)}</span></span>
						<button class="btn" type="button" onclick={() => alternarApoio(a.id, false)}>Desligar desta questão</button>
					</div>
				{/each}
				{#if livres.length > 0}
					{@render ligarTexto(ligados.length ? 'Ligar outro texto…' : 'Ligar a um texto…')}
				{:else if rascunho.apoios.length === 0}
					<p class="dim">A prova não tem textos de apoio; crie um na etapa Textos de apoio.</p>
				{/if}
			</div>

			<details class="mais">
				<summary>Mais opções</summary>
				<div class="mais-corpo">
					<Campo rotulo="Número" ajuda="O número da questão no caderno.">
						{#snippet children({ id, ajuda })}
							<input {id} aria-describedby={ajuda} class="numero" type="number" min="1" bind:value={q.numero} oninput={alterar} />
						{/snippet}
					</Campo>
					<Campo rotulo="Situação no gabarito" ajuda="O que o gabarito diz além da letra, como “anulada”. Vazio na maioria das questões.">
						{#snippet children({ id, ajuda })}
							<input {id} aria-describedby={ajuda} type="text" bind:value={q.situacao} oninput={alterar} />
						{/snippet}
					</Campo>
					<button class="btn danger" type="button" onclick={onremover}>Remover esta questão</button>
				</div>
			</details>
		</div>
	{/if}
</div>

<style>
	.questao {
		display: grid;
		/* Sem o minmax, a linha mais longa de um bloco de código alarga a coluna
		   e corta o texto na borda do cartão; assim, só o código rola. */
		grid-template-columns: minmax(0, 1fr);
		gap: 14px;
	}
	.classificacao {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: 14px;
	}
	.rotulo {
		font-size: 13px;
		font-weight: 600;
	}
	.ajuda {
		margin: 0;
		font-size: 12.5px;
		line-height: 1.45;
		color: var(--text-muted);
	}
	.previa {
		display: grid;
		gap: 4px;
		padding: 12px 16px 14px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		font-size: 14.5px;
	}
	.rotulo-previa {
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.previa ol {
		list-style: none;
		margin: 6px 0 0;
		padding: 0;
		display: grid;
		gap: 4px;
	}
	.previa li {
		display: grid;
		grid-template-columns: 22px minmax(0, 1fr);
		gap: 6px;
	}
	.previa li b {
		color: var(--accent);
		font-family: var(--font-mono);
	}
	.problemas {
		display: grid;
		gap: 6px;
		margin: 0;
		font-size: 13px;
	}
	.problemas ul {
		margin: 0;
		padding-left: 18px;
		line-height: 1.5;
	}
	/* O select mede a opção mais longa (o nome e o começo do texto): sem largura
	   fixa, ele alargava a coluna e cortava os campos da edição. */
	.ligar {
		width: 100%;
		min-width: 0;
	}
	.conferencia {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 12px;
		padding: 14px 0;
		border-top: 1px solid var(--border);
		border-bottom: 1px solid var(--border);
	}
	.conferencia :global(select) {
		max-width: 180px;
	}
	.marcas {
		display: grid;
		gap: 10px;
	}
	.opcao {
		display: flex;
		gap: 10px;
		align-items: flex-start;
		font-size: 13px;
		cursor: pointer;
	}
	.opcao > span {
		display: grid;
		gap: 1px;
	}
	.conferencia > .btn {
		justify-self: start;
	}
	.editar {
		justify-self: start;
	}
	.edicao {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 18px;
		padding-top: 4px;
	}
	.grupo {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 6px;
	}
	.alternativa {
		display: grid;
		grid-template-columns: 22px minmax(0, 1fr) 28px;
		gap: 8px;
		align-items: start;
	}
	.alternativa b {
		padding-top: 7px;
		color: var(--accent);
		font-family: var(--font-mono);
	}
	.remover {
		width: 28px;
		height: 28px;
		margin-top: 3px;
		font-size: 16px;
		line-height: 1;
		color: var(--text-faint);
		background: transparent;
		border: 1px solid transparent;
		border-radius: 6px;
		cursor: pointer;
	}
	.remover:hover {
		color: var(--danger);
		border-color: var(--border);
	}
	.ligado {
		display: flex;
		flex-wrap: wrap;
		gap: 6px 12px;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
	}
	.dim {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	.mais summary {
		cursor: pointer;
		font-size: 13px;
		color: var(--text-muted);
	}
	.mais-corpo {
		display: grid;
		gap: 14px;
		padding-top: 12px;
	}
	.mais-corpo .btn {
		justify-self: start;
	}
	.numero {
		max-width: 110px;
	}
	.callout {
		margin: 0;
		font-size: 13px;
	}
</style>
