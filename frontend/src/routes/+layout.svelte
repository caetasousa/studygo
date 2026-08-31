<script lang="ts">
	import '$lib/styles/app.css';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore, applyTheme, ehTema } from '$lib/stores/plano.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import NavIcon from '$lib/components/NavIcon.svelte';
	import { browser } from '$app/environment';

	let { children } = $props();

	let menuOpen = $state(false);
	let botaoMenu = $state<HTMLButtonElement | null>(null);

	// Mobile drawer: never persisted (only the desktop rail preference is), and
	// while it is open the page behind it must not scroll.
	$effect(() => {
		if (!browser) return;
		document.body.style.overflow = menuOpen ? 'hidden' : '';
		return () => {
			document.body.style.overflow = '';
		};
	});

	function fecharMenu() {
		if (!menuOpen) return;
		menuOpen = false;
		botaoMenu?.focus(); // focus returns to the control that opened it
	}

	const RAIL_KEY = 'annygo:rail';
	let railOnly = $state(browser && localStorage.getItem(RAIL_KEY) === '1');

	$effect(() => {
		if (browser) localStorage.setItem(RAIL_KEY, railOnly ? '1' : '0');
	});

	const publicRoutes = ['/login', '/registro'];
	const isPublic = $derived(publicRoutes.includes(page.url.pathname));
	const isConcursoAdmin = $derived(page.url.pathname.startsWith('/concursos'));

	$effect(() => {
		if (!auth.isAuthenticated && !isPublic) {
			goto('/login');
		} else if (auth.isAuthenticated && isPublic) {
			goto('/');
		}
	});

	$effect(() => {
		if (auth.isAuthenticated && !concursoStore.carregado) {
			concursoStore.carregar();
		}
	});

	// First run: no concursos yet -> send to the registration form.
	$effect(() => {
		if (
			auth.isAuthenticated &&
			concursoStore.carregado &&
			concursoStore.lista.length === 0 &&
			!isConcursoAdmin
		) {
			goto('/concursos/novo');
		}
	});

	// Load / reload the plan whenever the active concurso changes.
	$effect(() => {
		if (auth.isAuthenticated && concursoStore.ativoSlug) {
			planoStore.carregar();
		}
	});

	// The plan is the single source of truth for the theme. Before one is loaded
	// (login, registration, a brand-new account) there is no stored preference to
	// honour, so pin the app's default look rather than letting the OS decide for
	// those screens; once the plan arrives, the saved choice — including
	// 'system' — takes over.
	$effect(() => {
		const salvo = planoStore.plano?.config.temaUi;
		applyTheme(ehTema(salvo) ? salvo : 'dark');
	});
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape') fecharMenu();
	}}
/>

{#if isPublic}
	{@render children()}
{:else if auth.isAuthenticated}
	<button
		bind:this={botaoMenu}
		type="button"
		class="menu-toggle"
		aria-label={menuOpen ? 'Fechar menu' : 'Abrir menu'}
		aria-expanded={menuOpen}
		aria-controls="nav-principal"
		onclick={() => (menuOpen = !menuOpen)}
	>
		<NavIcon name={menuOpen ? 'fechar' : 'menu'} />
	</button>
	<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
	<div class="scrim" class:on={menuOpen} onclick={fecharMenu}></div>
	<div class="app" class:rail-only={railOnly}>
		<Sidebar bind:open={menuOpen} bind:railOnly />
		<main class="main">
			{#if planoStore.erro && !isConcursoAdmin}
				<div class="form-error" style="margin-bottom:16px">{planoStore.erro}</div>
			{/if}
			{@render children()}
		</main>
	</div>
	<div class="saved-toast" class:on={planoStore.salvo}>Salvo</div>
{:else}
	<!-- Signed out on a private route: the redirect effect above is already on its
	     way to /login. Without this branch the document renders empty, which reads
	     as a broken app rather than a moment of transition. -->
	<p class="page-sub" style="padding:32px">Carregando…</p>
{/if}
