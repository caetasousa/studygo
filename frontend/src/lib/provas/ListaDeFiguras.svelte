<script lang="ts" module>
	import type { Bloco } from './types';

	export interface Figura {
		/** O destino do recorte: "q:1", "a:2:0", "p:r0-t1:3". */
		chave: string;
		/** "Enunciado · figura 1": o mesmo número do [figura 1] no campo. */
		onde: string;
		bloco: Bloco;
	}

	/** As figuras de um campo, com o destino do recorte de cada uma. */
	export function figurasDoCampo(blocos: Bloco[], prefixo: string, campo: string): Figura[] {
		const out: Figura[] = [];
		blocos.forEach((bloco, j) => {
			if (bloco.tipo === 'imagem')
				out.push({ chave: `${prefixo}:${j}`, onde: `${campo} · figura ${out.length + 1}`, bloco });
		});
		return out;
	}
</script>

<script lang="ts">
	import Imagem from './Imagem.svelte';

	let {
		figuras,
		onajustarFigura,
		onalterar
	}: {
		figuras: Figura[];
		/** Abre o recorte da figura no painel do original. */
		onajustarFigura: (chave: string) => void;
		onalterar: () => void;
	} = $props();
</script>

<section class="figuras">
	<h3>Figuras</h3>
	{#each figuras as f (f.chave)}
		<div class="figura">
			<div class="miniatura">
				{#if f.bloco.arquivo}
					<Imagem id={f.bloco.arquivo} alt={f.bloco.descricao || 'Figura'} />
				{:else}
					<span class="sem">sem recorte</span>
				{/if}
			</div>
			<div class="controles">
				<span class="onde">{f.onde}</span>
				<button class="btn" type="button" onclick={() => onajustarFigura(f.chave)}>
					{f.bloco.arquivo ? 'Ajustar recorte' : 'Recortar no original'}
				</button>
				{#if f.bloco.arquivo}
					<label class="tamanho">
						Tamanho
						<input
							type="range"
							min="10"
							max="100"
							step="5"
							value={f.bloco.largura || 100}
							oninput={(e) => {
								f.bloco.largura = Number(e.currentTarget.value);
								onalterar();
							}}
						/>
						<span>{f.bloco.largura ? `${f.bloco.largura}%` : '—'}</span>
						{#if f.bloco.largura}
							<button
								type="button"
								class="automatico"
								title="Voltar ao tamanho que a figura tem no caderno"
								onclick={() => {
									f.bloco.largura = 0;
									onalterar();
								}}>automático</button
							>
						{:else}
							<span class="dim">automático: ao salvar, fica na proporção do caderno</span>
						{/if}
					</label>
					<label class="opcao">
						<input
							type="checkbox"
							class="checkbox"
							checked={f.bloco.revisado}
							onchange={(e) => {
								f.bloco.revisado = e.currentTarget.checked;
								onalterar();
							}}
						/>
						Conferi o recorte
					</label>
				{/if}
			</div>
		</div>
	{/each}
</section>

<style>
	.figuras {
		display: grid;
		gap: 8px;
	}
	h3 {
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
		margin: 6px 0 0;
	}
	.figura {
		display: grid;
		grid-template-columns: 120px minmax(0, 1fr);
		gap: 12px;
		align-items: center;
		padding: 8px;
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.miniatura {
		width: 120px;
	}
	.sem {
		font-size: 12px;
		color: var(--warn);
	}
	.controles {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 12px;
		align-items: center;
	}
	.onde {
		font-size: 12px;
		color: var(--text-muted);
		width: 100%;
	}
	.tamanho {
		display: flex;
		gap: 6px;
		align-items: center;
		font-size: 12px;
		color: var(--text-muted);
	}
	.tamanho span {
		font-family: var(--font-mono);
		min-width: 4ch;
	}
	.tamanho .dim {
		font-family: inherit;
		color: var(--text-faint);
	}
	.automatico {
		font: inherit;
		font-size: 12px;
		color: var(--accent);
		background: none;
		border: 0;
		padding: 0;
		text-decoration: underline;
		cursor: pointer;
	}
	.opcao {
		display: flex;
		gap: 8px;
		align-items: center;
		font-size: 13px;
	}
</style>
