<script lang="ts">
	import IconButton from './IconButton.svelte';
	import NavIcon from './NavIcon.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import {
		assuntosDaLinha,
		chaveDoAssunto as chave,
		pareceEmentaCorrida,
		ROTULO_BLOCO,
		ROTULOS_DO_MOTOR as ROTULOS,
		semNumeroInicial
	} from '$lib/estudo';
	import { nf0, partesTema, tagStyle } from '$lib/format';
	import type { Atividade } from '$lib/types';

	/**
	 * O conteúdo programático de UMA matéria, aberto de qualquer linha do
	 * cronograma.
	 *
	 * É a mesma casca de diálogo do registro de estudo e do dossiê: o conteúdo
	 * aqui é uma lista para LER, e centrada ela ganha largura de leitura. Um
	 * painel lateral só se pagaria se houvesse o que cruzar com o cronograma
	 * atrás — e não há: a ementa se explica sozinha.
	 *
	 * Os temas já vieram no plano. A numeração é a da posição, como em
	 * /conteudo — o texto guardado pode trazer a sua própria, e duas numerações
	 * na mesma linha não se explicam.
	 *
	 * Marcar "já estudei" registra a próxima 1ª passada do tópico como
	 * concluída: se ela estava agendada para a frente, o servidor a traz para
	 * hoje e encosta o resto do cronograma (antecipar compra tempo, não abre
	 * vão). Quando a atividade junta vários tópicos, só o marcado sai dela.
	 * Revisão e 2ª passada ficam onde estão — estudar antes não dispensa
	 * revisar.
	 */
	let {
		codigo,
		tema = '',
		onclose
	}: {
		codigo: string;
		/** O assunto da linha que abriu a ementa, destacado no meio dela. */
		tema?: string;
		onclose: () => void;
	} = $props();

	const plano = $derived(planoStore.plano);
	const disc = $derived(plano?.concurso.disciplinas.find((d) => d.codigo === codigo) ?? null);
	const nome = $derived(disc?.nome ?? codigo);
	const temas = $derived(disc?.temas ?? []);
	const slug = $derived(plano?.concurso.slug ?? '');
	// Os mapas mentais da matéria: o que se revê antes de abrir a ementa. Cada
	// tópico com mapa escolhido o mostra na própria linha.
	const mapas = $derived(mapasStore.doCodigo(codigo));
	const comMapa = $derived(temas.filter((t) => mapasStore.doTopico(codigo, t).length > 0).length);

	/** Os assuntos da linha clicada, através do rótulo e do bloco que o motor junta. */
	const doBloco = $derived(new Set(assuntosDaLinha(tema)));

	const marcado = (t: string) => doBloco.has(chave(t));

	// --- já estudei ------------------------------------------------------------

	/** As 1ª passadas desta matéria, na ordem do cronograma, com o que cada uma cobre. */
	const passadas = $derived(
		(plano?.dias ?? []).flatMap((d) =>
			d.itens
				.filter((i) => i.disciplina === codigo && i.passada <= 1 && !ROTULOS.some((p) => i.tema.startsWith(p)))
				.map((i) => ({ item: i, cobre: new Set(partesTema(i.tema).map(chave)) }))
		)
	);

	const doTopico = (t: string): Atividade[] =>
		passadas.filter((p) => p.cobre.has(chave(t))).map((p) => p.item);

	let salvando = $state<string | null>(null);
	let erroMarca = $state<string | null>(null);

	async function marcarEstudado(t: string, sim: boolean) {
		const atividades = doTopico(t);
		// Marcar conclui a próxima pendente; desmarcar desfaz a última concluída.
		const alvo = sim ? atividades.find((a) => !a.concluido) : atividades.findLast((a) => a.concluido);
		if (!alvo) return;
		salvando = chave(t);
		erroMarca = null;
		// Marcar vai pelo tópico: se a atividade junta vários ("AD · LDAP"), o
		// servidor separa só este. O texto enviado é o da atividade, não o da
		// ementa, que pode trazer numeração.
		const parte = partesTema(alvo.tema).find((p) => chave(p) === chave(t)) ?? alvo.tema;
		const erro = sim
			? await planoStore.estudarTema(alvo.id, parte)
			: await planoStore.salvarAtividade(alvo.id, {
					horas: alvo.horas,
					questoes: alvo.questoes,
					acertos: alvo.acertos,
					nota: alvo.nota,
					concluido: false
				});
		salvando = null;
		erroMarca = erro;
	}

	// --- foco e teclado -----------------------------------------------------
	let painel = $state<HTMLElement | null>(null);

	$effect(() => {
		painel?.focus();
		// A linha que abriu a ementa pode estar no fim de uma lista de sessenta
		// tópicos; abrir em cima dela poupa a rolagem de procurar.
		painel?.querySelector('[data-marcado="1"]')?.scrollIntoView({ block: 'center' });
	});

	/** Prende o Tab dentro do diálogo enquanto ele está aberto. */
	function prender(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onclose();
			return;
		}

		if (e.key !== 'Tab' || !painel) return;

		const foco = painel.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled)');
		if (foco.length === 0) return;

		const primeiro = foco[0];
		const ultimo = foco[foco.length - 1];

		if (e.shiftKey && document.activeElement === primeiro) {
			e.preventDefault();
			ultimo.focus();
		} else if (!e.shiftKey && document.activeElement === ultimo) {
			e.preventDefault();
			primeiro.focus();
		}
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="dlg-fundo"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		class="dlg"
		role="dialog"
		aria-modal="true"
		aria-labelledby="ementa-titulo"
		tabindex="-1"
		bind:this={painel}
		onkeydown={prender}
	>
		<header class="topo">
			<span class="chip" style={tagStyle(disc?.cor ?? 0)}>{codigo}</span>
			<div class="ident">
				<h2 id="ementa-titulo">{nome}</h2>
				<p>
					{ROTULO_BLOCO[disc?.bloco ?? 'esp']} · {temas.length}
					{temas.length === 1 ? 'tópico' : 'tópicos'}
				</p>
			</div>
			<IconButton icon="fechar" label="Fechar o conteúdo programático" onclick={onclose} />
		</header>

		<div class="corpo">
			{#if mapas.length > 0}
				<section class="mapas" aria-label="Mapas mentais da matéria">
					<h3>Mapas mentais</h3>
					<ul>
						{#each mapas as m (m.slug)}
							<li>
								<a href="/mapas/{m.slug}">
									<NavIcon name="mapa" size="sm" />
									<span class="m-titulo">{m.titulo}</span>
									<span class="m-conta"
										>{m.materiaInteira
											? 'a matéria inteira'
											: `${m.temas.length} ${m.temas.length === 1 ? 'tópico' : 'tópicos'}`}</span
									>
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
			{#if temas.length === 0}
				<p class="vazia">
					Sem tópicos cadastrados — os dias desta matéria mostram só o nome dela. Os temas ficam em
					<b>“Temas e fontes”</b>, dentro de
					<a href="/concursos/{slug}/editar">editar o concurso</a>.
				</p>
			{:else}
				<p class="dica">
					Marque o que você já estudou: o tópico vem para hoje e o cronograma se reorganiza.
					{#if comMapa > 0}
						<span class="dica-mapa"
							><NavIcon name="mapa" size="sm" />
							{comMapa} de {temas.length} tópicos com mapa mental</span
						>
					{/if}
				</p>
				{#if erroMarca}<div class="form-error" role="alert">{erroMarca}</div>{/if}
				<ol class="topicos">
					{#each temas as t, i (i)}
						{@const aqui = marcado(t)}
						{@const atividades = doTopico(t)}
						{@const estudado = atividades.some((a) => a.concluido)}
						{@const mapasDoTopico = mapasStore.doTopico(codigo, t)}
						<li
							class:marcado={aqui}
							class:estudado
							class:com-mapa={mapasDoTopico.length > 0}
							data-marcado={aqui ? '1' : '0'}
							aria-current={aqui}
						>
							<input
								type="checkbox"
								class="estudei"
								aria-label="Já estudei: {semNumeroInicial(t)}"
								title={atividades.length === 0 ? 'Este tópico não está no cronograma' : 'Já estudei'}
								checked={estudado}
								disabled={atividades.length === 0 || salvando !== null}
								onchange={(e) => marcarEstudado(t, e.currentTarget.checked)}
							/>
							<span class="num">{i + 1}</span>
							<span class="txt">
								{semNumeroInicial(t)}
								{#if mapasDoTopico.length > 0}
									<span class="t-mapas">
										{#each mapasDoTopico as m (m.slug)}
											<a href="/mapas/{m.slug}" title="Abrir o mapa mental: {m.titulo}">
												<NavIcon name="mapa" size="sm" />{m.titulo}
											</a>
										{/each}
									</span>
								{/if}
							</span>
							{#if pareceEmentaCorrida(t)}
								<a
									class="dividir"
									href="/concursos/{slug}/editar"
									title="Este tópico reúne a ementa inteira; abra a edição para dividi-lo"
								>
									dividir
								</a>
							{/if}
						</li>
					{/each}
				</ol>
			{/if}
		</div>
	</div>
</div>

<style>
	/* Mesma casca do formulário de registro e do dossiê; os estilos deles são
	   escopados ao próprio componente, então a forma é restabelecida aqui. */
	.dlg-fundo {
		position: fixed;
		inset: 0;
		background: rgba(20, 18, 14, 0.45);
		display: grid;
		place-items: center;
		padding: 20px;
		z-index: 70;
	}
	.dlg {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		box-shadow: var(--shadow-pop);
		width: min(640px, 100%);
		max-height: 84vh;
		display: flex;
		flex-direction: column;
		animation: entrar 0.16s ease-out;
	}
	@keyframes entrar {
		from {
			transform: translateY(8px);
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.dlg {
			animation: none;
		}
	}
	.dlg:focus {
		outline: none;
	}

	.topo {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		padding: 16px 16px 12px;
		border-bottom: 1px solid var(--border);
	}
	.ident {
		min-width: 0;
		flex: 1;
	}
	.topo h2 {
		margin: 0;
		font-size: 17px;
		font-weight: 700;
		letter-spacing: -0.01em;
		line-height: 1.25;
	}
	.topo p {
		margin: 3px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}
	.chip {
		flex: none;
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 4px 8px;
		border-radius: 5px;
		margin-top: 2px;
	}

	.corpo {
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 6px 16px 16px;
	}

	/* Como a ementa em /conteudo: o número é uma coluna só dele, para nunca
	   espremer o texto que ele rotula. */
	.topicos {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.topicos li {
		display: grid;
		grid-template-columns: 1.2em 2.2em minmax(0, 1fr) auto;
		gap: 10px;
		align-items: baseline;
		font-size: 14px;
		line-height: 1.6;
		color: var(--text);
		padding: 8px;
	}
	.topicos li + li {
		border-top: 1px solid var(--border);
	}
	/* O assunto da linha que abriu a ementa: uma marca, não um segundo conteúdo.
	   A barra é pintada POR DENTRO (inset), e não como borda: uma borda alarga a
	   caixa e a linha marcada passa a começar dois pixels à esquerda de todas as
	   outras — de perto é nada, de relance é a tela inteira parecendo torta. */
	.topicos li.marcado {
		background: var(--accent-soft);
		box-shadow: inset 2px 0 0 var(--accent);
		border-radius: 6px;
	}
	.num {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
		text-align: right;
	}
	.txt {
		min-width: 0;
	}
	.estudei {
		margin: 0;
		align-self: center;
		accent-color: var(--good);
		cursor: pointer;
	}
	.estudei:disabled {
		cursor: default;
	}
	/* O que já foi estudado sai da frente sem sumir: é a ementa inteira que se lê. */
	.topicos li.estudado .txt {
		color: var(--text-muted);
	}
	/* Os mapas mentais da matéria ficam acima da ementa: é o que se revê antes. */
	.mapas {
		margin: 8px 8px 6px;
		padding-bottom: 10px;
		border-bottom: 1px solid var(--border);
	}
	.mapas h3 {
		margin: 0 0 6px;
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.mapas ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.mapas a {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 7px 8px;
		border-radius: 7px;
		color: var(--text);
		font-size: 14px;
		text-decoration: none;
	}
	.mapas a:hover {
		background: var(--bg-hover);
	}
	.m-titulo {
		flex: 1;
		min-width: 0;
		font-weight: 600;
	}
	.m-conta {
		flex: none;
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--text-faint);
	}
	.dica {
		margin: 8px 8px 4px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.dica-mapa {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		margin-left: 6px;
		color: var(--accent);
		white-space: nowrap;
	}
	/* Os mapas do tópico, debaixo do texto dele: o atalho para rever o assunto. */
	.t-mapas {
		display: flex;
		flex-wrap: wrap;
		gap: 2px 12px;
		margin-top: 2px;
	}
	.t-mapas a {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-height: 28px;
		font-size: 12.5px;
		font-weight: 500;
		color: var(--accent);
		text-decoration: none;
	}
	.t-mapas a:hover {
		text-decoration: underline;
	}
	.dividir {
		font-family: var(--font-mono);
		font-size: 10px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--warn);
		white-space: nowrap;
	}
	.vazia {
		margin: 10px 0 0;
		font-size: 13.5px;
		line-height: 1.6;
		color: var(--text-muted);
	}

	@media (max-width: 620px) {
		.dlg-fundo {
			padding: 12px;
		}
		.topicos li {
			grid-template-columns: 1.2em 2.2em minmax(0, 1fr);
		}
		.dividir {
			grid-column: 3;
		}
	}
</style>
