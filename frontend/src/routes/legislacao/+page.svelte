<script lang="ts">
	import { api } from '$lib/api';
	import IconButton from '$lib/components/IconButton.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { ImportacaoDeLei, LeiResumo, LeisDaMateria } from '$lib/types';

	/**
	 * As leis do concurso, matéria por matéria.
	 *
	 * O vínculo lei ↔ matéria é SUGERIDO pelo tópico ("Lei nº 16.168" num tópico
	 * da matéria sugere a Lei 16.168) e confirmado por quem estuda — nunca feito
	 * sozinho: um número que aparece de passagem num tópico não faz da lei
	 * matéria da prova.
	 *
	 * A importação aparece só para a curadoria: o catálogo é o mesmo para todo
	 * mundo, e o servidor recusa quem não está em LEIS_CURADORES de qualquer
	 * jeito.
	 */
	const slug = $derived(concursoStore.ativoSlug);

	let materias = $state<LeisDaMateria[]>([]);
	let catalogo = $state<LeiResumo[]>([]);
	let erro = $state<string | null>(null);
	let carregado = $state(false);
	let importando = $state(false);
	let resultado = $state<string | null>(null);
	let erroImportacao = $state<string | null>(null);

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

	function descrever(r: ImportacaoDeLei): string {
		const q = r.questoes;
		const partes = [
			r.novaVersao ? 'versão nova' : 'mesma versão, nada duplicado',
			`${q.novas} questões novas`,
			q.atualizadas ? `${q.atualizadas} atualizadas` : '',
			q.desativadas ? `${q.desativadas} desativadas` : ''
		].filter(Boolean);
		return `${r.curto} importada — ${partes.join(', ')}.`;
	}

	async function importar(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!arquivo) return;
		importando = true;
		resultado = null;
		erroImportacao = null;
		try {
			let pacote: unknown;
			try {
				pacote = JSON.parse(await arquivo.text());
			} catch {
				throw new Error('o arquivo não é JSON — use o pacote de `make leis-pacote`');
			}
			resultado = descrever(await api.importarLei(pacote));
			if (slug) await carregar(slug);
		} catch (err) {
			erroImportacao = err instanceof Error ? err.message : 'A importação falhou';
		} finally {
			importando = false;
		}
	}

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

	{#if auth.usuario?.curador}
		<div class="card importar">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Importar lei</h2>
				<p class="page-sub" style="margin-top:0">
					O pacote sai de <code>make leis-pacote</code> (em <code>conteudo/leis/pacotes/</code>). Importar a
					mesma versão de novo não duplica nada; uma versão nova preserva as respostas das questões que
					continuam.
				</p>
				<label class="arquivo">
					<span>Pacote da lei (.json)</span>
					<input type="file" accept=".json,application/json" disabled={importando} onchange={importar} />
				</label>
				{#if importando}<p class="page-sub">Importando…</p>{/if}
				{#if resultado}<p class="ok" role="status">{resultado}</p>{/if}
				{#if erroImportacao}<div class="form-error" role="alert">{erroImportacao}</div>{/if}
			</div>
		</div>
	{/if}

	{#if carregado}
		{#each materias as m (m.disciplinaId)}
			<section class="materia" aria-labelledby="mat-{m.disciplinaId}">
				<h2 class="sec" id="mat-{m.disciplinaId}">{m.nome}</h2>

				{#if m.vinculadas.length === 0 && m.sugeridas.length === 0}
					<p class="vazia">Nenhuma lei vinculada.</p>
				{/if}

				<ul class="leis">
					{#each m.vinculadas as l (l.slug)}
						<li class="lei">
							<span class="ic"><NavIcon name="lei" size="sm" /></span>
							<a href="/leis/{l.slug}">{l.curto}</a>
							<span class="meta">{l.questoes} {l.questoes === 1 ? 'questão' : 'questões'}</span>
							<IconButton icon="fechar" label="Desvincular {l.curto}" onclick={() => vincular(m, l.slug, false)} />
						</li>
					{/each}
					{#each m.sugeridas as l (l.slug)}
						<li class="lei sugerida">
							<span class="ic"><NavIcon name="lei" size="sm" /></span>
							<span class="nome">{l.curto}</span>
							<span class="meta">Sugerida pelo tópico</span>
							<button class="btn" type="button" onclick={() => vincular(m, l.slug, true)}>
								Vincular {l.curto}
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
							<option value="">Vincular outra lei…</option>
							{#each livres(m) as l (l.slug)}<option value={l.slug}>{l.curto}</option>{/each}
						</select>
					</label>
				{/if}
			</section>
		{/each}

		<section class="catalogo" aria-labelledby="catalogo-t">
			<h2 class="sec" id="catalogo-t">Todas as leis do catálogo</h2>
			{#if catalogo.length === 0}
				<p class="vazia">O catálogo ainda está vazio.</p>
			{:else}
				<ul class="leis">
					{#each catalogo as l (l.slug)}
						<li class="lei">
							<span class="ic"><NavIcon name="lei" size="sm" /></span>
							<a href="/leis/{l.slug}">{l.curto}</a>
							<span class="meta">{l.nome}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{:else if !erro}
		<p class="page-sub">Carregando…</p>
	{/if}
</div>

<style>
	.importar {
		margin-bottom: 20px;
	}
	.arquivo {
		display: inline-flex;
		flex-direction: column;
		gap: 6px;
		font-size: 13px;
	}
	.ok {
		color: var(--good);
		font-size: 13px;
	}
	.materia,
	.catalogo {
		margin-bottom: 22px;
	}
	.vazia {
		color: var(--text-muted);
		font-size: 13px;
		margin: 4px 0;
	}
	.leis {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.lei {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 8px;
		border-radius: 7px;
		font-size: 14px;
	}
	.lei:hover {
		background: var(--bg-hover);
	}
	.lei a {
		color: var(--text);
		font-weight: 600;
	}
	.lei .ic {
		color: var(--text-muted);
		display: inline-flex;
	}
	.meta {
		flex: 1;
		color: var(--text-muted);
		font-size: 12px;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.sugerida .nome {
		font-weight: 600;
	}
	.outra select {
		margin-top: 6px;
		font: inherit;
		font-size: 13px;
		background: var(--bg-card);
		color: var(--text);
		border: 1px solid var(--border-strong);
		border-radius: 6px;
		padding: 5px 8px;
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
