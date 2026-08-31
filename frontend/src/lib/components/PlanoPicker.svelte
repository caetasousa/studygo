<script lang="ts">
	import { tick } from 'svelte';
	import NavIcon from './NavIcon.svelte';
	import { iniciaisPlano, planoCorresponde, rotuloPlano, rotuloPlanoTexto } from '$lib/estudo';
	import type { ConcursoResumo } from '$lib/types';

	/**
	 * Switches the active study plan.
	 *
	 * Named for what it actually changes: the stored records are one row per
	 * *cargo* (the two TCE-GO rows share órgão, concurso and cargo, and differ
	 * only by especialidade), so calling this a "concurso" picker would misname
	 * what the user is doing. See `rotuloPlano` for the field split.
	 *
	 * Built by hand because the project has no component library, and adding one
	 * for a single dropdown is not worth the dependency — so it implements the
	 * listbox keyboard and ARIA contract itself.
	 */
	let {
		planos,
		ativoSlug,
		carregando = false,
		compacto = false,
		onSelecionar,
		onAdicionar
	}: {
		planos: ConcursoResumo[];
		ativoSlug: string | null;
		carregando?: boolean;
		compacto?: boolean;
		onSelecionar: (slug: string) => void;
		onAdicionar: () => void;
	} = $props();

	let aberto = $state(false);
	let busca = $state('');
	let destaque = $state(0);
	let botao = $state<HTMLButtonElement | null>(null);
	let painel = $state<HTMLDivElement | null>(null);
	let campoBusca = $state<HTMLInputElement | null>(null);

	const ativo = $derived(planos.find((p) => p.slug === ativoSlug) ?? null);
	const rotulo = $derived(ativo ? rotuloPlano(ativo) : null);

	// With a single plan there is nothing to switch to, so the control promises
	// only what it can deliver: it still opens (that is how "Adicionar" is
	// reached), but says so.
	const soUmPlano = $derived(planos.length <= 1);
	const tituloBotao = $derived(
		!ativo
			? 'Selecionar plano de estudos'
			: soUmPlano
				? `${rotuloPlanoTexto(ativo)} — adicionar outro concurso`
				: `${rotuloPlanoTexto(ativo)} — trocar de plano`
	);

	// Search only earns its place once scanning the list becomes work.
	const mostrarBusca = $derived(planos.length >= 8);
	const visiveis = $derived(
		mostrarBusca ? planos.filter((p) => planoCorresponde(p, busca)) : planos
	);

	async function abrir() {
		aberto = true;
		busca = '';
		destaque = Math.max(
			0,
			visiveis.findIndex((p) => p.slug === ativoSlug)
		);
		await tick();
		if (mostrarBusca) campoBusca?.focus();
		else painel?.focus();
	}

	function fechar(devolverFoco = true) {
		aberto = false;
		if (devolverFoco) botao?.focus();
	}

	function escolher(slug: string) {
		if (slug !== ativoSlug) onSelecionar(slug);
		fechar();
	}

	function adicionar() {
		fechar(false);
		onAdicionar();
	}

	function navegar(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			fechar();
			return;
		}
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			if (visiveis.length === 0) return;
			const passo = e.key === 'ArrowDown' ? 1 : -1;
			destaque = (destaque + passo + visiveis.length) % visiveis.length;
			return;
		}
		if (e.key === 'Home' || e.key === 'End') {
			e.preventDefault();
			destaque = e.key === 'Home' ? 0 : visiveis.length - 1;
			return;
		}
		if (e.key === 'Enter' || e.key === ' ') {
			const alvo = visiveis[destaque];
			if (alvo) {
				e.preventDefault();
				escolher(alvo.slug);
			}
		}
	}

	// Clicking anywhere outside closes, without stealing focus back.
	function foraDoComponente(e: MouseEvent) {
		if (!aberto) return;
		const alvo = e.target as Node;
		if (botao?.contains(alvo) || painel?.contains(alvo)) return;
		fechar(false);
	}
</script>

<svelte:window onclick={foraDoComponente} />

