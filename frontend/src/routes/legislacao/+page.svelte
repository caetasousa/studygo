<script lang="ts">
	import { api } from '$lib/api';
	import CapturaDeLei from '$lib/components/CapturaDeLei.svelte';
	import IconButton from '$lib/components/IconButton.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { descreverRecorte } from '$lib/leis';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { LeiResumo, LeisDaMateria, PublicacaoDeLei } from '$lib/types';

	/**
	 * As leis do concurso, matéria por matéria.
	 *
	 * O vínculo lei ↔ matéria é SUGERIDO pelo tópico ("Lei nº 16.168" num tópico
	 * da matéria sugere a Lei 16.168) e confirmado por quem estuda — nunca feito
	 * sozinho: um número que aparece de passagem num tópico não faz da lei
	 * matéria da prova. Vinculada, a lei guarda o recorte que o tópico pede
	 * ("Administração Pública" é o Capítulo VII da Constituição), e o estudo
	 * mostra só ele.
	 *
	 * A captura fica aberta a qualquer conta enquanto o app é de teste
	 * (decisão de 25/09/2026): o catálogo é o mesmo para todo mundo, então
	 * publicar aqui publica a lei para todos.
	 */
	const slug = $derived(concursoStore.ativoSlug);

	let materias = $state<LeisDaMateria[]>([]);
	let catalogo = $state<LeiResumo[]>([]);
	let erro = $state<string | null>(null);
	let carregado = $state(false);
	let publicada = $state<PublicacaoDeLei | null>(null);

	async function carregar(s: string) {
		erro = null;
		try {
			const [doConcurso, todas] = await Promise.all([api.leisDoConcurso(s), api.catalogoDeLeis()]);
			materias = doConcurso.disciplinas;
			catalogo = todas.leis;
			carregado = true;
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível carregar a legislação';
		}
	}

	$effect(() => {
		if (slug) void carregar(slug);
	});

	async function vincular(m: LeisDaMateria, lei: string, ligar: boolean) {
		if (!slug) return;
		erro = null;
		try {
			await api.vincularLei(slug, m.disciplinaId, lei, ligar);
			await carregar(slug);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível gravar o vínculo';
		}
	}

	async function aoPublicar(r: PublicacaoDeLei) {
		publicada = r;
		if (slug) await carregar(slug);
	}

	const comLei = $derived(materias.filter((m) => m.vinculadas.length + m.sugeridas.length > 0));
	const semLei = $derived(materias.filter((m) => m.vinculadas.length + m.sugeridas.length === 0));

	const livres = (m: LeisDaMateria) =>
		catalogo.filter((l) => !m.vinculadas.some((v) => v.slug === l.slug) && !m.sugeridas.some((s) => s.slug === l.slug));
</script>

<PageHead
	icone="lei"
	titulo="Legislação"
	sub="As leis que o edital cobra, para ler artigo por artigo e resolver as questões de cada um."
	mostrarProps={false}
/>

