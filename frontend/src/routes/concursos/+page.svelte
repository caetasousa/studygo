<script lang="ts">
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { goto } from '$app/navigation';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, diffDays, hojeISO } from '$lib/format';

	let erro = $state<string | null>(null);

	$effect(() => {
		if (!concursoStore.carregado) concursoStore.carregar();
	});

	function abrir(slug: string) {
		concursoStore.setAtivo(slug);
		goto('/');
	}

	async function excluir(slug: string, nome: string) {
		if (!confirm(`Excluir "${nome}"? O plano e todo o progresso desse concurso serão apagados.`)) {
			return;
		}
		try {
			await concursoStore.remover(slug);
			planoStore.limpar();
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Erro ao excluir';
		}
	}
</script>

<div class="crumb">Estudos <span class="sep">/</span> Meus concursos</div>
<h1 class="page-title"><span>Meus concursos</span></h1>
<p class="page-sub">Cada concurso tem seu próprio plano, progresso e caderno de erros.</p>

<div class="page">
	{#if erro}<div class="form-error" style="margin-bottom:14px">{erro}</div>{/if}

	{#if concursoStore.lista.length === 0}
		<div class="callout">
			<span class="em"><NavIcon name="info" /></span>
			<div>Você ainda não cadastrou nenhum concurso.</div>
		</div>
	{:else}
		<div class="card">
			{#each concursoStore.lista as c (c.slug)}
				<div class="marco" style="grid-template-columns:1fr 120px 90px 90px">
					<span class="tx">
						<b style="color:var(--text)">{c.nome}</b>
						{c.banca || ''}{c.cargo ? ' · ' + c.cargo : ''}
						{#if c.slug === concursoStore.ativoSlug}
							<span class="pill" style="margin-left:8px">ativo</span>
						{/if}
					</span>
					<span class="fa">
						prova {fc(c.prova)}<br />
						<small>em {Math.max(0, diffDays(hojeISO(), c.prova))} dias</small>
					</span>
					<button class="btn" style="padding:6px 8px" onclick={() => abrir(c.slug)}>abrir</button>
					<span style="display:flex;gap:4px">
						<a class="btn" style="padding:6px 8px" href="/concursos/{c.slug}/editar">editar</a>
						<button
							class="mv-btn"
							title="Excluir"
							aria-label="Excluir concurso"
							onclick={() => excluir(c.slug, c.nome)}>✕</button
						>
					</span>
				</div>
			{/each}
		</div>
	{/if}

	<a class="btn primary" style="margin-top:16px" href="/concursos/novo">+ novo concurso</a>
</div>
