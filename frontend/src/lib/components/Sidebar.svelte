<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { diffDays, hojeISO } from '$lib/format';
	import SidebarNavItem from './SidebarNavItem.svelte';
	import IconButton from './IconButton.svelte';
	import PlanoPicker from './PlanoPicker.svelte';
	import type { NavIconName } from './NavIcon.svelte';

	let {
		open = $bindable(false),
		railOnly = $bindable(false)
	}: { open?: boolean; railOnly?: boolean } = $props();

	interface Item {
		href: string;
		icon: NavIconName;
		label: string;
	}

	// Grouped by what the user is doing, following the study flow:
	// plan the content -> follow the schedule -> check how it is going.
	const grupos: { titulo: string; itens: Item[] }[] = [
		{
			titulo: 'Estudar',
			itens: [
				{ href: '/', icon: 'hoje', label: 'Hoje' },
				{ href: '/cronograma', icon: 'cronograma', label: 'Cronograma' },
				{ href: '/caderno', icon: 'caderno', label: 'Caderno de erros' }
			]
		},
		{
			titulo: 'Meu concurso',
			itens: [
				{ href: '/conteudo', icon: 'conteudo', label: 'Conteúdo programático' },
				{ href: '/datas', icon: 'datas', label: 'Datas do edital' }
			]
		},
		{
			titulo: 'Acompanhar',
			itens: [
				{ href: '/estatisticas', icon: 'estatisticas', label: 'Estatísticas' },
				{ href: '/balanceamento', icon: 'balanceamento', label: 'Balanceamento' }
			]
		},
		{
			titulo: 'Ajustes',
			itens: [
				{ href: '/concursos', icon: 'concursos', label: 'Meus concursos' },
				{ href: '/config', icon: 'config', label: 'Configurações' }
			]
		}
	];

	const ativo = $derived(concursoStore.ativo);
	const diasParaProva = $derived(ativo ? Math.max(0, diffDays(hojeISO(), ativo.prova)) : null);
	const progresso = $derived(planoStore.plano?.props.progresso ?? 0);

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		if (href === '/concursos') return page.url.pathname === '/concursos';
		return page.url.pathname.startsWith(href);
	}

	// Collapsed only applies on desktop: in the mobile drawer the labels always show.
	const compacto = $derived(railOnly && !open);

	function trocar(slug: string) {
		concursoStore.setAtivo(slug);
		open = false;
		// Land on Hoje: the previous page may not exist for the new plan, and the
		// plan store reloads for the new slug (see the layout effect).
		goto('/');
	}
</script>

<aside id="nav-principal" class="sidebar" class:open aria-label="Navegação principal">
	<!-- 1) app identity + the collapse control; 2) the active plan; 3) navigation -->
	<div class="nav-head" class:compacto={compacto}>
		{#if !compacto}
			<span class="marca">annyGo</span>
		{/if}
		<IconButton
			icon={railOnly ? 'expandir' : 'recolher'}
			label={railOnly ? 'Expandir menu lateral' : 'Recolher menu lateral'}
			onclick={() => (railOnly = !railOnly)}
		/>
	</div>

	<PlanoPicker
		planos={concursoStore.lista}
		ativoSlug={concursoStore.ativoSlug}
		carregando={!concursoStore.carregado}
		{compacto}
		onSelecionar={trocar}
		onAdicionar={() => {
			open = false;
			goto('/concursos/novo');
		}}
	/>

	<nav class="nav-groups">
		{#each grupos as g (g.titulo)}
			<div class="nav-group">
				<h2 class="nav-group-title">{g.titulo}</h2>
				<ul>
					{#each g.itens as l (l.href)}
						<li>
							<SidebarNavItem
								href={l.href}
								icon={l.icon}
								label={l.label}
								active={isActive(l.href)}
								{compacto}
								onNavigate={() => (open = false)}
							/>
						</li>
					{/each}
				</ul>
			</div>
		{/each}
	</nav>

	<div class="nav-foot">
		<div class="nav-progress" data-label="{progresso}% do plano concluído">
			<div class="ring" style="--ring-pct:{progresso}" aria-hidden="true"><i></i></div>
			<div class="nav-progress-txt">
				<strong>{diasParaProva ?? '—'}</strong>
				<span>dias p/ prova · {progresso}% feito</span>
			</div>
		</div>

		<div class="nav-user">
			<span class="nav-user-id" title="{auth.usuario?.nome} · {auth.usuario?.email}">
				{auth.usuario?.nome}
			</span>
			<button
				class="nav-sair"
				onclick={() => {
					concursoStore.limpar();
					planoStore.limpar();
					auth.logout();
				}}>Sair</button
			>
		</div>
	</div>
</aside>
