import { request } from '$lib/api';
import type {
	Anotacao,
	Importacao,
	ImportacaoResumo,
	MateriaSugerida,
	Origem,
	Prova,
	ProvaResumo,
	QuestaoAvulsa,
	Rascunho
} from './types';

const imp = '/api/provas/importacoes';

/** O tamanho da página do catálogo no servidor (service.PorPaginaCatalogo). */
const POR_PAGINA_DO_CATALOGO = 20;

function comVersao(versao: number): RequestInit {
	return { method: 'POST', body: JSON.stringify({ versao }) };
}

export const provasApi = {
	/** A tela só usa isto para mostrar ou esconder a curadoria; quem decide o
	 *  acesso é o servidor. */
	souCurador: async () => !!(await request<{ curadorProvas?: boolean }>('/api/me')).curadorProvas,

	/** Provas publicadas, todas: o catálogo é curado à mão e cabe numa tela. */
	todasAsProvas: async (): Promise<ProvaResumo[]> => {
		const todas: ProvaResumo[] = [];
		for (;;) {
			const pagina = await request<ProvaResumo[]>(`/api/provas?offset=${todas.length}`);
			todas.push(...pagina);
			if (pagina.length < POR_PAGINA_DO_CATALOGO) return todas;
		}
	},

	/** Questões para treinar por matéria; sem matéria nem ano, o catálogo inteiro. */
	questoes: (disciplinas: string[] = [], ano = 0) => {
		const q = new URLSearchParams();
		for (const d of disciplinas) q.append('disciplina', d);
		if (ano) q.set('ano', String(ano));
		return request<QuestaoAvulsa[]>(`/api/provas/questoes?${q}`);
	},

	prova: (id: string) => request<Prova>(`/api/provas/${id}`),

	revisar: (id: string) => request<Importacao>(`/api/provas/${id}/revisar`, { method: 'POST' }),

	/** Revisão nova extraída do zero, com o extrator atual. */
	reextrair: (id: string) => request<Importacao>(`/api/provas/${id}/reextrair`, { method: 'POST' }),

	retirar: (id: string) => request<void>(`/api/provas/${id}`, { method: 'DELETE' }),

	/** As anotações do estudante nas questões da prova. */
	anotacoes: (provaId: string) => request<Anotacao[]>(`/api/provas/anotacoes/${provaId}`),

	/** Grava a anotação da questão; texto vazio apaga. */
	anotar: (provaId: string, numero: number, texto: string) =>
		request<Anotacao>(`/api/provas/anotacoes/${provaId}/${numero}`, {
			method: 'PUT',
			body: JSON.stringify({ texto })
		}),

	importacoes: () => request<ImportacaoResumo[]>(imp),

	importar: (prova: File, gabarito: File | null) => {
		const corpo = new FormData();
		corpo.set('prova', prova);
		if (gabarito) corpo.set('gabarito', gabarito);
		return request<Importacao>(imp, { method: 'POST', body: corpo });
	},

	importacao: (id: string) => request<Importacao>(`${imp}/${id}`),

	salvar: (id: string, versao: number, rascunho: Rascunho) =>
		request<Importacao>(`${imp}/${id}`, {
			method: 'PATCH',
			body: JSON.stringify({ versao, rascunho })
		}),

	publicar: (id: string, versao: number) =>
		request<{ provaId: string }>(`${imp}/${id}/publicar`, comVersao(versao)),

	cancelar: (id: string, versao: number) =>
		request<Importacao>(`${imp}/${id}/cancelar`, comVersao(versao)),

	reprocessar: (id: string, versao: number) =>
		request<Importacao>(`${imp}/${id}/reprocessar`, comVersao(versao)),

	/** Volta à fila só para reler as questões incompletas. */
	reler: (id: string, versao: number) => request<Importacao>(`${imp}/${id}/reler`, comVersao(versao)),

	/** Compara de novo com as provas publicadas do concurso e troca por referência as que elas já têm. */
	procurarCadastradas: (id: string, versao: number) =>
		request<Importacao>(`${imp}/${id}/cadastradas`, comVersao(versao)),

	excluir: (id: string, versao: number) => request<void>(`${imp}/${id}/excluir`, comVersao(versao)),

	/** PNG de um retângulo do original. O mesmo retângulo devolve o mesmo id. */
	recortar: (id: string, versao: number, origem: Origem) =>
		request<{ arquivo: string }>(`${imp}/${id}/recortar`, {
			method: 'POST',
			body: JSON.stringify({ versao, origem })
		}),

	/** A matéria que a IA sugere para cada questão do rascunho salvo. Não grava. */
	sugerirMaterias: (id: string) =>
		request<MateriaSugerida[]>(`${imp}/${id}/materias`, { method: 'POST' }),

	atualizarGabarito: (id: string, versao: number, gabarito: File) => {
		const corpo = new FormData();
		corpo.set('gabarito', gabarito);
		corpo.set('versao', String(versao));
		return request<Importacao>(`${imp}/${id}/gabarito`, { method: 'POST', body: corpo });
	}
};
