<script lang="ts">
	import Imagem from './Imagem.svelte';

	let {
		arquivo,
		descricao,
		largura = $bindable(),
		onredimensionar
	}: {
		arquivo: string;
		descricao: string;
		/** % da coluna do conteúdo; 0 é o tamanho natural. */
		largura: number;
		onredimensionar: () => void;
	} = $props();

	const MINIMO = 10;

	let caixa = $state<HTMLDivElement | null>(null);
	let arraste: { x: number; largura: number; lado: 1 | -1 } | null = null;

	const atual = $derived(largura || 100);

	// Como no Notion: as alças crescem a figura para os dois lados, centrada.
	function comecar(e: PointerEvent, lado: 1 | -1) {
		if (!caixa) return;
		e.preventDefault();
		arraste = { x: e.clientX, largura: (atual / 100) * caixa.clientWidth, lado };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}

	function arrastar(e: PointerEvent) {
		if (!arraste || !caixa) return;
		const px = arraste.largura + 2 * (e.clientX - arraste.x) * arraste.lado;
		largura = Math.round(Math.min(100, Math.max(MINIMO, (100 * px) / caixa.clientWidth)));
	}

	function soltar() {
		if (arraste) onredimensionar();
		arraste = null;
	}
</script>

<div class="caixa" bind:this={caixa}>
	<div class="figura" style:width="{atual}%">
		<Imagem id={arquivo} alt={descricao || 'Figura da questão'} />
		{#each [-1, 1] as const as lado (lado)}
			<span
				class="alca"
				class:esquerda={lado === -1}
				aria-hidden="true"
				onpointerdown={(e) => comecar(e, lado)}
				onpointermove={arrastar}
				onpointerup={soltar}
				onpointercancel={soltar}
			></span>
		{/each}
	</div>
</div>
<label class="tamanho">
	Tamanho na tela
	<input
		type="range"
		min={MINIMO}
		max="100"
		step="5"
		value={atual}
		oninput={(e) => (largura = Number(e.currentTarget.value))}
		onchange={onredimensionar}
	/>
	<span>{atual}%</span>
</label>

<style>
	.caixa {
		width: 100%;
	}
	.figura {
		position: relative;
		margin: 0 auto;
		max-width: 100%;
	}
	.alca {
		position: absolute;
		top: 50%;
		right: -7px;
		width: 6px;
		height: 44px;
		transform: translateY(-50%);
		border-radius: 4px;
		background: var(--accent);
		border: 1px solid var(--bg-card);
		cursor: ew-resize;
		opacity: 0;
		transition: opacity 0.15s;
		touch-action: none;
	}
	.alca.esquerda {
		right: auto;
		left: -7px;
	}
	.figura:hover .alca,
	.alca:active {
		opacity: 1;
	}
	.tamanho {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.tamanho span {
		font-family: var(--font-mono);
		min-width: 3.5ch;
	}
</style>
