const SEM = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];

export const nf1 = new Intl.NumberFormat('pt-BR', {
	minimumFractionDigits: 1,
	maximumFractionDigits: 1
});
export const nf0 = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 });

function parts(iso: string): [number, number, number] {
	const [y, m, d] = iso.split('-').map(Number);
	return [y, m, d];
}

/** "2026-09-01" -> "01/09" */
export function fc(iso: string): string {
	const [, m, d] = parts(iso);
	return `${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')}`;
}

/** "2026-09-01" -> "ter, 01/09/2026" */
export function fl(iso: string): string {
	const [y, m, d] = parts(iso);
	const wd = new Date(Date.UTC(y, m - 1, d)).getUTCDay();
	return `${SEM[wd]}, ${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')}/${y}`;
}

export function weekdayShort(iso: string): string {
	const [y, m, d] = parts(iso);
	return SEM[new Date(Date.UTC(y, m - 1, d)).getUTCDay()];
}

export function hojeISO(): string {
	const d = new Date();
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export function diffDays(a: string, b: string): number {
	const [ay, am, ad] = parts(a);
	const [by, bm, bd] = parts(b);
	const ms = Date.UTC(by, bm - 1, bd) - Date.UTC(ay, am - 1, ad);
	return Math.round(ms / 86_400_000);
}

export const DIAS_SEMANA = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'];
export const DIAS_CURTOS = ['D', 'S', 'T', 'Q', 'Q', 'S', 'S'];
export const ORDEM_DIAS = [1, 2, 3, 4, 5, 6, 0];

export function tagStyle(cor: number): string {
	return `background:var(--c${cor}-bg);color:var(--c${cor}-tx)`;
}

export function rotulo(tipo: string): string {
	switch (tipo) {
		case 'sim':
			return 'SIMULADO';
		case 'disc':
			return 'DISCURSIVA';
		case 'vespera':
			return 'VÉSPERA';
		case 'rev':
			return 'REVISÃO — RESOLUÇÃO DE QUESTÕES';
		default:
			return 'REVISÃO';
	}
}

export function fmtMinutos(min: number): string {
	return `${min} min`;
}
