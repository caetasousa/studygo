<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore, applyTheme } from '$lib/stores/plano.svelte';
	import { diffDays, hojeISO } from '$lib/format';
	import NavIcon from './NavIcon.svelte';
	import type { NavIconName } from './NavIcon.svelte';

	let {
		open = $bindable(false),
		railOnly = $bindable(false)
	}: { open?: boolean; railOnly?: boolean } = $props();

	const links: { href: string; icon: NavIconName; label: string }[] = [
		{ href: '/', icon: 'hoje', label: 'Hoje' },
		{ href: '/cronograma', icon: 'cronograma', label: 'Cronograma' },
		{ href: '/balanceamento', icon: 'balanceamento', label: 'Balanceamento' },
		{ href: '/estatisticas', icon: 'estatisticas', label: 'Estatísticas' },
		{ href: '/caderno', icon: 'caderno', label: 'Caderno de erros' },
		{ href: '/datas', icon: 'datas', label: 'Datas do edital' },
		{ href: '/conteudo', icon: 'conteudo', label: 'Conteúdo programático' }
	];

	const admin: { href: string; icon: NavIconName; label: string }[] = [
		{ href: '/config', icon: 'config', label: 'Configurações' },
		{ href: '/concursos', icon: 'concursos', label: 'Meus concursos' }
	];

	const plano = $derived(planoStore.plano);
	const ativo = $derived(concursoStore.ativo);
	const diasParaProva = $derived(ativo ? Math.max(0, diffDays(hojeISO(), ativo.prova)) : null);
	const progresso = $derived(plano?.props.progresso ?? 0);
	const tema = $derived(plano?.config.temaUi ?? 'system');

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		if (href === '/concursos') return page.url.pathname === '/concursos';
		return page.url.pathname.startsWith(href);
	}

	function trocar(e: Event) {
		const slug = (e.target as HTMLSelectElement).value;
		if (slug === '__novo') {
			goto('/concursos/novo');
			return;
		}
		concursoStore.setAtivo(slug);
		open = false;
		goto('/');
	}

	async function setTema(t: 'light' | 'dark' | 'system') {
		applyTheme(t);
		await planoStore.salvarConfig({ temaUi: t });
	}
</script>

<aside class="sidebar" class:open>
	<!-- level 1: icon rail, always visible -->
	<div class="rail">
		<button
			class="rail-mark"
			title={railOnly ? 'Expandir navegação' : 'Recolher navegação'}
			aria-label={railOnly ? 'Expandir navegação' : 'Recolher navegação'}
			onclick={() => (railOnly = !railOnly)}
		>
			a
		</button>

		{#each links as l (l.href)}
			<a
				class="rail-item"
				class:active={isActive(l.href)}
				href={l.href}
				data-label={l.label}
				aria-label={l.label}
				onclick={() => (open = false)}
			>
				<NavIcon name={l.icon} />
			</a>
		{/each}

		<div class="rail-sep"></div>

		{#each admin as l (l.href)}
			<a
				class="rail-item"
				class:active={isActive(l.href)}
				href={l.href}
				data-label={l.label}
				aria-label={l.label}
				onclick={() => (open = false)}
			>
				<NavIcon name={l.icon} />
			</a>
		{/each}

		<div class="rail-foot">
			<div class="ring" style="--ring-pct:{progresso}" title="{progresso}% do plano concluído">
				<i></i>
			</div>
		</div>
	</div>

	<!-- level 2: labels + footer -->
	<div class="side-panel">
		<a class="side-top" href="/concursos" onclick={() => (open = false)}>
			<div class="side-title">
				<strong>{ativo?.nome ?? 'Meus concursos'}</strong>
				<span>{ativo?.banca || 'gerenciar concursos'}</span>
			</div>
		</a>

		{#if concursoStore.lista.length > 1}
			<select value={concursoStore.ativoSlug} onchange={trocar} style="width:100%">
				{#each concursoStore.lista as c (c.slug)}
					<option value={c.slug}>{c.nome}</option>
				{/each}
				<option value="__novo">+ novo concurso…</option>
			</select>
		{/if}

		<nav class="side-nav">
			{#each links as l (l.href)}
				<a
					class="side-item"
					class:active={isActive(l.href)}
					href={l.href}
					onclick={() => (open = false)}
				>
					{l.label}
				</a>
			{/each}
			<div class="side-sep"></div>
			{#each admin as l (l.href)}
				<a
					class="side-item"
					class:active={isActive(l.href)}
					href={l.href}
					onclick={() => (open = false)}
				>
					{l.label}
				</a>
			{/each}
		</nav>

		<div class="side-bottom">
			<div class="side-count">
				<strong>{diasParaProva ?? '—'}</strong>
				<span>dias p/ prova</span>
			</div>
		</div>

		<div class="theme-row">
			<button
				class="theme-btn"
				class:active={tema === 'light'}
				onclick={() => setTema('light')}
				title="Tema claro"
				aria-label="Tema claro"><NavIcon name="sol" /></button
			>
			<button
				class="theme-btn"
				class:active={tema === 'system'}
				onclick={() => setTema('system')}
				title="Seguir o sistema"
				aria-label="Seguir o sistema"><NavIcon name="sistema" /></button
			>
			<button
				class="theme-btn"
				class:active={tema === 'dark'}
				onclick={() => setTema('dark')}
				title="Tema escuro"
				aria-label="Tema escuro"><NavIcon name="lua" /></button
			>
		</div>

		<div class="side-user">
			<span>{auth.usuario?.nome} · {auth.usuario?.email}</span>
			<button
				onclick={() => {
					concursoStore.limpar();
					planoStore.limpar();
					auth.logout();
				}}>Sair</button
			>
		</div>
	</div>
</aside>
