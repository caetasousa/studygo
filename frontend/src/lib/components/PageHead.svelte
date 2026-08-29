<script lang="ts">
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, nf0, nf1 } from '$lib/format';

	let {
		emoji,
		titulo,
		sub,
		mostrarProps = true
	}: { emoji: string; titulo: string; sub: string; mostrarProps?: boolean } = $props();

	const plano = $derived(planoStore.plano);
	const metricas = $derived(plano?.props);
</script>

<div class="crumb">Estudos <span class="sep">/</span> {titulo}</div>
<div class="head-row">
	<h1 class="page-title"><span>{emoji}</span><span>{titulo}</span></h1>
</div>
<p class="page-sub">{sub}</p>

{#if mostrarProps && plano && metricas}
	<div class="props">
		<div class="prop">📅 Prova <b>{fc(plano.config.prova)}</b></div>
		<div class="prop">⏳ Faltam <b>{metricas.faltamDias}</b> dias</div>
		<div class="prop">📈 Progresso <b>{metricas.progresso}%</b></div>
		<div class="prop">
			⏱️ Horas <b>{nf1.format(metricas.horasTotal)}</b>/{nf0.format(metricas.horasAlvo)}h
		</div>
		<div class="prop">
			🎯 Acerto <b>{metricas.acertoPct !== null ? metricas.acertoPct + '%' : '—'}</b>
		</div>
	</div>
{/if}

{#if plano && plano.alertas.length > 0}
	<div class="alert-slot">
		{#each plano.alertas as a (a.titulo)}
			<div class="callout {a.nivel}">
				<span class="em">{a.nivel === 'danger' ? '🔴' : '🟡'}</span>
				<div><b>{a.titulo}</b><br />{a.texto}</div>
			</div>
		{/each}
	</div>
{/if}
