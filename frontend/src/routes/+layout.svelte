<script lang="ts">
	import '$lib/styles/app.css';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore, applyTheme } from '$lib/stores/plano.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';

	let { children } = $props();

	let menuOpen = $state(false);

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

	$effect(() => {
		applyTheme(planoStore.plano?.config.temaUi);
	});
</script>

{#if isPublic}
	{@render children()}
{:else if auth.isAuthenticated}
	<button class="menu-toggle" aria-label="Abrir menu" onclick={() => (menuOpen = !menuOpen)}>☰</button>
	<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
	<div class="scrim" class:on={menuOpen} onclick={() => (menuOpen = false)}></div>
	<div class="app">
		<Sidebar bind:open={menuOpen} />
		<main class="main">
			{#if planoStore.erro && !isConcursoAdmin}
				<div class="form-error" style="margin-bottom:16px">{planoStore.erro}</div>
			{/if}
			{@render children()}
		</main>
	</div>
	<div class="saved-toast" class:on={planoStore.salvo}>Salvo</div>
{/if}
