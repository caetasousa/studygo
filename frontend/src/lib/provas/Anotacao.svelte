<script lang="ts">
	import { onDestroy, tick, untrack } from 'svelte';
	import Markdown from './Markdown.svelte';
	import { provasApi } from './api';
	import { envolver } from './marcacao';
	import { alternarTarefa, continuarLista, prefixarLinhas } from './markdown';
	import type { Anotacao } from './types';

	let {
		provaId,
		numero,
		inicial,
		onsalvo
	}: {
		provaId: string;
		numero: number;
		/** O texto que o servidor tinha; vazio se a questão ainda não tem nota. */
		inicial: string;
		onsalvo: (a: Anotacao) => void;
	} = $props();

	let texto = $state(untrack(() => inicial));
	let editando = $state(false);
	let estado = $state<'salvo' | 'pendente' | 'salvando' | 'erro'>('salvo');
	let salvoEm = $state('');
	let campo = $state<HTMLTextAreaElement | null>(null);
	let espera: ReturnType<typeof setTimeout> | null = null;

	// Salva sozinho logo depois de parar de digitar, como o Notion: não há botão
	// de salvar para esquecer.
	function agendar() {
		estado = 'pendente';
		if (espera) clearTimeout(espera);
		espera = setTimeout(() => void salvar(), 700);
	}

	async function salvar() {
		if (espera) clearTimeout(espera);
		espera = null;
		const enviado = texto;
		estado = 'salvando';
		try {
			const a = await provasApi.anotar(provaId, numero, enviado);
			onsalvo(a);
			// Digitou enquanto salvava: a próxima gravação já está agendada.
			if (texto === enviado) {
				estado = 'salvo';
				salvoEm = new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
			}
		} catch {
			estado = 'erro';
		}
	}

	function mudar(novo: string) {
		texto = novo;
		agendar();
	}

	export async function editar() {
		editando = true;
		await tick();
		campo?.focus();
		campo?.setSelectionRange(texto.length, texto.length);
	}

	function concluir() {
		editando = false;
		if (estado === 'pendente') void salvar();
	}

	// Trocar de questão desmonta o bloco: o que ficou por gravar vai agora.
	onDestroy(() => {
		if (espera) void salvar();
	});

	// A altura acompanha o texto, como um bloco de página.
	$effect(() => {
		void texto;
		if (!campo) return;
		campo.style.height = 'auto';
		campo.style.height = `${campo.scrollHeight + 2}px`;
	});

	async function aplicar(r: { texto: string; inicio: number; fim: number }) {
		mudar(r.texto);
		await tick();
		campo?.focus();
		campo?.setSelectionRange(r.inicio, r.fim);
	}

	function destacar(marca: string) {
		if (!campo) return;
		const { selectionStart: i, selectionEnd: f } = campo;
		const r = envolver(texto, i, f, marca);
		void aplicar({ texto: r.texto, inicio: r.cursor, fim: r.cursor });
	}

	function prefixar(prefixo: string) {
		if (!campo) return;
		void aplicar(prefixarLinhas(texto, campo.selectionStart, campo.selectionEnd, prefixo));
	}

	function link() {
		if (!campo) return;
		const { selectionStart: i, selectionEnd: f } = campo;
		const rotulo = texto.slice(i, f) || 'link';
		const novo = `${texto.slice(0, i)}[${rotulo}](https://)${texto.slice(f)}`;
		const url = i + rotulo.length + 3;
		void aplicar({ texto: novo, inicio: url, fim: url + 8 });
	}

	function teclas(e: KeyboardEvent) {
		const ctrl = e.ctrlKey || e.metaKey;
		if (e.key === 'Escape' || (ctrl && e.key === 'Enter')) {
			e.preventDefault();
			concluir();
			return;
		}
		if (ctrl) {
			const atalho: Record<string, () => void> = {
				b: () => destacar('**'),
				i: () => destacar('*'),
				e: () => destacar('`'),
				k: link
			};
			const fazer = atalho[e.key.toLowerCase()];
			if (fazer) {
				e.preventDefault();
				fazer();
			}
			return;
		}
		if (e.key === 'Enter' && !e.shiftKey && campo) {
			const r = continuarLista(texto, campo.selectionStart);
			if (r) {
				e.preventDefault();
				void aplicar({ texto: r.texto, inicio: r.cursor, fim: r.cursor });
			}
		}
	}

	const BARRA = [
		{ rotulo: 'Título', titulo: 'Título (## no começo da linha)', fazer: () => prefixar('## ') },
		{ rotulo: '•', titulo: 'Lista (- no começo da linha)', fazer: () => prefixar('- ') },
		{ rotulo: '☐', titulo: 'Tarefa (- [ ] no começo da linha)', fazer: () => prefixar('- [ ] ') },
		{ rotulo: '❝', titulo: 'Citação (> no começo da linha)', fazer: () => prefixar('> ') },
		{ rotulo: 'N', titulo: 'Negrito (Ctrl+B)', fazer: () => destacar('**'), estilo: 'font-weight:700' },
		{ rotulo: 'I', titulo: 'Itálico (Ctrl+I)', fazer: () => destacar('*'), estilo: 'font-style:italic' },
		{ rotulo: 'S', titulo: 'Riscado', fazer: () => destacar('~~'), estilo: 'text-decoration:line-through' },
		{ rotulo: '</>', titulo: 'Código (Ctrl+E)', fazer: () => destacar('`') },
		{ rotulo: '🔗', titulo: 'Link (Ctrl+K)', fazer: link }
	];
