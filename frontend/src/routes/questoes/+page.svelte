<script lang="ts">
	import { onMount } from 'svelte';
	import { goto, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { browser } from '$app/environment';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { tagStyle } from '$lib/format';
	import { chave } from '$lib/storageKey';
	import { provasApi } from '$lib/provas/api';
	import { corDaMateria } from '$lib/provas/resolucao';
	import { lerRespostas } from '$lib/provas/respostas';
	import { comTitulo, semAProva, tituloDigitado } from '$lib/provas/curadoria';
	import { provasDoPacote } from '$lib/provas/pacote';
	import {
		ajustarAoCatalogo,
		chaveDoAssunto,
		enderecoDoTreino,
		filtrarTreino,
		lerFiltro,
		nomeDoAssunto,
		opcoesDoTreino,
		type FiltroDoTreino,
		type GrupoDeMaterias,
		type RespostasPorProva,
		type SituacaoDoFiltro
	} from '$lib/provas/treino';
	import type { ImportacaoResumo, ProvaResumo, QuestaoAvulsa } from '$lib/provas/types';

	type Aba = 'provas' | 'materia' | 'curadoria';

	const ABAS: Aba[] = ['provas', 'materia', 'curadoria'];

	const SITUACOES: [SituacaoDoFiltro, string][] = [
		['todas', 'Todas'],
		['abertas', 'Não resolvidas'],
		['erradas', 'Que errei']
	];

	const ESTADO: Record<ImportacaoResumo['estado'], string> = {
		na_fila: 'na fila',
		processando: 'processando',
		em_revisao: 'em revisão',
		falhou: 'falhou',
		publicada: 'publicada',
		cancelada: 'cancelada'
	};

	// O filtro e a aba ficam no navegador: quem treina Português volta amanhã e
	// encontra Português marcado.
	const CHAVE_FILTRO = chave('.questoes.filtro');
	const CHAVE_ABA = chave('.questoes.aba');

	function ler(k: string): string | null {
		if (!browser) return null;
		try {
			return localStorage.getItem(k);
		} catch {
			return null;
		}
	}

	function gravar(k: string, v: string) {
		try {
			localStorage.setItem(k, v);
		} catch {
			// Sem armazenamento: o filtro vale só enquanto a tela está aberta.
		}
	}

	function abaValida(a: string | null): Aba | null {
		return ABAS.find((x) => x === a) ?? null;
	}

	// O endereço manda (a curadoria volta para ?aba=curadoria); sem ele, a última aba usada.
	let aba = $state<Aba>(abaValida(page.url.searchParams.get('aba')) ?? abaValida(ler(CHAVE_ABA)) ?? 'provas');
	let filtro = $state<FiltroDoTreino>(lerFiltro(ler(CHAVE_FILTRO)));

	let provas = $state<ProvaResumo[]>([]);
	let avulsas = $state<QuestaoAvulsa[]>([]);
	let respostas = $state<RespostasPorProva>({});
	let curador = $state(false);
	let importacoes = $state<ImportacaoResumo[]>([]);
	let erro = $state('');
	let ocupado = $state(false);
	let carregado = $state(false);
	let arquivoProva = $state<HTMLInputElement | null>(null);
	let arquivoGabarito = $state<HTMLInputElement | null>(null);
	// O seletor do navegador corta o nome; o curador precisa dele inteiro para
	// saber que prova está mandando.
	let nomeProva = $state('');
	let nomeGabarito = $state('');

	$effect(() => {
		// Antes de carregar, o filtro ainda não foi conferido com o catálogo.
		if (carregado) gravar(CHAVE_FILTRO, JSON.stringify(filtro));
	});

	function trocarAba(nova: Aba) {
		aba = nova;
		gravar(CHAVE_ABA, nova);
		try {
			const url = new URL(page.url);
			url.searchParams.set('aba', nova);
			replaceState(url, {});
		} catch {
			// Roteador ainda não pronto: a aba troca, só o endereço não.
		}
	}

	// --- provas -------------------------------------------------------------

	/** As provas em grupos por ano, do mais recente, como uma galeria agrupada. */
	const porAno = $derived.by(() => {
		const grupos = new Map<number, ProvaResumo[]>();
		for (const p of provas) grupos.set(p.ano, [...(grupos.get(p.ano) ?? []), p]);
		for (const lista of grupos.values())
			lista.sort((a, b) => a.orgao.localeCompare(b.orgao, 'pt-BR') || a.cargo.localeCompare(b.cargo, 'pt-BR'));
		return [...grupos].sort(([a], [b]) => b - a);
	});

	function feitas(p: ProvaResumo): number {
		return Object.values(respostas[p.id] ?? {}).filter((r) => r.conferida).length;
	}

	// --- por matéria --------------------------------------------------------

	const opcoes = $derived(opcoesDoTreino(avulsas, filtro, respostas));
	// Qual grupo está aberto na lista de matérias: "todos" mostra a lista inteira.
	let grupoAberto = $state('todos');
	const grupoDaVez = $derived(opcoes.grupos.find((g) => g.grupo === grupoAberto));
	const materiasDaVez = $derived(grupoDaVez ? grupoDaVez.materias : opcoes.materias);
	const escolhidas = $derived(filtrarTreino(avulsas, filtro, respostas));
	const deQuantasProvas = $derived(new Set(escolhidas.map((q) => q.provaId)).size);

	function alternarMateria(m: string) {
		if (filtro.materias.includes(m)) {
			filtro.materias = filtro.materias.filter((x) => x !== m);
			// Sem a matéria, os assuntos dela ficariam escolhidos sem chip para desmarcar.
			filtro.assuntos = filtro.assuntos.filter((k) => !k.startsWith(chaveDoAssunto(m, '')));
		} else {
			filtro.materias = [...filtro.materias, m];
		}
	}

	/** O grupo inteiro de uma vez: marca as que faltam, ou desmarca todas se já estavam. */
	function alternarGrupo(g: GrupoDeMaterias) {
		const doGrupo = g.materias.map(([m]) => m);
		if (doGrupo.every((m) => filtro.materias.includes(m))) {
			filtro.materias = filtro.materias.filter((m) => !doGrupo.includes(m));
			filtro.assuntos = filtro.assuntos.filter((k) => !doGrupo.some((m) => k.startsWith(chaveDoAssunto(m, ''))));
		} else {
			filtro.materias = [...filtro.materias, ...doGrupo.filter((m) => !filtro.materias.includes(m))];
		}
	}

	function alternarAssunto(k: string) {
		filtro.assuntos = filtro.assuntos.includes(k) ? filtro.assuntos.filter((x) => x !== k) : [...filtro.assuntos, k];
	}

	/** "Língua Portuguesa e Redes" — o que o botão vai resolver, numa frase. */
	const descricao = $derived.by(() => {
		// O grupo marcado inteiro vira o nome dele: "Básicas", não as duas matérias.
		const restantes = new Set(filtro.materias);
		const ms: string[] = [];
		for (const g of opcoes.grupos) {
			const doGrupo = g.materias.map(([m]) => m);
			if (doGrupo.length > 1 && doGrupo.every((m) => restantes.has(m))) {
				ms.push(g.rotulo);
				doGrupo.forEach((m) => restantes.delete(m));
			}
		}
		ms.push(...filtro.materias.filter((m) => restantes.has(m)));
		const materias =
			ms.length === 0
				? 'Todas as matérias'
				: ms.length <= 2
					? ms.join(' e ')
					: `${ms.slice(0, 2).join(', ')} e mais ${ms.length - 2}`;
		const partes = [materias];
		const as = filtro.assuntos.map(nomeDoAssunto);
		if (as.length) partes.push(as.length <= 2 ? as.join(' e ') : `${as.slice(0, 2).join(', ')} e mais ${as.length - 2}`);
		if (filtro.ano) partes.push(`provas de ${filtro.ano}`);
		if (filtro.situacao === 'abertas') partes.push('só as não resolvidas');
		if (filtro.situacao === 'erradas') partes.push('só as que você errou');
		if (escolhidas.length) partes.push(deQuantasProvas === 1 ? 'de 1 prova' : `de ${deQuantasProvas} provas`);
		return partes.join(' · ');
	});

	// --- curadoria ----------------------------------------------------------

	async function executar(fn: () => Promise<void>) {
		ocupado = true;
		erro = '';
		try {
			await fn();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'erro inesperado';
		} finally {
			ocupado = false;
		}
	}

	// Ações de uma importação na lista. O erro fica na linha: o do alto da
	// página, no celular, some quando a lista é comprida.
	let erroDaLinha = $state<{ id: string; msg: string } | null>(null);
	let editando = $state<{ id: string; provaId: string; titulo: string } | null>(null);

	async function naLinha(id: string, fn: () => Promise<void>) {
		fecharMenus();
		ocupado = true;
		erroDaLinha = null;
		try {
			await fn();
		} catch (e) {
			erroDaLinha = { id, msg: e instanceof Error ? e.message : 'erro inesperado' };
		} finally {
			ocupado = false;
		}
	}

	function tituloDaImportacao(i: ImportacaoResumo): string {
		return [i.orgao, i.ano || '', i.cargoNome || (i.cargo && `cargo ${i.cargo}`)].filter(Boolean).join(' ');
	}

	function excluirImportacao(i: ImportacaoResumo) {
		if (!confirm(`Excluir a importação ${tituloDaImportacao(i) || i.nomeDocumento}? O rascunho e o que a extração leu somem de vez.`))
			return;
		void naLinha(i.id, async () => {
			const atual = await provasApi.importacao(i.id);
			await provasApi.excluir(i.id, atual.versao);
			importacoes = importacoes.filter((x) => x.id !== i.id);
		});
	}

	function excluirProva(i: ImportacaoResumo) {
		if (
			!confirm(
				`Excluir de vez a prova ${tituloDaImportacao(i)}? Ela sai do catálogo com as questões, o gabarito, as importações dela e as anotações que os alunos fizeram. Não tem volta — para só escondê-la, use "Tirar do catálogo" na prova.`
			)
		)
			return;
		void naLinha(i.id, async () => {
			await provasApi.excluirProva(i.provaId);
			importacoes = semAProva(importacoes, i.provaId);
			provas = provas.filter((p) => p.id !== i.provaId);
			avulsas = avulsas.filter((q) => q.provaId !== i.provaId);
		});
	}

	function salvarTitulo() {
		if (!editando) return;
		const { id, provaId } = editando;
		const titulo = tituloDigitado(editando.titulo);
		void naLinha(id, async () => {
			await provasApi.renomearProva(provaId, titulo);
			importacoes = comTitulo(importacoes, provaId, titulo);
			provas = provas.map((p) => (p.id === provaId ? { ...p, cargoNome: titulo } : p));
			editando = null;
		});
	}

	/** O campo aberto já com o teclado: `autofocus` não vale para o que entra depois da carga. */
	function focar(el: HTMLInputElement) {
		el.focus();
		el.select();
	}

	function fecharMenus(e?: MouseEvent) {
		const alvo = e?.target as HTMLElement | undefined;
		for (const m of document.querySelectorAll<HTMLDetailsElement>('details.acoes[open]')) {
			if (!alvo || !m.contains(alvo)) m.open = false;
		}
	}

	// Levar provas de um ambiente a outro: um .zip só com todas. Ao importar, o
	// navegador o abre e envia uma prova por vez — o arquivo inteiro passaria do
	// limite de envio do servidor.
	let pacote = $state<HTMLInputElement | null>(null);
	let levando = $state('');
	let resultadosDosPacotes = $state<{ arquivo: string; ok: boolean; msg: string; provaId?: string }[]>([]);

	function salvarNoAparelho(blob: Blob, nome: string) {
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = nome;
		a.click();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	}

	function exportarProva(i: ImportacaoResumo) {
		void naLinha(i.id, async () => {
			const { blob, nome } = await provasApi.exportarProvas(i.provaId);
			salvarNoAparelho(blob, nome);
		});
	}

	async function exportarTodas() {
		levando = `Preparando o .zip com ${provas.length} provas…`;
		erroDosPacotes = '';
		try {
			const { blob, nome } = await provasApi.exportarProvas();
			salvarNoAparelho(blob, nome);
		} catch (e) {
			erroDosPacotes = e instanceof Error ? e.message : 'não consegui exportar';
		} finally {
			levando = '';
		}
	}

	let erroDosPacotes = $state('');

	async function importarPacote() {
		const arquivo = pacote?.files?.[0];
		if (!arquivo) return;
		erroDosPacotes = '';
		resultadosDosPacotes = [];
		try {
			const lidas = await provasDoPacote(arquivo);
			for (const [k, p] of lidas.entries()) {
				levando = `Importando ${k + 1} de ${lidas.length}: ${p.pasta}…`;
				try {
					const r = await provasApi.importarProvaDoPacote(p);
					resultadosDosPacotes = [
						...resultadosDosPacotes,
						{ arquivo: p.pasta, ok: true, msg: `${r.orgao} ${r.ano} · ${r.cargoNome || r.cargo} publicada`, provaId: r.id }
					];
				} catch (e) {
					const msg = e instanceof Error ? e.message : 'erro inesperado';
					resultadosDosPacotes = [...resultadosDosPacotes, { arquivo: p.pasta, ok: false, msg }];
				}
			}
		} catch (e) {
			erroDosPacotes = e instanceof Error ? e.message : 'não consegui ler o pacote';
		}
		if (pacote) pacote.value = '';
		levando = '';
		// O catálogo, as questões por matéria e a curadoria ganharam as publicadas.
		[provas, avulsas, importacoes] = await Promise.all([
			provasApi.todasAsProvas(),
			provasApi.questoes(),
			provasApi.importacoes()
		]);
		respostas = Object.fromEntries(provas.map((p) => [p.id, lerRespostas(p.id)]));
	}

	async function importar() {
		const prova = arquivoProva?.files?.[0];
		if (!prova) throw new Error('escolha o PDF da prova');
		const nova = await provasApi.importar(prova, arquivoGabarito?.files?.[0] ?? null);
		// O mesmo caderno devolve a importação que já existe; a nova nasce na
		// fila, na primeira versão.
		const existente = nova.versao > 1 || nova.estado !== 'na_fila';
		await goto(`/provas/importacoes/${nova.id}${existente ? '?existente=1' : ''}`);
	}

	onMount(() => {
		void executar(async () => {
			const [ps, qs, souCurador] = await Promise.all([
				provasApi.todasAsProvas(),
				provasApi.questoes(),
				provasApi.souCurador()
			]);
			provas = ps;
			avulsas = qs;
			curador = souCurador;
			respostas = Object.fromEntries(ps.map((p) => [p.id, lerRespostas(p.id)]));
			filtro = ajustarAoCatalogo(filtro, qs);
			if (aba === 'curadoria' && !curador) aba = 'provas';
			carregado = true;
			if (curador) importacoes = await provasApi.importacoes();
		});
	});
</script>

<svelte:head><title>Questões</title></svelte:head>
<svelte:window onclick={fecharMenus} />

<PageHead
	icone="questoes"
	titulo="Questões"
	sub="Resolva uma prova inteira, como no dia da prova, ou treine uma matéria com as questões de todas as provas."
	mostrarProps={false}
/>

<div class="page">
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	<div class="abas" role="tablist" aria-label="Como resolver">
		<button type="button" role="tab" aria-selected={aba === 'provas'} onclick={() => trocarAba('provas')}>
			<NavIcon name="prova" size="sm" /> Provas {#if carregado}<span class="n">{provas.length}</span>{/if}
		</button>
		<button type="button" role="tab" aria-selected={aba === 'materia'} onclick={() => trocarAba('materia')}>
			<NavIcon name="conteudo" size="sm" /> Por matéria {#if carregado}<span class="n">{avulsas.length}</span>{/if}
		</button>
		{#if curador}
			<button type="button" role="tab" aria-selected={aba === 'curadoria'} onclick={() => trocarAba('curadoria')}>
				<NavIcon name="config" size="sm" /> Curadoria
			</button>
		{/if}
	</div>

	{#if !carregado}
		{#if !erro}<p class="vazio">Carregando as questões…</p>{/if}
	{:else if aba === 'provas'}
		<p class="dica">A prova como caiu: as questões na ordem do caderno, com o texto de apoio e o gabarito oficial.</p>
		{#each porAno as [ano, lista] (ano)}
			<section>
				<h2 class="grupo">{ano} <span class="n">{lista.length}</span></h2>
				<div class="galeria">
					{#each lista as p (p.id)}
						{@const feito = feitas(p)}
						<a class="prova" href="/provas/{p.id}">
							<span class="orgao">{p.orgao}</span>
							<span class="cargo" title={p.cargoNome}>{p.cargoNome || `Cargo ${p.cargo}`}</span>
							<span class="meta">
								{p.banca} · {p.cargo} ·
								{#if p.total === 0}
									<span class="preliminar">sem gabarito</span>
								{:else}
									{p.total} questões
									{#if p.gabaritoTipo === 'preliminar'}· <span class="preliminar">gabarito preliminar</span>{/if}
								{/if}
							</span>
							<span class="andamento">
								<span class="trilho" aria-hidden="true"><i style="width:{(100 * feito) / Math.max(1, p.total)}%"></i></span>
								{#if feito === 0}
									Não começada
								{:else if feito >= p.total}
									Resolvida inteira
								{:else}
									{feito} de {p.total} resolvidas
								{/if}
							</span>
						</a>
					{/each}
				</div>
			</section>
		{:else}
			<p class="vazio">Nenhuma prova publicada ainda.</p>
		{/each}
	{:else if aba === 'materia'}
		<p class="dica">
			As questões de todas as provas, juntas por matéria. A questão que caiu igual em dois cargos aparece uma vez só.
		</p>

		<div class="propriedades">
			<div class="propriedade">
				<span class="rotulo"><NavIcon name="conteudo" size="sm" /> Matérias</span>
				<div class="valor materias">
					<!-- Primeiro o grupo, depois as matérias dele: uma decisão por vez. -->
					<div class="abas-grupo" role="tablist" aria-label="Grupos de matéria">
						<button
							type="button"
							role="tab"
							aria-selected={grupoAberto === 'todos'}
							onclick={() => (grupoAberto = 'todos')}
						>
							Todas <span class="n">{opcoes.materias.reduce((t, [, n]) => t + n, 0)}</span>
						</button>
						{#each opcoes.grupos as g (g.grupo)}
							{@const soma = g.materias.reduce((t, [, n]) => t + n, 0)}
							{@const escolhidas = g.materias.filter(([m]) => filtro.materias.includes(m)).length}
							<button
								type="button"
								role="tab"
								aria-selected={grupoAberto === g.grupo}
								onclick={() => (grupoAberto = g.grupo)}
							>
								{g.rotulo}
								<span class="n">{soma}</span>
								{#if escolhidas > 0}<span class="marcadas">{escolhidas} ✓</span>{/if}
							</button>
						{/each}
					</div>
					{#if grupoDaVez}
						<p class="explicacao">
							{grupoDaVez.explicacao}
							<button type="button" class="ir" onclick={() => alternarGrupo(grupoDaVez!)}>
								{grupoDaVez.materias.every(([m]) => filtro.materias.includes(m))
									? 'Desmarcar todas'
									: 'Marcar todas'}
							</button>
						</p>
					{/if}
					<ul class="lista-materias">
						{#each materiasDaVez as [m, n] (m)}
							{@const marcada = filtro.materias.includes(m)}
							<li>
								<button type="button" aria-pressed={marcada} onclick={() => alternarMateria(m)}>
									<span class="caixa" aria-hidden="true">{marcada ? '✓' : ''}</span>
									<span class="cor" aria-hidden="true" style={tagStyle(corDaMateria(m))}></span>
									<span class="nome">{m}</span>
									<span class="n">{n}</span>
								</button>
							</li>
						{:else}
							<li class="dim">Nenhuma matéria neste grupo.</li>
						{/each}
					</ul>
					{#if filtro.materias.length}
						<button
							type="button"
							class="limpar"
							onclick={() => {
								filtro.materias = [];
								filtro.assuntos = [];
							}}>Limpar as {filtro.materias.length} escolhidas</button
						>
					{/if}
				</div>
			</div>

			<!-- Os assuntos aparecem com a matéria escolhida: sem ela, seriam centenas.
			     Sem assunto classificado, a linha diz isso em vez de sumir. -->
			{#if filtro.materias.length > 0 && opcoes.assuntos.length === 0}
				<div class="propriedade">
					<span class="rotulo"><NavIcon name="questoes" size="sm" /> Assuntos</span>
					<p class="explicacao sem-assunto">
						{filtro.materias.length === 1 ? 'Esta matéria ainda não tem' : 'Estas matérias ainda não têm'} assuntos
						classificados; o treino leva a matéria inteira.
					</p>
				</div>
			{/if}
			{#if opcoes.assuntos.length}
				<div class="propriedade">
					<span class="rotulo"><NavIcon name="questoes" size="sm" /> Assuntos</span>
					<div class="valor assuntos">
						{#each opcoes.assuntos as g (g.materia)}
							<div class="grupo-assuntos">
								{#if opcoes.assuntos.length > 1}<span class="de-materia">{g.materia}</span>{/if}
								{#each g.assuntos as [a, n] (a)}
									{@const k = chaveDoAssunto(g.materia, a)}
									<button
										type="button"
										class="opcao"
										aria-pressed={filtro.assuntos.includes(k)}
										onclick={() => alternarAssunto(k)}
									>
										{a || 'Sem assunto classificado'} <span class="n">{n}</span>
									</button>
								{/each}
							</div>
						{/each}
						{#if filtro.assuntos.length}
							<button type="button" class="limpar" onclick={() => (filtro.assuntos = [])}>Todos os assuntos</button>
						{/if}
					</div>
				</div>
			{/if}

			{#if opcoes.anos.length > 1}
				<div class="propriedade">
					<span class="rotulo"><NavIcon name="datas" size="sm" /> Ano</span>
					<div class="valor">
						<button type="button" class="opcao" aria-pressed={!filtro.ano} onclick={() => (filtro.ano = 0)}>
							Todos
						</button>
						{#each opcoes.anos as [a, n] (a)}
							<button type="button" class="opcao" aria-pressed={filtro.ano === a} onclick={() => (filtro.ano = a)}>
								{a} <span class="n">{n}</span>
							</button>
						{/each}
					</div>
				</div>
			{/if}

			<div class="propriedade">
				<span class="rotulo"><NavIcon name="acerto" size="sm" /> Situação</span>
				<div class="valor">
					{#each SITUACOES as [s, rotulo] (s)}
						<button
							type="button"
							class="opcao"
							aria-pressed={filtro.situacao === s}
							onclick={() => (filtro.situacao = s)}
						>
							{rotulo} <span class="n">{opcoes.situacoes[s]}</span>
						</button>
					{/each}
				</div>
			</div>
		</div>

		<div class="resumo">
			<div>
				<strong>{escolhidas.length} {escolhidas.length === 1 ? 'questão' : 'questões'}</strong>
				<span>{descricao}</span>
			</div>
			{#if escolhidas.length}
				<a class="nbtn primario" href={enderecoDoTreino(filtro)}>Resolver →</a>
			{:else}
				<button type="button" class="nbtn primario" disabled>Resolver →</button>
			{/if}
		</div>
		{#if filtro.situacao === 'erradas' && escolhidas.length}
			<p class="dica rodape">As que você errou voltam em branco, para resolver de novo sem ver o gabarito.</p>
		{/if}
	{:else}
		<section>
			<h2 class="grupo">Importar prova da FCC</h2>
			<p class="dica">
				PDFs de até 25 MiB cada. A extração roda em segundo plano; a prova só entra no catálogo depois de revisada.
			</p>
			<div class="importar">
				<label class="arquivo">
					<span>Caderno de prova</span>
					<input
						type="file"
						accept="application/pdf"
						bind:this={arquivoProva}
						onchange={(e) => (nomeProva = e.currentTarget.files?.[0]?.name ?? '')}
					/>
					{#if nomeProva}<span class="nome-arquivo">{nomeProva}</span>{/if}
				</label>
				<label class="arquivo">
					<span>Gabarito oficial <em>opcional</em></span>
					<input
						type="file"
						accept="application/pdf"
						bind:this={arquivoGabarito}
						onchange={(e) => (nomeGabarito = e.currentTarget.files?.[0]?.name ?? '')}
					/>
					{#if nomeGabarito}<span class="nome-arquivo">{nomeGabarito}</span>{/if}
				</label>
				<button class="nbtn primario" type="button" disabled={ocupado} onclick={() => executar(importar)}>
					Importar
				</button>
			</div>
		</section>

		<section>
			<h2 class="grupo">Levar provas para outro ambiente</h2>
			<p class="ajuda-curadoria">
				Para passar o catálogo de staging para produção sem importar e revisar de novo: exporte aqui um .zip só,
				com uma pasta por prova — as questões, as figuras de cada questão e o PDF —, e importe esse mesmo arquivo
				na curadoria do outro ambiente, sem descompactar. Lá, cada prova entra publicada; a que já estiver no
				catálogo é recusada.
			</p>
			<div class="importar">
				<button class="nbtn" type="button" disabled={!!levando || provas.length === 0} onclick={exportarTodas}>
					Exportar as {provas.length} provas publicadas
				</button>
				<label class="arquivo">
					<span>Pacote exportado <em>o .zip, inteiro</em></span>
					<input type="file" accept=".zip,application/zip" bind:this={pacote} />
				</label>
				<button class="nbtn primario" type="button" disabled={!!levando} onclick={importarPacote}>
					Importar provas
				</button>
			</div>
			{#if levando}<p class="sub" role="status">{levando}</p>{/if}
			{#if erroDosPacotes}<p class="erro" role="alert">{erroDosPacotes}</p>{/if}
			{#if resultadosDosPacotes.length > 0}
				<ul class="resultados" aria-live="polite">
					{#each resultadosDosPacotes as r, k (k)}
						<li class:falhou={!r.ok}>
							<b>{r.arquivo}</b>:
							{#if r.ok && r.provaId}<a href="/provas/{r.provaId}">{r.msg}</a>{:else}{r.msg}{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section>
			<h2 class="grupo">Importações recentes</h2>
			<div class="linhas">
				{#each importacoes as i (i.id)}
					<div class="item">
						{#if editando?.id === i.id}
							<form
								class="editar-titulo"
								onsubmit={(e) => {
									e.preventDefault();
									salvarTitulo();
								}}
							>
								<label>
									<span class="sub">Título da prova ({i.orgao} {i.ano} · código {i.cargo || '—'})</span>
									<input type="text" maxlength="200" bind:value={editando.titulo} use:focar />
								</label>
								<button class="nbtn primario" type="submit" disabled={ocupado || !tituloDigitado(editando.titulo)}>
									Salvar
								</button>
								<button class="nbtn" type="button" onclick={() => (editando = null)}>Cancelar</button>
							</form>
						{:else}
							<a class="linha" href="/provas/importacoes/{i.id}">
								<span class="titulo">
									{i.orgao || 'Prova sem identificação'}
									{i.ano || ''}
									<span class="sub">{i.cargoNome || `cargo ${i.cargo || '—'}`}</span>
								</span>
								{#if i.estado === 'na_fila' || i.estado === 'processando'}
									<span class="sub">etapa {i.etapa} de {i.totalEtapas}</span>
								{/if}
								<span class="estado {i.estado}">{ESTADO[i.estado]}</span>
								{#if i.nomeDocumento}
									<span class="nome-arquivo">
										{i.nomeDocumento}{#if i.nomeGabarito}<br />gabarito: {i.nomeGabarito}{/if}
									</span>
								{/if}
								{#if i.erro}<span class="erro">{i.erro}</span>{/if}
							</a>
						{/if}
						{#if i.estado !== 'processando'}
							<details class="acoes">
								<summary aria-label="Ações desta importação" title="Ações desta importação">•••</summary>
								<div class="acoes-corpo">
									{#if i.estado === 'publicada' && i.provaId}
										<button
											type="button"
											disabled={ocupado}
											onclick={() => {
												fecharMenus();
												editando = { id: i.id, provaId: i.provaId, titulo: i.cargoNome };
											}}
										>
											Editar título
											<small>O nome do cargo no catálogo e aqui</small>
										</button>
										<button type="button" disabled={ocupado} onclick={() => exportarProva(i)}>
											Exportar só esta
											<small>O .zip dela para importar em outro ambiente</small>
										</button>
										<button type="button" class="perigo" disabled={ocupado} onclick={() => excluirProva(i)}>
											Excluir prova
											<small>Some do catálogo, com tudo o que veio dela</small>
										</button>
									{:else}
										<button type="button" class="perigo" disabled={ocupado} onclick={() => excluirImportacao(i)}>
											Excluir importação
											<small>O rascunho some; nada do catálogo muda</small>
										</button>
									{/if}
								</div>
							</details>
						{/if}
						{#if erroDaLinha?.id === i.id}<p class="erro erro-da-linha" role="alert">{erroDaLinha.msg}</p>{/if}
					</div>
				{:else}
					<p class="vazio">Nenhuma importação ainda.</p>
				{/each}
			</div>
		</section>
	{/if}
</div>

<style>
	/* As abas de visão do Notion: texto, um traço embaixo da que está aberta. */
	.abas {
		display: flex;
		gap: 2px;
		margin-bottom: 14px;
		overflow-x: auto;
		border-bottom: 1px solid var(--border);
	}
	.abas button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 10px 9px;
		margin-bottom: -1px;
		font: inherit;
		font-size: 14px;
		white-space: nowrap;
		color: var(--text-muted);
		background: none;
		border: 0;
		border-bottom: 2px solid transparent;
		cursor: pointer;
	}
	.abas button:hover {
		color: var(--text);
	}
	.abas button[aria-selected='true'] {
		color: var(--text);
		font-weight: 600;
		border-bottom-color: var(--text);
	}
	.n {
		font-size: 12px;
		font-weight: 400;
		color: var(--text-faint);
		font-variant-numeric: tabular-nums;
	}
	.dica {
		margin: 0 0 18px;
		font-size: 13.5px;
		color: var(--text-muted);
	}
	.dica.rodape {
		margin: 10px 2px 0;
		font-size: 13px;
		color: var(--text-faint);
	}
	.vazio {
		color: var(--text-faint);
		font-size: 14px;
	}

	/* --- provas: galeria agrupada por ano --- */
	.grupo {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 26px 0 10px;
		font-size: 14px;
		font-weight: 600;
		color: var(--text-muted);
	}
	section:first-of-type .grupo {
		margin-top: 0;
	}
	.galeria {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
		gap: 12px;
	}
	.prova {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 14px 16px 12px;
		color: inherit;
		text-decoration: none;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		transition:
			background 0.12s,
			border-color 0.12s;
	}
	.prova:hover {
		background: var(--bg-soft);
		border-color: var(--border-strong);
	}
	.orgao {
		font-size: 17px;
		font-weight: 700;
	}
	/* Nome de cargo da FCC é comprido: três linhas bastam para distinguir. */
	.cargo {
		display: -webkit-box;
		overflow: hidden;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		font-size: 14px;
		line-height: 1.4;
	}
	.meta {
		font-size: 12.5px;
		color: var(--text-faint);
	}
	.preliminar {
		color: var(--warn);
	}
	.andamento {
		display: grid;
		gap: 6px;
		margin-top: auto;
		padding-top: 10px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.trilho {
		height: 4px;
		overflow: hidden;
		border-radius: 2px;
		background: var(--bg-hover);
	}
	.trilho i {
		display: block;
		height: 100%;
		background: var(--good);
	}

	/* --- por matéria: as propriedades de uma página do Notion --- */
	.propriedades {
		display: grid;
		gap: 4px;
		margin-bottom: 20px;
	}
	.propriedade {
		display: grid;
		grid-template-columns: 130px minmax(0, 1fr);
		gap: 10px;
		align-items: start;
	}
	.rotulo {
		display: flex;
		align-items: center;
		gap: 7px;
		height: 34px;
		font-size: 14px;
		color: var(--text-muted);
	}
	.valor {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
		min-height: 34px;
		padding: 4px 0;
	}
	/* Escolher matéria é uma lista: caixa, cor, nome e quantas questões. */
	.valor.materias {
		display: grid;
		gap: 8px;
		width: 100%;
	}
	.abas-grupo {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
	.abas-grupo button {
		display: inline-flex;
		align-items: baseline;
		gap: 5px;
		padding: 5px 10px;
		font: inherit;
		font-size: 13.5px;
		color: var(--text-muted);
		background: transparent;
		border: 1px solid transparent;
		border-radius: 999px;
		cursor: pointer;
	}
	.abas-grupo button:hover {
		background: var(--bg-hover);
	}
	.abas-grupo button[aria-selected='true'] {
		color: var(--text);
		font-weight: 600;
		background: var(--bg-soft);
		border-color: var(--border);
	}
	.marcadas {
		font-size: 11.5px;
		color: var(--accent);
	}
	.explicacao {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: baseline;
		margin: 0;
		font-size: 12.5px;
		color: var(--text-faint);
	}
	.lista-materias {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 2px 12px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.lista-materias button {
		display: flex;
		gap: 8px;
		align-items: center;
		width: 100%;
		padding: 7px 8px;
		font: inherit;
		font-size: 14px;
		text-align: left;
		color: var(--text);
		background: none;
		border: 0;
		border-radius: 7px;
		cursor: pointer;
	}
	.lista-materias button:hover {
		background: var(--bg-hover);
	}
	.lista-materias button[aria-pressed='true'] {
		background: var(--accent-soft);
	}
	.caixa {
		flex: none;
		display: grid;
		place-items: center;
		width: 17px;
		height: 17px;
		font-size: 11px;
		color: var(--accent);
		border: 1.5px solid var(--border-strong);
		border-radius: 4px;
	}
	.lista-materias button[aria-pressed='true'] .caixa {
		border-color: var(--accent);
	}
	.cor {
		flex: none;
		width: 9px;
		height: 9px;
		border-radius: 3px;
	}
	.nome {
		flex: 1;
		min-width: 0;
	}
	.lista-materias .n {
		flex: none;
		font-variant-numeric: tabular-nums;
	}
	.sem-assunto {
		align-self: center;
	}
	/* "Marcar todas" é um link ao lado da frase do grupo, não um botão. */
	.explicacao button {
		padding: 0;
		font: inherit;
		font-size: 12.5px;
		color: var(--accent);
		background: none;
		border: 0;
		cursor: pointer;
	}
	.explicacao button:hover {
		text-decoration: underline;
	}
	.limpar {
		padding: 2px 6px;
		font: inherit;
		font-size: 13px;
		color: var(--text-faint);
		background: none;
		border: 0;
		border-radius: 4px;
		cursor: pointer;
	}
	.limpar:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	/* Escolha de uma opção só: texto simples, a escolhida com fundo. */
	.opcao {
		padding: 3px 10px;
		font: inherit;
		font-size: 14px;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.opcao:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	.opcao[aria-pressed='true'] {
		color: var(--text);
		font-weight: 600;
		background: var(--bg-hover);
	}
	/* Um cartão por grupo: o nome do grupo marca ou desmarca o grupo inteiro. */
	.valor.grupos {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
		align-items: start;
		gap: 10px;
		width: 100%;
	}
	.grupo-materias {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-content: start;
		padding: 10px 12px 12px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.cabeca-grupo {
		display: grid;
		gap: 2px;
		width: 100%;
		margin-bottom: 2px;
	}
	.valor.assuntos {
		display: grid;
		justify-items: start;
		gap: 8px;
	}
	.grupo-assuntos {
		display: flex;
		flex-wrap: wrap;
		gap: 2px 4px;
		align-items: center;
	}
	.de-materia {
		width: 100%;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-faint);
	}
	.resumo {
		display: flex;
		flex-wrap: wrap;
		gap: 12px 20px;
		align-items: center;
		justify-content: space-between;
		padding: 14px 16px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.resumo div {
		display: grid;
		gap: 2px;
		min-width: 0;
	}
	.resumo strong {
		font-size: 18px;
		font-weight: 700;
	}
	.resumo span {
		font-size: 13.5px;
		color: var(--text-muted);
	}

	/* --- curadoria --- */
	.importar {
		display: flex;
		flex-wrap: wrap;
		gap: 12px 16px;
		align-items: flex-end;
	}
	.arquivo {
		display: grid;
		gap: 5px;
		font-size: 13.5px;
		color: var(--text-muted);
	}
	.arquivo em {
		font-style: normal;
		color: var(--text-faint);
	}
	.arquivo input {
		max-width: 100%;
		font: inherit;
		font-size: 13px;
		color: var(--text-muted);
	}
	.arquivo input::file-selector-button {
		height: 30px;
		padding: 0 12px;
		margin-right: 10px;
		font: inherit;
		font-size: 13.5px;
		color: var(--text);
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
	}
	.linhas {
		border-top: 1px solid var(--border);
	}
	.ajuda-curadoria {
		margin: 0 0 12px;
		max-width: 72ch;
		font-size: 13.5px;
		color: var(--text-muted);
	}
	.resultados {
		display: grid;
		gap: 4px;
		margin: 12px 0 0;
		padding: 0;
		list-style: none;
		font-size: 13.5px;
		overflow-wrap: anywhere;
	}
	.resultados li {
		color: var(--good);
	}
	.resultados li.falhou {
		color: var(--danger);
	}
	/* A linha e o menu dela lado a lado: botão dentro de link não é HTML válido. */
	.item {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		border-bottom: 1px solid var(--border);
	}
	.item .linha {
		flex: 1;
		min-width: 0;
		border-bottom: 0;
	}
	.acoes {
		position: relative;
		padding: 8px 4px;
	}
	.acoes summary {
		list-style: none;
		padding: 2px 10px;
		border-radius: 5px;
		color: var(--text-muted);
		cursor: pointer;
		letter-spacing: 1px;
	}
	.acoes summary::-webkit-details-marker {
		display: none;
	}
	.acoes summary:hover,
	.acoes[open] summary {
		background: var(--bg-hover);
	}
	.acoes-corpo {
		position: absolute;
		right: 0;
		top: calc(100% - 4px);
		z-index: 10;
		display: grid;
		width: min(280px, calc(100vw - 32px));
		padding: 6px;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow-pop);
	}
	.acoes-corpo button {
		display: grid;
		gap: 1px;
		padding: 7px 10px;
		text-align: left;
		font: inherit;
		font-size: 14px;
		color: var(--text);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.acoes-corpo button:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	.acoes-corpo button.perigo {
		color: var(--danger);
	}
	.acoes-corpo small {
		font-size: 12px;
		color: var(--text-faint);
	}
	.editar-titulo {
		flex: 1;
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: flex-end;
		padding: 9px 6px;
	}
	.editar-titulo label {
		flex: 1 1 280px;
		display: grid;
		gap: 4px;
	}
	.editar-titulo input {
		font: inherit;
		font-size: 14px;
		padding: 6px 8px;
		color: var(--text);
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.erro-da-linha {
		margin: 0 6px 8px;
	}
	.linha {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 12px;
		align-items: center;
		padding: 9px 6px;
		font-size: 14px;
		color: inherit;
		text-decoration: none;
		border-bottom: 1px solid var(--border);
	}
	.linha:hover {
		background: var(--bg-hover);
	}
	.titulo {
		flex: 1 1 280px;
		min-width: 0;
		font-weight: 600;
	}
	.sub {
		font-weight: 400;
		font-size: 13px;
		color: var(--text-muted);
	}
	.titulo .sub {
		margin-left: 6px;
	}
	.estado {
		padding: 1px 8px;
		font-size: 12.5px;
		border-radius: 4px;
		color: var(--text-muted);
		background: var(--bg-hover);
	}
	.estado.em_revisao {
		color: var(--c0-tx);
		background: var(--c0-bg);
	}
	.estado.publicada {
		color: var(--good);
		background: var(--good-soft);
	}
	.estado.falhou {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.estado.na_fila,
	.estado.processando {
		color: var(--warn);
		background: var(--warn-soft);
	}
	.erro {
		flex-basis: 100%;
		font-size: 12.5px;
		color: var(--danger);
	}
	/* O nome inteiro, quebrando onde precisar: é por ele que o curador sabe
	   qual prova mandou. */
	.nome-arquivo {
		flex-basis: 100%;
		max-width: 44ch;
		font-size: 12.5px;
		color: var(--text-muted);
		overflow-wrap: anywhere;
	}
	.linha .nome-arquivo {
		max-width: none;
	}

	@media (max-width: 560px) {
		.propriedade {
			grid-template-columns: 1fr;
			gap: 0;
		}
		.rotulo {
			height: auto;
			padding-top: 6px;
		}
	}
</style>
