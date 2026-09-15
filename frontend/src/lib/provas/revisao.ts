// Regras da tela de revisão que não dependem de componente: onde um recorte
// pode ser aplicado, qual é a próxima pendência, como a resposta anda junto do
// gabarito. Quem decide se a prova pode ser publicada é o servidor (as
// `pendencias` da importação); aqui é só navegação e edição.

import type { Apoio, Bloco, Origem, Questao, Rascunho, TipoBloco } from './types';

export const LETRAS = ['A', 'B', 'C', 'D', 'E'] as const;

export function novoBloco(tipo: TipoBloco = 'texto'): Bloco {
	return { tipo, texto: '', formato: '', arquivo: '', descricao: '', origem: null, revisado: false, largura: 0 };
}

/** Questão em branco, para transcrever uma que a extração perdeu. */
export function novaQuestao(numero: number, origem?: Origem): Questao {
	return {
		numero,
		disciplina: '',
		blocos: [novoBloco()],
		alternativas: LETRAS.map((letra) => ({ letra, blocos: [novoBloco()] })),
		apoios: [],
		origens: origem ? [origem] : [],
		resposta: '',
		situacao: '',
		revisada: false,
		completa: false,
		igualA: ''
	};
}

/** Números de 1 ao total que não têm questão no rascunho. */
export function numerosFaltando(r: Rascunho): number[] {
	const presentes = new Set(r.questoes.map((q) => q.numero));
	const faltam: number[] = [];
	for (let n = 1; n <= r.total; n++) if (!presentes.has(n)) faltam.push(n);
	return faltam;
}

function blocosDaQuestao(q: Questao): Bloco[] {
	return [...q.blocos, ...q.alternativas.flatMap((a) => a.blocos)];
}

/**
 * A questão tem figura, no enunciado ou numa alternativa. Questão com figura
 * nunca é confirmada sozinha: só olhando o original se sabe se o recorte pegou
 * a figura inteira.
 */
export function temFigura(q: Questao): boolean {
	return blocosDaQuestao(q).some((b) => b.tipo === 'imagem');
}

/** O que ainda falta o curador fazer nesta questão. */
export function questaoPendente(q: Questao): boolean {
	return (
		!q.revisada ||
		!q.completa ||
		q.alternativas.length !== LETRAS.length ||
		blocosDaQuestao(q).some((b) => b.tipo === 'imagem' && (!b.arquivo || !b.revisado))
	);
}

/**
 * Incompleta como o servidor a vê para reler: sem as cinco alternativas, ou
 * marcada como cortada pela extração.
 */
export function incompleta(q: Questao): boolean {
	return !q.completa || q.alternativas.length !== LETRAS.length;
}

function vazio(blocos: Bloco[]): boolean {
	return !blocos.some((b) => b.tipo === 'imagem' || b.texto.trim());
}

/**
 * O que está errado na questão, em frases que dizem o que fazer. Não inclui a
 * falta de conferência: isso é trabalho a fazer, não defeito.
 */
export function problemasDaQuestao(q: Questao, r: Rascunho): string[] {
	const out: string[] = [];
	if (vazio(q.blocos)) out.push('Está sem enunciado.');
	if (q.alternativas.length !== LETRAS.length) {
		const faltam = LETRAS.filter((l) => !q.alternativas.some((a) => a.letra === l));
		out.push(
			`Tem ${q.alternativas.length} de 5 alternativas` + (faltam.length ? `; faltam ${faltam.join(', ')}.` : '.')
		);
	}
	for (const a of q.alternativas) if (vazio(a.blocos)) out.push(`A alternativa ${a.letra} está vazia.`);
	if (!q.completa) out.push('A extração marcou a questão como cortada.');
	if (blocosDaQuestao(q).some((b) => b.tipo === 'imagem' && !b.arquivo))
		out.push('Tem figura sem recorte: recorte-a no original.');
	const oficial = r.gabarito.respostas[String(q.numero)];
	if (Object.keys(r.gabarito.respostas).length > 0 && oficial !== undefined && q.resposta !== oficial)
		out.push(`A resposta (${q.resposta || 'nenhuma'}) difere do gabarito (${oficial || 'anulada'}).`);
	if (citaTexto(q) && !q.apoios.some((id) => r.apoios.some((a) => a.id === id)))
		out.push('Cita um texto, mas nenhum texto de apoio está ligado a ela.');
	return out;
}

