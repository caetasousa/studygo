// Dublê do edital-processor para o stack de E2E.
//
// Responde o mesmo contrato HTTP que o backend usa (ver
// backend/internal/adapter/editalproc/client.go) com uma leitura fixa de
// edital: dois cargos, disciplinas por grupo, tópicos e datas. Assim o
// assistente do edital é testado de ponta a ponta sem Gemini — sem custo, sem
// rede e com a mesma resposta a cada execução.
//
// O que ele NÃO testa é o processador de verdade (PDF, OCR, IA): isso é da
// suíte do edital-processor. Aqui só importa que o backend e a tela levem até
// o plano o que a leitura trouxe.

import { createServer } from 'node:http';
import { readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';

const TOKEN = process.env.EP_SERVICE_TOKEN ?? '';

// Datas relativas ao dia da execução: uma prova fixa no calendário cairia no
// passado um dia, e o assistente recusaria o concurso.
function emDias(dias) {
	const d = new Date(Date.now() + dias * 86_400_000);
	return new Intl.DateTimeFormat('en-CA', { timeZone: 'America/Sao_Paulo' }).format(d);
}

const CARGOS = [
	{ codigo: 'A01', nome: 'Analista Judiciário', especialidade: 'Tecnologia da Informação', escolaridade: 'superior', totalVagas: 3 },
	{ codigo: 'B02', nome: 'Técnico Judiciário', especialidade: 'Área Administrativa', escolaridade: 'médio', totalVagas: 12 }
];

const disciplina = (nome, numeroQuestoes) => ({ nome, numeroQuestoes, peso: null });

// A estrutura muda com o cargo: é o que prova que o assistente pediu a do
// cargo escolhido, e não a do primeiro da lista.
const ESTRUTURA = {
	A01: {
		gerais: [disciplina('Língua Portuguesa', 20), disciplina('Raciocínio Lógico', 10)],
		especificas: [disciplina('Engenharia de Software', 25), disciplina('Banco de Dados', 15)]
	},
	B02: {
		gerais: [disciplina('Língua Portuguesa', 25), disciplina('Matemática', 15)],
		especificas: [disciplina('Direito Administrativo', 20), disciplina('Arquivologia', 10)]
	}
};

const TEMAS = {
	'Língua Portuguesa': ['Crase', 'Concordância verbal', 'Regência nominal'],
	'Raciocínio Lógico': ['Proposições', 'Tabela-verdade'],
	'Engenharia de Software': ['Scrum', 'Testes de software', 'Padrões de projeto'],
	'Banco de Dados': ['Normalização', 'SQL'],
	Matemática: ['Porcentagem', 'Regra de três'],
	'Direito Administrativo': ['Atos administrativos', 'Licitações'],
	Arquivologia: ['Gestão de documentos']
};

function estrutura(codigo) {
	const cargo = CARGOS.find((c) => c.codigo === codigo || c.nome === codigo) ?? CARGOS[0];
	const e = ESTRUTURA[cargo.codigo];
	const soma = (ds) => ds.reduce((t, d) => t + d.numeroQuestoes, 0);
	return {
		nomeSugerido: `TRT E2E — ${cargo.nome}`,
		dataProva: emDias(75),
		gruposGerais: [{ kind: 'ger', rotulo: 'Conhecimentos Gerais', totalQuestoes: soma(e.gerais), peso: 1, pesoScope: 'group', disciplinas: e.gerais }],
		gruposEspecificos: [{ kind: 'esp', rotulo: 'Conhecimentos Específicos', totalQuestoes: soma(e.especificas), peso: 2, pesoScope: 'group', disciplinas: e.especificas }],
		provaDiscursiva: [],
		duracao: { minutos: 240, scope: 'exam_set' },
		cronograma: [
			{ dataInicio: emDias(10), dataFim: emDias(20), titulo: 'Inscrições', exigeAcao: true },
			{ dataInicio: emDias(25), dataFim: '', titulo: 'Pagamento da taxa', exigeAcao: true },
			{ dataInicio: emDias(75), dataFim: '', titulo: 'Prova objetiva', exigeAcao: false }
		],
		alerts: []
	};
}

// A captura de leis: o link diz qual lei da fixture devolver, e as marcas no
// caminho produzem os casos que a tela precisa mostrar. A fonte é "do
// Planalto" para passar pela mesma regra de fonte oficial do processador real.
const LEIS = {
	v1: JSON.parse(readFileSync(new URL('./fixtures/lei-exemplo.json', import.meta.url), 'utf8')),
	v2: JSON.parse(readFileSync(new URL('./fixtures/lei-exemplo-v2.json', import.meta.url), 'utf8'))
};
const capturas = new Map();

function oficial(link) {
	try {
		const u = new URL(link);
		return u.protocol === 'https:' && /\.(gov|leg|jus)\.br$/.test(u.hostname) && !u.port && !u.username;
	} catch {
		return false;
	}
}

const AGRUPAMENTOS = ['parte', 'livro', 'titulo', 'capitulo', 'secao', 'subsecao'];

// O número da lei vem no link (lei-exemplo-12.345.htm): cada teste tem a sua,
// e o catálogo, que é de todos, não mistura as leis de testes diferentes.
function numeroDoLink(link) {
	return new URL(link).pathname.match(/(\d{2}\.\d{3})/)?.[1] ?? null;
}

function comNumero(dispositivos, numero) {
	if (!numero) return dispositivos;
	return dispositivos.map((d) => (d.ref === 'preambulo1' ? { ...d, texto: d.texto.replace('99.999', numero) } : d));
}

/** O que o recorte guarda: sob as raízes, as divisões acima e a epígrafe. */
function cortar(dispositivos, raizes) {
	const pais = new Map(dispositivos.map((d) => [d.ref, d.pai]));
	const guardar = new Set(['preambulo1']);
	for (const d of dispositivos) {
		for (let ref = d.ref; ref; ref = pais.get(ref)) {
			if (raizes.includes(ref)) {
				guardar.add(d.ref);
				break;
			}
		}
	}
	for (const r of raizes) for (let pai = pais.get(r); pai; pai = pais.get(pai)) guardar.add(pai);
	return dispositivos.filter((d) => guardar.has(d.ref));
}

function pesquisa(tema, link) {
	let fonte = link;
	if (!fonte) {
		const numero = tema.match(/(\d{2}\.\d{3})/)?.[1];
		if (!numero || /resolu[çc][ãa]o/i.test(tema)) return null;
		fonte = `https://www.planalto.gov.br/e2e/lei-exemplo-${numero}.htm`;
	}
	const ds = comNumero(LEIS.v1.dispositivos, numeroDoLink(fonte));
	return {
		fonte,
		link: fonte,
		epigrafe: ds[0].texto,
		estrutura: ds
			.filter((d) => d.ref === 'preambulo1' || d.tipo === 'artigo' || AGRUPAMENTOS.includes(d.tipo))
			.map((d) => ({ ...d, texto: d.tipo === 'preambulo' ? d.texto : d.tipo === 'artigo' ? d.texto.slice(0, 160) : '' }))
	};
}

function resultado(link, recorte = []) {
	const caminho = new URL(link).pathname;
	const lei = caminho.includes('lei-exemplo-v2') ? LEIS.v2 : LEIS.v1;
	const todos = comNumero(lei.dispositivos, numeroDoLink(link));
	const faltam = recorte.filter((r) => !todos.some((d) => d.ref === r));
	const dispositivos = recorte.length ? cortar(todos, recorte) : todos;
	const base = {
		fonte: link,
		gemini: true,
		versao: lei.versao,
		originalSha256: 'e2e',
		paragrafos: lei.dispositivos.length,
		publicavel: true,
		dispositivos,
		recorte,
		bloqueios: [],
		avisos: [],
		resumo: { vigentes: lei.dispositivos.length, anteriores: 0, notas: 0, revogados: 0, tipos: {}, descartados: [], riscados: [], juncoes: [] }
	};
	if (recorte.length) base.versao = `${lei.versao}:${[...recorte].sort().join(',')}`;
	if (faltam.length) {
		return { ...base, publicavel: false, dispositivos: null, versao: null, bloqueios: [`o recorte cita ${faltam.join(', ')}, que a lei não tem`] };
	}
	if (caminho.includes('bloqueio')) {
		return { ...base, publicavel: false, dispositivos: null, versao: null, bloqueios: ['o texto remontado da árvore difere do texto dos parágrafos (sha256 diferente)'] };
	}
	if (caminho.includes('aviso')) {
		return {
			...base,
			avisos: [{ id: 'p0003: a regra diz artigo', texto: 'p0003: a regra diz artigo, o Gemini diz solto', trecho: lei.dispositivos[2].texto }]
		};
	}
	return base;
}

function responder(res, status, corpo) {
	res.writeHead(status, { 'Content-Type': 'application/json' });
	res.end(JSON.stringify(corpo));
}

function recusar(res, status, code, message) {
	responder(res, status, { code, message, transient: false, requestId: null });
}

createServer((req, res) => {
	const partes = [];
	req.on('data', (p) => partes.push(p));
	req.on('end', () => {
		const corpo = Buffer.concat(partes).toString('utf8');

		if (req.method === 'GET' && req.url === '/healthz') return responder(res, 200, { status: 'ok', gemini: true });

		// O mesmo porteiro do processador real: sem token não há rota interna,
		// e todo passo diz de quem é o documento.
		if (!TOKEN || req.headers.authorization !== `Bearer ${TOKEN}`) {
			return recusar(res, 401, 'unauthorized', 'invalid or missing service token');
		}
		if (!req.headers['x-owner-ref']) return recusar(res, 401, 'unauthorized', 'missing owner reference');

		if (req.method === 'POST' && req.url === '/internal/editais/analisar') {
			// O texto que diz não ter a lista de cargos reproduz a leitura que
			// volta vazia — o cenário B9.
			const semCargos = /sem a lista de cargos/i.test(corpo);
			return responder(res, 200, {
				documentId: semCargos ? 'doc-sem-cargos' : 'doc-e2e',
				banca: semCargos ? '' : 'FCC',
				totalPages: 12,
				ocrPages: 0,
				cargos: semCargos ? [] : CARGOS,
				alerts: []
			});
		}

		if (req.method === 'POST' && req.url === '/internal/editais/estrutura') {
			const { cargo } = JSON.parse(corpo || '{}');
			return responder(res, 200, estrutura(cargo));
		}

		if (req.method === 'POST' && req.url === '/internal/editais/conteudo') {
			const { disciplinas = [] } = JSON.parse(corpo || '{}');
			return responder(res, 200, {
				itens: disciplinas.map((d) => ({ disciplina: d, itens: TEMAS[d] ?? [] })),
				alerts: []
			});
		}

		if (req.method === 'POST' && req.url === '/internal/leis/capturas') {
			const { link = '', recorte = null } = JSON.parse(corpo || '{}');
			if (!oficial(link)) {
				return recusar(res, 422, 'fonte_invalida', 'não é uma fonte oficial: use o link do Planalto, da Casa Civil de Goiás ou de outro site .gov.br, .leg.br ou .jus.br');
			}
			const id = randomUUID();
			// A primeira consulta ainda encontra a captura rodando: a tela tem de
			// saber esperar (L16).
			capturas.set(id, { dono: req.headers['x-owner-ref'], link, recorte: recorte ?? [], consultas: 0 });
			return responder(res, 202, { id });
		}

		if (req.method === 'POST' && req.url === '/internal/leis/pesquisas') {
			const { tema = '', link = '' } = JSON.parse(corpo || '{}');
			if (link && !oficial(link)) {
				return recusar(res, 422, 'fonte_invalida', 'não é uma fonte oficial');
			}
			const achada = pesquisa(tema, link);
			if (!achada) return recusar(res, 404, 'fonte_nao_encontrada', 'não achei a fonte oficial desta norma pelo tópico: cole o link dela');
			return responder(res, 200, achada);
		}

		const consulta = req.method === 'GET' && req.url.match(/^\/internal\/leis\/capturas\/([\w-]+)$/);
		if (consulta) {
			const c = capturas.get(consulta[1]);
			if (!c || c.dono !== req.headers['x-owner-ref']) {
				return recusar(res, 404, 'captura_nao_encontrada', 'captura não encontrada ou expirada');
			}
			c.consultas += 1;
			if (c.consultas === 1) {
				return responder(res, 200, { id: consulta[1], estado: 'rodando', etapa: 'classificando', progresso: { feitos: 1, total: 2 } });
			}
			return responder(res, 200, { id: consulta[1], estado: 'pronta', etapa: 'verificando', progresso: { feitos: 2, total: 2 }, resultado: resultado(c.link, c.recorte) });
		}

		recusar(res, 404, 'not_found', `${req.method} ${req.url}`);
	});
}).listen(8000, () => console.log('dublê do edital-processor na porta 8000'));
