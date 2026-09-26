<script lang="ts">
	import { api } from '$lib/api';
	import CapturaDeLei from '$lib/components/CapturaDeLei.svelte';
	import IconButton from '$lib/components/IconButton.svelte';
	import ImportarDoTema from '$lib/components/ImportarDoTema.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import PageHead from '$lib/components/PageHead.svelte';
	import { descreverRecorte } from '$lib/leis';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { LeiNaMateria, LeiResumo, LeisDaMateria, PublicacaoDeLei, ResumoDaExclusao } from '$lib/types';

	/**
	 * As leis do concurso, matéria por matéria.
	 *
	 * A importação começa pelo tópico do edital: "Pesquisar e importar" acha a
	 * fonte, mostra o que cada assunto pede da lei, e só o que ficou marcado
	 * entra (ver ImportarDoTema). O link colado à mão fica para a norma que a
	 * pesquisa não acha.
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
	let mensagem = $state<string | null>(null);
	// O tópico com a importação aberta, por matéria: "disciplinaId|texto".
	let abertoEm = $state<string | null>(null);
	let excluindo = $state<(ResumoDaExclusao & { slug: string }) | null>(null);

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

	// A matéria em que "Vincular todas" está gravando.
	let vinculando = $state<string | null>(null);

	/** Uma de cada vez: cada vínculo grava o recorte que os tópicos pedem dela. */
	async function vincularTodas(m: LeisDaMateria) {
		if (!slug) return;
		erro = null;
		vinculando = m.disciplinaId;
		try {
			for (const l of m.sugeridas) await api.vincularLei(slug, m.disciplinaId, l.slug, true);
			mensagem = `${m.sugeridas.length} leis vinculadas a ${m.nome}.`;
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível gravar os vínculos';
		} finally {
			vinculando = null;
			await carregar(slug);
		}
	}

	async function aoPublicar(r: PublicacaoDeLei) {
		publicada = r;
		if (slug) await carregar(slug);
	}

	// O que parece nome de norma num tópico: é onde cabe "Pesquisar e importar".
	const NORMA =
		/\b(lei|constitui[çc][ãa]o|resolu[çc][ãa]o|decreto|c[óo]digo|regimento|portaria|instru[çc][ãa]o normativa|medida provis[óo]ria|emenda constitucional)\b|n[º°o]\s*\d/i;
	const normas = (m: LeisDaMateria) => m.temas.filter((t) => NORMA.test(t.texto));

	const comLei = $derived(
		materias.filter((m) => m.vinculadas.length + m.sugeridas.length > 0 || normas(m).length > 0)
	);
	const semLei = $derived(materias.filter((m) => !comLei.includes(m)));

	async function importado(texto: string) {
		abertoEm = null;
		mensagem = texto;
		if (slug) await carregar(slug);
	}

	async function pedirExclusao(lei: string) {
		erro = null;
		try {
			excluindo = { ...(await api.resumirExclusao(lei)), slug: lei };
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível preparar a exclusão';
		}
	}

	async function excluir() {
		if (!excluindo) return;
		const { slug: lei, curto } = excluindo;
		erro = null;
		try {
			await api.excluirLei(lei);
			excluindo = null;
			mensagem = `${curto} excluída.`;
			if (slug) await carregar(slug);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'A exclusão falhou';
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
	{#if mensagem}<p class="ok" role="status">{mensagem}</p>{/if}
	{#if excluindo}
		<div class="confirmar" role="alertdialog" aria-label="Excluir {excluindo.curto}">
			<p>
				Excluir <b>{excluindo.curto}</b> do catálogo? Vão junto
				{excluindo.questoes}
				{excluindo.questoes === 1 ? 'questão' : 'questões'} e {excluindo.respostas}
				{excluindo.respostas === 1 ? 'resposta' : 'respostas'}, de quem quer que as tenha respondido. Não dá para
				desfazer.
			</p>
			<button class="btn danger" type="button" onclick={excluir}>Excluir de vez</button>
			<button class="btn" type="button" onclick={() => (excluindo = null)}>Cancelar</button>
		</div>
	{/if}

	{#if carregado}
		{#each comLei as m (m.disciplinaId)}
			{@render materia(m)}
		{/each}

		{#if comLei.length === 0}
			<p class="vazia">
				Nenhum tópico do edital cita uma norma. Se faltar alguma, adicione-a pelo link, abaixo.
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
			<summary>Adicionar lei pelo link</summary>
			<p class="page-sub">
				Para a norma que a pesquisa pelo tópico não acha. O texto é baixado e organizado em artigos, incisos e
				alíneas sem que a IA toque em uma palavra; a prévia mostra o que o edital pede dela, e você publica.
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
							<button class="discreto" type="button" onclick={() => pedirExclusao(l.slug)}>Excluir</button>
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
	{@const soltas = m.vinculadas.filter((l) => !m.temas.some((t) => t.leis.includes(l.slug)))}
	<section class="materia" aria-labelledby="mat-{m.disciplinaId}">
		<div class="mat-cabeca">
			<h2 class="sec" id="mat-{m.disciplinaId}">{m.nome}</h2>
			{#if m.sugeridas.length > 1}
				<button class="vincular-todas" type="button" disabled={vinculando === m.disciplinaId} onclick={() => vincularTodas(m)}>
					Vincular todas as sugeridas ({m.sugeridas.length})
				</button>
			{/if}
		</div>

		{#if normas(m).length > 0}
			<ul class="temas" aria-label="Tópicos do edital que citam normas">
				{#each normas(m) as t (t.texto)}
					{@const chave = `${m.disciplinaId}|${t.texto}`}
					<li class="tema">
						<p class="texto-tema">{t.texto}</p>
						{#each t.leis as s (s)}
							{@const l = m.vinculadas.find((v) => v.slug === s)}
							{#if l}{@render vinculada(m, l)}{/if}
						{/each}
						{#each t.sugeridas as s (s)}
							{@const l = m.sugeridas.find((v) => v.slug === s)}
							{#if l}{@render sugerida(m, l)}{/if}
						{/each}
						{#if t.leis.length === 0 && t.sugeridas.length === 0 && abertoEm !== chave}
							<button
								class="pesquisar"
								type="button"
								onclick={() => {
									abertoEm = chave;
									mensagem = null;
								}}
							>
								Pesquisar e importar
							</button>
						{/if}
						{#if abertoEm === chave && slug}
							<ImportarDoTema tema={t.texto} concurso={slug} disciplinaId={m.disciplinaId} aoTerminar={importado} />
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

		{#if soltas.length > 0}
			<p class="rotulo-outras">{normas(m).length > 0 ? 'Outras leis vinculadas' : 'Leis vinculadas'}</p>
			{#each soltas as l (l.slug)}{@render vinculada(m, l)}{/each}
		{/if}
		{#if normas(m).length === 0 && m.sugeridas.length > 0}
			{#each m.sugeridas as l (l.slug)}{@render sugerida(m, l)}{/each}
		{/if}

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

{#snippet vinculada(m: LeisDaMateria, l: LeiNaMateria)}
	<div class="lei">
		<span class="ic"><NavIcon name="lei" size="sm" /></span>
		<a class="nome" href="/leis/{l.slug}">{l.curto}</a>
		<span class="recorte">{l.recorte.length === 0 ? 'a lei inteira' : descreverRecorte(l.recorte)}</span>
		<span class="meta">{l.questoes} {l.questoes === 1 ? 'questão' : 'questões'}</span>
		<IconButton icon="fechar" label="Desvincular {l.curto}" onclick={() => vincular(m, l.slug, false)} />
	</div>
{/snippet}

{#snippet sugerida(m: LeisDaMateria, l: LeiNaMateria)}
	<div class="lei sugerida">
		<span class="ic"><NavIcon name="lei" size="sm" /></span>
		<span class="nome">{l.curto}</span>
		<span class="recorte">
			No catálogo · o edital pede {l.recorte.length === 0 ? 'a lei inteira' : descreverRecorte(l.recorte)}
		</span>
		<button class="vincular" type="button" aria-label="Vincular {l.curto}" onclick={() => vincular(m, l.slug, true)}>
			Vincular
		</button>
	</div>
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
	.mat-cabeca {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
	}
	.vincular-todas {
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		padding: 4px 11px;
		border-radius: 6px;
		border: 1px solid var(--border-strong);
		background: var(--bg-card);
		color: var(--text);
		cursor: pointer;
	}
	.vincular-todas:hover {
		background: var(--bg-hover);
	}
	/* Um tópico do edital por linha, e embaixo dele a lei que o responde. */
	.temas {
		list-style: none;
		margin: 0 0 8px;
		padding: 0;
	}
	.tema {
		padding: 10px 0 12px;
		border-bottom: 1px solid var(--border);
	}
	.texto-tema {
		margin: 0 0 6px;
		font-size: 13.5px;
		line-height: 1.5;
		color: var(--text-muted);
	}
	.lei {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 5px 8px;
		margin-left: -8px;
		border-radius: 6px;
		font-size: 14px;
	}
	.lei:hover {
		background: var(--bg-hover);
	}
	.lei .ic {
		color: var(--text-muted);
		display: inline-flex;
	}
	.lei.sugerida .ic,
	.lei.sugerida .nome {
		color: var(--text-muted);
	}
	.nome {
		flex: none;
		color: var(--text);
		font-weight: 600;
		text-decoration: none;
	}
	a.nome:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.recorte {
		flex: 1;
		min-width: 0;
		color: var(--text-muted);
		font-size: 12.5px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.meta {
		color: var(--text-faint);
		font-size: 12px;
		white-space: nowrap;
	}
	.rotulo-outras {
		margin: 14px 0 4px;
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
	}
	.pesquisar {
		flex: none;
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		padding: 3px 10px;
		border-radius: 6px;
		border: 1px solid var(--border-strong);
		background: var(--bg-card);
		color: var(--text);
		cursor: pointer;
	}
	.pesquisar:hover {
		background: var(--bg-hover);
	}
	.discreto {
		font: inherit;
		font-size: 12px;
		background: none;
		border: 0;
		padding: 2px 4px;
		color: var(--text-faint);
		cursor: pointer;
	}
	.discreto:hover {
		color: var(--danger);
	}
	.confirmar {
		display: flex;
		gap: 10px;
		align-items: center;
		flex-wrap: wrap;
		padding: 12px 14px;
		margin: 0 0 18px;
		border-radius: 8px;
		border: 1px solid var(--danger);
		background: var(--danger-soft);
		font-size: 13.5px;
	}
	.confirmar p {
		flex: 1 1 100%;
		margin: 0;
	}
	.vincular {
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		padding: 3px 10px;
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