/** O nome da região numa lista: releituras e trechos dizem de qual questão são. */
export function rotuloDaRegiao(regiao: Origem, indice: number, total: number): string {
	const releitura = regiao.regiao.match(/^q(\d+)$/);
	if (releitura) return `Página ${regiao.pagina} · releitura da questão ${releitura[1]}`;
	const trecho = regiao.regiao.match(/^t(\d+)$/);
	if (trecho) return `Página ${regiao.pagina} · trecho marcado da questão ${trecho[1]}`;
	return `Página ${regiao.pagina} · região ${indice + 1} de ${total}`;
}

/** O trecho que o curador marcou para reler uma questão (prova.ETrecho no servidor). */
export function eTrecho(o: Origem): boolean {
	return /^t\d+$/.test(o.regiao);
}

/** Faixa do caderno — não a releitura nem o trecho de uma questão, que são pedaços dela. */
export function regiaoDoCaderno(o: Origem): boolean {
	return !/^[qt]\d+$/.test(o.regiao);
}

/**
 * Em que região começar a marcar o trecho da questão: a faixa do caderno da
 * página em que ela foi lida, a que contém o meio dela. Sem origem, a primeira.
 */
export function regiaoDoTrecho(regioes: Origem[], q: Questao | undefined): number {
	const faixas = regioes.flatMap((r, i) => (regiaoDoCaderno(r) ? [i] : []));
	const o = q?.origens.find((x) => x.retangulo.length === 4);
	if (!o) return faixas[0] ?? 0;
	const meio = (o.retangulo[1] + o.retangulo[3]) / 2;
	const daPagina = faixas.filter((i) => regioes[i].pagina === o.pagina);
	const comOMeio = daPagina.find((i) => regioes[i].retangulo[1] <= meio && meio <= regioes[i].retangulo[3]);
	return comOMeio ?? daPagina[0] ?? faixas[0] ?? 0;
}

/**
 * O retângulo com que o trecho começa: do alto da questão até onde começa a
 * seguinte na mesma página — ou o pé da região —, na largura da região. A área
 * que a extração deu é só o que ela leu: na 60 do TRT-15, que veio sem
 * alternativas, era só o enunciado, e o trecho que começava nela relia o mesmo
 * pedaço. A largura é a da região porque a do modelo erra para os lados.
 */
export function trechoInicial(regiao: Origem, q: Questao | undefined, questoes: Questao[] = []): number[] {
	const [x0, y0, x1, y1] = regiao.retangulo;
	const o = q?.origens.find((x) => x.pagina === regiao.pagina && x.retangulo.length === 4);
	if (!q || !o) return [x0, y0, x1, y1];
	const inicio = o.retangulo[1];
	const seguintes = questoes
		.filter((x) => x.numero > q.numero)
		.flatMap((x) => x.origens)
		.filter((p) => p.pagina === regiao.pagina && p.retangulo.length === 4 && p.retangulo[1] > inicio)
		.map((p) => p.retangulo[1]);
	// A área do modelo erra por dezenas de pontos — a da 25 do TJCE começava
	// antes do fim da (E) da 24 —, então o trecho sobe um pouco e passa um pouco
	// do começo da seguinte. A ponta da vizinha não atrapalha: vale a questão
	// com o número pedido.
	const altura = y1 - y0;
	const topo = Math.max(y0, inicio - FOLGA_ACIMA * altura);
	const pe = Math.min(y1, Math.min(...seguintes) + FOLGA_ABAIXO * altura);
	return pe - topo >= 4 ? [x0, topo, x1, pe] : [x0, y0, x1, y1];
}

/**
 * O retângulo que vai ao servidor: ordenado, dentro da região, com tamanho, e
 * em centésimos de ponto. Arredondar pode passar da borda — 841,9199 (o pé da
 * folha A4) vira 841,92 —, e o processador recusava o retângulo "fora da
 * página": o trecho da 60 do TRT-15, no pé da página, nunca era lido. Depois de
 * arredondar, ele volta para dentro. Nulo se não sobrar tamanho.
 */
