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
	<h1 class="page-title"><span class="title-ic"><NavIcon name={icone} size="md" /></span><span>{titulo}</span></h1>
</div>
<p class="page-sub">{sub}</p>

{#if mostrarProps && plano && metricas}
	<div class="props">
		<div class="prop"><span class="prop-ic"><NavIcon name="prova" size="sm" /></span> Prova <b>{fc(plano.config.prova)}</b></div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="faltam" size="sm" /></span> Faltam <b>{metricas.faltamDias}</b> dias
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="estatisticas" size="sm" /></span> Progresso <b>{metricas.progresso}%</b>
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="horas" size="sm" /></span> Horas
			<b>{nf1.format(metricas.horasTotal)}</b>/{nf0.format(metricas.horasAlvo)}h
		</div>
		<div class="prop">
			<span class="prop-ic"><NavIcon name="acerto" size="sm" /></span> Acerto
			<b>{metricas.acertoPct !== null ? metricas.acertoPct + '%' : '—'}</b>
		</div>
	</div>
{/if}

<!-- Só as datas que não podem passar — inscrições, pagamento, prova —, numa
     faixa só. Os demais prazos do edital ficam em Datas do edital. -->
{#if plano && plano.alertas.length > 0}
	<ul class="prazos" aria-label="Datas importantes">
		{#each plano.alertas as a (a.titulo)}
			<li class="prazo {a.nivel}">
				<span class="prazo-ic"><NavIcon name="prova" size="sm" /></span>
				<b>{a.titulo}</b>
				<span class="prazo-tx">{a.texto}</span>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.prazos {
		list-style: none;
		margin: 14px 0 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.prazo {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 6px 11px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--bg-card);
		font-size: 13px;
	}
	.prazo-ic {
		display: inline-flex;
		color: var(--text-faint);
	}
	.prazo-tx {
		color: var(--text-muted);
	}
	.prazo.warn {
		border-color: color-mix(in srgb, var(--warn) 45%, transparent);
		background: color-mix(in srgb, var(--warn) 10%, transparent);
	}
	.prazo.warn .prazo-ic,
	.prazo.warn b {
		color: var(--warn);
	}
	.prazo.danger {
		border-color: color-mix(in srgb, var(--danger) 50%, transparent);
		background: color-mix(in srgb, var(--danger) 12%, transparent);
	}
	.prazo.danger .prazo-ic,
	.prazo.danger b {
		color: var(--danger);
	}
	/* The title icon supports the heading; it does not compete with it. */
	.title-ic {
		display: grid;
		place-items: center;
		flex: none;
		color: var(--text-muted);
	}
	/* On a metric badge the icon is the smallest element: the number is the point. */
	.prop-ic {
		display: grid;
		place-items: center;
		flex: none;
		align-self: center;
		color: var(--text-faint);
	}
</style>
