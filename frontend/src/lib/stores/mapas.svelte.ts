import { api } from '$lib/api';
import { concursoStore } from '$lib/stores/concurso.svelte';
import { assuntosDaLinha, chaveDoAssunto } from '$lib/estudo';
import type { MapaDaMateria, MapasDaMateria } from '$lib/types';

/**
 * Os mapas mentais do concurso aberto, por matéria.
 *
 * É o que o cronograma consulta para saber onde oferecer o mapa: o assunto que
 * tem mapa ganha o acesso, o que não tem, não. A carga acompanha o concurso
 * aberto, como o plano.
 *
 * O mapa é um extra: se a carga falha, o cronograma continua sem ele — o erro
 * fica no `erro` para a tela dos mapas dizer, e a próxima carga tenta de novo.
 */
class MapasStore {
	disciplinas = $state<MapasDaMateria[]>([]);
	carregado = $state(false);
	erro = $state<string | null>(null);

	private slug: string | null = null;

	/** Os mapas de uma matéria, pelo código que o cronograma mostra. */
	doCodigo(codigo: string): MapaDaMateria[] {
		return this.disciplinas.find((d) => d.codigo === codigo)?.mapas ?? [];
	}

	/**
	 * Os mapas de uma linha do cronograma: os que cobrem algum dos assuntos
	 * dela, e os que valem para a matéria inteira. É o que põe o mapa só onde
	 * se estuda o assunto dele.
	 */
	doTema(codigo: string, tema: string): MapaDaMateria[] {
		const assuntos = new Set(assuntosDaLinha(tema));
		return this.doCodigo(codigo).filter(
			(m) => m.materiaInteira || m.temas.some((t) => assuntos.has(chaveDoAssunto(t)))
		);
	}

	/** Os mapas escolhidos para um tópico da ementa (sem os da matéria inteira). */
	doTopico(codigo: string, topico: string): MapaDaMateria[] {
		const k = chaveDoAssunto(topico);
		return this.doCodigo(codigo).filter((m) => m.temas.some((t) => chaveDoAssunto(t) === k));
	}

	async carregar(force = false) {
		const slug = concursoStore.ativoSlug;
		if (!slug) {
			this.limpar();
			return;
		}
		if (this.slug === slug && this.carregado && !force) return;

		// Outro concurso: o que havia era de lá.
		if (this.slug !== slug) {
			this.disciplinas = [];
			this.carregado = false;
		}
		this.slug = slug;

		try {
			const res = await api.mapasDoConcurso(slug);
			// Uma resposta que chega depois de trocar de concurso é do concurso velho.
			if (concursoStore.ativoSlug !== slug) return;
			this.disciplinas = res.disciplinas;
			this.carregado = true;
			this.erro = null;
		} catch (e) {
			this.erro = e instanceof Error ? e.message : 'Não foi possível carregar os mapas mentais';
		}
	}

	limpar() {
		this.disciplinas = [];
		this.carregado = false;
		this.erro = null;
		this.slug = null;
	}
}

export const mapasStore = new MapasStore();