export function retanguloParaEnviar(rect: number[], limite: number[]): number[] | null {
	const dentro = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v));
	const arred = (v: number) => Math.round(v * 100) / 100;
	const [x0, x1] = [Math.min(rect[0], rect[2]), Math.max(rect[0], rect[2])];
	const [y0, y1] = [Math.min(rect[1], rect[3]), Math.max(rect[1], rect[3])];
	const r = [
		dentro(arred(x0), limite[0], limite[2]),
		dentro(arred(y0), limite[1], limite[3]),
		dentro(arred(x1), limite[0], limite[2]),
		dentro(arred(y1), limite[1], limite[3])
	];
	return r[2] - r[0] >= 4 && r[3] - r[1] >= 4 ? r : null;
}

/** Folgas do trecho inicial, em fração da altura da região. */
const FOLGA_ACIMA = 0.01;
const FOLGA_ABAIXO = 0.04;

/** Índice da próxima questão pendente depois de `atual`, dando a volta; -1 se não houver. */
export function proximaPendente(questoes: Questao[], atual: number): number {
	for (let passo = 1; passo <= questoes.length; passo++) {
		const i = (atual + passo) % questoes.length;
		if (questaoPendente(questoes[i])) return i;
	}
	return -1;
}

/**
 * A resposta da questão e a do gabarito andam juntas: o servidor recusa a
 * publicação quando elas divergem, porque a resposta só pode vir do oficial.
 */
export function definirResposta(r: Rascunho, q: Questao, letra: string) {
	q.resposta = letra;
	r.gabarito.respostas[String(q.numero)] = letra;
	q.revisada = false;
}

/** Índice, em `regioes`, da primeira região em que a questão foi lida. */
export function regiaoDaQuestao(regioes: Origem[], q: Questao | undefined): number {
	const origem = q?.origens[0];
	if (!origem) return 0;
	return Math.max(
		0,
		regioes.findIndex((r) => r.regiao === origem.regiao && r.pagina === origem.pagina)
	);
}

export interface Destino {
	chave: string;
	rotulo: string;
}

/**
 * Para onde um recorte pode ir: substituir uma figura que já existe na
 * questão ou nos materiais de apoio dela, ou virar uma figura nova.
 */
export function destinosDoRecorte(q: Questao, apoios: Apoio[]): Destino[] {
	const out: Destino[] = [];
	q.blocos.forEach((b, j) => {
		if (b.tipo === 'imagem') out.push({ chave: `q:${j}`, rotulo: `Enunciado · figura ${j + 1}` });
	});
	q.alternativas.forEach((a, i) =>
		a.blocos.forEach((b, j) => {
			if (b.tipo === 'imagem')
				out.push({ chave: `a:${i}:${j}`, rotulo: `Alternativa ${a.letra} · figura ${j + 1}` });
		})
	);
	for (const apoio of apoios.filter((a) => q.apoios.includes(a.id))) {
		apoio.blocos.forEach((b, j) => {
			if (b.tipo === 'imagem')
				out.push({ chave: `p:${apoio.id}:${j}`, rotulo: `Apoio ${apoio.id} · figura ${j + 1}` });
		});
	}
	out.push({ chave: 'novo:q', rotulo: 'Nova figura no enunciado' });
	q.alternativas.forEach((a, i) =>
		out.push({ chave: `novo:a:${i}`, rotulo: `Nova figura na alternativa ${a.letra}` })
	);
	for (const apoio of apoios.filter((a) => q.apoios.includes(a.id))) {
		out.push({ chave: `novo:p:${apoio.id}`, rotulo: `Nova figura no apoio ${apoio.id}` });
	}
	return out;
}

/** O bloco que um destino já aponta, se for uma figura existente. */
export function blocoDoDestino(r: Rascunho, q: Questao, chave: string): Bloco | undefined {
	const [tipo, a, b] = chave.split(':');
	if (tipo === 'q') return q.blocos[Number(a)];
	if (tipo === 'a') return q.alternativas[Number(a)]?.blocos[Number(b)];
	if (tipo === 'p') return r.apoios.find((p) => p.id === a)?.blocos[Number(b)];
	return undefined;
}

