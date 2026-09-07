<script lang="ts">
	import { untrack } from 'svelte';
	import { fl } from '$lib/format';
	import { horasEmMinutos, minutosEmHoras, valoresIniciais, valoresInvalidos } from '$lib/estudo';
	import type { Atividade } from '$lib/types';
	import NavIcon, { type NavIconName } from './NavIcon.svelte';

	/**
	 * Logs ONE scheduled activity — not the day.
	 *
	 * The day-level form asked for every subject at once, so recording one meant
	 * looking past the others. This shows only the activity you opened it from,
	 * and saves only that activity: the other subjects of the day are neither
	 * displayed nor written.
	 *
	 * Edits are local until Salvar. Cancelling (button, Escape, backdrop, close)
	 * discards them without touching the store or the API, so an abandoned edit
	 * can never leak into the schedule.
	 */
	let {
		item,
		data,
		nome,
		registro,
		cadernoUrl = '',
		notebookUrl = '',
		soTeoria = false,
		salvando = false,
		erro = null,
		onSalvar,
		onSalvarLinks,
		onCancelar
	}: {
		item: Atividade;
		/** ISO date of the activity, shown so the form says what it is editing. */
		data: string;
		/** full discipline name, for the title */
		nome: string;
		/** what is already recorded for THIS activity, if anything */
		registro: Atividade | null;
		/** links da MATÉRIA (valem no cronograma todo), não desta atividade */
		cadernoUrl?: string;
		notebookUrl?: string;
		/** a matéria está configurada como "só teoria" no plano */
		soTeoria?: boolean;
		salvando?: boolean;
		erro?: string | null;
		onSalvar: (v: {
			horas: number | null;
			questoes: number | null;
			acertos: number | null;
			concluido: boolean;
			nota: string;
		}) => void;
		/** grava os links da matéria; devolve mensagem de erro ou null */
		onSalvarLinks?: (links: {
			cadernoUrl: string;
			notebookUrl: string;
		}) => Promise<string | null>;
		onCancelar: () => void;
	} = $props();

	// The pristine copy, snapshotted once on open — `untrack` states that the
	// capture is the point: later store updates must NOT rewrite the form under
	// someone who is typing in it. Everything below edits `form`; Cancelar simply
	// throws it away, which is why no restore logic is needed anywhere else.
	const original = untrack(() => valoresIniciais(registro));

	let form = $state({ ...original });

	// Os links são da MATÉRIA, não desta atividade — capturados do mesmo jeito e
	// gravados por outra rota, só quando de fato mudaram.
	const linksOriginais = untrack(() => ({ caderno: cadernoUrl, notebook: notebookUrl }));
	let links = $state({ ...linksOriginais });
	let linkEmEdicao = $state<ChaveLink | null>(null);
	let erroLinks = $state<string | null>(null);

	type ChaveLink = 'caderno' | 'notebook';

	const LINKS: { chave: ChaveLink; rotulo: string; icone: NavIconName; exemplo: string }[] = [
		{
			chave: 'caderno',
			rotulo: 'Caderno de erros',
			icone: 'caderno',
			exemplo: 'https://www.tecconcursos.com.br/questoes/caderno/…'
		},
		{
			chave: 'notebook',
			rotulo: 'NotebookLM',
			icone: 'conteudo',
			exemplo: 'https://notebooklm.google.com/notebook/…'
		}
	];

	/**
	 * "Só estudei teoria": esconde Questões e Acertos.
	 *
	 * NÃO é um campo gravado, e não precisa ser — o modelo já diz isso, porque
	 * questões em branco significa "não lancei". Guardar um booleano ao lado
	 * criaria um segundo lugar para a mesma verdade, com a chance de os dois
	 * discordarem.
	 *
	 * Por isso o estado inicial é DEDUZIDO: um registro que já existe e não tem
	 * questões era teoria, e reabre recolhido. Atividade ainda não lançada abre
	 * com as caixas à mostra, que é o caso comum.
	 */
	const jaRegistrado = untrack(
		() => original.concluido || original.horas !== null || original.nota !== ''
	);
	let apenasTeoria = $state(jaRegistrado && original.questoes === null);

	// A matéria em modo "só teoria" no plano nem oferece a opção: ali as caixas
	// não fazem sentido nenhum, e um botão para reexibi-las só confundiria.
	const mostrarQuestoes = $derived(!soTeoria && !apenasTeoria);

	function alternarTeoria(marcado: boolean) {
		apenasTeoria = marcado;

		// Recolher e continuar mandando o que estava digitado gravaria um número
		// que ninguém vê mais.
		if (marcado) {
			form.questoes = null;
			form.acertos = null;
		}
	}

	/** O host, que é o que identifica o link de relance. */
	function host(url: string): string {
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	}

	/** Foca o campo assim que ele aparece — quem clicou em "adicionar" quer digitar. */
	function focar(el: HTMLInputElement) {
		el.focus();
		el.select();
	}

	const erros = $derived(
		form.questoes !== null && form.acertos !== null
			? Math.max(0, form.questoes - form.acertos)
			: null
	);

	// Acertos above questões is the one input that produces nonsense downstream.
	const invalido = $derived(valoresInvalidos(form));

	function num(bruto: string): number | null {
		const t = bruto.trim();
		if (t === '') return null;
		const v = Number(t.replace(',', '.'));
		return Number.isFinite(v) && v >= 0 ? v : null;
	}

	function inteiro(bruto: string): number | null {
		const v = num(bruto);
		return v === null ? null : Math.round(v);
	}

	let salvandoLinks = $state(false);

	async function salvar() {
		if (salvando || salvandoLinks || invalido) return;

		// Os links da matéria primeiro, para que uma falha ali apareça antes de o
		// registro da atividade fechar o diálogo.
		const caderno = links.caderno.trim();
		const notebook = links.notebook.trim();
		const mudou =
			caderno !== linksOriginais.caderno.trim() ||
			notebook !== linksOriginais.notebook.trim();

		if (onSalvarLinks && mudou) {
			salvandoLinks = true;
			erroLinks = await onSalvarLinks({ cadernoUrl: caderno, notebookUrl: notebook });
			salvandoLinks = false;
			if (erroLinks) return;
		}

		onSalvar({ ...form, nota: form.nota.trim() });
	}

	// --- focus management -------------------------------------------------
	let painel = $state<HTMLDivElement | null>(null);
	let primeiro = $state<HTMLInputElement | null>(null);

	$effect(() => {
		primeiro?.focus();
	});

	/** Keeps Tab inside the dialog while it is open. */
	function prender(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onCancelar();
			return;
		}

		if (e.key !== 'Tab' || !painel) return;

		const foco = painel.querySelectorAll<HTMLElement>(
			'button:not(:disabled), input:not(:disabled), textarea:not(:disabled), [href]'
		);
		if (foco.length === 0) return;

		const primeiroEl = foco[0];
		const ultimoEl = foco[foco.length - 1];

		if (e.shiftKey && document.activeElement === primeiroEl) {
			e.preventDefault();
			ultimoEl.focus();
		} else if (!e.shiftKey && document.activeElement === ultimoEl) {
			e.preventDefault();
			primeiroEl.focus();
		}
	}

	// Derived, not captured: the dialog remounts per activity, and $derived keeps
	// the ids correct if the props ever change under it.
	const tituloID = $derived(`atv-form-${item.id || data}`);
	const descID = $derived(`${tituloID}-desc`);
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="dlg-fundo"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onCancelar();
	}}
