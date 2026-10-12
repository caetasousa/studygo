/**
 * O que da ementa ainda não tem mapa mental.
 *
 * Como isto quebra (escrito antes do código):
 * - o tópico escolhido num mapa continua como "sem mapa", porque a ementa o
 *   escreve com a numeração da frente ("1.2 Relevo") e o mapa sem ela;
 * - o mapa que vale para a matéria inteira dá todos os tópicos por cobertos,
 *   e a lista some sem que ninguém tenha escolhido tópico algum;
 * - a matéria sem ementa conta como coberta (0 de 0) ou como pendente;
 * - a conta geral soma tópicos repetidos, ou perde a ordem da ementa.
 */
import { describe, expect, it } from 'vitest';
import { cobertura } from './cobertura';
import type { MapaDaMateria, MapasDaMateria } from '$lib/types';

const mapa = (slug: string, temas: string[]): MapaDaMateria => ({
	slug,
	titulo: slug,
	fonte: '',
	materia: '',
	ramos: 1,
	itens: 1,
	importadoEm: '',
	materiaInteira: temas.length === 0,
	temas
});

const materia = (codigo: string, temas: string[], mapas: MapaDaMateria[] = []): MapasDaMateria => ({
	disciplinaId: codigo.toLowerCase(),
	codigo,
	nome: codigo,
	temas,
	mapas
});

describe('cobertura', () => {
	it('tira da lista o tópico escolhido num mapa, mesmo com a numeração da ementa', () => {
		const c = cobertura([
			materia('GEO', ['1.1 Ciclo hidrológico', '1.2 Relevo e solos'], [mapa('relevo', ['Relevo e solos'])])
		]);
		expect(c.materias[0].faltam).toEqual(['1.1 Ciclo hidrológico']);
		expect(c.materias[0].cobertos).toBe(1);
		expect(c).toMatchObject({ topicos: 2, cobertos: 1 });
	});

	it('o mapa da matéria inteira não cobre tópico nenhum, mas fica dito', () => {
		const c = cobertura([materia('DIR', ['Princípios', 'Poderes'], [mapa('constituicao', [])])]);
		expect(c.materias[0].faltam).toEqual(['Princípios', 'Poderes']);
		expect(c.materias[0].daMateriaInteira.map((m) => m.slug)).toEqual(['constituicao']);
	});

	it('a matéria sem ementa fica de fora da conta', () => {
		const c = cobertura([materia('SEM', []), materia('GEO', ['Relevo'])]);
		expect(c.materias.map((m) => m.codigo)).toEqual(['GEO']);
		expect(c).toMatchObject({ topicos: 1, cobertos: 0 });
	});

	it('o tópico repetido na ementa conta uma vez só', () => {
		const c = cobertura([materia('GEO', ['Relevo', 'relevo ', 'Clima'], [mapa('m', ['Relevo'])])]);
		expect(c.materias[0]).toMatchObject({
			topicos: 2,
			cobertos: 1,
			faltam: ['Clima']
		});
	});
});
