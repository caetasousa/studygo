<script lang="ts">
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, nf0, nf1 } from '$lib/format';
	import NavIcon from './NavIcon.svelte';
	import type { NavIconName } from './NavIcon.svelte';

	let {
		icone,
		titulo,
		sub,
		mostrarProps = true
	}: { icone: NavIconName; titulo: string; sub: string; mostrarProps?: boolean } = $props();

	const plano = $derived(planoStore.plano);
	const metricas = $derived(plano?.props);
</script>

<div class="crumb">Estudos <span class="sep">/</span> {titulo}</div>
<div class="head-row">
	<h1 class="page-title"><span class="title-ic"><NavIcon name={icone} /></span><span>{titulo}</span></h1>
</div>
<p class="page-sub">{sub}</p>

{#if mostrarProps && plano && metricas}
	<div class="props">
		<div class="prop"><span class="prop-ic"><NavIcon name="prova" /></span> Prova <b>{fc(plano.config.prova)}</b></div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="faltam" /></span> Faltam <b>{metricas.faltamDias}</b> dias
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="estatisticas" /></span> Progresso <b>{metricas.progresso}%</b>
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="horas" /></span> Horas
			<b>{nf1.format(metricas.horasTotal)}</b>/{nf0.format(metricas.horasAlvo)}h
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="acerto" /></span> Acerto
			<b>{metricas.acertoPct !== null ? metricas.acertoPct + '%' : '—'}</b>
		</div>
	</div>
{/if}

{#if plano && plano.alertas.length > 0}
	<div class="alert-slot">
		{#each plano.alertas as a (a.titulo)}
			<div class="callout {a.nivel}">
				<span class="em"><NavIcon name="alerta" /></span>
				<div><b>{a.titulo}</b><br />{a.texto}</div>
			</div>
		{/each}
	</div>
{/if}

<style>
	.title-ic {
		width: 27px;
		height: 27px;
		flex: none;
		color: var(--text-muted);
		display: block;
	}
	.prop-ic {
		width: 14px;
		height: 14px;
		flex: none;
		display: block;
		align-self: center;
	}
</style>