</script>

<div class="anotacao" class:editando>
	{#if editando}
		<div class="barra" role="toolbar" aria-label="Formatação da anotação">
			{#each BARRA as b (b.titulo)}
				<button
					type="button"
					title={b.titulo}
					style={b.estilo}
					onmousedown={(e) => e.preventDefault()}
					onclick={b.fazer}>{b.rotulo}</button
				>
			{/each}
			<span class="espaco"></span>
			<button type="button" class="concluir" onclick={concluir}>Concluir</button>
		</div>
		<textarea
			bind:this={campo}
			aria-label="Anotação da questão {numero}"
			placeholder="Escreva o porquê da resposta, o artigo de lei, o que pesquisou…"
			value={texto}
			oninput={(e) => mudar(e.currentTarget.value)}
			onkeydown={teclas}
			onblur={(e) => {
				// Clique na barra não conta como sair.
				if (!(e.relatedTarget instanceof HTMLElement && e.relatedTarget.closest('.anotacao'))) concluir();
			}}
			rows="4"
		></textarea>
		<p class="dica">
			Markdown como no Notion: <code>#</code> título, <code>-</code> lista, <code>- [ ]</code> tarefa,
			<code>&gt;</code> citação, <code>**negrito**</code>, <code>`código`</code>. Esc conclui.
		</p>
	{:else if texto.trim()}
		<!-- Clicar em qualquer ponto da nota edita; link e tarefa têm clique próprio. -->
		<div
			class="leitura"
			role="button"
			tabindex="0"
			title="Clique para editar"
			onclick={(e) => {
				if (!(e.target instanceof HTMLElement && e.target.closest('a, input'))) void editar();
			}}
			onkeydown={(e) => {
				if (e.key === 'Enter') void editar();
			}}
		>
			<Markdown
				{texto}
				onalternarTarefa={(linha) => {
					mudar(alternarTarefa(texto, linha));
				}}
			/>
		</div>
	{:else}
		<button type="button" class="vazia" onclick={editar}>
			Clique para anotar o porquê da resposta, o artigo de lei, o que pesquisou…
		</button>
	{/if}

	<div class="estado" aria-live="polite">
		{#if estado === 'salvando'}Salvando…
		{:else if estado === 'pendente'}Editando…
		{:else if estado === 'erro'}
			<span class="erro">Não salvou.</span>
			<button type="button" onclick={() => void salvar()}>Tentar de novo</button>
		{:else if salvoEm}Salvo às {salvoEm}{/if}
	</div>
</div>

<style>
	.anotacao {
		display: grid;
		gap: 6px;
	}
	.leitura {
		padding: 6px 8px;
		margin: 0 -8px;
		border-radius: 6px;
		cursor: text;
	}
	.leitura:hover {
		background: var(--bg-hover);
	}
	.leitura:focus-visible {
		outline: 2px solid var(--accent);
	}
	.vazia {
		padding: 8px;
		margin: 0 -8px;
		text-align: left;
		font: inherit;
		font-size: 15px;
		color: var(--text-faint);
		background: transparent;
		border: 0;
		border-radius: 6px;
		cursor: text;
	}
	.vazia:hover {
		background: var(--bg-hover);
	}
	.barra {
		display: flex;
		flex-wrap: wrap;
		gap: 2px;
		align-items: center;
		padding: 3px;
		background: var(--bg-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow-pop);
	}
	.barra button {
		min-width: 30px;
		height: 28px;
		padding: 0 8px;
		font: inherit;
		font-size: 13px;
		color: var(--text-muted);
		background: transparent;
		border: 0;
		border-radius: 5px;
		cursor: pointer;
	}
	.barra button:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	.barra .concluir {
		color: var(--accent);
		font-weight: 600;
	}
	.espaco {
		flex: 1;
	}
	textarea {
		width: 100%;
		box-sizing: border-box;
		min-height: 110px;
		padding: 10px 12px;
		font: inherit;
		font-size: 15.5px;
		line-height: 1.65;
		color: var(--text);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 8px;
		resize: none;
		overflow: hidden;
	}
	textarea:focus {
		outline: none;
		border-color: var(--accent);
	}
	.dica {
		margin: 0;
		font-size: 12px;
		color: var(--text-faint);
	}
	.dica code {
		font-family: var(--font-mono);
		font-size: 11.5px;
	}
	.estado {
		min-height: 16px;
		font-size: 12px;
		color: var(--text-faint);
	}
	.estado .erro {
		color: var(--danger);
	}
	.estado button {
		font: inherit;
		color: var(--accent);
		background: none;
		border: 0;
		padding: 0 4px;
		cursor: pointer;
		text-decoration: underline;
	}
</style>
