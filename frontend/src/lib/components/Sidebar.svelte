<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore, applyTheme } from '$lib/stores/plano.svelte';
	import { diffDays, hojeISO } from '$lib/format';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const links = [
		{ href: '/', ic: '☀️', label: 'Hoje' },
		{ href: '/cronograma', ic: '🗂️', label: 'Cronograma' },
		{ href: '/balanceamento', ic: '⚖️', label: 'Balanceamento' },
		{ href: '/estatisticas', ic: '📈', label: 'Estatísticas' },
		{ href: '/caderno', ic: '📓', label: 'Caderno de erros' },
		{ href: '/datas', ic: '📌', label: 'Datas do edital' },
		{ href: '/conteudo', ic: '📖', label: 'Conteúdo programático' }
	];

	const plano = $derived(planoStore.plano);
	const ativo = $derived(concursoStore.ativo);
	const diasParaProva = $derived(
		ativo ? Math.max(0, diffDays(hojeISO(), ativo.prova)) : null
	);
	const progresso = $derived(plano?.props.progresso ?? 0);
	const tema = $derived(plano?.config.temaUi ?? 'system');

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
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
	<a class="side-top" href="/concursos" onclick={() => (open = false)} style="text-decoration:none;color:inherit">
		<span class="side-emoji">{ativo ? '📚' : '🏛️'}</span>
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
			<a class="side-item" class:active={isActive(l.href)} href={l.href} onclick={() => (open = false)}>
				<span class="ic">{l.ic}</span>{l.label}
			</a>
		{/each}
		<div class="side-sep"></div>
		<a
			class="side-item"
			class:active={isActive('/config')}
			href="/config"
			onclick={() => (open = false)}
		>
			<span class="ic">⚙️</span>Configurações
		</a>
		<a
			class="side-item"
			class:active={page.url.pathname === '/concursos'}
			href="/concursos"
			onclick={() => (open = false)}
		>
			<span class="ic">📁</span>Meus concursos
		</a>
	</nav>

	<div class="side-bottom">
		<div class="ring" style="--ring-pct:{progresso}"><i></i></div>
		<div class="side-count">
			<strong>{diasParaProva ?? '—'}</strong>
			<span>dias p/ prova</span>
		</div>
	</div>

	<div class="theme-row">
		<button class="theme-btn" class:active={tema === 'light'} onclick={() => setTema('light')} title="Claro"
			>☀️</button
		>
		<button
			class="theme-btn"
			class:active={tema === 'system'}
			onclick={() => setTema('system')}
			title="Sistema">🖥️</button
		>
		<button class="theme-btn" class:active={tema === 'dark'} onclick={() => setTema('dark')} title="Escuro"
			>🌙</button
		>
	</div>

	<div class="side-user">
		<span>{auth.usuario?.nome} · {auth.usuario?.email}</span>
		<button onclick={() => { concursoStore.limpar(); planoStore.limpar(); auth.logout(); }}>Sair</button>
	</div>
</aside>
