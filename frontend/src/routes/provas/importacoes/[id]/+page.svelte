<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { beforeNavigate, goto } from '$app/navigation';
	import { ApiError } from '$lib/api';
	import PageHead from '$lib/components/PageHead.svelte';
	import Campo from '$lib/provas/Campo.svelte';
	import EtapaTextos from '$lib/provas/EtapaTextos.svelte';
	import PreviaDoOriginal from '$lib/provas/PreviaDoOriginal.svelte';
	import QuestaoEditor from '$lib/provas/QuestaoEditor.svelte';
	import Recorte from '$lib/provas/Recorte.svelte';
	import { provasApi } from '$lib/provas/api';
	import {
		aplicarMaterias,
		aplicarRecorte,
		blocoDoDestino,
		destinosDoRecorte,
		incompleta,
		novaQuestao,
		numerosFaltando,
		problemasDaQuestao,
		proximaPendente,
		questaoPendente,
		regiaoDaQuestao,
		regiaoDoCaderno,
		regiaoDoTrecho,
		retanguloInicial,
		rotuloDaRegiao,
		temFigura,
		escreverNumeros,
		eTrecho,
		trechoInicial
	} from '$lib/provas/revisao';
	import type { EstadoImportacao, Importacao, Origem } from '$lib/provas/types';

	const ROTULO: Record<EstadoImportacao, string> = {
		na_fila: 'Na fila',
		processando: 'Processando',
		em_revisao: 'Em revisão',
		falhou: 'Falhou',
		publicada: 'Publicada',
		cancelada: 'Cancelada'
	};

	let imp = $state<Importacao | null>(null);
	let erro = $state('');
	let aviso = $state('');
	let conflito = $state(false);
	let ocupado = $state(false);
	let alterado = $state(false);
	let indice = $state(0);
	let regiaoIdx = $state(0);
	let destino = $state('novo:q');
	// O painel da direita mostra a questão no caderno; a região inteira e o
	// recorte de figura só quando o curador pede — a maioria das questões nem
	// tem figura.
	let painel = $state<'questao' | 'regiao' | 'recorte' | 'trecho'>('questao');
	let zoom = $state(100);
	let faltando = $state('');
	let novoGabarito = $state<HTMLInputElement | null>(null);
	// O trecho relido: em que faixa do caderno ele é marcado, e para qual
	// questão voltar quando a leitura terminar — a fila devolve a tela ao começo.
	let trechoIdx = $state(0);
	let voltarPara = $state<{ numero: number; alertas: number } | null>(null);

	type Etapa = 'dados' | 'textos' | 'questoes' | 'publicar';
	const ETAPAS: { id: Etapa; nome: string }[] = [
		{ id: 'dados', nome: 'Dados da prova' },
		{ id: 'textos', nome: 'Textos de apoio' },
		{ id: 'questoes', nome: 'Questões' },
		{ id: 'publicar', nome: 'Publicar' }
	];
	let etapa = $state<Etapa>('questoes');
	let textoIdx = $state(0);
	/** Quais questões o mapa mostra: as com defeito primeiro, que são o trabalho. */
	let filtro = $state<'problema' | 'conferir' | 'figura' | 'todas'>('todas');

	const id = $derived(page.params.id ?? '');
	const emRevisao = $derived(imp?.estado === 'em_revisao');
	const andando = $derived(imp?.estado === 'na_fila' || imp?.estado === 'processando');
	const q = $derived(imp?.rascunho.questoes[indice]);
	const regiao = $derived<Origem | undefined>(imp?.regioes[regiaoIdx]);
	const destinos = $derived(q && imp ? destinosDoRecorte(q, imp.rascunho.apoios) : []);
	const inicial = $derived(
		regiao && q && imp ? retanguloInicial(regiao, blocoDoDestino(imp.rascunho, q, destino)) : [0, 0, 0, 0]
	);
	const faltam = $derived(imp ? numerosFaltando(imp.rascunho) : []);
	const faixas = $derived(imp ? imp.regioes.flatMap((r, i) => (regiaoDoCaderno(r) ? [i] : [])) : []);
	const regiaoTrecho = $derived<Origem | undefined>(imp?.regioes[trechoIdx]);
	const inicialTrecho = $derived(regiaoTrecho ? trechoInicial(regiaoTrecho, q) : [0, 0, 0, 0]);
	// Importação anterior à posição exata da questão: a origem é a região toda.
	// O trecho marcado não conta: ele já é o lugar da questão.
	const soRegiao = $derived(
		!!q &&
			!!imp &&
			q.origens.length > 0 &&
			q.origens.every((o) =>
				imp!.regioes.some(
					(r) =>
						!eTrecho(r) &&
						r.regiao === o.regiao &&
						r.retangulo.every((v, k) => Math.abs(v - o.retangulo[k]) < 0.5)
				)
			)
	);
	const conferidas = $derived(imp?.rascunho.questoes.filter((x) => !questaoPendente(x)).length ?? 0);
	// A publicada é o histórico da prova; a que processa precisa parar antes.
	const excluivel = $derived(!!imp && imp.estado !== 'publicada' && imp.estado !== 'processando');
	const textosOk = $derived(imp?.rascunho.apoios.filter((a) => a.revisado && a.questoes.length > 0).length ?? 0);
	/** O que falta em cada etapa, na barra: o curador vê onde está o trabalho. */
	const progresso = $derived.by((): Record<Etapa, { texto: string; falta: boolean } | null> => {
		if (!imp) return { dados: null, textos: null, questoes: null, publicar: null };
		const r = imp.rascunho;
		const semDados = !r.orgao || !r.gabarito.tipo;
		return {
			dados: semDados ? { texto: 'falta', falta: true } : null,
			textos: r.apoios.length ? { texto: `${textosOk}/${r.apoios.length}`, falta: textosOk < r.apoios.length } : null,
			questoes: { texto: `${conferidas}/${r.questoes.length}`, falta: conferidas < r.questoes.length },
			publicar: imp.pendencias.length
				? { texto: `${imp.pendencias.length} pendências`, falta: true }
				: { texto: 'pronta', falta: false }
		};
	});

	/**
	 * As questões desta prova que são o trabalho da revisão. As reaproveitadas já
	 * estão cadastradas em outra prova do concurso e entram nesta por referência:
	 * não aparecem, a não ser que o curador peça.
	 */
	const questoesProprias = $derived(imp ? imp.rascunho.questoes.flatMap((x, i) => (x.igualA ? [] : [i])) : []);
	let mostrarReaproveitadas = $state(false);
	const numerosReaproveitados = $derived(imp?.rascunho.questoes.filter((x) => x.igualA).map((x) => x.numero) ?? []);
	const reaproveitadas = $derived(numerosReaproveitados.length);
	/** De quais provas vieram — "TJCE 2026 · F06", sem o número da questão lá. */
	const provasDeOrigem = $derived([
		...new Set(imp?.rascunho.questoes.filter((x) => x.igualA).map((x) => x.igualA.replace(/, questão \d+$/, '')) ?? [])
	]);
	const daRevisao = (f: (i: number) => boolean) => questoesProprias.filter(f);
	/** Índices das questões com algum defeito — faltam alternativas, figura sem recorte… */
	const comProblema = $derived(daRevisao((i) => problemasDaQuestao(imp!.rascunho.questoes[i], imp!.rascunho).length > 0));
	const faltaConferir = $derived(daRevisao((i) => questaoPendente(imp!.rascunho.questoes[i])));
	/** Questões com figura: o recorte, só o curador confere. */
	const comFigura = $derived(daRevisao((i) => temFigura(imp!.rascunho.questoes[i])));
	const figurasAConferir = $derived(comFigura.filter((i) => questaoPendente(imp!.rascunho.questoes[i])));
	const visiveis = $derived(
		filtro === 'problema'
			? comProblema
			: filtro === 'conferir'
				? faltaConferir
				: filtro === 'figura'
					? comFigura
					: mostrarReaproveitadas
						? (imp?.rascunho.questoes.map((_, i) => i) ?? [])
						: questoesProprias
	);
	const incompletas = $derived(imp?.rascunho.questoes.filter(incompleta).length ?? 0);

	function escolherFiltro(f: typeof filtro) {
		filtro = f;
		const lista =
			f === 'problema' ? comProblema : f === 'conferir' ? faltaConferir : f === 'figura' ? comFigura : null;
		if (lista && lista.length && !lista.includes(indice)) irPara(lista[0]);
	}

	/** Anterior e próxima andam dentro do filtro. */
	function passo(delta: number) {
		const pos = visiveis.indexOf(indice);
		const alvo = pos < 0 ? visiveis[0] : visiveis[pos + delta];
		if (alvo !== undefined) irPara(alvo);
	}

	async function reler() {
		if (!imp) return;
		receber(await provasApi.reler(imp.id, imp.versao), false);
	}

	function abrirTrecho() {
		if (!imp) return;
		trechoIdx = regiaoDoTrecho(imp.regioes, q);
		painel = 'trecho';
	}

	// A fila lê o rascunho salvo: o que foi editado vai antes, ou a leitura
	// nova chegaria por cima e a edição se perderia.
	async function relerTrecho(origem: Origem) {
		await executar(async () => {
			if (!imp || !q) return;
			if (alterado) await salvar();
			const numero = q.numero;
			const nova = await provasApi.relerTrecho(imp.id, imp.versao, numero, origem);
			voltarPara = { numero, alertas: imp.rascunho.alertas.length };
			receber(nova);
		});
	}

	/** O que o relógio traz enquanto a fila anda; ao voltar do trecho, a questão dele. */
	function acompanhar(nova: Importacao) {
		receber(nova, false);
		if (nova.estado !== 'em_revisao' || !voltarPara) return;
		const { numero, alertas } = voltarPara;
		voltarPara = null;
		irPara(nova.rascunho.questoes.findIndex((x) => x.numero === numero));
		const novos = nova.rascunho.alertas.slice(alertas);
		aviso = novos.length
			? novos.join(' ')
			: `A questão ${numero} foi lida de novo pelo trecho marcado. Confira com o original e marque como conferida.`;
	}

	async function procurarCadastradas() {
		if (!imp) return;
		if (alterado) await salvar();
		const antes = reaproveitadas;
		receber(await provasApi.procurarCadastradas(imp.id, imp.versao));
		const achadas = reaproveitadas - antes;
		aviso =
			achadas > 0
				? `${achadas} questões já estavam cadastradas em outra prova do concurso e entraram por referência.`
				: 'Nenhuma questão nova igual às das provas publicadas deste concurso.';
	}

	/** Leva à questão, na etapa de questões — do texto de apoio, por exemplo. */
	function abrirQuestao(numero: number) {
		const i = imp?.rascunho.questoes.findIndex((x) => x.numero === numero) ?? -1;
		if (i < 0) return;
		irPara(i);
		etapa = 'questoes';
	}

	/** Leva ao texto de apoio, na etapa de textos. */
	function abrirTexto(apoioId: string) {
		const i = imp?.rascunho.apoios.findIndex((a) => a.id === apoioId) ?? -1;
		if (i < 0) return;
		textoIdx = i;
		etapa = 'textos';
	}

	function receber(nova: Importacao, manterPosicao = true) {
		imp = nova;
		alterado = false;
		conflito = false;
		if (!manterPosicao) indice = 0;
		indice = Math.min(indice, Math.max(0, nova.rascunho.questoes.length - 1));
	}

	function falhar(e: unknown) {
		conflito = e instanceof ApiError && e.status === 409;
		erro = e instanceof Error ? e.message : 'erro inesperado';
	}

	async function executar(fn: () => Promise<void>) {
		ocupado = true;
		erro = '';
		aviso = '';
		try {
			await fn();
		} catch (e) {
			falhar(e);
		} finally {
			ocupado = false;
		}
	}

	async function recarregar() {
		receber(await provasApi.importacao(id));
	}

	function irPara(i: number) {
		if (!imp || i < 0 || i >= imp.rascunho.questoes.length) return;
		indice = i;
		regiaoIdx = regiaoDaQuestao(imp.regioes, imp.rascunho.questoes[i]);
		destino = destinosDoRecorte(imp.rascunho.questoes[i], imp.rascunho.apoios)[0]?.chave ?? 'novo:q';
		painel = 'questao';
	}

	/** Abre o recorte apontado para uma figura, na região em que ela está. */
	function ajustarFigura(chave: string) {
		if (!imp || !q) return;
		destino = chave;
		const origem = blocoDoDestino(imp.rascunho, q, chave)?.origem;
		const i = origem ? imp.regioes.findIndex((r) => r.regiao === origem.regiao) : -1;
		regiaoIdx = i >= 0 ? i : regiaoDaQuestao(imp.regioes, q);
		painel = 'recorte';
		// A figura pode estar num material de apoio lá embaixo; o painel fica em cima.
		document.querySelector('.lateral')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	function alterar() {
		alterado = true;
	}

	async function salvar() {
		if (!imp) return;
		receber(await provasApi.salvar(imp.id, imp.versao, $state.snapshot(imp.rascunho)));
	}

	// O gabarito cita o cargo pelo código; a leitura da capa às vezes traz o nome
	// no lugar dele. Um clique põe o código do gabarito na prova, guarda o que
	// estava como nome e salva — a pendência vem da versão salva.
	const codigoDoGabarito = $derived(imp?.rascunho.gabarito.cargo.trim() ?? '');
	const cargoDiverge = $derived(
		!!imp && !!codigoDoGabarito && imp.rascunho.cargo.trim().toUpperCase() !== codigoDoGabarito.toUpperCase()
	);

	async function usarCargoDoGabarito() {
		if (!imp) return;
		const antes = imp.rascunho.cargo.trim();
		if (antes && !imp.rascunho.cargoNome.trim()) imp.rascunho.cargoNome = antes;
		imp.rascunho.cargo = codigoDoGabarito;
		await salvar();
	}

	async function publicar() {
		if (!imp) return;
		const { provaId } = await provasApi.publicar(imp.id, imp.versao);
		await goto(`/provas/${provaId}`);
	}

	// O que foi editado entra na conta: salva, e as pendências que o servidor
	// devolve são as da versão que vai ao catálogo. Sobrando alguma, a aba mostra.
	async function salvarEPublicar() {
		if (!imp) return;
		if (alterado) await salvar();
		if (imp.pendencias.length === 0) await publicar();
	}

	/** Aonde levar o curador para resolver a pendência. */
	function destinoDaPendencia(p: string): { rotulo: string; ir: () => void } | null {
		const numero = p.match(/quest(?:ão|ao)\s+(\d+)/i)?.[1];
		if (numero) return { rotulo: `Abrir a questão ${numero}`, ir: () => abrirQuestao(Number(numero)) };
		if (/texto de apoio/i.test(p)) return { rotulo: 'Abrir os textos', ir: () => (etapa = 'textos') };
		if (/cargo|caderno|banca|órgão|gabarito|total/i.test(p)) return { rotulo: 'Abrir os dados', ir: () => (etapa = 'dados') };
		return null;
	}

	function adicionarFaltando() {
		if (!imp || !faltando) return;
		const numero = Number(faltando);
		imp.rascunho.questoes = [...imp.rascunho.questoes, novaQuestao(numero, regiao)].sort(
			(a, b) => a.numero - b.numero
		);
		irPara(imp.rascunho.questoes.findIndex((x) => x.numero === numero));
		faltando = '';
		alterar();
	}

	function removerQuestao() {
		if (!imp || !q || !confirm(`Remover a questão ${q.numero} do rascunho?`)) return;
		imp.rascunho.questoes = imp.rascunho.questoes.filter((_, k) => k !== indice);
		irPara(Math.max(0, indice - 1));
		alterar();
	}

	function recorteAplicado(arquivo: string, origem: Origem) {
		if (!imp || !q) return;
		aplicarRecorte(imp.rascunho, q, destino, arquivo, origem);
		// Figura nova passa a ser um destino existente: o próximo ajuste mira nela.
		const existentes = destinosDoRecorte(q, imp.rascunho.apoios).filter((d) => !d.chave.startsWith('novo:'));
		if (destino.startsWith('novo:') && existentes.length) destino = existentes.at(-1)!.chave;
		alterar();
	}

	// A IA lê o rascunho salvo e sugere a matéria de cada questão; a sugestão só
	// muda a tela, e vale depois de salvar.
	async function sugerirMaterias() {
		if (!imp) return;
		const n = aplicarMaterias(imp.rascunho, await provasApi.sugerirMaterias(imp.id));
		if (n) alterar();
		aviso = n
			? `${n} questões mudaram de matéria. Confira e salve a revisão.`
			: 'As matérias sugeridas são as que já estão no rascunho.';
	}

	async function excluir() {
		if (!imp) return;
		if (!confirm('Excluir esta importação? O rascunho e tudo o que a extração leu somem de vez.')) return;
		await provasApi.excluir(imp.id, imp.versao);
		// Não há mais o que salvar: a saída não pede confirmação.
		alterado = false;
		await goto('/questoes?aba=curadoria');
	}

	async function trocarGabarito() {
		const arquivo = novoGabarito?.files?.[0];
		if (!imp || !arquivo) return;
		receber(await provasApi.atualizarGabarito(imp.id, imp.versao, arquivo));
	}

	onMount(() => {
		void executar(async () => {
			await recarregar();
			if (!imp) return;
			irPara(questoesProprias[0] ?? 0);
			// Questão com defeito vem primeiro no mapa: é o que precisa de mão.
			if (comProblema.length > 0) escolherFiltro('problema');
			else if (figurasAConferir.length > 0) escolherFiltro('figura');
			// Texto por conferir ou sem questão vem primeiro: ligar o texto às dez
			// questões antes evita conferir cada uma sem ele.
			if (imp.rascunho.apoios.some((a) => !a.revisado || a.questoes.length === 0)) etapa = 'textos';
		});
		const relogio = setInterval(() => {
			if (andando && !ocupado) provasApi.importacao(id).then(acompanhar).catch(falhar);
		}, 4000);
		return () => clearInterval(relogio);
	});

	beforeNavigate(({ cancel }) => {
		if (alterado && !confirm('Há alterações não salvas. Sair mesmo assim?')) cancel();
	});
</script>

{#snippet cargoDoGabarito()}
	<div class="callout warn cargo-gabarito">
		<span>
			O gabarito é do cargo <b>{codigoDoGabarito}</b>, e a prova está
			{imp?.rascunho.cargo ? `com o cargo “${imp.rascunho.cargo}”` : 'sem o código do cargo'}. Se a capa diz
			“Caderno de Prova '{codigoDoGabarito}'”, é o mesmo cargo.
		</span>
		<button class="btn" type="button" disabled={ocupado} onclick={() => executar(usarCargoDoGabarito)}>
			Usar {codigoDoGabarito} e salvar
		</button>
	</div>
{/snippet}

<svelte:window
	onbeforeunload={(e) => {
		if (alterado) e.preventDefault();
	}}
/>

<PageHead
	icone="prova"
	titulo="Importação de prova"
	sub="Confira cada questão com o original ao lado, ajuste os recortes e publique no catálogo."
	mostrarProps={false}
/>

<div class="page">
	<a class="voltar" href="/questoes?aba=curadoria">← Curadoria</a>

	{#if aviso}<p class="callout" role="status"><span>{aviso}</span></p>{/if}
	{#if erro}
		<div class="form-error" role="alert">
			{erro}
			{#if conflito}
				<button class="btn" type="button" onclick={() => executar(recarregar)}>Recarregar</button>
			{/if}
		</div>
	{/if}

	{#if imp}
		{#if !emRevisao}
			<section class="card">
				<div class="card-top">
					<span class="pill muted">{ROTULO[imp.estado]}</span>
					<span>{imp.rascunho.orgao || 'Órgão não identificado'} · {imp.rascunho.ano || '—'}</span>
					<span>
						{imp.rascunho.cargoNome || `Cargo ${imp.rascunho.cargo || '—'}`} · caderno {imp.rascunho.caderno || '—'}
					</span>
				</div>
				<div class="card-body estado">
					{#if andando && imp.etapa < 0}
						<!-- Etapa negativa relê só uma parte — o gabarito novo ou um trecho
						     marcado — e volta à revisão; não há "etapa X de Y" para mostrar. -->
						<p>
							Lendo de novo só o que você pediu; o resto do rascunho fica como está. A revisão
							volta sozinha quando terminar.
						</p>
						<progress></progress>
					{:else if andando}
						<p>
							Etapa {imp.etapa} de {imp.totalEtapas}. A extração continua mesmo com esta tela
							fechada.
						</p>
						<progress max={imp.totalEtapas} value={imp.etapa}></progress>
					{/if}
					{#if imp.erro && andando}
						<!-- Na fila com erro é repetição agendada, não trava: sem isto, a
						     mensagem crua parecia pedir alguma ação. -->
						<p class="callout warn">
							<span>
								A última tentativa desta etapa falhou, e a fila tenta de novo sozinha, esperando um
								pouco mais a cada vez. Não precisa fazer nada.
								<span class="detalhe">{imp.erro}</span>
							</span>
						</p>
					{:else if imp.erro}
						<p class="callout warn">{imp.erro}</p>
					{/if}
					<div class="acoes">
						{#if imp.estado === 'falhou'}
							<button
								class="btn primary"
								type="button"
								disabled={ocupado}
								onclick={() => executar(async () => receber(await provasApi.reprocessar(imp!.id, imp!.versao)))}
							>
								Retomar de onde parou
							</button>
						{/if}
						{#if imp.estado === 'publicada' && imp.provaId}
							<a class="btn primary" href="/provas/{imp.provaId}">Ver a prova publicada</a>
						{/if}
						{#if andando}
							<button
								class="btn danger"
								type="button"
								disabled={ocupado}
								onclick={() => {
									if (confirm('Parar a extração? O que já foi lido fica, mas ela não continua.'))
										void executar(async () => receber(await provasApi.cancelar(imp!.id, imp!.versao)));
								}}
							>
								Parar a extração
							</button>
						{/if}
						{#if excluivel}
							<button class="btn danger" type="button" disabled={ocupado} onclick={() => executar(excluir)}>
								Excluir importação
							</button>
						{:else if imp.estado === 'processando'}
							<span class="dim">Para excluir, pare a extração antes.</span>
						{/if}
					</div>
				</div>
			</section>
		{:else}
			<p class="identificacao">
				<span class="pill">Em revisão</span>
				{imp.rascunho.orgao || 'Órgão não identificado'} · {imp.rascunho.ano || '—'} ·
				{imp.rascunho.cargoNome || `cargo ${imp.rascunho.cargo || '—'}`} · caderno {imp.rascunho.caderno || '—'}
			</p>

			<!-- A revisão em quatro etapas, na ordem em que se faz: o que é a prova, os
			     textos que várias questões usam, as questões, e publicar. -->
			<div class="barra">
				<div class="etapas" role="tablist" aria-label="Etapas da revisão">
					{#each ETAPAS as e, i (e.id)}
						{@const p = progresso[e.id]}
						<button
							type="button"
							role="tab"
							aria-selected={etapa === e.id}
							class:atual={etapa === e.id}
							onclick={() => (etapa = e.id)}
						>
							<span class="num">{i + 1}</span>
							<span>{e.nome}</span>
							{#if p}<span class="contagem" class:falta={p.falta}>{p.texto}</span>{/if}
						</button>
					{/each}
				</div>
				<span class="salvo" class:pendente={alterado}>{alterado ? 'Alterações não salvas' : 'Tudo salvo'}</span>
				<button class="btn primary" type="button" disabled={ocupado || !alterado} onclick={() => executar(salvar)}>
					Salvar revisão
				</button>
			</div>

			{#if etapa === 'dados'}
				<section class="card">
					<div class="card-top">Identificação da prova</div>
					<div class="card-body formulario">
						<p class="intro">
							Como a prova aparece no catálogo. Os dados vêm da capa do caderno; corrija só se a leitura
							errou.
						</p>
						{#if cargoDiverge}{@render cargoDoGabarito()}{/if}
						<div class="grade">
							<Campo rotulo="Banca" ajuda="Quem aplicou a prova. Por enquanto só FCC; outra banca vira pendência.">
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.banca} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo rotulo="Órgão" ajuda="Sigla do órgão, como TJCE. É por ela que o aluno acha a prova no catálogo.">
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.orgao} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo rotulo="Ano" ajuda="O ano em que a prova foi aplicada.">
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="number" bind:value={imp!.rascunho.ano} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo
								rotulo="Código do cargo"
								ajuda="O código entre aspas em “Caderno de Prova 'F06'”, no quadro do nome do candidato e no alto das páginas. É por ele que o gabarito é conferido."
							>
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.cargo} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo
								rotulo="Nome do cargo"
								ajuda="Por extenso, como no alto da capa: “Analista Judiciário – Especialidade: Sistemas da Informação”. É o que o aluno lê no catálogo e na prova."
							>
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.cargoNome} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo rotulo="Tipo do caderno" ajuda="Como 004. O gabarito tem uma coluna de respostas por tipo de caderno.">
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.caderno} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo rotulo="Total de questões" ajuda="Quantas questões o caderno diz ter. Número que faltar vira pendência.">
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="number" bind:value={imp!.rascunho.total} oninput={alterar} />
								{/snippet}
							</Campo>
						</div>
					</div>
				</section>

				<section class="card">
					<div class="card-top">Gabarito oficial</div>
					<div class="card-body formulario">
						<p class="intro">
							É de onde vem a resposta de cada questão — a IA nunca responde. Sem gabarito, a prova é
							publicada, mas o aluno resolve sem correção.
						</p>
						<div class="grade">
							<Campo
								rotulo="Situação do gabarito"
								ajuda="Preliminar ainda pode mudar com os recursos; definitivo é o final. O aluno vê um aviso quando é preliminar."
							>
								{#snippet children({ id, ajuda })}
									<select {id} aria-describedby={ajuda} bind:value={imp!.rascunho.gabarito.tipo} onchange={alterar}>
										<option value="">Sem gabarito</option>
										<option value="preliminar">Preliminar</option>
										<option value="definitivo">Definitivo</option>
										<option value="nao_informado">O PDF não diz</option>
									</select>
								{/snippet}
							</Campo>
							<Campo
								rotulo="Cargo no gabarito"
								ajuda="O código do cargo lido no PDF do gabarito. Diferente do da capa, as respostas são de outra prova."
							>
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.gabarito.cargo} oninput={alterar} />
								{/snippet}
							</Campo>
							<Campo
								rotulo="Caderno no gabarito"
								ajuda="O tipo de caderno da coluna usada no gabarito. Tem de bater com o tipo do caderno da capa."
							>
								{#snippet children({ id, ajuda })}
									<input {id} aria-describedby={ajuda} type="text" bind:value={imp!.rascunho.gabarito.caderno} oninput={alterar} />
								{/snippet}
							</Campo>
						</div>
						<Campo
							rotulo="Trocar o gabarito"
							ajuda="Envie o PDF novo — por exemplo, o definitivo depois dos recursos. Só o gabarito é lido de novo; as questões e a sua revisão ficam. Salve a revisão antes."
						>
							{#snippet children({ id, ajuda })}
								<div class="gabarito-novo">
									<input {id} aria-describedby={ajuda} type="file" accept="application/pdf" bind:this={novoGabarito} />
									<button
										class="btn"
										type="button"
										disabled={ocupado || alterado}
										onclick={() => executar(trocarGabarito)}
									>
										Ler o gabarito novo
									</button>
								</div>
							{/snippet}
						</Campo>
					</div>
				</section>

				<section class="card perigo">
					<div class="card-top">Excluir importação</div>
					<div class="card-body formulario">
						<p class="intro">
							Apaga este rascunho e tudo o que a extração leu; não dá para desfazer. Os PDFs saem na limpeza
							automática. Rascunho parado por 30 dias é cancelado sozinho.
						</p>
						<div>
							<button class="btn danger" type="button" disabled={ocupado} onclick={() => executar(excluir)}>
								Excluir importação
							</button>
						</div>
					</div>
				</section>
			{:else if etapa === 'textos'}
				<EtapaTextos
					bind:rascunho={imp.rascunho}
					bind:indice={textoIdx}
					importacao={imp.id}
					versao={imp.versao}
					regioes={imp.regioes}
					onalterar={alterar}
					onabrirQuestao={abrirQuestao}
				/>
			{:else if etapa === 'questoes'}
				<div class="cabeca-etapa">
					<p class="intro">
						Confira cada questão com o caderno ao lado e marque. Comece por "Com problema": são as que
						a extração deixou com defeito. Número verde já está conferido; laranja tem problema.
					</p>
					<button
						class="btn"
						type="button"
						disabled={ocupado}
						title="A IA lê o rascunho salvo e sugere a matéria de cada questão; nada muda até você salvar"
						onclick={() => executar(sugerirMaterias)}
					>
						Sugerir matérias com IA
					</button>
				</div>

				{#if incompletas > 0}
					<div class="callout warn reler">
						<span>
							<b>{incompletas === 1 ? '1 questão está incompleta' : `${incompletas} questões estão incompletas`}</b>
							— faltam alternativas, ou a extração as cortou entre duas faixas do caderno. A IA relê só
							essas, num recorte centrado onde foram cortadas; o resto do rascunho fica como está.
						</span>
						<button
							class="btn"
							type="button"
							disabled={ocupado || alterado}
							title={alterado ? 'Salve a revisão antes: a releitura parte do rascunho salvo' : ''}
							onclick={() => executar(reler)}
						>
							{incompletas === 1 ? 'Reler a questão' : `Reler as ${incompletas} questões`}
						</button>
					</div>
				{/if}

				{#if reaproveitadas > 0}
					<p class="callout referencia">
						<span>
							As questões <b>{escreverNumeros(numerosReaproveitados)}</b> já estão cadastradas em
							{provasDeOrigem.join(', ')} e entram nesta prova por referência: não precisam de conferência, e o aluno
							responde a prova inteira.
						</span>
						<button type="button" class="ir" onclick={() => (mostrarReaproveitadas = !mostrarReaproveitadas)}>
							{mostrarReaproveitadas ? 'esconder' : 'mostrar mesmo assim'}
						</button>
					</p>
				{/if}
				<p class="procurar">
					<button
						type="button"
						class="ir"
						disabled={ocupado}
						title="Compara as questões desta prova com as das provas já publicadas do mesmo órgão e ano"
						onclick={() => executar(procurarCadastradas)}
					>
						Procurar questões já cadastradas no concurso
					</button>
				</p>
				<div class="filtros" role="tablist" aria-label="Quais questões mostrar">
					<button type="button" role="tab" aria-selected={filtro === 'problema'} onclick={() => escolherFiltro('problema')}>
						Com problema <span class="contagem" class:falta={comProblema.length > 0}>{comProblema.length}</span>
					</button>
					<button type="button" role="tab" aria-selected={filtro === 'conferir'} onclick={() => escolherFiltro('conferir')}>
						Falta conferir <span class="contagem" class:falta={faltaConferir.length > 0}>{faltaConferir.length}</span>
					</button>
					<button
						type="button"
						role="tab"
						aria-selected={filtro === 'figura'}
						title="Questão com figura espera você conferir cada recorte com o original"
						onclick={() => escolherFiltro('figura')}
					>
						Com figura <span class="contagem" class:falta={figurasAConferir.length > 0}>{figurasAConferir.length}</span>
					</button>
					<button type="button" role="tab" aria-selected={filtro === 'todas'} onclick={() => escolherFiltro('todas')}>
						Todas <span class="contagem">{mostrarReaproveitadas ? imp.rascunho.questoes.length : questoesProprias.length}</span>
					</button>
				</div>

				<!-- As questões num relance: verde é conferida, laranja tem problema, a atual em destaque. -->
				<nav class="mapa" aria-label="Questões">
					{#each visiveis as i (i)}
						{@const item = imp.rascunho.questoes[i]}
						<button
							type="button"
							class:atual={i === indice}
							class:ok={!questaoPendente(item)}
							class:problema={comProblema.includes(i)}
							class:figura={temFigura(item)}
							aria-current={i === indice ? 'true' : undefined}
							title="Questão {item.numero}{comProblema.includes(i)
								? ' · tem problema'
								: questaoPendente(item)
									? ' · falta conferir'
									: ' · conferida'}{temFigura(item) ? ' · tem figura' : ''}"
							onclick={() => irPara(i)}
						>
							{item.numero}
						</button>
					{:else}
						<p class="dim">
							{filtro === 'problema'
								? 'Nenhuma questão com problema.'
								: filtro === 'figura'
									? 'Nenhuma questão com figura.'
									: 'Todas as questões estão conferidas.'}
						</p>
					{/each}
				</nav>

				<div class="navegacao">
					<button class="btn" type="button" disabled={visiveis.indexOf(indice) <= 0} onclick={() => passo(-1)}>
						Anterior
					</button>
					<button
						class="btn"
						type="button"
						disabled={visiveis.indexOf(indice) >= visiveis.length - 1}
						onclick={() => passo(1)}
					>
						Próxima
					</button>
					{#if faltam.length > 0}
						<span class="faltam">
							<span class="dim">A extração não achou as questões {faltam.join(', ')}.</span>
							<select bind:value={faltando} aria-label="Questão que faltou">
								<option value="">Escolha uma para transcrever…</option>
								{#each faltam as n (n)}<option value={String(n)}>Questão {n}</option>{/each}
							</select>
							<button class="btn" type="button" disabled={!faltando} onclick={adicionarFaltando}>Adicionar</button>
						</span>
					{/if}
				</div>

				<div class="revisao">
					<section class="card">
						<div class="card-top">
							<span class="pill" class:muted={!q?.revisada}>{q?.numero ?? '—'}</span>
							<span>Questão {q?.numero ?? '—'} de {imp.rascunho.questoes.length}</span>
							{#if q?.revisada}<span class="dim">conferida</span>{/if}
							{#if q && temFigura(q)}<span class="pill tem-figura">com figura</span>{/if}
							{#if q?.igualA}
								<span
									class="pill reaproveitada"
									title="O conteúdo é o já publicado nessa prova, guardado uma vez só; a resposta é a do gabarito desta. Editar cria uma versão só desta prova."
									>igual à {q.igualA}</span
								>
							{/if}
						</div>
						<div class="card-body">
							{#if q && temFigura(q) && !q.revisada}
								<p class="callout warn aviso-figura">
									<span>
										<b>Esta questão tem figura.</b> Compare cada recorte com o original ao lado — a figura inteira,
										com título e legenda, sem pedaço da questão vizinha — e marque "Conferi o recorte". Conferidos
										todos, a questão fica conferida ao salvar.
									</span>
								</p>
							{/if}
							{#if q}
								<!-- Uma instância por questão: a edição aberta não vaza para a próxima. -->
								{#key indice}
									<QuestaoEditor
										bind:rascunho={imp.rascunho}
										{indice}
										onalterar={alterar}
										onremover={removerQuestao}
										onajustarFigura={ajustarFigura}
										onabrirTexto={abrirTexto}
										onproxima={() => {
											const i = proximaPendente(imp!.rascunho.questoes, indice);
											irPara(i >= 0 ? i : Math.min(indice + 1, imp!.rascunho.questoes.length - 1));
										}}
									/>
								{/key}
							{:else}
								<p>O rascunho não tem questões. Adicione as que faltam.</p>
							{/if}
						</div>
					</section>

					<aside class="card lateral">
						<div class="card-top">
							{#if painel === 'recorte'}
								Recortar figura · questão {q?.numero ?? '—'}
							{:else if painel === 'trecho'}
								Ler de novo · questão {q?.numero ?? '—'}
							{:else if painel === 'regiao'}
								Região do caderno
							{:else}
								Questão {q?.numero ?? '—'} no caderno original
							{/if}
						</div>
						<div class="card-body lateral-corpo">
							<div class="abas" role="group" aria-label="O que mostrar">
								<button class="btn" class:primary={painel === 'questao'} type="button" onclick={() => (painel = 'questao')}>
									Só a questão
								</button>
								<button class="btn" class:primary={painel === 'regiao'} type="button" onclick={() => (painel = 'regiao')}>
									Região inteira
								</button>
								<button
									class="btn"
									class:primary={painel === 'recorte'}
									type="button"
									title="Para uma figura que a extração não pegou ou pegou mal"
									onclick={() => {
										if (!destinos.some((d) => d.chave === destino)) destino = 'novo:q';
										painel = 'recorte';
									}}
								>
									Recortar figura
								</button>
								<button
									class="btn"
									class:primary={painel === 'trecho'}
									type="button"
									disabled={!q}
									title="Para a questão que a extração leu mal: marque no caderno a parte que ficou errada, e a IA lê só esse trecho"
									onclick={abrirTrecho}
								>
									Ler de novo
								</button>
							</div>

							{#if painel !== 'recorte' && painel !== 'trecho'}
								<label class="zoom">
									Zoom
									<input type="range" min="60" max="220" step="10" bind:value={zoom} />
									<span>{zoom}%</span>
								</label>
							{/if}

							{#if painel === 'questao'}
								{#if q && q.origens.length > 0}
									{#if soRegiao}
										<p class="dim">
											Esta importação é anterior à posição exata da questão, então aparece a região
											inteira. Para ver só a questão, use "Extrair de novo" na prova publicada.
										</p>
									{/if}
									{#each q.origens as o, i (o.regiao + ':' + i)}
										<PreviaDoOriginal
											importacao={imp.id}
											versao={imp.versao}
											origem={o}
											zoom={zoom}
											rotulo={q.origens.length > 1
												? `Parte ${i + 1} de ${q.origens.length} · página ${o.pagina}`
												: `Página ${o.pagina}`}
										/>
									{/each}
								{:else if q}
									<p class="dim">Questão adicionada à mão: não há trecho do caderno ligado a ela.</p>
								{/if}
							{:else if painel === 'regiao'}
								<Campo rotulo="Região do caderno" ajuda="O caderno é lido em faixas que se sobrepõem; escolha a que quer ver.">
									{#snippet children({ id, ajuda })}
										<select {id} aria-describedby={ajuda} bind:value={regiaoIdx}>
											{#each imp!.regioes as r, i (i)}
												<option value={i}>{rotuloDaRegiao(r, i, imp!.regioes.length)}</option>
											{/each}
										</select>
									{/snippet}
								</Campo>
								{#if regiao}
									<PreviaDoOriginal
										importacao={imp.id}
										versao={imp.versao}
										origem={regiao}
										zoom={zoom}
										rotulo="Página {regiao.pagina} · região {regiaoIdx + 1}"
									/>
								{/if}
							{:else if painel === 'trecho'}
								{#if q}
									<p class="dim">
										Desenhe o retângulo em volta da parte da questão {q.numero} que ficou errada — ela
										inteira, só o enunciado ou só as alternativas que faltaram. A IA lê só esse trecho
										e troca o que ele trouxer: o enunciado, se veio, e cada alternativa pela letra. O
										resto da questão, a matéria, a resposta e os textos ligados ficam.
									</p>
									<Campo rotulo="Onde a questão está" ajuda="A faixa do caderno em que está a parte que ficou errada.">
										{#snippet children({ id, ajuda })}
											<select {id} aria-describedby={ajuda} bind:value={trechoIdx}>
												{#each faixas as i (i)}
													<option value={i}>{rotuloDaRegiao(imp!.regioes[i], i, imp!.regioes.length)}</option>
												{/each}
											</select>
										{/snippet}
									</Campo>
									{#if regiaoTrecho}
										<Recorte
											importacao={imp.id}
											versao={imp.versao}
											regiao={regiaoTrecho}
											inicial={inicialTrecho}
											rotulo="Ler este trecho da questão {q.numero}"
											onmarcar={relerTrecho}
										/>
									{/if}
								{/if}
							{:else}
								<p class="dim">
									Escolha para onde vai a figura, desenhe o retângulo sobre ela e aplique. O recorte sai
									do PDF original.
								</p>
								{#if q}
									<Campo rotulo="A figura vai para" ajuda="Trocar uma figura que já existe, ou pôr uma nova no enunciado ou numa alternativa.">
										{#snippet children({ id, ajuda })}
											<select {id} aria-describedby={ajuda} bind:value={destino}>
												{#each destinos as d (d.chave)}<option value={d.chave}>{d.rotulo}</option>{/each}
											</select>
										{/snippet}
									</Campo>
								{/if}
								<Campo rotulo="Região do caderno" ajuda="A faixa do caderno em que a figura está.">
									{#snippet children({ id, ajuda })}
										<select {id} aria-describedby={ajuda} bind:value={regiaoIdx}>
											{#each imp!.regioes as r, i (i)}
												<option value={i}>{rotuloDaRegiao(r, i, imp!.regioes.length)}</option>
											{/each}
										</select>
									{/snippet}
								</Campo>
								{#if regiao}
									<Recorte
										importacao={imp.id}
										versao={imp.versao}
										{regiao}
										{inicial}
										onaplicar={recorteAplicado}
									/>
								{/if}
							{/if}
						</div>
					</aside>
				</div>
			{:else}
				<section class="card">
					<div class="card-top">Publicar no catálogo</div>
					<div class="card-body formulario">
						<p class="intro">
							Publicar põe a prova no catálogo para todos os alunos. Depois, "Abrir revisão" na página da
							prova traz ela de volta para corrigir; publicar de novo substitui a versão no ar.
						</p>
						<ul class="resumo">
							<li><b>{conferidas} de {imp.rascunho.questoes.length}</b> questões conferidas</li>
						{#if reaproveitadas > 0}
							<li>
								<b>{reaproveitadas}</b> questões iguais às de outra prova já publicada do mesmo concurso — usam o conteúdo de lá,
								guardado uma vez só
							</li>
						{/if}
						{#if comFigura.length > 0}
							<li class:falta-figura={figurasAConferir.length > 0}>
								<b>{comFigura.length - figurasAConferir.length} de {comFigura.length}</b> questões com figura conferidas — cada recorte
								passa por você
							</li>
						{/if}
							<li>
								<b>{textosOk} de {imp.rascunho.apoios.length}</b> textos de apoio conferidos e ligados às questões
							</li>
							<li>Gabarito: <b>{imp.rascunho.gabarito.tipo || 'não enviado'}</b></li>
						</ul>
						{#if !imp.conferenciaObrigatoria}
							<p class="dim">
								Neste ambiente a conferência não é obrigatória: dá para publicar sem marcar as questões.
							</p>
						{/if}
						{#if alterado}
							<p class="callout warn">
								<span>
									Há alterações não salvas: as pendências abaixo são as da última versão salva. “Salvar e
									publicar” grava o que você mudou, confere de novo e publica se não sobrar nada.
								</span>
							</p>
						{/if}

						<h3>Pendências ({imp.pendencias.length})</h3>
						<p class="ajuda">O que impede publicar. Resolva cada uma na etapa em que ela está e salve.</p>
						{#if imp.pendencias.length > 0}
							<ul class="pendencias">
								{#each imp.pendencias as p, i (i)}
									{@const destino = destinoDaPendencia(p)}
									<li>
										{p}
										{#if destino}<button type="button" class="ir" onclick={destino.ir}>{destino.rotulo} 	</button>{/if}
									</li>
								{/each}
							</ul>
						{:else}
							<p class="pronta">Nenhuma pendência: a prova pode ser publicada.</p>
						{/if}
						{#if cargoDiverge}{@render cargoDoGabarito()}{/if}

						{#if imp.rascunho.alertas.length > 0}
							<h3>Alertas da extração ({imp.rascunho.alertas.length})</h3>
							<p class="ajuda">Não impedem publicar: são pontos em que a leitura teve dúvida. Confira no original.</p>
							{#each imp.rascunho.alertas as a, i (i)}<p class="callout warn">{a}</p>{/each}
						{/if}

						<div>
							<button
								class="btn primary"
								type="button"
								disabled={ocupado || (!alterado && imp.pendencias.length > 0)}
								onclick={() => executar(salvarEPublicar)}
							>
								{alterado ? 'Salvar e publicar' : 'Publicar no catálogo'}
							</button>
						</div>
					</div>
				</section>
			{/if}
		{/if}
	{:else if !erro}
		<p class="page-sub">Carregando…</p>
	{/if}
</div>

<style>
	.voltar {
		display: inline-block;
		margin-bottom: 14px;
		font-size: 13px;
		color: var(--text-muted);
	}
	.estado {
		display: grid;
		gap: 10px;
	}
	.estado p {
		margin: 0;
	}
	progress {
		width: 100%;
	}
	.acoes,
	.navegacao,
	.gabarito-novo {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}
	.identificacao {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
		margin: 0 0 10px;
		font-size: 13.5px;
		color: var(--text-muted);
	}

	/* A barra fica no alto: as etapas e o salvar sempre à mão. */
	.barra {
		position: sticky;
		top: 0;
		z-index: 3;
		display: flex;
		flex-wrap: wrap;
		gap: 10px 14px;
		align-items: center;
		margin: 0 0 16px;
		padding: 8px 10px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.etapas {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		flex: 1;
	}
	.etapas button {
		display: flex;
		gap: 8px;
		align-items: center;
		padding: 7px 12px;
		font: inherit;
		font-size: 13px;
		color: var(--text-muted);
		background: transparent;
		border: 1px solid transparent;
		border-radius: 8px;
		cursor: pointer;
	}
	.etapas button:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	.etapas button.atual {
		color: var(--text);
		background: var(--accent-soft);
		border-color: var(--accent);
		font-weight: 600;
	}
	.etapas .num {
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		font-size: 11px;
		font-weight: 700;
		color: var(--bg);
		background: var(--text-faint);
	}
	.etapas button.atual .num {
		background: var(--accent);
	}
	.contagem {
		font-family: var(--font-mono);
		font-size: 11px;
		padding: 1px 6px;
		border-radius: 999px;
		color: var(--good);
		background: var(--good-soft);
	}
	.contagem.falta {
		color: var(--warn);
		background: var(--warn-soft);
	}
	.salvo {
		font-size: 12.5px;
		color: var(--text-faint);
	}
	.salvo.pendente {
		color: var(--warn);
	}

	.formulario {
		display: grid;
		gap: 16px;
	}
	.intro {
		margin: 0;
		font-size: 13.5px;
		line-height: 1.5;
		color: var(--text-muted);
		max-width: 80ch;
	}
	.grade {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
		gap: 16px 20px;
	}
	.perigo {
		border-color: color-mix(in srgb, var(--danger) 40%, var(--border));
	}

	.cabeca-etapa {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 16px;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}
	.mapa {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		margin-bottom: 10px;
	}
	.mapa button {
		min-width: 34px;
		height: 30px;
		padding: 0 6px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
	}
	.mapa button:hover {
		border-color: var(--border-strong);
		color: var(--text);
	}
	.mapa button.ok {
		color: var(--good);
		background: var(--good-soft);
		border-color: transparent;
	}
	.mapa button.problema {
		color: var(--warn);
		background: var(--warn-soft);
		border-color: transparent;
	}
	/* Questão com figura: um ponto no canto, na cor do estado dela. */
	.mapa button.figura {
		position: relative;
	}
	.mapa button.figura::after {
		content: '';
		position: absolute;
		top: 3px;
		right: 3px;
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: currentColor;
	}
	.mapa button.atual {
		color: var(--bg);
		background: var(--accent);
		border-color: var(--accent);
		font-weight: 700;
	}
	.navegacao {
		margin-bottom: 12px;
	}
	.reler {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 14px;
		align-items: center;
		margin: 0 0 12px;
		font-size: 13px;
	}
	.filtros {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		margin-bottom: 8px;
	}
	.filtros button {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 5px 10px;
		font: inherit;
		font-size: 12.5px;
		color: var(--text-muted);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 999px;
		cursor: pointer;
	}
	.filtros button[aria-selected='true'] {
		color: var(--text);
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.faltam {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}
	.revisao {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 16px;
		align-items: start;
	}
	.revisao > .card + .card {
		margin-top: 0;
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

	.resumo {
		margin: 0;
		padding-left: 18px;
		font-size: 14px;
		line-height: 1.8;
	}
	h3 {
		margin: 4px 0 -8px;
		font-size: 14px;
		font-weight: 700;
	}
	.ajuda {
		margin: 0;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.procurar {
		margin: 0 0 10px;
	}
	.procurar .ir {
		padding: 0;
		font: inherit;
		font-size: 13px;
		color: var(--accent);
		background: none;
		border: 0;
		cursor: pointer;
		text-decoration: underline;
	}
	.referencia {
		display: flex;
		flex-wrap: wrap;
		gap: 6px 12px;
		align-items: baseline;
		margin: 0 0 10px;
	}
	.referencia .ir {
		padding: 0;
		font: inherit;
		font-size: 12.5px;
		color: var(--accent);
		background: none;
		border: 0;
		cursor: pointer;
		text-decoration: underline;
	}
	.pendencias .ir {
		margin-left: 6px;
		padding: 0;
		font: inherit;
		font-size: 12.5px;
		color: var(--accent);
		background: none;
		border: 0;
		cursor: pointer;
		text-decoration: underline;
	}
	.pendencias {
		margin: 0;
		padding-left: 18px;
		font-size: 13px;
		line-height: 1.6;
	}
	.pronta {
		margin: 0;
		color: var(--good);
	}
	.dim {
		color: var(--text-faint);
		font-size: 12.5px;
	}
	.detalhe {
		display: block;
		margin-top: 4px;
		font-size: 12px;
		opacity: 0.8;
	}
	.callout {
		margin: 0;
	}
	.aviso-figura {
		margin: 0 0 12px;
	}
	.reaproveitada {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.tem-figura {
		color: var(--warn);
		background: var(--warn-soft);
	}
	.falta-figura b {
		color: var(--warn);
	}
	.cargo-gabarito {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 12px;
		align-items: center;
		justify-content: space-between;
		margin: 0 0 12px;
	}
	section.card {
		margin-bottom: 16px;
	}
	@media (max-width: 1000px) {
		.revisao {
			grid-template-columns: minmax(0, 1fr);
		}
		.lateral {
			position: static;
		}
	}
</style>
