<script lang="ts">
	import { goto } from '$app/navigation';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { api, ApiError } from '$lib/api';
	import ConcursoForm from '$lib/components/ConcursoForm.svelte';
	import type { ConcursoInput } from '$lib/types';

	let modo = $state<'escolher' | 'form'>('escolher');
	let inicial = $state<ConcursoInput | undefined>(undefined);
	let avisos = $state<string[]>([]);
	let erro = $state<string | null>(null);
	let enviando = $state(false);
	let importando = $state(false);
	let textoEdital = $state('');

	const primeiroConcurso = $derived(concursoStore.carregado && concursoStore.lista.length === 0);

	async function importarTexto() {
		if (!textoEdital.trim()) return;
		importando = true;
		erro = null;
		try {
			const res = await api.importarEditalTexto(textoEdital);
			inicial = res.concurso;
			avisos = res.avisos;
			modo = 'form';
		} catch (e) {
			erro = tratar(e);
		} finally {
			importando = false;
		}
	}

	async function importarPDF(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		importando = true;
		erro = null;
		try {
			const res = await api.importarEditalPDF(file);
			inicial = res.concurso;
			avisos = res.avisos;
			modo = 'form';
		} catch (err) {
			erro = tratar(err);
		} finally {
			importando = false;
		}
	}

	function tratar(e: unknown): string {
		if (e instanceof ApiError && e.status === 503) {
			return 'A importação por IA não está configurada neste servidor (GEMINI_API_KEY). Cadastre manualmente abaixo.';
		}
		return e instanceof Error ? e.message : 'Não consegui ler o edital';
	}

	async function salvar(input: ConcursoInput) {
		enviando = true;
		erro = null;
		try {
			const slug = await concursoStore.criar(input);
			await goto('/');
			void slug;
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Erro ao salvar';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="crumb">Concursos <span class="sep">/</span> Novo</div>
<h1 class="page-title"><span>➕</span><span>Cadastrar concurso</span></h1>
<p class="page-sub">
	{primeiroConcurso
		? 'Bem-vindo! Cadastre seu primeiro concurso e o plano de estudos é gerado na hora.'
		: 'O plano é gerado a partir das disciplinas, do peso de cada uma e da data da prova.'}
</p>

{#if modo === 'escolher'}
	<div class="page">
		{#if erro}<div class="form-error" style="margin-bottom:14px">{erro}</div>{/if}

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">📄 Tenho o edital</h2>
				<p class="page-sub" style="margin-top:0">
					Envie o PDF ou cole o texto do edital — a IA extrai disciplinas, conteúdo, nº de questões e o
					cronograma. Você revisa tudo antes de salvar.
				</p>
				<div class="form-grid">
					<label class="btn" style="cursor:pointer">
						{importando ? 'Lendo…' : 'Enviar PDF do edital'}
						<input
							type="file"
							accept="application/pdf"
							onchange={importarPDF}
							disabled={importando}
							hidden
						/>
					</label>
					{#if !concursoStore.importacaoEdital}
						<span class="page-sub" style="margin:0">
							(indisponível neste servidor — cadastre manualmente)
						</span>
					{/if}
				</div>
				<div class="field" style="margin-top:12px">
					<label for="edital-texto">…ou cole o texto do edital</label>
					<textarea
						id="edital-texto"
						rows="5"
						bind:value={textoEdital}
						placeholder="Cole aqui a parte de disciplinas / conteúdo programático / cronograma"
						style="width:100%"
					></textarea>
				</div>
				<button
					type="button"
					class="btn"
					style="margin-top:10px"
					disabled={importando || !textoEdital.trim()}
					onclick={importarTexto}
				>
					{importando ? 'Lendo…' : 'Ler edital colado'}
				</button>
			</div>
		</div>

		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">✍️ Preencher manualmente</h2>
				<p class="page-sub" style="margin-top:0">
					Nome, data da prova e as disciplinas com o número de questões estimado. Leva um minuto.
				</p>
				<button type="button" class="btn primary" onclick={() => (modo = 'form')}>
					Cadastrar manualmente
				</button>
			</div>
		</div>

		{#if !primeiroConcurso}
			<p style="margin-top:16px"><a href="/concursos">← voltar</a></p>
		{/if}
	</div>
{:else}
	<ConcursoForm {inicial} {avisos} {erro} {enviando} textoBotao="Criar concurso" onsubmit={salvar} />
	<p style="margin-top:12px">
		<button type="button" class="btn" onclick={() => (modo = 'escolher')}>← trocar de método</button>
	</p>
{/if}
