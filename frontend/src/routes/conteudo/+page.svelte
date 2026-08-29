<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';

	const plano = $derived(planoStore.plano);
</script>

<PageHead
	emoji="📖"
	titulo="Conteúdo programático"
	sub="O texto do edital, matéria por matéria."
	mostrarProps={false}
/>

{#if plano && plano.concurso.conteudo.length === 0}
	<div class="page">
		<div class="callout">
			<span class="em">📖</span>
			<div>
				Nenhum conteúdo programático cadastrado. Ele é opcional — o plano usa as disciplinas e seus
				temas. Se quiser guardar o texto do edital aqui,
				<a href="/concursos/{plano.concurso.slug}/editar">edite o concurso</a>.
			</div>
		</div>
	</div>
{:else if plano}
	<div class="page prog">
		{#each plano.concurso.conteudo as item, i (i)}
			{#if item.tipo === 'ficha'}
				<div class="prog-card">{item.texto}</div>
			{:else if item.tipo === 'rot'}
				<h3>{item.texto}</h3>
			{:else if item.tipo === 'h'}
				<h4>{item.texto}</h4>
			{:else}
				<p>{item.texto}</p>
			{/if}
		{/each}
	</div>
{/if}
