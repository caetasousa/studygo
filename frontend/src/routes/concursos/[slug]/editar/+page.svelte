<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import ConcursoForm from '$lib/components/ConcursoForm.svelte';
	import type { ConcursoInput } from '$lib/types';

	const slug = $derived(page.params.slug);

	let inicial = $state<ConcursoInput | undefined>(undefined);
	let carregando = $state(true);
	let erro = $state<string | null>(null);
	let enviando = $state(false);

	$effect(() => {
		const s = slug;
		if (!s) return;
		carregando = true;
		api
			.getConcurso(s)
			.then((d) => {
				inicial = d.dados;
				carregando = false;
			})
			.catch((e) => {
				erro = e instanceof Error ? e.message : 'Erro ao carregar';
				carregando = false;
			});
	});

	async function salvar(input: ConcursoInput) {
		if (!slug) return;
		enviando = true;
		erro = null;
		try {
			await concursoStore.atualizar(slug, input);
			await planoStore.carregar(true);
			await goto('/');
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Erro ao salvar';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="crumb">Concursos <span class="sep">/</span> Editar</div>
<h1 class="page-title"><span>✏️</span><span>Editar concurso</span></h1>
<p class="page-sub">
	Alterar disciplinas depois de começar a estudar pode reajustar os pesos por disciplina — as
	horas e questões já registradas por dia são preservadas.
</p>

{#if carregando}
	<p class="page-sub">Carregando…</p>
{:else if inicial}
	<ConcursoForm {inicial} {erro} {enviando} textoBotao="Salvar alterações" onsubmit={salvar} />
{:else}
	<div class="form-error">{erro}</div>
{/if}
