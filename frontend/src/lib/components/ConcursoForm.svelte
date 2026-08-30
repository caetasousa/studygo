<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import { sintetizarConteudo } from '$lib/conteudo';
	import { api } from '$lib/api';
	import type { ConcursoInput, DisciplinaInput } from '$lib/types';

	let {
		inicial,
		enviando = false,
		erro = null,
		avisos = [],
		textoBotao = 'Salvar concurso',
		// When set, an "extract topics with AI" panel is shown: it reprocesses the
		// edital and fills every discipline that has no topics yet. Only useful when
		// editing an existing concurso (the wizard already does this inline).
		mostrarExtracao = false,
		onsubmit
	}: {
		inicial?: ConcursoInput;
		enviando?: boolean;
		erro?: string | null;
		avisos?: string[];
		textoBotao?: string;
		mostrarExtracao?: boolean;
		onsubmit: (input: ConcursoInput) => void;
	} = $props();

	// ---- AI topic extraction (edit mode) ----
	let editalTexto = $state('');
	let editalPdf = $state<File | null>(null);
	let extraindo = $state(false);
	let extracaoErro = $state<string | null>(null);
	let extracaoOk = $state<string | null>(null);

	async function extrairTemas() {
		const fonte = editalPdf
			? { pdf: editalPdf }
			: editalTexto.trim()
				? { texto: editalTexto.trim() }
				: null;
		if (!fonte) {
			extracaoErro = 'Cole o texto do edital ou anexe o PDF.';
			return;
		}

		extraindo = true;
		extracaoErro = null;
		extracaoOk = null;
		try {
			const nomes = discs.map((d) => d.nome.trim()).filter(Boolean);
			const res = await api.conteudoEdital(fonte, nomes);
			const mapa = new Map(res.itens.map((it) => [it.nome.trim().toLowerCase(), it.temas]));
			let preenchidas = 0;
			for (const d of discs) {
				if (d.temasTexto.trim()) continue; // não sobrescreve o que já existe
				const t = mapa.get(d.nome.trim().toLowerCase());
				if (t?.length) {
					d.temasTexto = t.join('\n');
					preenchidas++;
				}
			}
			extracaoOk =
				preenchidas > 0
					? `Temas preenchidos em ${preenchidas} disciplina(s). Confira e salve.`
					: 'A IA não retornou temas para as disciplinas ainda vazias.';
		} catch (e) {
			extracaoErro =
				e instanceof Error
					? e.message
					: 'Não foi possível extrair os temas agora — tente de novo em instantes.';
		} finally {
			extraindo = false;
		}
	}

	function vazia(): DisciplinaInput {
		return { nome: '', bloco: 'esp', questoes: 0, temas: [], fontes: [] };
	}

	// The form owns editable copies seeded from `inicial` at mount. Parents only
	// render <ConcursoForm> once `inicial` is resolved, so this is intentional.
	// svelte-ignore state_referenced_locally
	const seed: ConcursoInput = inicial ?? {
		nome: '',
		banca: '',
		cargo: '',
		emoji: '📚',
		prova: '',
		retaFinalDias: 28,
		disciplinas: [vazia()],
		marcos: [],
		conteudo: []
	};

	let nome = $state(seed.nome);
	let banca = $state(seed.banca);
	let cargo = $state(seed.cargo);
	let emoji = $state(seed.emoji || '📚');
	let prova = $state(seed.prova);
	let retaFinalDias = $state(seed.retaFinalDias || 28);

	// Disciplinas — temas/fontes edited as text, split on submit.
	interface DiscForm {
		nome: string;
		bloco: 'esp' | 'ger';
		questoes: number;
		temasTexto: string;
		fontesTexto: string;
	}

	let discs = $state<DiscForm[]>(
		(seed.disciplinas.length ? seed.disciplinas : [vazia()]).map((d) => ({
			nome: d.nome,
			bloco: d.bloco,
			questoes: d.questoes,
			temasTexto: (d.temas ?? []).join('\n'),
			fontesTexto: (d.fontes ?? []).map((f) => `${f.titulo} | ${f.url}`).join('\n')
		}))
	);

	interface MarcoForm {
		data: string;
		dataFim: string;
		titulo: string;
		exigeAcao: boolean;
	}
	let marcos = $state<MarcoForm[]>(
		seed.marcos.map((m) => ({
			data: m.data,
			dataFim: m.dataFim,
			titulo: m.titulo,
			exigeAcao: m.exigeAcao
		}))
	);

	const totalEsp = $derived(
		discs.filter((d) => d.bloco === 'esp').reduce((a, d) => a + (d.questoes || 0), 0)
	);
	const totalGer = $derived(
		discs.filter((d) => d.bloco === 'ger').reduce((a, d) => a + (d.questoes || 0), 0)
	);

	function addDisc() {
		discs.push({ nome: '', bloco: 'esp', questoes: 0, temasTexto: '', fontesTexto: '' });
	}
	function rmDisc(i: number) {
		discs.splice(i, 1);
	}
	function addMarco() {
		marcos.push({ data: '', dataFim: '', titulo: '', exigeAcao: false });
	}
	function rmMarco(i: number) {
		marcos.splice(i, 1);
	}

	function parseFontes(texto: string) {
		return texto
			.split('\n')
			.map((l) => l.trim())
			.filter(Boolean)
			.map((l) => {
				const [titulo, ...resto] = l.split('|');
				return { titulo: titulo.trim(), url: resto.join('|').trim(), tipo: 'lei' };
			});
	}

	function submit(e: SubmitEvent) {
		e.preventDefault();
		const disciplinas: DisciplinaInput[] = discs.map((d) => ({
			nome: d.nome.trim(),
			bloco: d.bloco,
			questoes: Math.max(0, d.questoes || 0),
			temas: d.temasTexto
				.split('\n')
				.map((t) => t.trim())
				.filter(Boolean),
			fontes: parseFontes(d.fontesTexto)
		}));

		const input: ConcursoInput = {
			nome: nome.trim(),
			banca: banca.trim(),
			cargo: cargo.trim(),
			emoji: emoji.trim() || '📚',
			prova,
			retaFinalDias,
			disciplinas,
			marcos: marcos
				.filter((m) => m.data && m.titulo)
				.map((m) => ({
					data: m.data,
					dataFim: m.dataFim,
					titulo: m.titulo.trim(),
					exigeAcao: m.exigeAcao
				})),
			// A ementa é regerada dos temas, para não ficar defasada ao editar.
			conteudo: sintetizarConteudo(disciplinas)
		};
		onsubmit(input);
	}
