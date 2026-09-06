export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number) {
	let t: ReturnType<typeof setTimeout> | undefined;
	return (...args: A) => {
		clearTimeout(t);
		t = setTimeout(() => fn(...args), ms);
	};
}

export function parseInteger(v: string): number | null {
	const n = parseInt(v, 10);
	return Number.isNaN(n) ? null : n;
}
