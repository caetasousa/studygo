<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import CapturaDeLei from '$lib/components/CapturaDeLei.svelte';
	import { nomeLegivel } from '$lib/leis';
	import type { PesquisaDoTema, PublicacaoDeLei } from '$lib/types';

	/**
	 * Importar a lei a partir do tópico do edital.
	 *
	 * Primeiro a pesquisa: acha a fonte oficial pelo tópico e mostra o que cada
	 * assunto dele pede da lei ("Administração Pública" → Capítulo VII), sem
	 * importar nada. Quem estuda confere, ajusta e só então importa — só o que
	 * ficou marcado, e a matéria já sai vinculada com esse recorte.
	 *
	 * Se a lei já está no catálogo, importar é ampliar a que existe (ou só
	 * vincular, quando ela já tem tudo): nunca uma segunda cópia.
	 */
	interface Props {
		tema: string;
		concurso: string;
		disciplinaId: string;
		aoTerminar: (mensagem: string) => void;
	}

	let { tema, concurso, disciplinaId, aoTerminar }: Props = $props();

	let pesquisa = $state<PesquisaDoTema | null>(null);
	let pesquisando = $state(true);
	let semFonte = $state(false);
	let link = $state('');
	let erro = $state<string | null>(null);
	let escolhidos = $state<string[]>([]);
	let artigos = $state('');
	let nome = $state('');
	let curto = $state('');
	let importando = $state(false);
	let vinculando = $state(false);

	const divisoes = $derived(
		(pesquisa?.estrutura ?? []).filter((d) => d.tipo !== 'artigo' && d.tipo !== 'preambulo' && d.nome)
	);
	const inteira = $derived(!!pesquisa && pesquisa.recorte.length === 0 && escolhidos.length === 0 && !artigos.trim());
	const acao = $derived(artigos.trim() && pesquisa?.acao === 'vincular' ? 'ampliar' : (pesquisa?.acao ?? 'importar'));

	async function pesquisar(comLink = '') {
		pesquisando = true;
		erro = null;
		try {
			pesquisa = await api.pesquisarTema(tema, comLink);
			semFonte = false;
			escolhidos = [...pesquisa.recorte];
			nome = nomeLegivel(pesquisa.nome);
			curto = pesquisa.curto;
		} catch (e) {
			// 404: o tópico não diz com certeza que norma é — pede o link.
			if (e instanceof ApiError && e.status === 404) semFonte = true;
			else erro = e instanceof Error ? e.message : 'A pesquisa falhou';
		} finally {
			pesquisando = false;
		}
	}

	onMount(() => void pesquisar());

	function marcar(ref: string, sim: boolean) {
		escolhidos = sim ? [...escolhidos, ref] : escolhidos.filter((r) => r !== ref);
	}

	async function vincular() {
		if (!pesquisa?.existente) return;
		vinculando = true;
		erro = null;
		try {
			// Soma ao que a matéria já cobra desta lei: é outro tópico dela.
			await api.vincularLei(concurso, disciplinaId, pesquisa.existente.slug, true, escolhidos, artigos, true);
			aoTerminar(`${pesquisa.existente.curto} vinculada a esta matéria.`);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'Não foi possível vincular';
		} finally {
			vinculando = false;
		}
	}

	function publicada(r: PublicacaoDeLei) {
		aoTerminar(`${r.curto} ${acao === 'ampliar' ? 'ampliada' : 'importada'} e vinculada a esta matéria.`);
	}
</script>

