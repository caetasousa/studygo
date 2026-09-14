<script lang="ts">
	import { untrack } from 'svelte';
	import { fetchAutenticado } from '$lib/api';
	import { provasApi } from './api';
	import type { Origem } from './types';

	let {
		importacao,
		versao,
		regiao,
		inicial,
		onaplicar
	}: {
		importacao: string;
		versao: number;
		/** A região mostrada; o retângulo fica sempre dentro dela. */
		regiao: Origem;
		/** Retângulo com que o seletor começa, em pontos do PDF. */
		inicial: number[];
		onaplicar: (arquivo: string, origem: Origem) => void;
	} = $props();

	let url = $state('');
	let erro = $state('');
	let ocupado = $state(false);
	let zoom = $state(100);
	let rect = $state<number[]>([0, 0, 0, 0]);
	let area = $state<HTMLDivElement | null>(null);

	// A prévia é o recorte da região inteira. O id é derivado do retângulo, então
	// voltar à região devolve o mesmo arquivo em vez de gravar outro. A versão é
	// lida sem rastrear: salvar a revisão muda a versão e não pede outra prévia.
	$effect(() => {
		const r = regiao;
		const v = untrack(() => versao);
		let ativo = true;
		let blob = '';
		url = '';
		erro = '';

		provasApi
			.recortar(importacao, v, r)
			.then(async ({ arquivo }) => {
				const resp = await fetchAutenticado(`/api/provas/arquivos/${arquivo}`);
				if (!resp.ok) throw new Error('prévia da região indisponível');
				const conteudo = await resp.blob();
				if (!ativo) return;
				blob = URL.createObjectURL(conteudo);
				url = blob;
			})
			.catch((e: Error) => {
				if (ativo) erro = e.message;
			});

		return () => {
			ativo = false;
			if (blob) URL.revokeObjectURL(blob);
		};
	});

	$effect(() => {
		rect = [...inicial];
	});

	const limite = $derived(regiao.retangulo);
	const largura = $derived(limite[2] - limite[0]);
	const altura = $derived(limite[3] - limite[1]);

	const estilo = $derived(
		`left:${(100 * (rect[0] - limite[0])) / largura}%;` +
			`top:${(100 * (rect[1] - limite[1])) / altura}%;` +
			`width:${(100 * (rect[2] - rect[0])) / largura}%;` +
			`height:${(100 * (rect[3] - rect[1])) / altura}%`
	);

	type Modo = 'desenhar' | 'mover' | 'redimensionar';
	let arraste: { modo: Modo; inicio: number[]; antes: number[] } | null = null;

	const limitar = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v));

	/** Posição do ponteiro em pontos do PDF, presa à região. */
	function ponto(e: PointerEvent): number[] {
		const caixa = area!.getBoundingClientRect();
		return [
			limite[0] + limitar((e.clientX - caixa.left) / caixa.width, 0, 1) * largura,
			limite[1] + limitar((e.clientY - caixa.top) / caixa.height, 0, 1) * altura
		];
	}

	function comecar(e: PointerEvent, modo: Modo) {
		if (!area) return;
		e.stopPropagation();
		e.preventDefault();
		arraste = { modo, inicio: ponto(e), antes: [...rect] };
		area.setPointerCapture(e.pointerId);
	}

	function arrastar(e: PointerEvent) {
		if (!arraste) return;
		const [px, py] = ponto(e);
		const [ix, iy] = arraste.inicio;
		const a = arraste.antes;

		if (arraste.modo === 'desenhar') {
			rect = [Math.min(ix, px), Math.min(iy, py), Math.max(ix, px), Math.max(iy, py)];
		} else if (arraste.modo === 'mover') {
			const dx = limitar(px - ix, limite[0] - a[0], limite[2] - a[2]);
			const dy = limitar(py - iy, limite[1] - a[1], limite[3] - a[3]);
			rect = [a[0] + dx, a[1] + dy, a[2] + dx, a[3] + dy];
		} else {
			rect = [a[0], a[1], Math.max(a[0] + 4, px), Math.max(a[1] + 4, py)];
		}
	}

	function soltar() {
		arraste = null;
	}

	/** O retângulo que vai ao servidor: ordenado, dentro da região e com tamanho. */
	function normalizado(): number[] | null {
		const [x0, x1] = [Math.min(rect[0], rect[2]), Math.max(rect[0], rect[2])];
		const [y0, y1] = [Math.min(rect[1], rect[3]), Math.max(rect[1], rect[3])];
		const r = [
			limitar(x0, limite[0], limite[2]),
			limitar(y0, limite[1], limite[3]),
			limitar(x1, limite[0], limite[2]),
			limitar(y1, limite[1], limite[3])
		];
		return r[2] - r[0] >= 4 && r[3] - r[1] >= 4 ? r.map((v) => Math.round(v * 100) / 100) : null;
	}

	async function aplicar() {
		const r = normalizado();
		if (!r) {
			erro = 'o recorte precisa ter tamanho e ficar dentro da região';
			return;
		}
		ocupado = true;
		erro = '';
		try {
			const origem: Origem = { ...regiao, retangulo: r };
			const { arquivo } = await provasApi.recortar(importacao, versao, origem);
			onaplicar(arquivo, origem);
		} catch (e) {
			erro = e instanceof Error ? e.message : 'não foi possível recortar';
		} finally {
			ocupado = false;
		}
	}
