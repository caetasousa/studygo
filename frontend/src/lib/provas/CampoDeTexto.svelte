<script lang="ts">
	import { untrack } from 'svelte';
	import { blocosParaTexto, envolver, textoParaBlocos } from './marcacao';
	import { marcarComoCodigo } from './texto';
	import type { Bloco } from './types';

	let {
		blocos = $bindable(),
		rotulo,
		linhas = 2,
		onalterar,
		onfoco
	}: {
		blocos: Bloco[];
		/** Nome do campo para leitores de tela: "Enunciado", "Alternativa C". */
		rotulo: string;
		linhas?: number;
		onalterar: () => void;
		/** O campo ganhou o foco: a barra de destaque passa a agir nele. */
		onfoco?: (marcar: (marca: string) => void) => void;
	} = $props();

	let texto = $state(untrack(() => blocosParaTexto(blocos)));
	let campo = $state<HTMLTextAreaElement | null>(null);

	// Mudança de fora (um recorte aplicado no painel do original) reescreve o
	// campo. A do próprio campo não: o que o curador digitou continua como ele
	// digitou, e o cursor não pula a cada tecla.
	$effect(() => {
		const atual = blocosParaTexto(blocos);
		const meu = untrack(() => blocosParaTexto(textoParaBlocos(texto, blocos)));
		if (atual !== meu) texto = atual;
	});

	// O campo cresce com o texto, contando as linhas que quebram por largura;
	// contar só os "\n" deixava alternativa longa cortada dentro da caixa.
	$effect(() => {
		void texto;
		if (!campo) return;
		campo.style.height = 'auto';
		campo.style.height = `${Math.min(campo.scrollHeight + 2, window.innerHeight * 0.7)}px`;
	});

	function aplicar(novo: string) {
		texto = novo;
		blocos = textoParaBlocos(novo, blocos);
		onalterar();
	}

	/** "**", "*", "__" ou "codigo", em volta da seleção. */
	function marcar(marca: string) {
		if (!campo) return;
		const { selectionStart: inicio, selectionEnd: fim } = campo;
		const r = marca === 'codigo' ? marcarComoCodigo(texto, inicio, fim) : envolver(texto, inicio, fim, marca);
		aplicar(r.texto);
		requestAnimationFrame(() => {
			campo?.focus();
			campo?.setSelectionRange(r.cursor, r.cursor);
		});
	}

	const ATALHOS: Record<string, string> = { b: '**', i: '*', u: '__', e: 'codigo' };
</script>

<textarea
	bind:this={campo}
	aria-label={rotulo}
	rows={linhas}
	value={texto}
	oninput={(e) => aplicar(e.currentTarget.value)}
	onfocus={() => onfoco?.(marcar)}
	onkeydown={(e) => {
		const marca = (e.ctrlKey || e.metaKey) && ATALHOS[e.key.toLowerCase()];
		if (marca) {
			e.preventDefault();
			marcar(marca);
		}
	}}
></textarea>

<style>
	textarea {
		width: 100%;
		box-sizing: border-box;
		resize: vertical;
		font: inherit;
		font-size: 14px;
		line-height: 1.55;
	}
</style>