/**
 * Aplica um recorte ao destino escolhido. A figura muda, então a conferência
 * dela e da questão caem — o servidor faria o mesmo na gravação.
 */
export function aplicarRecorte(
	r: Rascunho,
	q: Questao,
	chave: string,
	arquivo: string,
	origem: Origem
): void {
	const existente = blocoDoDestino(r, q, chave);
	if (existente) {
		existente.arquivo = arquivo;
		existente.origem = origem;
		existente.revisado = false;
	} else {
		const bloco: Bloco = { ...novoBloco('imagem'), arquivo, origem };
		const [, onde, ref] = chave.split(':');
		if (onde === 'a') q.alternativas[Number(ref)].blocos.push(bloco);
		else if (onde === 'p') {
			const apoio = r.apoios.find((p) => p.id === ref);
			if (apoio) {
				apoio.blocos.push(bloco);
				apoio.revisado = false;
			}
		} else q.blocos.push(bloco);
	}
	q.revisada = false;
}

/** O mesmo, para uma figura que já está num texto de apoio ("p:id:índice"). */
export function aplicarRecorteNoApoio(r: Rascunho, chave: string, arquivo: string, origem: Origem): void {
	const [, id, j] = chave.split(':');
	const apoio = r.apoios.find((p) => p.id === id);
	const bloco = apoio?.blocos[Number(j)];
	if (!apoio || !bloco) return;
	Object.assign(bloco, { arquivo, origem, revisado: false });
	apoio.revisado = false;
}

/**
 * Retângulo inicial do seletor: o da figura escolhida, se ela estiver nesta
 * região; senão, um quadro no meio da região para o curador arrastar.
 */
export function retanguloInicial(regiao: Origem, alvo: Bloco | undefined): number[] {
	const o = alvo?.origem;
	if (o && o.pagina === regiao.pagina && dentro(o.retangulo, regiao.retangulo)) return [...o.retangulo];
	const [x0, y0, x1, y1] = regiao.retangulo;
	const w = x1 - x0;
	const h = y1 - y0;
	return [x0 + w * 0.25, y0 + h * 0.35, x0 + w * 0.75, y0 + h * 0.65];
}

function dentro(r: number[], limite: number[]): boolean {
	return r[0] >= limite[0] && r[1] >= limite[1] && r[2] <= limite[2] && r[3] <= limite[3];
}

/**
 * Liga o material de apoio exatamente às questões `numeros`. As questões que
 * ganham ou perdem o material mudaram, então perdem a conferência.
 */
export function vincularApoio(r: Rascunho, apoio: Apoio, numeros: number[]): void {
	const quer = new Set(numeros);
	apoio.questoes = [...quer].sort((a, b) => a - b);
	apoio.revisado = false;
	for (const q of r.questoes) {
		const tem = q.apoios.includes(apoio.id);
		if (quer.has(q.numero) && !tem) {
			q.apoios.push(apoio.id);
			q.revisada = false;
		} else if (!quer.has(q.numero) && tem) {
			q.apoios = q.apoios.filter((id) => id !== apoio.id);
			q.revisada = false;
		}
	}
}

/** Texto de apoio em branco, para o que a extração não achou. */
export function novoApoio(existentes: Apoio[]): Apoio {
	let n = existentes.length + 1;
	while (existentes.some((a) => a.id === `m${n}`)) n++;
	return { id: `m${n}`, blocos: [novoBloco()], questoes: [], aviso: '', origens: [], revisado: false, igualA: '' };
}

/**
 * Os números que a frase do caderno cita: "questões de 1 a 10", "de números
 * 11 a 15", "questões 21 e 22". O mesmo que o processador lê na extração.
 */