<div class="importar" role="region" aria-label="Importar do tópico">
	{#if pesquisando}
		<p class="andamento" role="status">Pesquisando a fonte e o que o tópico pede…</p>
	{:else if semFonte}
		<form
			class="sem-fonte"
			onsubmit={(e) => {
				e.preventDefault();
				void pesquisar(link.trim());
			}}
		>
			<p>Não achei a fonte oficial desta norma pelo tópico. Cole o link dela:</p>
			<div class="linha">
				<label class="sr-only" for="link-tema">Link da norma na fonte oficial</label>
				<input id="link-tema" type="text" inputmode="url" bind:value={link} placeholder="https://…gov.br/…" />
				<button class="btn" type="submit" disabled={!link.trim()}>Pesquisar</button>
			</div>
		</form>
	{:else if pesquisa && !importando}
		<p class="fonte">
			<b>{nomeLegivel(pesquisa.epigrafe || pesquisa.nome)}</b> ·
			<a href={pesquisa.fonte} target="_blank" rel="noopener noreferrer">fonte oficial ↗</a>
			{#if pesquisa.existente}· já no catálogo como <a href="/leis/{pesquisa.existente.slug}">{pesquisa.existente.curto}</a>{/if}
		</p>

		{#if pesquisa.recorte.length === 0 && pesquisa.assuntos.length === 0}
			<p class="assunto-vazio">O tópico cita a norma inteira: ela entra toda.</p>
		{:else}
			<ul class="assuntos" aria-label="O que cada assunto pede">
				{#each pesquisa.assuntos as a (a.texto)}
					<li>
						<q>{a.texto}</q>
						{#if a.trechos.length === 0}
							<span class="sem-titulo">
								não é título de nenhuma divisão — confira: costuma estar dentro do que já foi marcado; se não
								estiver, acrescente os artigos abaixo
							</span>
						{:else}
							{#each a.trechos as t (t.ref)}
								<label class="trecho">
									<input
										type="checkbox"
										checked={escolhidos.includes(t.ref)}
										onchange={(e) => marcar(t.ref, e.currentTarget.checked)}
									/>
									{t.rotulo}{t.nome ? ` — ${nomeLegivel(t.nome)}` : ''}{t.artigos ? ` (${t.artigos})` : ''}
								</label>
							{/each}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

		<div class="mais">
			{#if divisoes.length > 0}
				<label>
					<span class="sr-only">Acrescentar uma divisão da lei</span>
					<select
						onchange={(e) => {
							const ref = e.currentTarget.value;
							e.currentTarget.value = '';
							if (ref && !escolhidos.includes(ref)) escolhidos = [...escolhidos, ref];
						}}
					>
						<option value="">+ Acrescentar uma divisão…</option>
						{#each divisoes.filter((d) => !escolhidos.includes(d.ref)) as d (d.ref)}
							<option value={d.ref}>{d.rotulo} — {nomeLegivel(d.nome)}</option>
						{/each}
					</select>
				</label>
			{/if}
			<label class="avulsos">
				<span>Acrescentar artigos</span>
				<input type="text" bind:value={artigos} placeholder="ex.: 74, 75 ou 37 a 41" />
			</label>
			{#if escolhidos.some((r) => !pesquisa?.assuntos.some((a) => a.refs.includes(r)))}
				<p class="extras">
					Acrescentado: {escolhidos
						.filter((r) => !pesquisa?.assuntos.some((a) => a.refs.includes(r)))
						.map((r) => pesquisa?.estrutura.find((d) => d.ref === r)?.rotulo ?? r)
						.join(', ')}
				</p>
			{/if}
		</div>

		{#if acao === 'importar'}
			<div class="form-grid campos">
				<div class="field largo">
					<label for="nome-tema">Nome da lei</label>
					<input id="nome-tema" type="text" bind:value={nome} />
				</div>
				<div class="field">
					<label for="curto-tema">Nome curto</label>
					<input id="curto-tema" type="text" bind:value={curto} />
				</div>
			</div>
		{/if}

		<div class="acoes">
			{#if acao === 'vincular'}
				<button class="btn primary" type="button" disabled={vinculando || escolhidos.length === 0} onclick={vincular}>
					Vincular a esta matéria
				</button>
			{:else}
				<button
					class="btn primary"
					type="button"
					disabled={acao === 'importar' && (!nome.trim() || !curto.trim())}
					onclick={() => (importando = true)}
				>
					{acao === 'ampliar' ? `Ampliar ${pesquisa.existente?.curto} e vincular` : inteira ? 'Importar a lei inteira' : 'Importar só isso'}
				</button>
			{/if}
		</div>
	{:else if pesquisa && importando}
		<CapturaDeLei
			inicio={{
				link: pesquisa.link,
				recorte: escolhidos,
				artigos,
				slug: pesquisa.existente?.slug,
				inteira,
				nome,
				curto
			}}
			vincularA={{ concurso, disciplinaId, recorte: escolhidos, artigos, somar: true }}
			autoPublicar
			aoPublicar={publicada}
		/>
	{/if}
	{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}
</div>

<style>
	.importar {
		margin: 8px 0 14px 18px;
		padding: 12px 14px;
		border-radius: 8px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		font-size: 13.5px;
	}
	.andamento,
	.assunto-vazio {
		margin: 0;
		color: var(--text-muted);
	}
	.fonte {
		margin: 0 0 10px;
	}
	.fonte a {
		color: var(--text-muted);
	}
	.assuntos {
		list-style: none;
		margin: 0 0 10px;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.assuntos q {
		display: block;
		color: var(--text-muted);
		font-size: 12.5px;
		margin-bottom: 2px;
	}
	.trecho {
		display: flex;
		gap: 8px;
		align-items: baseline;
		padding: 2px 0;
	}
	.sem-titulo {
		color: var(--warn);
		font-size: 12.5px;
	}
	.mais {
		display: flex;
		gap: 14px;
		flex-wrap: wrap;
		align-items: flex-end;
		margin: 6px 0 10px;
	}
	.mais select {
		font: inherit;
		font-size: 13px;
		background: none;
		color: var(--text-muted);
		border: 0;
		padding: 4px 0;
		cursor: pointer;
	}
	.avulsos {
		display: flex;
		flex-direction: column;
		gap: 3px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.extras {
		width: 100%;
		margin: 0;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.campos {
		margin: 6px 0 10px;
	}
	.largo {
		flex: 1;
		min-width: min(100%, 280px);
	}
	.largo input {
		width: 100%;
	}
	.sem-fonte p {
		margin: 0 0 8px;
	}
	.linha {
		display: flex;
		gap: 8px;
	}
	.linha input {
		flex: 1;
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
