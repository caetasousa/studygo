<script lang="ts">
	import { confirmacao } from '$lib/stores/confirmacao.svelte';

	/**
	 * O diálogo de "tem certeza?", montado uma vez no layout.
	 *
	 * Mesma casca dos outros diálogos do app (AtividadeForm, RevisaoForm): fundo
	 * escurecido, Escape e clique fora cancelam, e o Tab não sai de dentro
	 * enquanto ele está aberto.
	 *
	 * O botão que confirma é nomeado pela ação — "Excluir", "Importar" — porque
	 * "OK" não diz o que vai acontecer.
	 */
	const pedido = $derived(confirmacao.pedido);

	let painel = $state<HTMLDivElement | null>(null);
	let confirmarBtn = $state<HTMLButtonElement | null>(null);

	// O foco entra no diálogo assim que ele abre, e no botão que confirma: é o
	// que permite responder no teclado sem procurar.
	$effect(() => {
		if (pedido) confirmarBtn?.focus();
	});

	function prender(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			confirmacao.responder(false);

			return;
		}

		if (e.key !== 'Tab' || !painel) return;

		const foco = painel.querySelectorAll<HTMLElement>('button:not(:disabled)');
		if (foco.length === 0) return;

		const primeiro = foco[0];
		const ultimo = foco[foco.length - 1];

		if (e.shiftKey && document.activeElement === primeiro) {
			e.preventDefault();
			ultimo.focus();
		} else if (!e.shiftKey && document.activeElement === ultimo) {
			e.preventDefault();
			primeiro.focus();
		}
	}
</script>

{#if pedido}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dlg-fundo"
		role="presentation"
		onclick={(e) => {
			if (e.target === e.currentTarget) confirmacao.responder(false);
		}}
	>
		<div
			class="dlg"
			role="alertdialog"
			aria-modal="true"
			aria-labelledby="confirmacao-titulo"
			aria-describedby="confirmacao-texto"
			tabindex="-1"
			bind:this={painel}
			onkeydown={prender}
		>
			<h2 id="confirmacao-titulo" class="sec" style="margin-top:0">{pedido.titulo}</h2>
			<p id="confirmacao-texto" class="texto">{pedido.texto}</p>

			<div class="dlg-acoes">
				<button type="button" class="btn" onclick={() => confirmacao.responder(false)}>
					Cancelar
				</button>
				<button
					type="button"
					class="btn"
					class:danger={pedido.tom === 'perigo'}
					class:primario={pedido.tom !== 'perigo'}
					bind:this={confirmarBtn}
					onclick={() => confirmacao.responder(true)}
				>
					{pedido.rotulo ?? 'Confirmar'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Mesma casca dos diálogos de registro — os estilos são escopados por
	   componente, então a forma é repetida em vez de compartilhada, o mesmo
	   tradeoff já feito lá. */
	.dlg-fundo {
		position: fixed;
		inset: 0;
		background: rgba(20, 18, 14, 0.45);
		display: grid;
		place-items: center;
		padding: 20px;
		z-index: 80;
	}
	.dlg {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 20px;
		width: min(420px, 100%);
		box-shadow: var(--shadow-pop);
	}
	.texto {
		margin: 10px 0 0;
		font-size: 13.5px;
		line-height: 1.55;
		color: var(--text-muted);
	}
	.dlg-acoes {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 20px;
	}
	.btn.primario {
		border-color: var(--accent);
		color: var(--accent);
	}
	.btn.primario:hover:not(:disabled) {
		background: var(--accent-soft);
	}
</style>
