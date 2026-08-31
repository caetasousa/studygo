<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import type { NavIconName } from './NavIcon.svelte';

	/**
	 * One row of the sidebar. Every row is this component, so icon box, label
	 * offset and row height cannot drift apart between items.
	 *
	 * The active state is carried by background, colour, a slightly heavier label
	 * and a small leading bar — never by growing the icon, which would make one
	 * row read as a different size from its neighbours.
	 */
	let {
		href,
		icon,
		label,
		active = false,
		/** Collapsed rail: the label is hidden and shown as a tooltip instead. */
		compacto = false,
		onNavigate
	}: {
		href: string;
		icon: NavIconName;
		label: string;
		active?: boolean;
		compacto?: boolean;
		onNavigate?: () => void;
	} = $props();
</script>

<a
	class="nav-item"
	class:active
	class:compacto
	{href}
	data-label={label}
	aria-current={active ? 'page' : undefined}
	onclick={onNavigate}
>
	<span class="nav-ico" aria-hidden="true"><NavIcon name={icon} size="nav" /></span>
	<span class="nav-label">{label}</span>
</a>

<style>
	.nav-item {
		display: grid;
		/* icon box | label — fixed first column keeps every label on the same x */
		grid-template-columns: var(--icon-box) minmax(0, 1fr);
		align-items: center;
		gap: 11px;
		min-height: 34px;
		padding: 0 10px;
		border-radius: 8px;
		text-decoration: none;
		font-size: 13.5px;
		font-weight: 500;
		color: var(--text-muted);
		position: relative;
	}
	/* The icon is centred in its box, so glyphs of different internal shape still
	   line up with each other and with the text. */
	.nav-ico {
		display: grid;
		place-items: center;
		width: var(--icon-box);
		height: var(--icon-box);
		color: inherit;
	}
	.nav-label {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.nav-item:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.nav-item:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.nav-item.active {
		background: var(--accent-soft);
		color: var(--accent-strong);
		font-weight: 600;
	}
	.nav-item.active::before {
		content: '';
		position: absolute;
		left: 0;
		top: 7px;
		bottom: 7px;
		width: 3px;
		border-radius: 0 3px 3px 0;
		background: var(--accent-strong);
	}

	/* Collapsed rail: only the icon shows, and the label becomes the tooltip so
	   the row stays identifiable without it. */
	.compacto {
		grid-template-columns: var(--icon-box);
		justify-content: center;
		padding: 0 7px;
	}
	.compacto .nav-label {
		display: none;
	}
	.compacto::after {
		content: attr(data-label);
		position: absolute;
		left: calc(100% + 8px);
		top: 50%;
		transform: translateY(-50%);
		background: var(--text);
		color: var(--bg);
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
		padding: 5px 9px;
		border-radius: 6px;
		box-shadow: var(--shadow-pop);
		opacity: 0;
		pointer-events: none;
		transition: opacity 0.12s ease;
		z-index: 50;
	}
	.compacto:hover::after,
	.compacto:focus-visible::after {
		opacity: 1;
	}
</style>
