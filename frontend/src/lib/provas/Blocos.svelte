<script lang="ts">
	import BlocoDeCodigo from './BlocoDeCodigo.svelte';
	import Imagem from './Imagem.svelte';
	import { partesDoTexto, separador, trechosEmLinha } from './texto';
	import type { Bloco } from './types';

	let {
		blocos,
		interativo = true
	}: {
		blocos: Bloco[];
		/** Falso dentro de um botão (a alternativa na resolução): sem botão de copiar. */
		interativo?: boolean;
	} = $props();

	type Peca =
		| { tipo: 'texto'; blocos: Bloco[] }
		| { tipo: 'codigo'; texto: string; linguagem?: string }
		| { tipo: 'imagem'; bloco: Bloco };

	/**
	 * Blocos de texto seguidos formam um parágrafo só: é assim que a extração
	 * marca um destaque no meio da frase ("não lhes sobra tempo" + o trecho
	 * sublinhado + "."). A quebra de parágrafo vem do "\n" dentro do texto ou,
	 * quando a extração não o pôs, de separador(). Código — o bloco do tipo
	 * código ou o ``` escrito no texto — e figura interrompem o parágrafo.
	 */
	const pecas = $derived.by(() => {
		const out: Peca[] = [];
		const emTexto = (b: Bloco) => {
			const ultima = out.at(-1);
			if (ultima?.tipo === 'texto') ultima.blocos.push(b);
			else out.push({ tipo: 'texto', blocos: [b] });
		};
		for (const b of blocos) {
			if (b.tipo === 'imagem') out.push({ tipo: 'imagem', bloco: b });
			else if (b.tipo === 'codigo') out.push({ tipo: 'codigo', texto: b.texto });
			else
				for (const parte of partesDoTexto(b.texto)) {
					if (parte.codigo) out.push({ tipo: 'codigo', texto: parte.texto, linguagem: parte.linguagem });
					else emTexto({ ...b, texto: parte.texto });
				}
		}
		return out;
	});
</script>

{#each pecas as peca, i (i)}
	{#if peca.tipo === 'imagem'}
		{#if peca.bloco.arquivo}
			<!-- A largura vem do servidor, na proporção que a figura tem no caderno. -->
			<figure style:width={peca.bloco.largura ? `${peca.bloco.largura}%` : null}>
				<Imagem id={peca.bloco.arquivo} alt={peca.bloco.descricao || 'Figura da questão'} />
			</figure>
		{:else}
			<p class="sem-recorte">Figura sem recorte.</p>
		{/if}
	{:else if peca.tipo === 'codigo'}
		<BlocoDeCodigo texto={peca.texto} linguagem={peca.linguagem} copiavel={interativo} />
	{:else}
		<p>
			{#each peca.blocos as t, j (j)}{j > 0
					? separador(peca.blocos[j - 1].texto, t.texto, peca.blocos[j - 1].formato !== t.formato)
					: ''}<span
					class:negrito={t.formato === 'negrito'}
					class:italico={t.formato === 'italico'}
					class:sublinhado={t.formato === 'sublinhado'}
					>{#each trechosEmLinha(t.texto) as trecho, k (k)}{#if trecho.codigo}<code>{trecho.texto}</code
							>{:else}{trecho.texto}{/if}{/each}</span
				>{/each}
		</p>
	{/if}
{/each}

<style>
	p {
		margin: 0 0 10px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		line-height: 1.55;
	}
	code {
		font-family: var(--font-mono);
		font-size: 0.88em;
		padding: 0.1em 0.35em;
		border-radius: 4px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		color: var(--danger);
	}
	figure {
		margin: 6px auto 12px;
		max-width: 100%;
	}
	/* Figura alta não passa da altura da tela: a questão continua à vista. */
	figure :global(img) {
		width: auto;
		max-height: 60vh;
		margin: 0 auto;
	}
	/* Na coluna estreita do celular, a proporção do caderno deixa o gráfico
	   ilegível: a figura pode ocupar a largura toda. */
	@media (max-width: 560px) {
		figure {
			width: auto !important;
		}
	}
	.negrito {
		font-weight: 700;
	}
	.italico {
		font-style: italic;
	}
	.sublinhado {
		text-decoration: underline;
	}
	.sem-recorte {
		color: var(--warn);
		font-size: 12.5px;
	}
</style>
