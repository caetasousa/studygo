import { chaveDoAssunto } from '$lib/estudo';
import type { MapaDaMateria, MapasDaMateria } from '$lib/types';

/** Uma matéria e o que da ementa dela ainda não tem mapa. */
export interface CoberturaDaMateria extends MapasDaMateria {
	/** Os tópicos da ementa, sem repetição. */
	topicos: number;
	cobertos: number;
	/** Os tópicos sem mapa, na ordem e com o texto da ementa. */
	faltam: string[];
	/** Os mapas vinculados sem tópico escolhido: não cobrem nada aqui, e a tela diz. */
	daMateriaInteira: MapaDaMateria[];
}

export interface Cobertura {
	materias: CoberturaDaMateria[];
	topicos: number;
	cobertos: number;
}

/**
 * Quanto da ementa do concurso já tem mapa mental.
 *
 * Coberto é o tópico que algum mapa escolheu — o mesmo critério que marca a
 * ementa (`mapasStore.doTopico`). O mapa que vale para a matéria inteira não
 * conta: ninguém disse que tópicos ele cobre, e dar a matéria por coberta
 * esconderia justamente o que falta.
 */
export function cobertura(disciplinas: MapasDaMateria[]): Cobertura {
	const materias = disciplinas
		.filter((d) => d.temas.length > 0)
		.map((d): CoberturaDaMateria => {
			const escolhidos = new Set(d.mapas.flatMap((m) => m.temas.map(chaveDoAssunto)));
			const vistos = new Set<string>();
			const faltam: string[] = [];
			let cobertos = 0;
			for (const t of d.temas) {
				const k = chaveDoAssunto(t);
				if (vistos.has(k)) continue;
				vistos.add(k);
				if (escolhidos.has(k)) cobertos++;
				else faltam.push(t);
			}
			return {
				...d,
				topicos: vistos.size,
				cobertos,
				faltam,
				daMateriaInteira: d.mapas.filter((m) => m.materiaInteira)
			};
		});

	return {
		materias,
		topicos: materias.reduce((s, m) => s + m.topicos, 0),
		cobertos: materias.reduce((s, m) => s + m.cobertos, 0)
	};
}
