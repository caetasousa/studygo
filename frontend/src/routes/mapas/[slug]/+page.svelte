<script lang="ts">
	import { page } from '$app/state';
	import { beforeNavigate, goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { salvarArquivo } from '$lib/download';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { semNumeroInicial } from '$lib/estudo';
	import { tagStyle } from '$lib/format';
	import { imagensCitadas, indexar, paraBusca, textoVisivel, type NoDoMapa } from '$lib/mapas/arvore';
	import { fornecerImagens } from '$lib/mapas/imagens';
	import Mapa from '$lib/mapas/Mapa.svelte';
	import Questoes from '$lib/mapas/Questoes.svelte';
	import { descreverPlacar, placar, porBanca, porRamo } from '$lib/mapas/questoes';
	import { confirmar } from '$lib/stores/confirmacao.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { mapasStore } from '$lib/stores/mapas.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import type {
		CorrecaoDoMapa,
		ItemDoMapa,
		MapaDaMateria,
		MapaLido,
		MapasDaMateria,
		QuestaoDoMapa,
		QuestoesImportadas
	} from '$lib/types';

	/**
	 * Um mapa mental aberto: o cabeçalho com as propriedades (matérias, fonte,
	 * tamanho) e, em duas abas, o mapa e as questões da aula. Antes as questões
	 * vinham depois do mapa — no celular, várias telas abaixo dele. Na aba
	 * delas, escolhe-se vê-las por conteúdo (os ramos) ou por banca, e filtrar
	 * o que falta ou o que se errou; cada grupo se resolve num diálogo, como as
	 * da lei. O ramo do mapa também mostra as questões dele e leva a elas.
	 *
	 * O mapa tem um modo de edição para tirar tópicos. A exclusão vale na hora
	 * na tela e só vai ao servidor alguns segundos depois: é a janela do
	 * "Desfazer". Manter o mapa (importar questões, excluir) fica no fim.
	 */
	const slug = $derived(page.params.slug ?? '');

	let lido = $state<MapaLido | null>(null);
	let erro = $state<string | null>(null);
	let carregando = $state(true);

	async function carregar(s: string) {
		carregando = true;
		erro = null;
		lido = null;
		try {
			const res = await api.lerMapa(s);
			if (slug === s) lido = res;
		} catch (e) {
			if (slug === s) erro = e instanceof Error ? e.message : 'Não foi possível abrir o mapa';
		} finally {
			if (slug === s) carregando = false;
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	const nos = $derived(lido ? indexar(lido.arvore) : []);

	// --- imagens -----------------------------------------------------------
	// O outline cita a imagem pelo nome; o arquivo chega à parte, aqui. O que o
	// mapa cita e ainda não chegou fica listado, para não se perder no meio dos
	// ramos recolhidos.
	let exportando = $state(false);
	let erroExportar = $state<string | null>(null);

	async function exportar() {
		exportando = true;
		erroExportar = null;
		try {
			salvarArquivo(await api.exportarMapas(slug), `${slug}.zip`);
		} catch (e) {
			erroExportar = e instanceof Error ? e.message : 'Não foi possível exportar o mapa';
		} finally {
			exportando = false;
		}
	}

	let versaoDasImagens = $state(0);
	let enviandoImagens = $state(false);
	let avisoImagens = $state<string | null>(null);
	let erroImagens = $state<string | null>(null);

	const citadas = $derived(lido ? imagensCitadas(lido.arvore) : []);
	const faltando = $derived(citadas.filter((n) => !lido?.imagens.includes(n)));

	fornecerImagens({
		get slug() {
			return slug;
		},
		get enviadas() {
			return lido?.imagens ?? [];
		},
		get versao() {
			return versaoDasImagens;
		}
	});

	async function enviarImagens(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivos = [...(e.currentTarget.files ?? [])];
		e.currentTarget.value = '';
		if (arquivos.length === 0) return;
		enviandoImagens = true;
		avisoImagens = null;
		erroImagens = null;
		try {
			const r = await api.enviarImagensDoMapa(slug, arquivos);
			avisoImagens = r.gravadas === 1 ? '1 imagem enviada.' : `${r.gravadas} imagens enviadas.`;
			const novo = await api.lerMapa(slug);
			if (lido) lido.imagens = novo.imagens;
			versaoDasImagens++;
		} catch (err) {
			erroImagens = err instanceof Error ? err.message : 'O envio das imagens falhou';
		} finally {
			enviandoImagens = false;
		}
	}

	// --- matérias ---------------------------------------------------------
	const vinculadas = $derived(mapasStore.disciplinas.filter((d) => d.mapas.some((m) => m.slug === slug)));
	const livres = $derived(mapasStore.disciplinas.filter((d) => !vinculadas.includes(d)));
	let erroVinculo = $state<string | null>(null);
	let gravandoVinculo = $state(false);

	/** O vínculo deste mapa numa matéria vinculada. */
	const vinculoEm = (d: MapasDaMateria): MapaDaMateria => d.mapas.find((m) => m.slug === slug)!;

	async function vincular(disciplinaId: string, ligar: boolean, temas: string[] = []) {
		const concurso = concursoStore.ativoSlug;
		if (!concurso) return;
		erroVinculo = null;
		gravandoVinculo = true;
		try {
			await api.vincularMapa(concurso, disciplinaId, slug, ligar, temas);
			await mapasStore.carregar(true);
		} catch (e) {
			erroVinculo = e instanceof Error ? e.message : 'Não foi possível gravar o vínculo';
		} finally {
			gravandoVinculo = false;
		}
	}

	// --- tópicos ------------------------------------------------------------
	// A matéria cuja lista de tópicos está aberta para escolher.
	let escolhendo = $state<string | null>(null);

	/**
	 * Marca ou desmarca um tópico do mapa e grava na hora. Desmarcar o último
	 * volta à matéria inteira: o vínculo continua, só deixa de ter recorte.
	 */
	function marcarTema(d: MapasDaMateria, tema: string, marcado: boolean) {
		const v = vinculoEm(d);
		const atuais = v.materiaInteira ? [] : v.temas;
		const novos = marcado ? [...atuais, tema] : atuais.filter((t) => t !== tema);
		void vincular(d.disciplinaId, true, novos);
	}

	/** O que o mapa cobre na matéria, numa linha. */
	function cobertura(d: MapasDaMateria): string {
		const v = vinculoEm(d);
		if (v.materiaInteira) return 'a matéria inteira';
		if (v.temas.length === 0) return 'nenhum tópico da ementa atual';
		return v.temas.map(semNumeroInicial).join(' · ');
	}

	const cor = (codigo: string) => planoStore.discIndex[codigo]?.cor ?? 0;

	// --- abas -------------------------------------------------------------
	// A aba fica no endereço: voltar, recarregar ou mandar o link abre onde estava.
	const questoes = $derived(lido?.questoes ?? []);
	const aba = $derived(page.url.searchParams.get('aba') === 'questoes' && questoes.length > 0 ? 'questoes' : 'mapa');

	function escolherAba(a: 'mapa' | 'questoes') {
		const url = new URL(page.url);
		if (a === 'mapa') url.searchParams.delete('aba');
		else url.searchParams.set('aba', a);
		void goto(url, { replaceState: true, noScroll: true, keepFocus: true });
	}

	// As setas trocam de aba, como em toda lista de abas.
	function teclasDasAbas(e: KeyboardEvent) {
		if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
		e.preventDefault();
		const outra = aba === 'mapa' ? 'questoes' : 'mapa';
		escolherAba(outra);
		document.getElementById(`aba-${outra}`)?.focus();
	}

	// --- questões ----------------------------------------------------------
	// Por conteúdo (os ramos, na ordem do mapa) ou por banca, para treinar a da
	// prova; e todas, as que faltam ou as que se errou. As escolhas ficam no
	// aparelho: quem estuda por banca quer abrir já assim.
	type Agrupar = 'ramo' | 'banca';
	type Mostrar = 'todas' | 'sem-resposta' | 'erradas';
	const CHAVE_AGRUPAR = 'studygo:mapas:agrupar-questoes';
	const CHAVE_MOSTRAR = 'studygo:mapas:mostrar-questoes';

	function lerEscolha<T extends string>(chave: string, validas: readonly T[], padrao: T): T {
		try {
			const v = localStorage.getItem(chave);
			return validas.includes(v as T) ? (v as T) : padrao;
		} catch {
			return padrao;
		}
	}
	function guardarEscolha(chave: string, valor: string) {
		try {
			localStorage.setItem(chave, valor);
		} catch {
			// sem armazenamento, a escolha vale só nesta visita
		}
	}

	let agrupar = $state<Agrupar>(lerEscolha(CHAVE_AGRUPAR, ['ramo', 'banca'], 'ramo'));
	let mostrar = $state<Mostrar>(lerEscolha(CHAVE_MOSTRAR, ['todas', 'sem-resposta', 'erradas'], 'todas'));

	const ROTULO_MOSTRAR: Record<Mostrar, string> = {
		todas: 'Todas',
		'sem-resposta': 'Sem resposta',
		erradas: 'Que errei'
	};
	const VAZIO_MOSTRAR: Record<Mostrar, string> = {
		todas: 'Sem questões.',
		'sem-resposta': 'Todas as questões já têm resposta.',
		erradas: 'Nenhum erro por aqui.'
	};

	const filtradas = $derived(
		questoes.filter((q) =>
			mostrar === 'todas' ? true : mostrar === 'sem-resposta' ? q.resposta === null : q.resposta !== null && !q.resposta.acertou
		)
	);
	const grupos = $derived(
		!lido ? [] : agrupar === 'banca' ? porBanca(filtradas) : porRamo(lido.arvore, filtradas)
	);
	const geral = $derived(placar(questoes));

	/** As questões de um ramo do mapa, pelo título como a importação o compara. */
	function questoesDoNo(no: NoDoMapa): QuestaoDoMapa[] {
		const titulo = paraBusca(no.item.texto);
		return questoes.filter((q) => paraBusca(q.ramo) === titulo);
	}
	function placarDoRamo(no: NoDoMapa) {
		const qs = questoesDoNo(no);
		return qs.length > 0 ? placar(qs) : null;
	}
	let aberto = $state<{ titulo: string; ids: string[] } | null>(null);
	const doDialogo = $derived(
		aberto ? questoes.filter((q) => aberto!.ids.includes(q.id)) : ([] as QuestaoDoMapa[])
	);

	function abrirQuestoes(titulo: string, qs: QuestaoDoMapa[]) {
		aberto = { titulo, ids: qs.map((q) => q.id) };
	}

	/** A resposta atualiza o placar sem recarregar o mapa (e sem fechar os tópicos abertos). */
	function respondida(id: string, c: CorrecaoDoMapa) {
		for (const q of lido?.questoes ?? []) {
			if (q.id === id) q.resposta = c;
		}
	}

	let importandoQuestoes = $state(false);
	let avisoQuestoes = $state<string | null>(null);
	let erroQuestoes = $state<string | null>(null);

	function descreverImportacao(r: QuestoesImportadas): string {
		const n = (x: number, um: string, varios: string) => (x === 1 ? `1 ${um}` : `${x} ${varios}`);
		const partes = [
			r.novas && n(r.novas, 'questão nova', 'questões novas'),
			r.atualizadas && n(r.atualizadas, 'atualizada', 'atualizadas'),
			r.desativadas && n(r.desativadas, 'retirada', 'retiradas'),
			r.mantidas && n(r.mantidas, 'sem mudança', 'sem mudança')
		].filter(Boolean);
		return partes.length ? `${partes.join(', ')}.` : 'Nada mudou.';
	}

	// O arquivo vai como está: é o servidor que confere e lista os problemas.
	async function importarQuestoes(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!arquivo) return;
		importandoQuestoes = true;
		avisoQuestoes = null;
		erroQuestoes = null;
		try {
			const r = await api.importarQuestoesDoMapa(slug, await arquivo.text());
			avisoQuestoes = descreverImportacao(r);
			const novo = await api.lerMapa(slug);
			if (lido) lido.questoes = novo.questoes;
		} catch (err) {
			erroQuestoes = err instanceof Error ? err.message : 'A importação das questões falhou';
		} finally {
			importandoQuestoes = false;
		}
	}

	// --- editar ------------------------------------------------------------
	let editando = $state(false);
	let erroEdicao = $state<string | null>(null);

	/**
	 * A exclusão que ainda não foi ao servidor: o tópico já saiu da tela, e
	 * "Desfazer" o põe de volta no mesmo lugar. `raw`: são os próprios objetos
	 * do mapa, que têm de voltar como eram (é deles o estado de aberto).
	 */
	interface Pendente {
		slug: string;
		caminho: number[];
		texto: string;
		irmaos: ItemDoMapa[];
		indice: number;
		item: ItemDoMapa;
		timer: ReturnType<typeof setTimeout>;
	}
	let pendente = $state.raw<Pendente | null>(null);
	const JANELA_DE_DESFAZER = 6000;

	const semNegrito = textoVisivel;

	function irmaosDe(caminho: number[]): ItemDoMapa[] {
		let lista = lido!.arvore;
		for (const i of caminho.slice(0, -1)) lista = lista[i].filhos;
		return lista;
	}

	async function excluirTopico(no: NoDoMapa) {
		if (!lido) return;
		erroEdicao = null;
		const titulo = semNegrito(no.item.texto);

		// O que leva conteúdo junto pergunta antes, dizendo quanto.
		if (no.total > 0) {
			const nq = no.nivel === 0 ? questoesDoNo(no).length : 0;
			const itens = no.total === 1 ? 'o item que há dentro dele' : `os ${no.total} itens que há dentro dele`;
			const qs = nq === 0 ? '' : nq === 1 ? ' e a questão do ramo' : ` e as ${nq} questões do ramo`;
			const ok = await confirmar({
				titulo: `Excluir “${titulo}”?`,
				texto: `Sai junto ${itens}${qs}${nq ? ' (as respostas ficam guardadas)' : ''}. Dá para desfazer logo em seguida.`,
				rotulo: 'Excluir',
				tom: 'perigo'
			});
			if (!ok) return;
		}

		// Uma exclusão por vez no servidor: a anterior vai antes desta.
		if (!(await gravarPendente()) || !lido) return;

		const irmaos = irmaosDe(no.caminho);
		const indice = no.caminho[no.caminho.length - 1];
		const item = irmaos[indice];
		if (!item || item.texto !== no.item.texto) return;

		irmaos.splice(indice, 1);
		pendente = {
			slug,
			caminho: no.caminho,
			texto: no.item.texto,
			irmaos,
			indice,
			item,
			timer: setTimeout(() => void gravarPendente(), JANELA_DE_DESFAZER)
		};
	}

	function desfazer() {
		const p = pendente;
		if (!p) return;
		clearTimeout(p.timer);
		pendente = null;
		p.irmaos.splice(p.indice, 0, p.item);
	}

	/** Manda ao servidor a exclusão pendente. Falhou, o tópico volta à tela. */
	async function gravarPendente(keepalive = false): Promise<boolean> {
		const p = pendente;
		if (!p) return true;
		clearTimeout(p.timer);
		pendente = null;
		try {
			const r = await api.excluirItemDoMapa(p.slug, p.caminho, p.texto, keepalive);
			if (lido && slug === p.slug) {
				lido.mapa = r.mapa;
				lido.questoes = r.questoes;
			}
			return true;
		} catch (e) {
			p.irmaos.splice(p.indice, 0, p.item);
			erroEdicao = e instanceof Error ? e.message : 'Não foi possível excluir o tópico';
			return false;
		}
	}

	// Sair da página não perde a exclusão: ela vai na hora, e o fechamento da
	// aba espera o pedido (keepalive).
	beforeNavigate(() => void gravarPendente());
	$effect(() => {
		const aoFechar = () => void gravarPendente(true);
		window.addEventListener('pagehide', aoFechar);
		return () => window.removeEventListener('pagehide', aoFechar);
	});

	// --- excluir -----------------------------------------------------------
	async function excluir() {
		if (!lido) return;
		const total = lido.questoes.length;
		const junto =
			total === 0 ? 'O mapa' : total === 1 ? 'O mapa, a questão e as suas respostas' : `O mapa, as ${total} questões e as suas respostas`;
		const ok = await confirmar({
			titulo: `Excluir “${lido.mapa.titulo}”?`,
			texto: `${junto} e o vínculo com as matérias saem do app. Para tê-lo de volta, é só importar de novo.`,
			rotulo: 'Excluir mapa',
			tom: 'perigo'
		});
		if (!ok) return;
		try {
			await api.excluirMapa(slug);
			await mapasStore.carregar(true);
			await goto('/mapas');
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A exclusão falhou';
		}
	}

	const nf = new Intl.NumberFormat('pt-BR');
</script>

<svelte:head>
	<title>{lido ? `${lido.mapa.titulo} — Mapas mentais` : 'Mapa mental'} — studygo</title>
</svelte:head>

<a class="voltar" href="/mapas">← Mapas mentais</a>

{#if erro}
	<div class="form-error" role="alert">{erro}</div>
{:else if carregando}
	<p class="page-sub">Carregando…</p>
{:else if lido}
	<h1 class="page-title">
		<span class="title-ic"><NavIcon name="mapa" size="md" /></span>
		<span>{lido.mapa.titulo}</span>
	</h1>

	<dl class="propriedades">
		<div class="linha">
			<dt>Matérias</dt>
			<dd>
				{#each vinculadas as d (d.disciplinaId)}
					<span class="materia">
						<span class="chip" style={tagStyle(cor(d.codigo))}>{d.codigo}</span>
						<span class="nome">{d.nome}</span>
						<button
							type="button"
							class="tirar"
							aria-label="Desvincular {d.nome}"
							title="Desvincular {d.nome}"
							onclick={() => vincular(d.disciplinaId, false)}>×</button
						>
					</span>
				{/each}
				{#if livres.length > 0}
					<select
						class="vincular"
						aria-label="Vincular a uma matéria"
						onchange={(e) => {
							const escolhida = e.currentTarget.value;
							e.currentTarget.value = '';
							if (!escolhida) return;
							void vincular(escolhida, true);
							// Recém-vinculado vale para a matéria inteira; a lista abre
							// para escolher os tópicos, se for o caso.
							escolhendo = escolhida;
						}}
					>
						<option value="">{vinculadas.length === 0 ? 'Vincular a uma matéria…' : '+ outra matéria'}</option>
						{#each livres as d (d.disciplinaId)}
							<option value={d.disciplinaId}>{d.codigo} — {d.nome}</option>
						{/each}
					</select>
				{/if}
				{#if vinculadas.length === 0 && lido.mapa.materia}
					<span class="indicada">a fonte indica: {lido.mapa.materia}</span>
				{/if}
			</dd>
		</div>
		{#if vinculadas.length > 0}
			<div class="linha">
				<dt>Tópicos</dt>
				<dd class="cobertura">
					{#each vinculadas as d (d.disciplinaId)}
						{@const v = vinculoEm(d)}
						{@const aberta = escolhendo === d.disciplinaId}
						<div class="cobre">
							<span class="chip" style={tagStyle(cor(d.codigo))}>{d.codigo}</span>
							<span class="cobre-txt" class:inteira={v.materiaInteira}>{cobertura(d)}</span>
							{#if d.temas.length > 0}
								<button
									type="button"
									class="escolher"
									aria-expanded={aberta}
									aria-controls="topicos-{d.disciplinaId}"
									onclick={() => (escolhendo = aberta ? null : d.disciplinaId)}
								>
									{aberta ? 'Pronto' : 'Escolher tópicos'}
								</button>
							{/if}
						</div>
						{#if aberta}
							<fieldset class="escolha" id="topicos-{d.disciplinaId}">
								<legend>Tópicos de {d.nome} que este mapa cobre</legend>
								<p class="escolha-dica">
									O cronograma mostra o mapa só nestes tópicos. Nenhum marcado: vale para a matéria inteira.
								</p>
								{#each d.temas as t, i (i)}
									<label>
										<input
											type="checkbox"
											checked={!v.materiaInteira && v.temas.includes(t)}
											disabled={gravandoVinculo}
											onchange={(e) => marcarTema(d, t, e.currentTarget.checked)}
										/>
										<span>{semNumeroInicial(t)}</span>
									</label>
								{/each}
							</fieldset>
						{/if}
					{/each}
				</dd>
			</div>
		{/if}
		{#if lido.mapa.fonte}
			<div class="linha">
				<dt>Fonte</dt>
				<dd class="fonte">{lido.mapa.fonte}</dd>
			</div>
		{/if}
		<div class="linha">
			<dt>Tamanho</dt>
			<dd>{lido.mapa.ramos} {lido.mapa.ramos === 1 ? 'ramo' : 'ramos'} · {nf.format(lido.mapa.itens)} itens</dd>
		</div>
	</dl>

	{#if erroVinculo}<div class="form-error" role="alert">{erroVinculo}</div>{/if}

	{#if questoes.length > 0}
		<div class="abas" role="tablist" aria-label="Partes do mapa">
			<button
				type="button"
				role="tab"
				id="aba-mapa"
				aria-selected={aba === 'mapa'}
				aria-controls="painel-mapa"
				tabindex={aba === 'mapa' ? 0 : -1}
				onclick={() => escolherAba('mapa')}
				onkeydown={teclasDasAbas}
			>
				Mapa
			</button>
			<button
				type="button"
				role="tab"
				id="aba-questoes"
				aria-selected={aba === 'questoes'}
				aria-controls="painel-questoes"
				tabindex={aba === 'questoes' ? 0 : -1}
				onclick={() => escolherAba('questoes')}
				onkeydown={teclasDasAbas}
			>
				Questões <span class="contagem">{questoes.length}</span>
			</button>
		</div>
	{/if}

	<div
		class="mapa"
		class:com-abas={questoes.length > 0}
		id="painel-mapa"
		role={questoes.length > 0 ? 'tabpanel' : undefined}
		aria-labelledby={questoes.length > 0 ? 'aba-mapa' : undefined}
		hidden={aba !== 'mapa'}
	>
		{#if erroEdicao}<div class="form-error" role="alert">{erroEdicao}</div>{/if}
		{#key slug}
			<Mapa
				{nos}
				{editando}
				onalternarEdicao={() => (editando = !editando)}
				onexcluir={excluirTopico}
				questoesDoRamo={placarDoRamo}
				onquestoes={(no) => abrirQuestoes(semNegrito(no.item.texto), questoesDoNo(no))}
			/>
		{/key}
	</div>

	{#if questoes.length > 0}
		<div
			class="questoes"
			id="painel-questoes"
			role="tabpanel"
			aria-labelledby="aba-questoes"
			hidden={aba !== 'questoes'}
		>
			<div class="resumo">
				<span class="barra grande" aria-hidden="true">
					<span class="certas" style="width:{(geral.certas / geral.total) * 100}%"></span>
					<span class="erradas" style="width:{(geral.erradas / geral.total) * 100}%"></span>
				</span>
				<p>
					{descreverPlacar(geral)}{geral.respondidas > 0
						? ` · ${Math.round((geral.certas / geral.respondidas) * 100)}% de acerto`
						: ''}
				</p>
			</div>

			<div class="controles">
				<label class="campo">
					<span>Ver por</span>
					<select bind:value={agrupar} onchange={() => guardarEscolha(CHAVE_AGRUPAR, agrupar)}>
						<option value="ramo">Conteúdo</option>
						<option value="banca">Banca</option>
					</select>
				</label>
				<label class="campo">
					<span>Mostrar</span>
					<select bind:value={mostrar} onchange={() => guardarEscolha(CHAVE_MOSTRAR, mostrar)}>
						<option value="todas">Todas</option>
						<option value="sem-resposta">Sem resposta</option>
						<option value="erradas">Que errei</option>
					</select>
				</label>
				<button
					type="button"
					class="btn primary resolver"
					disabled={filtradas.length === 0}
					onclick={() => abrirQuestoes(ROTULO_MOSTRAR[mostrar], filtradas)}
				>
					Resolver {filtradas.length}
				</button>
			</div>

			{#if grupos.length === 0}
				<p class="vazia">{VAZIO_MOSTRAR[mostrar]}</p>
			{/if}
			<ul class="ramos" aria-label={agrupar === 'banca' ? 'Questões por banca' : 'Questões por conteúdo'}>
				{#each grupos as g (g.titulo)}
					{@const p = placar(g.questoes)}
					<li>
						<button type="button" class="ramo" onclick={() => abrirQuestoes(g.titulo, g.questoes)}>
							<span class="r-titulo">{g.titulo}</span>
							<span class="r-placar">
								{p.total === 1 ? '1 questão' : `${p.total} questões`}{p.respondidas > 0
									? ` · ${p.respondidas} de ${p.total} respondidas · ${p.certas === 1 ? '1 certa' : `${p.certas} certas`}`
									: ''}
							</span>
							<span class="barra" aria-hidden="true">
								<span class="certas" style="width:{(p.certas / p.total) * 100}%"></span>
								<span class="erradas" style="width:{(p.erradas / p.total) * 100}%"></span>
							</span>
						</button>
					</li>
				{/each}
			</ul>
		</div>
	{/if}

	<details class="manter">
		<summary>Manter este mapa</summary>
		<p class="page-sub">
			Para corrigir ou ampliar o mapa, importe o texto de novo em <a href="/mapas">Mapas mentais</a>: o mesmo
			endereço troca o conteúdo e mantém as matérias vinculadas. Os tópicos excluídos aqui voltam se o texto
			importado ainda os tiver.
		</p>

		<h2 class="sec">Importar questões</h2>
		<p class="page-sub">
			O <code>{slug}.questoes.json</code>, com as questões da aula, cada uma presa a um ramo do mapa. Importar de novo
			não duplica nada, e as respostas das questões que continuam ficam.
		</p>
		{#if avisoQuestoes}<p class="ok" role="status">{avisoQuestoes}</p>{/if}
		{#if erroQuestoes}<div class="form-error" role="alert">{erroQuestoes}</div>{/if}
		<label class="arquivo">
			<span>Questões do mapa (.json)</span>
			<input type="file" accept=".json,application/json" disabled={importandoQuestoes} onchange={importarQuestoes} />
		</label>
		{#if importandoQuestoes}<p class="page-sub">Importando…</p>{/if}

		<h2 class="sec">Imagens</h2>
		<p class="page-sub">
			O item <code>![legenda](arquivo.png)</code> do texto mostra a imagem com esse nome. Envie os arquivos (PNG,
			JPEG ou WEBP, até 2 MB cada) com o mesmo nome que o texto cita; enviar de novo um nome troca a imagem.
		</p>
		{#if citadas.length > 0}
			<p class="page-sub">
				{#if faltando.length === 0}
					{citadas.length === 1 ? 'A imagem que o mapa cita já chegou.' : `As ${citadas.length} imagens que o mapa cita já chegaram.`}
				{:else}
					Faltam {faltando.length} de {citadas.length}: {#each faltando as n, i (n)}{i > 0 ? ', ' : ''}<code>{n}</code>{/each}.
				{/if}
			</p>
		{/if}
		{#if avisoImagens}<p class="ok" role="status">{avisoImagens}</p>{/if}
		{#if erroImagens}<div class="form-error" role="alert">{erroImagens}</div>{/if}
		<label class="arquivo">
			<span>Imagens do mapa</span>
			<input
				type="file"
				accept="image/png,image/jpeg,image/webp"
				multiple
				disabled={enviandoImagens}
				onchange={enviarImagens}
			/>
		</label>
		{#if enviandoImagens}<p class="page-sub">Enviando…</p>{/if}

		<h2 class="sec">Exportar</h2>
		<p class="page-sub">
			Um .zip com o texto, as questões, as imagens, os vínculos com as matérias e as suas respostas, como o mapa está
			agora. Ele volta inteiro em <a href="/mapas">Mapas mentais</a> → “Importar uma exportação (.zip)”.
		</p>
		{#if erroExportar}<div class="form-error" role="alert">{erroExportar}</div>{/if}
		<button type="button" class="btn" onclick={exportar} disabled={exportando}>
			{exportando ? 'Exportando…' : '⬇ Exportar mapa'}
		</button>

		<h2 class="sec">Excluir</h2>
		<button type="button" class="btn danger" onclick={excluir}>Excluir mapa</button>
	</details>

	{#if aberto}
		<Questoes titulo={aberto.titulo} questoes={doDialogo} onrespondida={respondida} onclose={() => (aberto = null)} />
	{/if}

	{#if pendente}
		<div class="desfazer" role="status">
			<span class="d-txt">“{semNegrito(pendente.texto)}” excluído.</span>
			<button type="button" onclick={desfazer}>Desfazer</button>
		</div>
	{/if}
{/if}

<style>
	.voltar {
		display: inline-block;
		margin-bottom: 10px;
		font-size: 13px;
		color: var(--text-muted);
		text-decoration: none;
	}
	.voltar:hover {
		color: var(--text);
	}
	.title-ic {
		display: grid;
		place-items: center;
		flex: none;
		color: var(--text-muted);
	}

	/* As propriedades no jeito do Notion: um rótulo pequeno e o valor ao lado. */
	.propriedades {
		display: grid;
		gap: 6px;
		margin: 16px 0 0;
		max-width: 860px;
	}
	.linha {
		display: grid;
		grid-template-columns: 84px minmax(0, 1fr);
		gap: 10px;
		align-items: center;
		font-size: 13.5px;
	}
	dt {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	dd {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px 8px;
		margin: 0;
		min-width: 0;
		color: var(--text);
	}
	.fonte {
		color: var(--text-muted);
		font-size: 13px;
	}
	.materia {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 3px 4px 3px 8px;
		border: 1px solid var(--border);
		border-radius: 7px;
		background: var(--bg-soft);
	}
	.chip {
		font-family: var(--font-mono);
		font-size: 10.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 2px 7px;
		border-radius: 5px;
	}
	.nome {
		font-size: 13px;
	}
	.tirar {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		padding: 0;
		border: 0;
		border-radius: 5px;
		background: transparent;
		color: var(--text-faint);
		font-size: 15px;
		line-height: 1;
		cursor: pointer;
	}
	.tirar:hover {
		background: var(--bg-hover);
		color: var(--danger);
	}
	.vincular {
		padding: 4px 8px;
		border: 1px dashed var(--border-strong);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 13px;
		max-width: 100%;
		cursor: pointer;
	}
	.vincular:hover {
		background: var(--bg-hover);
	}
	.indicada {
		font-size: 12.5px;
		color: var(--text-faint);
	}
	/* O que o mapa cobre em cada matéria: uma linha por matéria, e a lista de
	   tópicos abre embaixo dela. */
	.cobertura {
		flex-direction: column;
		align-items: stretch;
	}
	.cobre {
		display: flex;
		align-items: baseline;
		gap: 8px;
		min-width: 0;
	}
	.cobre-txt {
		flex: 1;
		min-width: 0;
		font-size: 13px;
		line-height: 1.5;
	}
	.cobre-txt.inteira {
		color: var(--text-muted);
	}
	.escolher {
		flex: none;
		min-height: 28px;
		padding: 2px 8px;
		border: 1px dashed var(--border-strong);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 12.5px;
		cursor: pointer;
	}
	.escolher:hover {
		background: var(--bg-hover);
	}
	.escolha {
		margin: 0 0 6px;
		padding: 8px 10px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-soft);
	}
	.escolha legend {
		padding: 0 4px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}
	.escolha-dica {
		margin: 0 0 6px;
		font-size: 12px;
		color: var(--text-faint);
	}
	.escolha label {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		min-height: 32px;
		padding: 4px 0;
		font-size: 13.5px;
		line-height: 1.45;
		cursor: pointer;
	}
	.escolha input {
		margin: 3px 0 0;
		flex: none;
	}

	.mapa {
		margin-top: 20px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}
	.mapa.com-abas {
		margin-top: 0;
		border-top: 0;
	}

	/* As abas no jeito do Notion: texto, e um traço embaixo da escolhida. */
	.abas {
		display: flex;
		gap: 4px;
		margin-top: 18px;
		border-bottom: 1px solid var(--border);
		max-width: 860px;
	}
	.abas button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-bottom: -1px;
		padding: 8px 12px;
		border: 0;
		border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		-webkit-tap-highlight-color: transparent;
	}
	.abas button[aria-selected='true'] {
		border-bottom-color: var(--text);
		color: var(--text);
	}
	@media (hover: hover) {
		.abas button:hover {
			color: var(--text);
		}
	}
	.abas button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.contagem {
		padding: 1px 7px;
		border-radius: 999px;
		background: var(--bg-soft);
		font-size: 11.5px;
		font-variant-numeric: tabular-nums;
	}

	/* O "Desfazer" no pé da tela, como no Gmail e no Notion. */
	.desfazer {
		position: fixed;
		left: 50%;
		bottom: calc(20px + env(safe-area-inset-bottom));
		transform: translateX(-50%);
		z-index: 60;
		display: flex;
		align-items: center;
		gap: 14px;
		max-width: calc(100vw - 32px);
		padding: 10px 10px 10px 16px;
		border-radius: 9px;
		background: var(--text);
		color: var(--bg);
		box-shadow: var(--shadow-pop);
		font-size: 13.5px;
	}
	.d-txt {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.desfazer button {
		flex: none;
		padding: 6px 10px;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: inherit;
		font: inherit;
		font-weight: 700;
		text-decoration: underline;
		cursor: pointer;
	}

	.manter {
		margin: 36px 0 24px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.manter > summary {
		cursor: pointer;
		color: var(--text-muted);
		font-size: 13px;
	}
	.manter .page-sub {
		margin: 10px 0 12px;
	}
	.manter .sec {
		margin: 18px 0 0;
		font-size: 14px;
		font-weight: 700;
	}
	.manter code {
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.ok {
		margin: 0 0 10px;
		font-size: 13px;
		color: var(--good);
	}
	.arquivo {
		display: grid;
		gap: 5px;
		margin-bottom: 8px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	/* As questões, como as questões por unidade da lei. */
	.questoes {
		padding-top: 16px;
		max-width: 860px;
	}
	.resumo {
		display: grid;
		gap: 6px;
		margin-bottom: 14px;
	}
	.resumo p {
		margin: 0;
		font-size: 13px;
		color: var(--text-muted);
	}
	.barra.grande {
		height: 6px;
		border-radius: 3px;
	}
	.controles {
		display: flex;
		align-items: flex-end;
		flex-wrap: wrap;
		gap: 10px;
		margin-bottom: 12px;
	}
	.campo {
		display: grid;
		gap: 4px;
		font-size: 12px;
		color: var(--text-faint);
	}
	.campo select {
		padding: 7px 10px;
		border: 1px solid var(--border-strong);
		border-radius: 7px;
		background: var(--bg-card);
		color: var(--text);
		font: inherit;
		font-size: 13.5px;
		cursor: pointer;
	}
	.resolver {
		margin-left: auto;
		padding: 8px 14px;
	}
	.vazia {
		margin: 4px 0 10px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.ramos {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 6px;
	}
	.ramo {
		display: grid;
		gap: 4px;
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-card);
		color: var(--text);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	@media (hover: hover) {
		.ramo:hover {
			background: var(--bg-hover);
		}
	}
	.ramo:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}
	.r-titulo {
		font-size: 14px;
		font-weight: 600;
		overflow-wrap: anywhere;
	}
	.r-placar {
		font-size: 12px;
		color: var(--text-muted);
	}
	.barra {
		display: flex;
		height: 4px;
		border-radius: 2px;
		overflow: hidden;
		background: var(--bg-soft);
	}
	.barra .certas {
		background: var(--good);
	}
	.barra .erradas {
		background: var(--danger);
	}

	@media (max-width: 620px) {
		.linha {
			grid-template-columns: minmax(0, 1fr);
			gap: 3px;
		}
		/* No celular as abas dividem a largura, e os controles também. */
		.abas button {
			flex: 1;
			justify-content: center;
		}
		.campo {
			flex: 1 1 40%;
		}
		.resolver {
			flex-basis: 100%;
			margin-left: 0;
		}
	}

	@media (pointer: coarse) {
		/* O × de 20px é pequeno para o dedo; e campo com fonte abaixo de 16px faz
		   o iPhone dar zoom na página ao ser tocado. */
		.tirar {
			width: 32px;
			height: 32px;
		}
		.vincular,
		.campo select {
			padding-block: 9px;
			font-size: 16px;
		}
		.abas button {
			min-height: 44px;
		}
		.resolver {
			padding-block: 11px;
		}
	}
</style>
