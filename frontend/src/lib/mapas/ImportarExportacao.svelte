<script lang="ts">
	import { api } from '$lib/api';
	import { lerZip, pacotesDoZip } from '$lib/mapas/pacote';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import type { PacoteImportado } from '$lib/types';

	/**
	 * O lugar de trazer de volta o .zip exportado (um mapa ou todos): a tela
	 * abre o .zip e manda mapa a mapa, cada um com o texto, as questões, as
	 * imagens, os vínculos com os tópicos e as respostas. Um mapa com problema
	 * não para os outros: no fim, a lista diz o que voltou e o que não.
	 */
	let { aoTerminar }: { aoTerminar: () => void } = $props();

	type Linha = { nome: string; ok: PacoteImportado } | { nome: string; erro: string };

	let andamento = $state<{ feitos: number; total: number; atual: string } | null>(null);
	let linhas = $state<Linha[]>([]);
	let erro = $state<string | null>(null);
	let aberto = $state(false);

	const comErro = $derived(linhas.filter((l) => 'erro' in l).length);
	const nf = new Intl.NumberFormat('pt-BR');
	const contar = (n: number, um: string, varios: string) => `${nf.format(n)} ${n === 1 ? um : varios}`;

	async function aoEscolher(e: Event & { currentTarget: HTMLInputElement }) {
		const arquivo = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (arquivo) await importar(arquivo);
	}

	/** Também chamado pelo painel de importar o texto, quando nele se escolhe um .zip. */
	export async function importar(arquivo: File) {
		aberto = true;
		erro = null;
		linhas = [];
		let pacotes;
		try {
			pacotes = pacotesDoZip(await lerZip(await arquivo.arrayBuffer())).pacotes;
		} catch (e) {
			erro = `${arquivo.name}: ${e instanceof Error ? e.message : 'não foi possível abrir o .zip'}`;
			return;
		}
		if (pacotes.length === 0) {
			erro = `${arquivo.name} não tem nenhum mapa (.md).`;
			return;
		}

		for (const [i, p] of pacotes.entries()) {
			andamento = { feitos: i, total: pacotes.length, atual: p.nome };
			try {
				linhas.push({ nome: p.nome, ok: await api.importarPacote(p, concursoStore.ativoSlug) });
			} catch (e) {
				linhas.push({ nome: p.nome, erro: e instanceof Error ? e.message : 'a importação falhou' });
			}
		}
		andamento = null;
		aoTerminar();
	}
</script>

<details class="importar-zip" bind:open={aberto}>
	<summary>Importar uma exportação (.zip)</summary>
	<div class="painel">
		<p class="ajuda">
			O .zip de <b>Exportar mapa</b> ou de <b>Exportar todos os mapas</b> volta inteiro por aqui: o texto, as questões, as
			imagens, os vínculos com as matérias e os tópicos, e as suas respostas. O mapa que já existe é atualizado, e
			importar o mesmo .zip de novo não repete nada. O vínculo de um concurso que esta conta não tem vai para o concurso
			aberto.
		</p>
		<label class="arquivo">
			<span>Arquivo exportado (.zip)</span>
			<input
				type="file"
				accept=".zip,application/zip"
				aria-label="Arquivo exportado (.zip)"
				disabled={andamento !== null}
				onchange={aoEscolher}
			/>
		</label>

		{#if andamento}
			<p class="andamento" role="status">
				Importando {andamento.feitos + 1} de {andamento.total}: <code>{andamento.atual}</code>…
				<progress max={andamento.total} value={andamento.feitos}></progress>
			</p>
		{/if}
		{#if erro}<div class="form-error" role="alert">{erro}</div>{/if}

		{#if linhas.length > 0 && !andamento}
			<p class="resumo" role="status">
				{linhas.length - comErro} de {linhas.length}
				{linhas.length === 1 ? 'mapa importado' : 'mapas importados'}{comErro > 0 ? `; ${comErro} com problema` : ''}.
			</p>
			<ul class="linhas" aria-label="Mapas importados">
				{#each linhas as l (l.nome)}
					<li class:falhou={'erro' in l}>
						{#if 'erro' in l}
							<b>{l.nome}</b>: <span role="alert">{l.erro}</span>
						{:else}
							<a href="/mapas/{l.ok.mapa.slug}"><b>{l.ok.mapa.titulo}</b></a>
							— {contar(l.ok.mapa.itens, 'item', 'itens')}, {contar(l.ok.questoes, 'questão', 'questões')}, {contar(
								l.ok.imagens,
								'imagem',
								'imagens'
							)}, {contar(l.ok.respostas, 'resposta', 'respostas')}{#if l.ok.vinculadas.length > 0}; vinculado a {l.ok.vinculadas
									.map((v) => `${v.codigo} — ${v.nome}`)
									.join(', ')}{/if}.
							{#each l.ok.avisos as a (a)}<span class="aviso">{a}</span>{/each}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</details>

<style>
	.importar-zip {
		margin-bottom: 8px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--bg-card);
	}
	.importar-zip summary {
		padding: 11px 14px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
	}
	.importar-zip summary:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
		border-radius: 10px;
	}
	.painel {
		display: grid;
		gap: 12px;
		padding: 0 14px 14px;
	}
	.ajuda {
		margin: 0;
		font-size: 13px;
		color: var(--text-muted);
	}
	.arquivo {
		display: grid;
		gap: 5px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	.andamento {
		display: grid;
		gap: 6px;
		margin: 0;
		font-size: 13px;
	}
	.andamento progress {
		width: 100%;
	}
	.resumo {
		margin: 0;
		font-weight: 600;
		font-size: 13.5px;
	}
	.linhas {
		display: grid;
		gap: 6px;
		margin: 0;
		padding-left: 18px;
		font-size: 13px;
	}
	.linhas .falhou {
		color: var(--danger, #b42318);
	}
	.aviso {
		display: block;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	code {
		font-family: var(--font-mono);
		font-size: 12px;
	}
</style>