<div class="page">
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

	{#if carregado}
		{#each comLei as m (m.disciplinaId)}
			{@render materia(m)}
		{/each}

		{#if comLei.length === 0}
			<p class="vazia">
				Nenhuma matéria do concurso tem lei ainda. Adicione abaixo as leis que o edital cita: a matéria cujo
				tópico cita a lei aparece aqui, com a parte que ela cobra.
			</p>
		{/if}

		{#if semLei.length > 0}
			<details class="toggle">
				<summary>Matérias sem lei <span class="conta">{semLei.length}</span></summary>
				{#each semLei as m (m.disciplinaId)}
					{@render materia(m)}
				{/each}
			</details>
		{/if}

		<details class="toggle adicionar" open={catalogo.length === 0}>
			<summary>Adicionar lei</summary>
			<p class="page-sub">
				Cole o link da lei na fonte oficial. O texto é baixado e organizado em artigos, incisos e alíneas sem que a
				IA toque em uma palavra; a prévia mostra o que o edital pede dela, e você publica.
			</p>
			<CapturaDeLei concurso={slug} {aoPublicar} />
		</details>
		{#if publicada}
			<p class="ok" role="status">
				{publicada.curto} publicada{publicada.novaVersao ? '' : ' — mesma versão, nada duplicado'}.
				<a href="/leis/{publicada.slug}">Abrir a lei</a>
			</p>
		{/if}

		{#if catalogo.length > 0}
			<details class="toggle">
				<summary>Todas as leis do catálogo <span class="conta">{catalogo.length}</span></summary>
				<ul class="catalogo">
					{#each catalogo as l (l.slug)}
						<li>
							<a href="/leis/{l.slug}">{l.curto}</a>
							<span class="meta">{l.nome}</span>
						</li>
					{/each}
				</ul>
			</details>
		{/if}
	{:else if !erro}
		<p class="page-sub">Carregando…</p>
	{/if}
</div>

{#snippet materia(m: LeisDaMateria)}
	<section class="materia" aria-labelledby="mat-{m.disciplinaId}">
		<h2 class="sec" id="mat-{m.disciplinaId}">{m.nome}</h2>

		{#if m.vinculadas.length === 0 && m.sugeridas.length === 0}
			<p class="vazia">Nenhuma lei vinculada.</p>
		{/if}

		<ul class="leis">
			{#each m.vinculadas as l (l.slug)}
				<li class="lei">
					<span class="ic"><NavIcon name="lei" size="sm" /></span>
					<div class="corpo">
						<a class="nome" href="/leis/{l.slug}">{l.curto}</a>
						<span class="recorte">
							{l.recorte.length === 0 ? 'A lei inteira' : descreverRecorte(l.recorte)}
						</span>
					</div>
					<span class="meta">{l.questoes} {l.questoes === 1 ? 'questão' : 'questões'}</span>
					<IconButton icon="fechar" label="Desvincular {l.curto}" onclick={() => vincular(m, l.slug, false)} />
				</li>
			{/each}
			{#each m.sugeridas as l (l.slug)}
				<li class="lei sugerida">
					<span class="ic"><NavIcon name="lei" size="sm" /></span>
					<div class="corpo">
						<span class="nome">{l.curto}</span>
						<span class="recorte">
							Sugerida pelo tópico · o edital pede {l.recorte.length === 0 ? 'a lei inteira' : descreverRecorte(l.recorte)}
						</span>
					</div>
					<button class="vincular" type="button" aria-label="Vincular {l.curto}" onclick={() => vincular(m, l.slug, true)}>
						Vincular
					</button>
				</li>
			{/each}
		</ul>

		{#if livres(m).length > 0}
			<label class="outra">
				<span class="sr-only">Vincular outra lei a {m.nome}</span>
				<select
					onchange={(e) => {
						const lei = e.currentTarget.value;
						e.currentTarget.value = '';
						if (lei) void vincular(m, lei, true);
					}}
				>
					<option value="">+ Vincular outra lei…</option>
					{#each livres(m) as l (l.slug)}<option value={l.slug}>{l.curto}</option>{/each}
				</select>
			</label>
		{/if}
	</section>
{/snippet}

<style>
	.ok {
		color: var(--good);
		font-size: 13px;
	}
	.materia {
		margin-bottom: 26px;
	}
	.vazia {
		color: var(--text-muted);
		font-size: 13px;
		margin: 4px 0 18px;
	}
	.leis {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.lei {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--bg-card);
		font-size: 14px;
	}
	.lei:hover {
		background: var(--bg-hover);
	}
	.lei.sugerida {
		border-style: dashed;
		background: none;
	}
	.lei .ic {
		color: var(--text-muted);
		display: inline-flex;
	}
	.corpo {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.nome {
		color: var(--text);
		font-weight: 600;
		text-decoration: none;
	}
	a.nome:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.recorte {
		color: var(--text-muted);
		font-size: 12.5px;
	}
	.meta {
		color: var(--text-faint);
		font-size: 12px;
		white-space: nowrap;
	}
	.vincular {
		font: inherit;
		font-size: 13px;
		font-weight: 600;
		padding: 5px 12px;
		border-radius: 6px;
		border: 1px solid var(--border-strong);
		background: var(--bg-card);
		color: var(--text);
		cursor: pointer;
	}
	.vincular:hover {
		background: var(--bg-hover);
	}
	.outra select {
		margin-top: 8px;
		font: inherit;
		font-size: 13px;
		background: none;
		color: var(--text-muted);
		border: 0;
		padding: 4px 2px;
		cursor: pointer;
	}
	.toggle {
		margin: 8px 0 18px;
	}
	.toggle > summary {
		cursor: pointer;
		font-weight: 600;
		font-size: 14px;
		list-style: none;
		padding: 6px 0;
	}
	.toggle > summary::before {
		content: '▸';
		display: inline-block;
		width: 16px;
		color: var(--text-faint);
		transition: transform 0.15s;
	}
	.toggle[open] > summary::before {
		transform: rotate(90deg);
	}
	.toggle > :global(:not(summary)) {
		margin-left: 16px;
	}
	.conta {
		color: var(--text-faint);
		font-weight: 400;
		margin-left: 4px;
	}
	.adicionar {
		padding: 4px 0 8px;
		border-top: 1px solid var(--border);
		border-bottom: 1px solid var(--border);
	}
	.catalogo {
		list-style: none;
		margin: 4px 0 0;
		padding: 0;
		font-size: 13.5px;
	}
	.catalogo li {
		display: flex;
		gap: 10px;
		padding: 4px 0;
		align-items: baseline;
	}
	.catalogo a {
		color: var(--text);
		font-weight: 600;
	}
	.catalogo .meta {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
	}
</style>
