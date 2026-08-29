import type { Disciplina } from '$lib/types';

// linkQuestoes builds the discipline's question-bank URL for a topic. A source
// of tipo "questoes" may carry a {tema} placeholder — that is how a TEC caderno
// filtered by assunto gets built from the day's topic.
export function linkQuestoes(d: Disciplina | undefined, tema: string): string {
	const fonte = d?.fontes?.find((f) => f.tipo === 'questoes' && f.url);
	if (!fonte) return '';
	return fonte.url.replaceAll('{tema}', encodeURIComponent(tema));
}