<div class="picker" class:compacto>
	{#if !compacto}
		<span class="picker-cap" id="picker-cap">Plano de estudos</span>
	{/if}

	<button
		bind:this={botao}
		type="button"
		class="picker-btn"
		class:aberto
		aria-haspopup="listbox"
		aria-expanded={aberto}
		aria-labelledby={compacto ? undefined : 'picker-cap'}
		aria-label={compacto ? 'Trocar plano de estudos' : undefined}
		title={tituloBotao}
		onclick={() => (aberto ? fechar() : abrir())}
		onkeydown={(e) => {
			if (!aberto && (e.key === 'ArrowDown' || e.key === 'Enter')) {
				e.preventDefault();
				abrir();
			}
		}}
	>
		{#if compacto}
			<span class="sigla" aria-hidden="true">{ativo ? iniciaisPlano(ativo) : '—'}</span>
		{:else if carregando && !ativo}
			<span class="picker-txt"><span class="orgao">Carregando…</span></span>
		{:else if rotulo}
			<span class="picker-txt">
				<span class="orgao">{rotulo.orgao}</span>
				{#if rotulo.cargo}
					<span class="cargo">
						{rotulo.cargo}{#if rotulo.especialidade} — {rotulo.especialidade}{/if}
					</span>
				{/if}
				{#if rotulo.banca}<span class="banca">{rotulo.banca}</span>{/if}
			</span>
			<span class="chev" aria-hidden="true"><NavIcon name="chevron" size="sm" /></span>
		{:else}
			<span class="picker-txt">
				<span class="orgao vazio">Nenhum plano selecionado</span>
			</span>
			<span class="chev" aria-hidden="true"><NavIcon name="chevron" size="sm" /></span>
		{/if}
	</button>

	{#if aberto}
		<div class="menu" bind:this={painel}>
			{#if mostrarBusca}
				<div class="menu-busca">
					<span class="busca-ic" aria-hidden="true"><NavIcon name="busca" size="sm" /></span>
					<input
						bind:this={campoBusca}
						type="text"
						placeholder="Buscar por órgão, cargo ou banca"
						aria-label="Buscar plano de estudos"
						value={busca}
						oninput={(e) => {
							busca = e.currentTarget.value;
							destaque = 0;
						}}
						onkeydown={navegar}
					/>
				</div>
			{/if}

			<!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
			<ul
				class="menu-lista"
				role="listbox"
				aria-label="Planos de estudo"
				tabindex="-1"
				onkeydown={navegar}
			>
				{#each visiveis as p, i (p.slug)}
					{@const r = rotuloPlano(p)}
					<li>
						<button
							type="button"
							role="option"
							aria-selected={p.slug === ativoSlug}
							class="opt"
							class:ativo={p.slug === ativoSlug}
							class:destacado={i === destaque}
							title={rotuloPlanoTexto(p)}
							onmouseenter={() => (destaque = i)}
							onclick={() => escolher(p.slug)}
						>
							<span class="opt-txt">
								<span class="opt-orgao">{r.orgao}</span>
								{#if r.cargo}<span class="opt-cargo">{r.cargo}</span>{/if}
								{#if r.especialidade || r.banca}
									<span class="opt-sub">{r.especialidade || r.banca}</span>
								{/if}
							</span>
							{#if p.slug === ativoSlug}
								<span class="opt-check" aria-hidden="true"><NavIcon name="check" size="sm" /></span>
							{/if}
						</button>
					</li>
				{:else}
					<li class="menu-vazio">Nenhum plano encontrado.</li>
				{/each}
			</ul>

			<div class="menu-rodape">
				<button type="button" class="add" onclick={adicionar}>
					<span class="add-ic" aria-hidden="true"><NavIcon name="mais" size="sm" /></span>
					Adicionar concurso
				</button>
			</div>
		</div>
	{/if}
</div>

<style>
	.picker {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.picker-cap {
		font-size: 10px;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-faint);
		padding-left: 10px;
	}

	/* --- closed state: a context switcher, not a form field --- */
	.picker-btn {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 8px 10px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-card);
		color: var(--text);
		font-family: inherit;
		text-align: left;
		cursor: pointer;
	}
	.picker-btn:hover {
		background: var(--bg-hover);
		border-color: var(--border-strong);
	}
	.picker-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.picker-txt {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}
	/* Each field truncates on its own line, so a long cargo never eats the órgão. */
	.orgao,
	.cargo,
	.banca {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.orgao {
		font-size: 13px;
		font-weight: 600;
		line-height: 1.25;
	}
	.orgao.vazio {
		color: var(--text-muted);
		font-weight: 500;
	}
	.cargo {
		font-size: 11.5px;
		color: var(--text-muted);
		line-height: 1.3;
	}
	.banca {
		font-size: 10.5px;
		color: var(--text-faint);
		line-height: 1.3;
	}
	.chev {
		display: grid;
		place-items: center;
		color: var(--text-faint);
		transition: transform 0.15s ease;
	}
	.picker-btn.aberto .chev {
		transform: rotate(180deg);
	}

	/* --- collapsed rail: a compact badge, still a real control --- */
	.compacto .picker-btn {
		grid-template-columns: 1fr;
		justify-items: center;
		padding: 7px 0;
	}
	.sigla {
		font-size: 11.5px;
		font-weight: 700;
		letter-spacing: 0.04em;
		color: var(--text);
	}

	/* --- open menu --- */
	.menu {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		/* At least as wide as the trigger, never wide enough to invade the page. */
		min-width: 100%;
		width: max-content;
		max-width: min(360px, calc(100vw - 24px));
		background: var(--bg-card);
		border: 1px solid var(--border-strong);
		border-radius: 10px;
		box-shadow: var(--shadow-pop);
		z-index: 60;
		overflow: hidden;
	}
	.compacto .menu {
		left: calc(100% + 8px);
		top: 0;
		min-width: 260px;
	}
	.menu-lista {
		list-style: none;
		margin: 0;
		padding: 4px;
		/* Scrolls internally once the list gets long. */
		max-height: 320px;
		overflow-y: auto;
	}
	.menu-lista:focus-visible {
		outline: none;
	}
	.opt {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 7px 9px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--text);
		font-family: inherit;
		text-align: left;
		cursor: pointer;
	}
	.opt-txt {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}
	.opt-orgao {
		font-size: 12.5px;
		font-weight: 600;
		line-height: 1.25;
	}
	.opt-cargo,
	.opt-sub {
		/* Two lines at most, then ellipsis — never one endless horizontal row. */
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		line-height: 1.35;
	}
	.opt-cargo {
		font-size: 11.5px;
		color: var(--text-muted);
	}
	.opt-sub {
		font-size: 10.5px;
		color: var(--text-faint);
	}
	.opt.destacado {
		background: var(--bg-hover);
	}
	/* Active plan: background + weight + a check, never colour alone. */
	.opt.ativo {
		background: var(--accent-soft);
	}
	.opt.ativo .opt-orgao {
		color: var(--accent-strong);
		font-weight: 700;
	}
	.opt-check {
		display: grid;
		place-items: center;
		color: var(--accent-strong);
	}
	.menu-vazio {
		padding: 12px 10px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.menu-busca {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		align-items: center;
		gap: 7px;
		padding: 8px 10px;
		border-bottom: 1px solid var(--border);
	}
	.busca-ic {
		display: grid;
		place-items: center;
		color: var(--text-faint);
	}
	.menu-busca input {
		width: 100%;
		border: 0;
		background: transparent;
		color: var(--text);
		font-family: inherit;
		font-size: 12.5px;
		padding: 2px 0;
	}
	.menu-busca input:focus {
		outline: none;
	}

	/* "Adicionar" is an action, not one more plan — hence its own footer. */
	.menu-rodape {
		border-top: 1px solid var(--border);
		padding: 4px;
	}
	.add {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		align-items: center;
		gap: 8px;
		width: 100%;
		min-height: 34px;
		padding: 0 9px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font-family: inherit;
		font-size: 12.5px;
		font-weight: 500;
		text-align: left;
		cursor: pointer;
	}
	.add:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.add:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.add-ic {
		display: grid;
		place-items: center;
	}

	@media (prefers-reduced-motion: reduce) {
		.chev {
			transition: none;
		}
	}
</style>