>
	<div
		class="dlg atv-dlg"
		role="dialog"
		aria-modal="true"
		aria-labelledby={tituloID}
		aria-describedby={descID}
		tabindex="-1"
		bind:this={painel}
		onkeydown={prender}
	>
		<h2 id={tituloID} class="sec" style="margin-top:0">Registrar estudo — {nome}</h2>
		<p id={descID} class="page-sub" style="margin-top:0">
			{fl(data)}{#if item.tema}
				· {item.tema}{/if}
		</p>

		<div class="campos" class:so-horas={!mostrarQuestoes}>
			<!-- Em MINUTOS: o cronograma anuncia o bloco em minutos, e "45" é o que
			     se tem na cabeça ao terminar de estudar. O registro continua sendo
			     gravado em horas — a conversão acontece aqui. -->
			<label class="campo">
				<span>Minutos estudados</span>
				<input
					type="number"
					min="0"
					max="1440"
					step="5"
					inputmode="numeric"
					bind:this={primeiro}
					value={horasEmMinutos(form.horas) ?? ''}
					oninput={(e) => (form.horas = minutosEmHoras(inteiro(e.currentTarget.value)))}
				/>
			</label>

			{#if mostrarQuestoes}
				<label class="campo">
					<span>Questões</span>
					<input
						type="number"
						min="0"
						step="1"
						inputmode="numeric"
						value={form.questoes ?? ''}
						oninput={(e) => (form.questoes = inteiro(e.currentTarget.value))}
					/>
				</label>

				<label class="campo">
					<span>Acertos</span>
					<input
						type="number"
						min="0"
						step="1"
						inputmode="numeric"
						aria-invalid={invalido}
						value={form.acertos ?? ''}
						oninput={(e) => (form.acertos = inteiro(e.currentTarget.value))}
					/>
				</label>

				<span class="campo-err" class:vazio={erros === null}>
					{#if erros !== null}<b>{erros}</b> {erros === 1 ? 'erro' : 'erros'}{:else}—{/if}
				</span>
			{/if}
		</div>

		{#if soTeoria}
			<p class="teoria-nota">Esta matéria está em <b>só teoria</b> no plano — sem questões.</p>
		{:else}
			<label class="teoria-lbl">
				<input
					type="checkbox"
					class="checkbox"
					checked={apenasTeoria}
					onchange={(e) => alternarTeoria(e.currentTarget.checked)}
				/>
				Só estudei teoria hoje
			</label>
		{/if}

		{#if invalido}
			<p class="aviso" role="alert">Acertos não pode ser maior que o número de questões.</p>
		{/if}

		<label class="ok-lbl">
			<input type="checkbox" class="checkbox" bind:checked={form.concluido} />
			Concluí esta matéria
		</label>

		<label class="campo nota">
			<span>Observação</span>
			<input
				type="text"
				placeholder="Dúvidas, questões erradas, o que revisar…"
				bind:value={form.nota}
			/>
		</label>

		{#if onSalvarLinks}
			<!-- Uma seção só para os dois links, em vez de dois campos de URL de
			     largura inteira com um parágrafo de ajuda cada: o link preenchido
			     vira um atalho clicável (dá para abrir o caderno daqui, o que antes
			     não dava), e o vazio ocupa uma linha discreta. -->
			<div class="links">
				<span class="links-titulo">Links de {nome}</span>

				{#each LINKS as l (l.chave)}
					<div class="link-linha">
						<span class="link-icone"><NavIcon name={l.icone} size="sm" /></span>
						<span class="link-rotulo">{l.rotulo}</span>

						{#if linkEmEdicao === l.chave}
							<input
								class="link-campo"
								type="url"
								inputmode="url"
								placeholder={l.exemplo}
								bind:value={links[l.chave]}
								use:focar
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === 'Escape') {
										e.preventDefault();
										e.stopPropagation();
										linkEmEdicao = null;
									}
								}}
							/>
							<button type="button" class="link-acao" onclick={() => (linkEmEdicao = null)}>
								pronto
							</button>
						{:else if links[l.chave]}
							<a
								class="link-valor"
								href={links[l.chave]}
								target="_blank"
								rel="noreferrer"
								title={links[l.chave]}
							>
								{host(links[l.chave])}
							</a>
							<button
								type="button"
								class="link-acao"
								onclick={() => (linkEmEdicao = l.chave)}
								aria-label="Editar link do {l.rotulo}"
							>
								editar
							</button>
							<button
								type="button"
								class="link-acao apagar"
								onclick={() => (links[l.chave] = '')}
								aria-label="Remover link do {l.rotulo}"
							>
								remover
							</button>
						{:else}
							<button
								type="button"
								class="link-add"
								onclick={() => (linkEmEdicao = l.chave)}
								aria-label="Adicionar link do {l.rotulo}"
							>
								adicionar
							</button>
						{/if}
					</div>
				{/each}

				<p class="links-dica">Valem para {nome} em todo o cronograma, não só neste dia.</p>
			</div>
		{/if}

		{#if erroLinks}
			<p class="aviso erro" role="alert">{erroLinks}</p>
		{/if}

		{#if erro}
			<p class="aviso erro" role="alert">{erro}</p>
		{/if}

		<div class="dlg-acoes">
			<button type="button" class="btn" onclick={onCancelar} disabled={salvando || salvandoLinks}>
				Cancelar
			</button>
			<button
				type="button"
				class="btn primario"
				onclick={salvar}
				disabled={salvando || salvandoLinks || invalido}
			>
				{salvando || salvandoLinks ? 'Salvando…' : 'Salvar'}
			</button>
		</div>
	</div>
</div>

<style>
	/* Same dialog shell as the concurso form's "dividir em tópicos" — those
	   styles are scoped to that component, so the shape is restated rather than
	   reached for. */
	.dlg-fundo {
		position: fixed;
		inset: 0;
		background: rgba(20, 18, 14, 0.45);
		display: grid;
		place-items: center;
		padding: 20px;
		z-index: 60;
	}
	.dlg {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 20px;
		max-height: 84vh;
		overflow-y: auto;
		box-shadow: var(--shadow-pop);
	}
	.atv-dlg {
		width: min(460px, 100%);
	}
	.campos {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr)) auto;
		gap: 10px;
		align-items: end;
		margin-top: 14px;
	}
	.campo {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	.campo span {
		font-size: 10.5px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-faint);
		font-weight: 600;
	}
	.campo input {
		width: 100%;
	}
	.campo-err {
		font-family: var(--font-mono);
		font-size: 11.5px;
		color: var(--danger);
		white-space: nowrap;
		padding-bottom: 8px;
	}
	.campo-err.vazio {
		color: var(--text-faint);
	}
	.nota {
		margin-top: 12px;
	}
	/* Com as questões escondidas sobra um campo só: sem isto ele continuaria
	   ocupando um terço da grade de três colunas, alinhado com o vazio. */
	.campos.so-horas {
		grid-template-columns: minmax(0, 160px);
	}
	.teoria-lbl {
		display: flex;
		align-items: center;
		gap: 9px;
		margin-top: 12px;
		font-size: 13px;
		color: var(--text-muted);
		cursor: pointer;
	}
	.teoria-nota {
		margin: 12px 0 0;
		font-size: 12.5px;
		color: var(--text-muted);
	}

	/* Os links da matéria, num painel só. Antes eram campos de URL de largura
	   inteira, cada um com um parágrafo de ajuda embaixo — dois deles fariam o
	   diálogo virar um formulário de cadastro. Aqui o link preenchido é uma
	   linha discreta e clicável, e o vazio quase não ocupa espaço. */
	.links {
		margin-top: 16px;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 10px 12px;
		background: var(--bg-soft);
	}
	.links-titulo {
		display: block;
		font-size: 10.5px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-faint);
		font-weight: 600;
		margin-bottom: 6px;
	}
	.link-linha {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 30px;
	}
	.link-linha + .link-linha {
		border-top: 1px solid var(--border);
	}
	.link-icone {
		display: inline-flex;
		flex: none;
		color: var(--text-faint);
	}
	.link-rotulo {
		flex: none;
		min-width: 106px;
		font-size: 12.5px;
		color: var(--text-muted);
	}
	/* O valor ganha o espaço que sobra e corta com reticências: uma URL de
	   caderno é longa demais para caber, e quebrar a linha desalinharia tudo. */
	.link-valor {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12.5px;
		color: var(--accent);
		text-decoration: none;
	}
	.link-valor:hover {
		text-decoration: underline;
	}
	.link-campo {
		flex: 1 1 auto;
		min-width: 0;
		font-size: 12.5px;
	}
	.link-acao,
	.link-add {
		flex: none;
		border: 0;
		background: none;
		padding: 3px 5px;
		border-radius: 4px;
		font: inherit;
		font-size: 11.5px;
		color: var(--text-faint);
		cursor: pointer;
	}
	.link-add {
		flex: 1 1 auto;
		text-align: left;
	}
	.link-acao:hover,
	.link-add:hover {
		color: var(--text);
		background: var(--bg-hover);
	}
	.link-acao.apagar:hover {
		color: var(--danger);
	}
	.links-dica {
		margin: 8px 0 0;
		font-size: 11.5px;
		color: var(--text-faint);
	}
	.ok-lbl {
		display: flex;
		align-items: center;
		gap: 9px;
		margin-top: 14px;
		font-size: 13.5px;
		cursor: pointer;
	}
	.aviso {
		margin: 10px 0 0;
		font-size: 12.5px;
		color: var(--warn);
	}
	.aviso.erro {
		color: var(--danger);
	}
	.dlg-acoes {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 18px;
	}
	.btn.primario {
		border-color: var(--accent);
		color: var(--accent);
	}
	.btn.primario:hover:not(:disabled) {
		background: var(--accent-soft);
	}

	@media (max-width: 620px) {
		.campos {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
</style>
