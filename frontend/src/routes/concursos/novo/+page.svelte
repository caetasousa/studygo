<script lang="ts">
	import IconButton from '$lib/components/IconButton.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { goto } from '$app/navigation';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { api, ApiError, type FonteEdital } from '$lib/api';
	import ConcursoForm from '$lib/components/ConcursoForm.svelte';
	import { sintetizarConteudo } from '$lib/conteudo';
	import type {
		AlertaResposta,
		CargoOpcao,
		ConcursoInput,
		DisciplinaInput,
		GrupoResposta
	} from '$lib/types';

	type Etapa = 'edital' | 'cargo' | 'gerais' | 'especificas' | 'conteudo' | 'revisao' | 'manual';

	let etapa = $state<Etapa>('edital');
	let processando = $state(false);
	let progresso = $state('');
	let erro = $state<string | null>(null);
	let enviando = $state(false);

	// edital step
	let arquivo = $state<File | null>(null);
	let textoEdital = $state('');

	// after "analisar" — the opaque handle the processor bound to this user's
	// document; the later steps carry only this.
	let documentoId = $state('');
	let banca = $state('');
	let cargos = $state<CargoOpcao[]>([]);
	let cargoSel = $state<string | null>(null);

	// after "estrutura". `questoes` is null when the edital gave only the group
	// total — the user must fill an estimate or ratear before saving.
	interface DiscRow {
		nome: string;
		questoes: number | null;
		peso: number | null;
		temasTexto: string;
	}
	let gerais = $state<DiscRow[]>([]);
	let especificas = $state<DiscRow[]>([]);
	let grupoGerais = $state<GrupoResposta | null>(null);
	let grupoEspecificos = $state<GrupoResposta | null>(null);
	let prova = $state('');
	let marcos = $state<ConcursoInput['marcos']>([]);
	let nomeSugerido = $state('');
	let alertas = $state<AlertaResposta[]>([]);

	// final
	let inicialRevisao = $state<ConcursoInput | undefined>(undefined);

	const primeiroConcurso = $derived(
		concursoStore.carregado && concursoStore.lista.length === 0
	);
	const semIA = $derived(concursoStore.carregado && !concursoStore.importacaoEdital);

	const passos = ['Edital', 'Cargo', 'Gerais', 'Específicas', 'Conteúdo', 'Revisão'] as const;
	const passoAtual = $derived(
		(
			{ edital: 0, cargo: 1, gerais: 2, especificas: 3, conteudo: 4, revisao: 5, manual: 5 } as Record<
				Etapa,
				number
			>
		)[etapa]
	);

	function trata(e: unknown): string {
		if (e instanceof ApiError && e.status === 503) {
			return 'A importação por IA não está configurada neste servidor. Use o cadastro manual.';
		}
		if (e instanceof ApiError && e.status === 413) {
			return 'o arquivo é grande demais. Tente um PDF menor ou cole só as seções de disciplinas e cronograma.';
		}
		return e instanceof Error ? e.message : 'algo deu errado';
	}

	function escolherArquivo(e: Event) {
		const f = (e.target as HTMLInputElement).files?.[0] ?? null;
		erro = null;
		if (f && f.size > 20 * 1024 * 1024) {
			erro = 'esse PDF passa de 20 MB.';
			arquivo = null;
			return;
		}
		arquivo = f;
	}

	function fmtTam(b: number) {
		return b < 1_048_576 ? `${Math.round(b / 1024)} KB` : `${(b / 1_048_576).toFixed(1)} MB`;
	}

	async function analisar() {
		processando = true;
		erro = null;
		progresso = arquivo
			? 'Enviando e lendo o edital para identificar os cargos…'
			: 'Lendo o edital e identificando os cargos…';
		const entrada: FonteEdital = arquivo ? { pdf: arquivo } : { texto: textoEdital };
		try {
			const res = await api.analisarEdital(entrada);
			documentoId = res.documentoId;
			banca = res.banca;
			cargos = res.cargos;
			// Prefer selecting by the stable cargo code.
			cargoSel = cargos.length === 1 ? cargos[0].codigo : null;
			etapa = 'cargo';
		} catch (e) {
			erro = trata(e);
		} finally {
			processando = false;
		}
	}

	async function buscarEstrutura() {
		if (!cargoSel || !documentoId) return;
		processando = true;
		erro = null;
		const rotulo = cargos.find((c) => c.codigo === cargoSel)?.nome ?? cargoSel;
		progresso = `Buscando a estrutura de "${rotulo}"…`;
		try {
			const est = await api.estruturaEdital(documentoId, cargoSel);
			nomeSugerido = est.nome;
			prova = est.prova;
			marcos = est.marcos;
			alertas = est.alertas;
			grupoGerais = est.gerais[0] ?? null;
			grupoEspecificos = est.especificas[0] ?? null;
			// The edital states the weight per GROUP ("Conhecimentos Gerais ... peso 1",
			// "Específicos ... peso 2"). Carry it down to each discipline so the user
			// does not retype what the edital already said; a discipline that had its
			// own weight keeps it.
			gerais = est.gerais.flatMap((g) => g.disciplinas.map((d) => paraLinha(d, g.peso)));
			especificas = est.especificas.flatMap((g) =>
				g.disciplinas.map((d) => paraLinha(d, g.peso))
			);
			// The edital gives the group total but rarely splits it per discipline.
			// Seed the even split so the step opens usable; it is an estimate and
			// every field stays editable.
			preencherEstimativa(gerais, grupoGerais);
			preencherEstimativa(especificas, grupoEspecificos);
			etapa = 'gerais';
		} catch (e) {
			erro = trata(e);
		} finally {
			processando = false;
		}
	}

	function paraLinha(
		d: { nome: string; questoes: number | null; peso: number | null },
		pesoGrupo: number | null = null
	): DiscRow {
		return {
			nome: d.nome,
			questoes: d.questoes,
			// Per-discipline weight wins; otherwise the group's, which the edital
			// almost always states.
			peso: d.peso ?? pesoGrupo,
			temasTexto: ''
		};
	}

	function addDisc(lista: DiscRow[]) {
		lista.push({ nome: '', questoes: null, peso: null, temasTexto: '' });
	}

	// Split a group's total equally across its disciplines — only on an explicit
	// user action, and the values are an estimate, not extracted data.
	function ratear(lista: DiscRow[], total: number | null) {
		if (!total || lista.length === 0) return;
		const base = Math.floor(total / lista.length);
		const resto = total % lista.length;
		lista.forEach((d, i) => (d.questoes = base + (i < resto ? 1 : 0)));
	}

	// Seed an even split when the edital gave the group total but no per-discipline
	// breakdown. Never overwrites a number the edital did provide.
	function preencherEstimativa(lista: DiscRow[], grupo: GrupoResposta | null) {
		if (!grupo || grupo.total === null) return;
		if (!lista.length || !lista.every((d) => d.questoes === null)) return;
		ratear(lista, grupo.total);
	}

	function somaQuestoes(lista: DiscRow[]): number {
		return lista.reduce((a, d) => a + (d.questoes ?? 0), 0);
	}

	// Per-step: each screen is gated only by the rows it actually shows. Checking
	// both lists at once deadlocks step 3, whose "next" would wait on disciplines
	// that are only filled in on step 4.
	function faltaEstimarEm(lista: DiscRow[]): boolean {
		return lista.some((d) => d.nome.trim() && d.questoes === null);
	}
	function rmDisc(lista: DiscRow[], i: number) {
		lista.splice(i, 1);
	}

	async function buscarConteudo() {
		const todas = [...gerais, ...especificas].map((d) => d.nome.trim()).filter(Boolean);
		if (todas.length === 0 || !documentoId) {
			montarRevisao();
			return;
		}
		etapa = 'conteudo';
		processando = true;
		erro = null;
		progresso = `Extraindo o conteúdo programático de ${todas.length} disciplinas…`;
		try {
			const res = await api.conteudoEdital(
				{ documentoId },
				cargoSel ?? '',
				todas
			);
			const mapa = new Map(res.itens.map((it) => [it.nome.trim().toLowerCase(), it.temas]));
			for (const d of [...gerais, ...especificas]) {
				const t = mapa.get(d.nome.trim().toLowerCase());
				if (t?.length) d.temasTexto = t.join('\n');
			}
			montarRevisao();
		} catch (e) {
			// A IA falhou nesta etapa. Não avança em silêncio: sem temas a página de
			// conteúdo fica vazia e o usuário não percebe que faltou. Ele pode tentar
			// de novo, ou seguir sem temas de propósito (botão na própria etapa).
			erro = trata(e);
			etapa = 'conteudo';
		} finally {
			processando = false;
		}
	}

	function toDisciplinaInput(rows: DiscRow[], bloco: 'ger' | 'esp'): DisciplinaInput[] {
		return rows
			.filter((d) => d.nome.trim())
			.map((d) => ({
				nome: d.nome.trim(),
				bloco,
				questoes: Math.max(0, d.questoes ?? 0),
				peso: Math.max(0, Math.round(d.peso ?? 0)),
				cadernoUrl: '',
				temas: d.temasTexto
					.split('\n')
					.map((t) => t.trim())
					.filter(Boolean),
				fontes: []
			}));
	}

	function montarRevisao() {
		const disciplinas = [
			...toDisciplinaInput(gerais, 'ger'),
			...toDisciplinaInput(especificas, 'esp')
		];

		inicialRevisao = {
			nome: nomeSugerido,
			banca,
			cargo: cargos.find((c) => c.codigo === cargoSel)?.nome ?? cargoSel ?? '',
			emoji: '📚',
			prova,
			retaFinalDias: 28,
			disciplinas,
			marcos,
			// ConcursoForm regenera o conteúdo a partir das disciplinas ao salvar.
			conteudo: sintetizarConteudo(disciplinas)
		};
		etapa = 'revisao';
	}

	async function salvar(input: ConcursoInput) {
		enviando = true;
		erro = null;
		try {
			await concursoStore.criar(input);
			await goto('/');
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Erro ao salvar';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="crumb">Concursos <span class="sep">/</span> Novo</div>
<h1 class="page-title"><span>Cadastrar concurso</span></h1>
<p class="page-sub">
	{primeiroConcurso
		? 'Bem-vindo! Cadastre seu primeiro concurso e o plano de estudos é gerado na hora.'
		: 'Envie o edital e o assistente monta o cadastro por etapas — ou preencha manualmente.'}
</p>

{#if etapa !== 'manual'}
	<div class="stepper">
		{#each passos as p, i (p)}
			<span class="st" class:on={i === passoAtual} class:done={i < passoAtual}>
				<b>{i < passoAtual ? '✓' : i + 1}</b>{p}
			</span>
		{/each}
	</div>
{/if}

<div class="page">
	{#if erro}<div class="form-error" style="margin-bottom:14px">{erro}</div>{/if}

	{#if processando}
		<div class="progress-card">
			<span class="spinner lg"></span>
			<div>
				<b>{progresso}</b>
				<p class="page-sub" style="margin:6px 0 0">
					A IA está lendo o edital. Cada etapa leva alguns segundos — não feche a página.
				</p>
			</div>
		</div>
	{/if}

	<!-- ETAPA 1: EDITAL -->
	{#if etapa === 'edital' && !processando}
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Enviar o edital</h2>
				{#if semIA}
					<div class="callout" style="margin-bottom:12px">
						<span class="em"><NavIcon name="info" /></span>
						<div>A importação por IA não está ligada neste servidor. Use o cadastro manual abaixo.</div>
					</div>
				{/if}
				<div class="form-grid" style="align-items:center">
					<label class="btn" class:primary={!arquivo} style="cursor:pointer">
						{arquivo ? 'Trocar PDF' : 'Escolher PDF do edital'}
						<input type="file" accept="application/pdf" onchange={escolherArquivo} disabled={semIA} hidden />
					</label>
					{#if arquivo}
						<span class="file-chip">{arquivo.name} · {fmtTam(arquivo.size)}</span>
					{/if}
				</div>
				<div class="field" style="margin-top:14px">
					<label for="edital-texto">…ou cole o texto do edital</label>
					<textarea
						id="edital-texto"
						rows="5"
						bind:value={textoEdital}
						disabled={semIA}
						placeholder="Cole a parte de cargos / disciplinas / conteúdo programático / cronograma"
						style="width:100%"
					></textarea>
				</div>
				<button
					class="btn primary"
					style="margin-top:12px"
					disabled={semIA || (!arquivo && !textoEdital.trim())}
					onclick={analisar}
				>
					Analisar edital →
				</button>
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Preencher manualmente</h2>
				<p class="page-sub" style="margin-top:0">Nome, data da prova e as disciplinas. Leva um minuto.</p>
				<button class="btn" onclick={() => (etapa = 'manual')}>Cadastrar manualmente</button>
			</div>
		</div>

		{#if !primeiroConcurso}
			<p style="margin-top:16px"><a href="/concursos">← voltar</a></p>
		{/if}
	{/if}

	<!-- ETAPA 2: CARGO -->
	{#if etapa === 'cargo' && !processando}
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Qual cargo você vai fazer?</h2>
				<p class="page-sub" style="margin-top:0">
					{banca ? `Banca: ${banca}. ` : ''}Um edital costuma ter vários cargos — o plano é montado só
					para o que você escolher.
				</p>
				{#each cargos as c (c.codigo + c.nome)}
					<label class="cargo-opt" class:sel={cargoSel === c.nome}>
						<input type="radio" name="cargo" value={c.nome} bind:group={cargoSel} />
						<span>
							<span class="nm">{c.codigo ? c.codigo + ' — ' : ''}{c.nome}</span>
							<span class="meta">
								{c.escolaridade || ''}{c.escolaridade && c.vagas ? ' · ' : ''}{c.vagas
									? c.vagas + ' vaga(s)'
									: ''}
							</span>
						</span>
					</label>
				{/each}
				<div style="display:flex;gap:10px;margin-top:16px">
					<button class="btn" onclick={() => (etapa = 'edital')}>← voltar</button>
					<button class="btn primary" disabled={!cargoSel} onclick={buscarEstrutura}>
						Continuar →
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- ETAPA 3 e 4: DISCIPLINAS -->
	{#snippet listaDisc(
		titulo: string,
		rows: DiscRow[],
		grupo: GrupoResposta | null,
		voltar: () => void,
		avancar: () => void,
		textoAvancar: string
	)}
		{@const falta = faltaEstimarEm(rows)}
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">{titulo}</h2>
				{#if grupo}
					<p class="page-sub" style="margin-top:0">
						{grupo.rotulo}{#if grupo.total !== null} — <b>{grupo.total} questões</b> no grupo{/if}{#if grupo.peso !== null}, peso <b>{grupo.peso}</b>{/if}.
						{#if grupo.disciplinas.every((x) => x.questoes === null)}
							O edital não dividiu as questões por disciplina — informe uma estimativa ou rateie.
						{/if}
					</p>
				{/if}
				{#if alertas.length}
					<div class="callout warn" style="margin-bottom:10px">
						<span class="em"><NavIcon name="alerta" /></span>
						<div>
							<ul style="margin:0;padding-left:16px">
								{#each alertas as a (a.codigo + (a.campo ?? ''))}<li>{a.mensagem}</li>{/each}
							</ul>
						</div>
					</div>
				{/if}
				<div style="font-size:10.5px;letter-spacing:.06em;text-transform:uppercase;color:var(--text-faint);display:grid;grid-template-columns:minmax(0,1fr) 90px 34px;gap:8px">
					<span>Disciplina</span><span>Questões</span><span></span>
				</div>
				{#each rows as d, i (i)}
					<div class="disc-linha">
						<input type="text" bind:value={d.nome} />
						<input
							type="number"
							min="0"
							max="80"
							placeholder="—"
							value={d.questoes ?? ''}
							oninput={(e) => {
								const v = (e.target as HTMLInputElement).value;
								d.questoes = v === '' ? null : Math.max(0, Number(v));
							}}
						/>
						<IconButton
							icon="fechar"
							label="Remover disciplina"
							tom="perigo"
							disabled={rows.length <= 1}
							onclick={() => rmDisc(rows, i)}
						/>
					</div>
				{/each}
				<div style="display:flex;gap:10px;margin-top:12px;flex-wrap:wrap">
					<button class="btn" onclick={() => addDisc(rows)}>+ disciplina</button>
					{#if grupo && grupo.total !== null}
						<button class="btn" onclick={() => ratear(rows, grupo.total)}>
							Ratear {grupo.total} como estimativa
						</button>
					{/if}
				</div>
				{#if grupo && grupo.total !== null}
					<p class="page-sub" style="margin-top:8px;font-size:12.5px">
						Soma informada: <b>{somaQuestoes(rows)}</b> / {grupo.total} do grupo{#if somaQuestoes(rows) !== grupo.total} — <span style="color:var(--warn)">não bate</span>{/if}
					</p>
				{/if}
				<div style="display:flex;gap:10px;margin-top:16px">
					<button class="btn" onclick={voltar}>← voltar</button>
					<button class="btn primary" disabled={falta} onclick={avancar}>{textoAvancar} →</button>
				</div>
				{#if falta}
					<p class="page-sub" style="margin-top:6px;font-size:12.5px;color:var(--warn)">
						Preencha o nº de questões (ou rateie) em todas as disciplinas antes de continuar.
					</p>
				{/if}
			</div>
		</div>
	{/snippet}

	{#if etapa === 'gerais' && !processando}
		{@render listaDisc(
			'Conhecimentos gerais',
			gerais,
			grupoGerais,
			() => (etapa = 'cargo'),
			() => (etapa = 'especificas'),
			'Próximo: específicas'
		)}
	{/if}

	{#if etapa === 'especificas' && !processando}
		{@render listaDisc(
			'Conhecimentos específicos do cargo',
			especificas,
			grupoEspecificos,
			() => (etapa = 'gerais'),
			buscarConteudo,
			'Buscar conteúdo programático'
		)}
	{/if}

	<!-- ETAPA 5: CONTEÚDO — só aparece quando a IA falhou nesta etapa -->
	{#if etapa === 'conteudo' && !processando}
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Conteúdo programático</h2>
				<p class="page-sub" style="margin-top:0">
					A IA não conseguiu extrair os temas agora — normalmente é sobrecarga
					temporária do provedor. Você pode tentar de novo, ou seguir sem os temas
					e adicioná-los depois em <b>editar o concurso</b>.
				</p>
				<div style="display:flex;gap:8px;flex-wrap:wrap;margin-top:14px">
					<button class="btn" onclick={buscarConteudo}>Tentar de novo</button>
					<button class="btn" onclick={montarRevisao}>Seguir sem os temas</button>
					<button class="btn" onclick={() => (etapa = 'especificas')}>← voltar</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- ETAPA 6: REVISÃO / MANUAL -->
	{#if etapa === 'revisao' || etapa === 'manual'}
		<ConcursoForm
			inicial={inicialRevisao}
			avisos={alertas.map((a) => a.mensagem)}
			{erro}
			{enviando}
			textoBotao="Criar concurso"
			onsubmit={salvar}
		/>
		<p style="margin-top:12px">
			<button
				class="btn"
				onclick={() => {
					etapa = 'edital';
					inicialRevisao = undefined;
					alertas = [];
				}}>← começar de novo</button
			>
		</p>
	{/if}
</div>
