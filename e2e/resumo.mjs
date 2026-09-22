// Gera e2e/relatorio/resumo.md a partir do resultados.json do Playwright.
//
// O resumo é o artefato que se confere sem rodar nada: cada id do CENARIOS.md
// com o resultado do teste que o cobre e o print do estado final, mais o
// commit e as imagens exatas contra as quais a suíte rodou — é o que torna a
// execução repetível. Um id sem teste aparece como lacuna.

import { execSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { relative } from 'node:path';

const projeto = process.argv[2] ?? 'studygo-e2e';
const resultados = JSON.parse(readFileSync('relatorio/resultados.json', 'utf8'));

function sh(cmd) {
	try {
		return execSync(cmd, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
	} catch {
		return '';
	}
}

// Os cenários, na ordem do catálogo: | A1 | como quebra | o que o usuário vê |
const cenarios = [];
for (const linha of readFileSync('CENARIOS.md', 'utf8').split('\n')) {
	const m = /^\| ([A-Z]\d+) \| (.+?) \| (.+?) \|$/.exec(linha);
	if (m) cenarios.push({ id: m[1], quebra: m[2] });
}

// A primeira linha do erro e, se houver, o seletor que não achou o que queria.
function resumirErro(mensagem) {
	const linhas = mensagem.replace(/\u001b\[\d+m/g, '').split('\n').map((l) => l.trim());
	const alvo = linhas.find((l) => /^(Locator|Expected|Received):/.test(l));
	return [linhas[0], alvo].filter(Boolean).join(' — ');
}

// Os testes, achatados: título, id, estado, duração e o print final.
const testes = [];
function percorrer(suite) {
	for (const spec of suite.specs ?? []) {
		for (const t of spec.tests ?? []) {
			const r = t.results.at(-1) ?? {};
			const print = (r.attachments ?? []).find((a) => a.name === 'screenshot' && a.path);
			testes.push({
				id: /\[([A-Z]\d+)\]/.exec(spec.title)?.[1] ?? null,
				titulo: spec.title.replace(/^\[[A-Z]\d+\]\s*/, ''),
				arquivo: spec.file,
				estado: t.status === 'expected' ? 'passou' : t.status === 'skipped' ? 'pulado' : 'FALHOU',
				ms: r.duration ?? 0,
				erro: resumirErro(r.error?.message ?? ''),
				print: print ? relative('relatorio', print.path) : ''
			});
		}
	}
	for (const s of suite.suites ?? []) percorrer(s);
}
for (const s of resultados.suites) percorrer(s);

const commit = sh('git rev-parse --short HEAD');
const sujo = sh('git status --porcelain') ? ' (com alterações não commitadas)' : '';
const imagens = sh(
	`docker ps --filter label=com.docker.compose.project=${projeto} --format '{{.Label "com.docker.compose.service"}} {{.ID}}'`
)
	.split('\n')
	.filter(Boolean)
	.map((l) => {
		const [servico, id] = l.split(' ');
		return `| ${servico} | \`${sh(`docker inspect --format '{{.Image}}' ${id}`).slice(7, 19)}\` |`;
	})
	.sort();

const passou = testes.filter((t) => t.estado === 'passou').length;
const falhou = testes.filter((t) => t.estado === 'FALHOU');
const semTeste = cenarios.filter((c) => !testes.some((t) => t.id === c.id));
const semId = testes.filter((t) => !t.id || !cenarios.some((c) => c.id === t.id));

const linhas = [
	'# Resultado da suíte E2E',
	'',
	`- quando: ${new Date(resultados.stats.startTime).toLocaleString('pt-BR', { timeZone: 'America/Sao_Paulo' })}`,
	`- commit: \`${commit}\`${sujo}`,
	`- duração: ${(resultados.stats.duration / 1000).toFixed(1)} s`,
	`- **${passou} de ${testes.length} passaram**${falhou.length ? `, ${falhou.length} falharam` : ''}`,
	'',
	'Refazer: `make e2e` (sobe o mesmo stack do zero e roda tudo de novo).',
	'',
	'## Por cenário',
	'',
	'| id | como quebra | resultado | print |',
	'|---|---|---|---|',
	...cenarios.map((c) => {
		const t = testes.find((x) => x.id === c.id);
		if (!t) return `| ${c.id} | ${c.quebra} | sem teste | |`;
		return `| ${c.id} | ${c.quebra} | ${t.estado} | ${t.print ? `[print](${t.print})` : ''} |`;
	}),
	''
];

if (falhou.length) {
	linhas.push('## Falhas', '');
	for (const t of falhou) linhas.push(`- **[${t.id}] ${t.titulo}** (${t.arquivo}): ${t.erro}`);
	linhas.push('', 'O relatório HTML (`html/index.html`) tem o trace de cada falha.', '');
}
if (semTeste.length) linhas.push('## Lacunas', '', ...semTeste.map((c) => `- ${c.id}: ${c.quebra}`), '');
if (semId.length) linhas.push('## Testes sem cenário no catálogo', '', ...semId.map((t) => `- ${t.titulo}`), '');
if (imagens.length) linhas.push('## Imagens usadas', '', '| serviço | imagem |', '|---|---|', ...imagens, '');

writeFileSync('relatorio/resumo.md', linhas.join('\n'));
console.log(`resumo: ${passou}/${testes.length} passaram${falhou.length ? ` — falharam: ${falhou.map((t) => t.id).join(', ')}` : ''}`);