</script>

<form class="page" onsubmit={submit}>
	{#if erro}<div class="form-error" style="margin-bottom:14px">{erro}</div>{/if}

	{#if avisos.length > 0}
		<div class="callout warn" style="margin-bottom:14px">
			<span class="em"><NavIcon name="alerta" /></span>
			<div>
				<b>Confira antes de salvar:</b>
				<ul style="margin:6px 0 0;padding-left:18px">
					{#each avisos as a (a)}<li>{a}</li>{/each}
				</ul>
			</div>
		</div>
	{/if}

	<div class="card">
		<div class="card-body">
			<h2 class="sec" style="margin-top:0">O concurso</h2>
			<div class="form-grid">
				<div class="field" style="flex:1 1 260px">
					<label for="cf-nome">Nome *</label>
					<input id="cf-nome" type="text" bind:value={nome} required placeholder="TJ-SP — Escrevente" style="width:100%" />
				</div>
				<div class="field">
					<label for="cf-prova">Data da prova *</label>
					<input id="cf-prova" type="date" bind:value={prova} required />
				</div>
				<div class="field">
					<label for="cf-banca">Banca</label>
					<input id="cf-banca" type="text" bind:value={banca} placeholder="VUNESP" />
				</div>
				<div class="field">
					<label for="cf-cargo">Cargo</label>
					<input id="cf-cargo" type="text" bind:value={cargo} />
				</div>
				<div class="field">
					<label for="cf-emoji">Emoji</label>
					<input id="cf-emoji" type="text" bind:value={emoji} maxlength="2" style="width:60px" />
				</div>
				<div class="field">
					<label for="cf-reta">Reta final (dias)</label>
					<input id="cf-reta" type="number" min="7" max="120" step="7" bind:value={retaFinalDias} />
				</div>
			</div>
		</div>
	</div>

	{#if mostrarExtracao}
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Extrair temas com IA</h2>
				<p class="page-sub" style="margin-top:0">
					Cole o edital (a parte de conteúdo programático) ou anexe o PDF. A IA lê e preenche os
					temas de cada disciplina que ainda está <b>sem temas</b> — o que já foi preenchido à mão
					não é tocado.
				</p>

				<div class="field" style="margin-top:10px">
					<label for="cf-edital-pdf">PDF do edital</label>
					<input
						id="cf-edital-pdf"
						type="file"
						accept="application/pdf"
						onchange={(e) => (editalPdf = e.currentTarget.files?.[0] ?? null)}
					/>
				</div>
				<div class="field" style="margin-top:10px">
					<label for="cf-edital-txt">…ou cole o texto do edital</label>
					<textarea
						id="cf-edital-txt"
						rows="6"
						bind:value={editalTexto}
						placeholder={'CONHECIMENTOS ESPECÍFICOS\n1 Língua Portuguesa: 1.1 ...'}
						style="width:100%"
					></textarea>
				</div>

				{#if extracaoErro}
					<div class="form-error" style="margin-top:10px">{extracaoErro}</div>
				{/if}
				{#if extracaoOk}
					<div class="callout" style="margin-top:10px">
						<span class="em"><NavIcon name="info" /></span>
						<div>{extracaoOk}</div>
					</div>
				{/if}

				<button
					type="button"
					class="btn"
					style="margin-top:12px"
					disabled={extraindo}
					onclick={extrairTemas}
				>
					{extraindo ? 'Extraindo…' : 'Extrair temas'}
				</button>
			</div>
		</div>
	{/if}

	<div class="card">
		<div class="card-body">
			<h2 class="sec" style="margin-top:0">
				Disciplinas — {totalGer} gerais + {totalEsp} específicas
			</h2>
			<p class="page-sub" style="margin-top:0">
				Cada questão de <b>específicas</b> vale 2 pontos; de <b>gerais</b>, 1. É essa proporção que
				divide o tempo. Os temas são opcionais — sem eles, o dia mostra o nome da disciplina.
			</p>

			{#each discs as d, i (i)}
				<div class="card" style="margin-top:12px">
					<div class="card-body">
						<div class="form-grid">
							<div class="field" style="flex:1 1 220px">
								<label for="cf-d{i}-nome">Disciplina *</label>
								<input id="cf-d{i}-nome" type="text" bind:value={d.nome} required style="width:100%" />
							</div>
							<div class="field">
								<label for="cf-d{i}-grupo">Grupo</label>
								<div class="day-sel" id="cf-d{i}-grupo">
									<button
										type="button"
										aria-pressed={d.bloco === 'esp'}
										style="width:auto;padding:0 10px"
										onclick={() => (d.bloco = 'esp')}>Específicas</button
									>
									<button
										type="button"
										aria-pressed={d.bloco === 'ger'}
										style="width:auto;padding:0 10px"
										onclick={() => (d.bloco = 'ger')}>Gerais</button
									>
								</div>
							</div>
							<div class="field">
								<label for="cf-d{i}-q">Questões</label>
								<input id="cf-d{i}-q" type="number" min="0" max="80" step="1" bind:value={d.questoes} />
							</div>
							<div class="field" style="justify-content:flex-end">
								<button
									type="button"
									class="btn danger"
									disabled={discs.length <= 1}
									onclick={() => rmDisc(i)}>Remover</button
								>
							</div>
						</div>

						<details style="margin-top:10px">
							<summary style="cursor:pointer;font-size:13px;color:var(--text-muted)">
								Temas e fontes (opcional)
							</summary>
							<div class="form-grid" style="margin-top:10px">
								<div class="field" style="flex:1 1 320px">
									<label for="cf-d{i}-temas">Temas — um por linha</label>
									<textarea
										id="cf-d{i}-temas"
										rows="4"
										bind:value={d.temasTexto}
										placeholder={'Crase\nConcordância verbal\nRegência nominal'}
										style="width:100%"
									></textarea>
								</div>
								<div class="field" style="flex:1 1 320px">
									<label for="cf-d{i}-fontes">Leis / materiais — "título | link" por linha</label>
									<textarea
										id="cf-d{i}-fontes"
										rows="4"
										bind:value={d.fontesTexto}
										placeholder={'Lei 8.112/90 | https://planalto.gov.br/...\nCF/88 arts. 37-41 | https://...'}
										style="width:100%"
									></textarea>
								</div>
							</div>
						</details>
					</div>
				</div>
			{/each}

			<button type="button" class="btn" style="margin-top:12px" onclick={addDisc}>+ disciplina</button>
		</div>
	</div>

	<div class="card">
		<div class="card-body">
			<h2 class="sec" style="margin-top:0">Datas do edital (opcional)</h2>
			{#each marcos as m, i (i)}
				<div class="form-grid" style="margin-top:8px">
					<div class="field">
						<label for="cf-m{i}-data">Data</label>
						<input id="cf-m{i}-data" type="date" bind:value={m.data} />
					</div>
					<div class="field">
						<label for="cf-m{i}-fim">Até (período)</label>
						<input id="cf-m{i}-fim" type="date" bind:value={m.dataFim} />
					</div>
					<div class="field" style="flex:1 1 240px">
						<label for="cf-m{i}-t">Descrição</label>
						<input id="cf-m{i}-t" type="text" bind:value={m.titulo} style="width:100%" />
					</div>
					<div class="field">
						<label for="cf-m{i}-a">Exige ação</label>
						<input id="cf-m{i}-a" type="checkbox" class="checkbox" bind:checked={m.exigeAcao} />
					</div>
					<div class="field" style="justify-content:flex-end">
						<button type="button" class="btn danger" onclick={() => rmMarco(i)}>×</button>
					</div>
				</div>
			{/each}
			<button type="button" class="btn" style="margin-top:12px" onclick={addMarco}>+ data</button>
		</div>
	</div>

	<div style="margin-top:18px;display:flex;gap:12px">
		<button type="submit" class="btn primary" disabled={enviando}>
			{enviando ? 'Salvando…' : textoBotao}
		</button>
	</div>
</form>
