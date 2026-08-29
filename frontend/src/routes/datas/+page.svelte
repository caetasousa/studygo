<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { fc, hojeISO, diffDays } from '$lib/format';
	import type { Marco } from '$lib/types';

	const plano = $derived(planoStore.plano);

	const prox = $derived.by(() => {
		if (!plano) return null;
		const h = hojeISO();
		return plano.marcos.find((m) => (m.dataFim ?? m.dataInicio) >= h) ?? null;
	});

	function faltaTexto(m: Marco): string {
		const h = hojeISO();
		if (m.eProva) return 'dia da prova';
		if ((m.dataFim ?? m.dataInicio) < h) return 'encerrado';
		const d = diffDays(h, m.dataInicio);
		return d <= 0 ? 'em andamento' : `em ${d} dias`;
	}
</script>

<PageHead
	emoji="📌"
	titulo="Datas do edital"
	sub="O cronograma oficial do concurso — o que exige ação sua vem destacado."
	mostrarProps={false}
/>

{#if plano && plano.marcos.length === 0}
	<div class="page">
		<div class="callout">
			<span class="em">📌</span>
			<div>
				Nenhuma data do edital cadastrada.
				<a href="/concursos/{plano.concurso.slug}/editar">Edite o concurso</a> para adicionar as datas
				de inscrição, isenção, pagamento e convocação — elas viram lembretes e alertas.
			</div>
		</div>
	</div>
{:else if plano}
	<div class="page">
		<div class="callout">
			<span class="em">📌</span>
			<div>
				<b>Cronograma oficial do concurso.</b> As linhas destacadas exigem ação sua e podem ser marcadas
				quando você cumprir. Confira sempre as datas no site da banca.
			</div>
		</div>

		<div class="card" style="margin-top:16px">
			{#each plano.marcos as m (m.id)}
				{@const passou = (m.dataFim ?? m.dataInicio) < hojeISO()}
				<div
					class="marco"
					class:info={!m.exigeAcao}
					class:past={passou}
					class:exam={m.eProva}
					class:next={!m.eProva && m === prox}
				>
					<span class="idx">{String(m.rotulo).padStart(2, '0')}</span>
					<span class="dt">
						{fc(m.dataInicio)}{m.dataFim ? ' a ' + fc(m.dataFim) : ''}
						<small>{m.dataInicio.slice(0, 4)}</small>
					</span>
					<span class="tx">
						{#if m.exigeAcao}<b>exige ação sua</b>{/if}{m.titulo}
					</span>
					<span class="fa">{faltaTexto(m)}</span>
					<input
						type="checkbox"
						class="checkbox"
						aria-label="Cumprido"
						checked={m.cumprido}
						onchange={(e) => planoStore.marcarMarco(m.id, (e.target as HTMLInputElement).checked)}
					/>
				</div>
			{/each}
		</div>
	</div>
{/if}