</script>

<div class="recorte">
	<div class="controles">
		<label>
			Zoom
			<input type="range" min="60" max="220" bind:value={zoom} />
		</label>
		<span class="dica">Arraste para desenhar; arraste o quadro para mover e o canto para ajustar.</span>
	</div>

	<div class="janela">
		{#if url}
			<!-- O arraste é do mouse; os campos numéricos abaixo fazem o mesmo pelo teclado. -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="area"
				bind:this={area}
				style:width="{zoom}%"
				onpointerdown={(e) => comecar(e, 'desenhar')}
				onpointermove={arrastar}
				onpointerup={soltar}
				onpointercancel={soltar}
			>
				<img src={url} alt="Região original da prova" draggable="false" />
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="quadro" style={estilo} onpointerdown={(e) => comecar(e, 'mover')}>
					<span
						class="canto"
						aria-hidden="true"
						onpointerdown={(e) => comecar(e, 'redimensionar')}
					></span>
				</div>
			</div>
		{:else}
			<p class="espera" role="status">{erro || 'Carregando a região original…'}</p>
		{/if}
	</div>

	<fieldset class="coordenadas">
		<legend>Retângulo, em pontos do PDF</legend>
		{#each ['Esquerda', 'Topo', 'Direita', 'Base'] as nome, i (nome)}
			<label>
				{nome}
				<input type="number" step="1" bind:value={rect[i]} />
			</label>
		{/each}
	</fieldset>

	<button class="btn primary" type="button" disabled={ocupado || !url} onclick={aplicar}>
		{ocupado ? 'Recortando…' : 'Aplicar recorte'}
	</button>
	{#if erro && url}<p class="form-error">{erro}</p>{/if}
</div>

<style>
	.recorte {
		display: grid;
		gap: 10px;
	}
	.controles {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		align-items: center;
		font-size: 12px;
		color: var(--text-muted);
	}
	.dica {
		color: var(--text-faint);
	}
	.janela {
		overflow: auto;
		max-height: 72vh;
		background: #fff;
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.area {
		position: relative;
		touch-action: none;
		line-height: 0;
		cursor: crosshair;
	}
	.area img {
		width: 100%;
		user-select: none;
	}
	.quadro {
		position: absolute;
		box-sizing: border-box;
		border: 2px solid var(--accent);
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		cursor: move;
	}
	.canto {
		position: absolute;
		right: -6px;
		bottom: -6px;
		width: 12px;
		height: 12px;
		background: var(--accent);
		border-radius: 2px;
		cursor: se-resize;
	}
	.coordenadas {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		border: 0;
		padding: 0;
		margin: 0;
	}
	.coordenadas legend {
		font-size: 11px;
		color: var(--text-faint);
		margin-bottom: 4px;
	}
	.coordenadas label {
		display: grid;
		gap: 3px;
		font-size: 11px;
		color: var(--text-muted);
	}
	.espera {
		padding: 16px;
		color: #555;
		font-size: 13px;
		line-height: 1.4;
	}
</style>
