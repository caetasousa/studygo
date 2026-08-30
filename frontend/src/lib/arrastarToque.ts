/**
 * Press-and-hold dragging for touch.
 *
 * The HTML5 drag-and-drop API does not fire on touch, so on a phone the only
 * way to move a subject used to be a menu. This action adds the gesture people
 * expect there: hold a row for a moment, it lifts, drag it over another row,
 * let go.
 *
 * It deliberately does NOT replace the native drag on pointers that support it
 * (mouse, trackpad, pen) — that one is better behaved, and the browser draws
 * its own drag image.
 *
 * The hold delay exists so a scroll never turns into a drag: moving more than a
 * few pixels before it elapses cancels the gesture and lets the page scroll.
 */

const ATRASO_MS = 320;
/** Movement above this before the hold elapses is a scroll, not a drag. */
const TOLERANCIA_PX = 10;

export interface ArrastarToqueOpts {
	/** false disables the gesture entirely (a day already concluded, say). */
	ativo?: boolean;
	/** the row lifted off */
	onInicio: () => void;
	/** pointer moved over this client point while dragging */
	onMover: (x: number, y: number) => void;
	/** released over this client point */
	onSoltar: (x: number, y: number) => void;
	/** gesture abandoned without a drop */
	onCancelar: () => void;
}

export function arrastarToque(node: HTMLElement, opts: ArrastarToqueOpts) {
	let atual = opts;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let arrastando = false;
	let origemX = 0;
	let origemY = 0;
	let pointerId: number | null = null;

	function limpar() {
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
	}

	function encerrar() {
		limpar();

		if (pointerId !== null && node.hasPointerCapture?.(pointerId)) {
			node.releasePointerCapture(pointerId);
		}

		pointerId = null;
		arrastando = false;
	}

	function onPointerDown(e: PointerEvent) {
		// Only touch and pen: the mouse keeps the native drag.
		if (e.pointerType === 'mouse' || atual.ativo === false) return;

		origemX = e.clientX;
		origemY = e.clientY;
		pointerId = e.pointerId;

		timer = setTimeout(() => {
			arrastando = true;
			node.setPointerCapture?.(e.pointerId);

			// A short buzz confirms the lift where the platform offers one.
			navigator.vibrate?.(15);
			atual.onInicio();
		}, ATRASO_MS);
	}

	function onPointerMove(e: PointerEvent) {
		if (pointerId !== e.pointerId) return;

		if (!arrastando) {
			// Still waiting for the hold: a real move means the user is scrolling.
			const dx = Math.abs(e.clientX - origemX);
			const dy = Math.abs(e.clientY - origemY);

			if (dx > TOLERANCIA_PX || dy > TOLERANCIA_PX) limpar();

			return;
		}

		// Dragging: stop the page from scrolling under the finger.
		e.preventDefault();
		atual.onMover(e.clientX, e.clientY);
	}

	function onPointerUp(e: PointerEvent) {
		if (pointerId !== e.pointerId) return;

		const estava = arrastando;
		const { clientX, clientY } = e;

		encerrar();

		if (estava) atual.onSoltar(clientX, clientY);
	}

	function onPointerCancel(e: PointerEvent) {
		if (pointerId !== e.pointerId) return;

		const estava = arrastando;
		encerrar();

		if (estava) atual.onCancelar();
	}

	node.addEventListener('pointerdown', onPointerDown);
	node.addEventListener('pointermove', onPointerMove, { passive: false });
	node.addEventListener('pointerup', onPointerUp);
	node.addEventListener('pointercancel', onPointerCancel);

	return {
		update(novo: ArrastarToqueOpts) {
			atual = novo;
		},
		destroy() {
			encerrar();
			node.removeEventListener('pointerdown', onPointerDown);
			node.removeEventListener('pointermove', onPointerMove);
			node.removeEventListener('pointerup', onPointerUp);
			node.removeEventListener('pointercancel', onPointerCancel);
		}
	};
}

/**
 * The activity row under a screen point, as its drop payload.
 *
 * Touch has no dragover, so the drop target is resolved from coordinates at
 * release. Rows publish their identity through data attributes.
 */
export function alvoNoPonto(
	x: number,
	y: number
): { data: string; posicao: number } | null {
	const el = document.elementFromPoint(x, y);
	const alvo = el?.closest<HTMLElement>('[data-atv-dia][data-atv-pos]');

	if (!alvo) return null;

	const data = alvo.dataset.atvDia;
	const pos = Number(alvo.dataset.atvPos);

	if (!data || !Number.isInteger(pos)) return null;

	return { data, posicao: pos };
}
