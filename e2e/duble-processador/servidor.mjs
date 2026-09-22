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

		recusar(res, 404, 'not_found', `${req.method} ${req.url}`);
	});
}).listen(8000, () => console.log('dublê do edital-processor na porta 8000'));
