import type { ConteudoInputItem, DisciplinaInput } from '$lib/types';

const ROTULO: Record<'esp' | 'ger', string> = {
	esp: 'Conhecimentos específicos',
	ger: 'Conhecimentos gerais'
};

// sintetizarConteudo renders the disciplines' temas as the artifact's block list
// (rot = group, h = discipline, p = topic), so the syllabus text is persisted
// alongside the structured temas the plan engine consumes.
export function sintetizarConteudo(disciplinas: DisciplinaInput[]): ConteudoInputItem[] {
	const out: ConteudoInputItem[] = [];

	for (const bloco of ['esp', 'ger'] as const) {
		const doGrupo = disciplinas.filter((d) => d.bloco === bloco && d.temas.length > 0);
		if (doGrupo.length === 0) continue;

		out.push({ tipo: 'rot', texto: ROTULO[bloco] });

		for (const d of doGrupo) {
			out.push({ tipo: 'h', texto: d.nome });
			for (const t of d.temas) out.push({ tipo: 'p', texto: t });
		}
	}

	return out;
}
