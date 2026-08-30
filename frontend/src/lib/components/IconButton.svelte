<script lang="ts">
	import NavIcon from './NavIcon.svelte';
	import type { NavIconName } from './NavIcon.svelte';

	/**
	 * A button whose only content is an icon.
	 *
	 * The hit area (36px) is larger than the glyph (17px): comfortable to click
	 * without the drawing growing heavy. `label` is mandatory — it becomes both
	 * the accessible name and the tooltip, so an icon-only control is never
	 * unlabelled.
	 */
	let {
		icon,
		label,
		onclick,
		onpointerdown,
		disabled = false,
		tom = 'normal'
	}: {
		icon: NavIconName;
		/** Accessible name and tooltip. Required: describe the action, e.g. "Adiar matéria". */
		label: string;
		/** Receives the click event, so a caller can keep the trigger for focus. */
		onclick?: (e: MouseEvent & { currentTarget: HTMLButtonElement }) => void;
		/** Lets a caller stop a parent's drag from starting on this button. */
		onpointerdown?: (e: PointerEvent) => void;
		disabled?: boolean;
		tom?: 'normal' | 'perigo';
	} = $props();
</script>

<button
	type="button"
	class="icon-btn"
	class:perigo={tom === 'perigo'}
	title={label}
	aria-label={label}
	{disabled}
	{onclick}
	{onpointerdown}
	draggable="false"
>
	<NavIcon name={icon} />
</button>

<style>
	.icon-btn {
		display: grid;
		place-items: center;
		width: var(--icon-hit);
		height: var(--icon-hit);
		padding: 0;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}
	.icon-btn:hover:not(:disabled) {
		background: var(--bg-hover);
		color: var(--text);
	}
	.icon-btn:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.icon-btn:active:not(:disabled) {
		background: var(--border);
	}
	.icon-btn:disabled {
		/* still identifiable, clearly not actionable */
		color: var(--text-faint);
		opacity: 0.55;
		cursor: not-allowed;
	}
	.perigo:hover:not(:disabled) {
		color: var(--danger);
	}
</style>