export function numerosDoAviso(aviso: string): number[] {
	const out = new Set<number>();
	const faixa = /quest(?:ão|ões|oes)\s+(?:de\s+)?(?:n[úu]meros?\s+)?(\d{1,3})\s*(a|e|até)\s*(\d{1,3})/gi;
	for (const [, de, ligacao, ate] of aviso.matchAll(faixa)) {
		const [a, b] = [Number(de), Number(ate)];
		if (ligacao.toLowerCase() === 'e') [a, b].forEach((n) => out.add(n));
		else if (a > 0 && a <= b && b <= a + 30) for (let n = a; n <= b; n++) out.add(n);
	}
	return [...out].sort((x, y) => x - y);
}

/** Índice, em `regioes`, da região em que o texto foi lido. */
export function regiaoDoApoio(regioes: Origem[], apoio: Apoio | undefined): number {
	const origem = apoio?.origens[0];
	if (!origem) return 0;
	return Math.max(
		0,
		regioes.findIndex((r) => r.regiao === origem.regiao && r.pagina === origem.pagina)
	);
}

/** "Texto das questões 1-10": o nome que o curador e o aluno reconhecem. */
export function rotuloDoApoio(apoio: Apoio): string {
	const numeros = escreverNumeros(apoio.questoes);
	return numeros ? `Texto das questões ${numeros}` : 'Texto sem questões ligadas';
}

/** O começo do texto, para distinguir um material do outro numa lista. */
export function comecoDoApoio(apoio: Apoio, max = 70): string {
	const t = apoio.blocos
		.filter((b) => b.tipo === 'texto')
		.map((b) => b.texto)
		.join(' ')
		.replace(/\s+/g, ' ')
		.trim();
	return t.length > max ? t.slice(0, max).trimEnd() + '…' : t;
}

/**
 * O enunciado fala de um texto ("De acordo com o texto", "no texto")? Sem
 * material ligado, falta à questão o que ela pergunta. "Editor de texto" e
 * "arquivo texto" não contam.
 */
export function citaTexto(q: Questao): boolean {
	const enunciado = q.blocos.map((b) => b.texto).join(' ');
	return /\b(?:o|no|do|ao|pelo)\s+texto\b/i.test(enunciado);
}

/** Remove o material e o solta de todas as questões. */
export function removerApoio(r: Rascunho, id: string): void {
	const apoio = r.apoios.find((a) => a.id === id);
	if (apoio) vincularApoio(r, apoio, []);
	r.apoios = r.apoios.filter((a) => a.id !== id);
}

/** "1-10, 12" → [1..10, 12]. Faixas invertidas ou absurdas são ignoradas. */
export function lerNumeros(texto: string): number[] {
	const out = new Set<number>();
	const normalizado = texto.replace(/\s*(?:-|–|\ba\b)\s*/g, '-');
	for (const parte of normalizado.split(/[,;\s]+/).filter(Boolean)) {
		const faixa = parte.match(/^(\d+)-(\d+)$/);
		if (faixa) {
			const [a, b] = [Number(faixa[1]), Number(faixa[2])];
			if (a <= b && b - a <= 200) for (let n = a; n <= b; n++) out.add(n);
		} else if (/^\d+$/.test(parte)) out.add(Number(parte));
	}
	return [...out].sort((a, b) => a - b);
}

/** [1..10, 12] → "1-10, 12", para o campo editável. */
export function escreverNumeros(numeros: number[]): string {
	const ordenados = [...numeros].sort((a, b) => a - b);
	const partes: string[] = [];
	for (let i = 0; i < ordenados.length; i++) {
		let j = i;
		while (j + 1 < ordenados.length && ordenados[j + 1] === ordenados[j] + 1) j++;
		partes.push(j > i ? `${ordenados[i]}-${ordenados[j]}` : String(ordenados[i]));
		i = j;
	}
	return partes.join(', ');
}

/**
 * Aplica as matérias sugeridas. Só marca como alteradas as questões cuja
 * matéria mudou, e devolve quantas foram.
 */
export function aplicarMaterias(r: Rascunho, materias: { numero: number; materia: string }[]): number {
	const porNumero = new Map(materias.map((m) => [m.numero, m.materia.trim()]));
	let mudaram = 0;
	for (const q of r.questoes) {
		const m = porNumero.get(q.numero);
		if (m && m !== q.disciplina) {
			q.disciplina = m;
			q.revisada = false;
			mudaram++;
		}
	}
	return mudaram;
}
