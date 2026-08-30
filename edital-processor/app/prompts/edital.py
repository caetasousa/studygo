"""Prompts and response schemas for the three extraction steps.

Ported from the Go adapter, adapted to: delimited chunks instead of the whole
PDF, the §8 data model, and ``null`` for absence. Schemas use the Gemini dialect
(``nullable``, inline definitions) so structured output works; consolidation and
validation happen in Python.
"""

from __future__ import annotations

from typing import Any

_EVIDENCE_ITEMS: dict[str, Any] = {
    "type": "object",
    "properties": {
        "physicalPage": {"type": "integer"},
        "snippet": {"type": "string"},
    },
    "required": ["physicalPage", "snippet"],
}
_EVIDENCE_LIST: dict[str, Any] = {"type": "array", "items": _EVIDENCE_ITEMS}


def _nullable(base: dict[str, Any]) -> dict[str, Any]:
    return {**base, "nullable": True}


# --- step 1: banca + cargos --------------------------------------------------

CARGOS_INSTRUCTION = (
    "Liste a banca organizadora e TODOS os cargos/especialidades oferecidos.\n"
    'Para cada cargo: codigo (código de opção, ex.: "B02"), nome (nome completo), '
    "especialidade (quando houver), escolaridade (exigência), totalVagas "
    "(número total de vagas, ou null).\n"
    "ATENÇÃO — os trechos vêm de tabelas achatadas por OCR: o nome de um cargo "
    "quase sempre aparece QUEBRADO em várias linhas, e outras colunas (vagas, "
    "escolaridade, número de questões) podem estar intercaladas no meio dele. "
    "Junte os pedaços e devolva o rótulo completo do cargo, exatamente como o "
    "edital o escreve por extenso.\n"
    'Exemplo: as linhas "Técnico de Controle Externo —", "A01 Especialidade: '
    'Técnico" e "Administrativo" descrevem UM cargo, cujo nome completo é '
    '"Técnico de Controle Externo — Especialidade: Técnico Administrativo".\n'
    "Em nome coloque SEMPRE o rótulo completo (incluindo a especialidade, quando "
    "o edital a apresenta como parte do nome); em especialidade repita apenas a "
    'parte da especialidade (ex.: "Tecnologia da Informação"), ou null.\n'
    'Corrija erros evidentes de OCR nas palavras do nome (ex.: "Extemo" -> '
    '"Externo", "TECNICO" -> "Técnico"), sem inventar nada que não esteja lá.\n'
    "Para cada cargo, inclua em evidence a página física (physicalPage) e um "
    "snippet curto de onde a informação veio."
)

CARGOS_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "banca": _nullable({"type": "string"}),
        "cargos": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "codigo": {"type": "string"},
                    "nome": {"type": "string"},
                    "especialidade": _nullable({"type": "string"}),
                    "escolaridade": _nullable({"type": "string"}),
                    "totalVagas": _nullable({"type": "integer"}),
                    "evidence": _EVIDENCE_LIST,
                },
                "required": ["codigo", "nome"],
            },
        },
    },
    "required": ["cargos"],
}


# --- step 2: exam structure for one cargo ------------------------------------

ESTRUTURA_INSTRUCTION = (
    "Você recebe também o nome/código de UM cargo. Extraia, referentes SOMENTE a "
    "esse cargo:\n"
    "- nomeSugerido: nome curto do concurso (órgão + cargo).\n"
    "- dataProva: data das provas objetivas, AAAA-MM-DD, ou null.\n"
    '- gruposGerais / gruposEspecificos: os GRUPOS de conhecimento. "Conhecimentos '
    'Gerais" e "Conhecimentos Específicos" são GRUPOS, nunca disciplinas.\n'
    '  Para cada grupo: rotulo (título literal), kind ("ger", "esp" ou "outro"), '
    "totalQuestoes (total do grupo, ou null), peso (ou null), pesoScope "
    '("group" se o peso foi dado para o grupo todo, "discipline" se por '
    "disciplina), disciplinas: [{ nome, numeroQuestoes (só quando o edital "
    "divide as questões por disciplina; caso contrário null), peso (só quando "
    "informado por disciplina) }].\n"
    '- provaDiscursiva: [{ modalidade ("redacao", "estudo_de_caso", "outro"), '
    "rotulo, questoes }].\n"
    '- duracao: { minutos, scope ("exam_set" se cobre o conjunto de provas, '
    '"single_prova" se é de uma prova só, "unknown") }, ou null.\n'
    "NUNCA distribua o total de um grupo entre as disciplinas por conta própria: "
    "se o edital não divide, numeroQuestoes fica null.\n"
    "COMO ACHAR AS DISCIPLINAS — na maioria dos editais a tabela de provas traz "
    'apenas o total do grupo (ex.: "Conhecimentos Gerais 25 1"), sem listar as '
    "disciplinas. Os nomes das disciplinas estão no CONTEÚDO PROGRAMÁTICO (Anexo "
    "II), que também foi enviado nos trechos: sob o título de cada grupo (ex.: "
    '"CONHECIMENTOS GERAIS PARA OS CARGOS ...") vêm os nomes das disciplinas '
    'daquele grupo como subtítulos (ex.: "Língua Portuguesa", "Matemática e '
    'Raciocínio Lógico", "Legislação Institucional"), cada um seguido da sua '
    "ementa em texto corrido.\n"
    "Use esses subtítulos para preencher disciplinas[].nome de cada grupo. "
    "Considere APENAS as seções do conteúdo programático que valem para o cargo "
    "escolhido (as de Conhecimentos Gerais costumam ser comuns a todos os "
    "cargos; as de Conhecimentos Específicos são por cargo — ignore as de outros "
    "cargos).\n"
    "Um subtítulo de disciplina é uma linha curta, sem ponto final, seguida de um "
    "parágrafo de ementa. NÃO confunda com um tópico da ementa nem com o título "
    "do grupo. Só devolva disciplinas: [] se o conteúdo programático daquele "
    "grupo realmente não estiver nos trechos.\n"
    "- cronograma: TODAS as linhas do cronograma de provas e publicações (o anexo "
    '"CRONOGRAMA DAS PROVAS E PUBLICAÇÕES"), na ordem em que aparecem, uma '
    "entrada por atividade: { dataInicio, dataFim, titulo, exigeAcao }.\n"
    "  dataInicio e dataFim em AAAA-MM-DD. Quando a linha traz um intervalo "
    '("05/10/2026 a 06/11/2026"), dataInicio é a primeira data e dataFim a '
    "segunda; quando traz uma data só, dataFim é null. Converta DD/MM/AAAA para "
    "AAAA-MM-DD.\n"
    "  titulo: a descrição da atividade como o edital escreve, sem o número do "
    "item e sem a data.\n"
    "  exigeAcao: true quando a atividade depende de uma ação do candidato "
    "(inscrição, pagamento, solicitação de isenção, interposição de recurso ou "
    "impugnação, comparecimento); false quando é apenas divulgação, publicação "
    "ou aplicação de prova pela banca.\n"
    '  Ignore a observação final do tipo "Cronograma sujeito a alterações"; '
    "ela não é um marco."
)

_GRUPO_ITEMS: dict[str, Any] = {
    "type": "object",
    "properties": {
        "rotulo": {"type": "string"},
        "kind": {"type": "string", "enum": ["ger", "esp", "outro"]},
        "totalQuestoes": _nullable({"type": "integer"}),
        "peso": _nullable({"type": "number"}),
        "pesoScope": _nullable({"type": "string", "enum": ["group", "discipline"]}),
        "disciplinas": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "nome": {"type": "string"},
                    "numeroQuestoes": _nullable({"type": "integer"}),
                    "peso": _nullable({"type": "number"}),
                },
                "required": ["nome"],
            },
        },
        "evidence": _EVIDENCE_LIST,
    },
    "required": ["rotulo", "kind"],
}

ESTRUTURA_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "nomeSugerido": _nullable({"type": "string"}),
        "dataProva": _nullable({"type": "string"}),
        "gruposGerais": {"type": "array", "items": _GRUPO_ITEMS},
        "gruposEspecificos": {"type": "array", "items": _GRUPO_ITEMS},
        "provaDiscursiva": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "modalidade": {
                        "type": "string",
                        "enum": ["redacao", "estudo_de_caso", "outro"],
                    },
                    "rotulo": {"type": "string"},
                    "questoes": _nullable({"type": "integer"}),
                    "evidence": _EVIDENCE_LIST,
                },
                "required": ["modalidade", "rotulo"],
            },
        },
        "duracao": _nullable(
            {
                "type": "object",
                "properties": {
                    "minutos": {"type": "integer"},
                    "scope": {
                        "type": "string",
                        "enum": ["exam_set", "single_prova", "unknown"],
                    },
                    "evidence": _EVIDENCE_LIST,
                },
                "required": ["minutos", "scope"],
            }
        ),
        "cronograma": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "dataInicio": {"type": "string"},
                    "dataFim": _nullable({"type": "string"}),
                    "titulo": {"type": "string"},
                    "exigeAcao": {"type": "boolean"},
                    "evidence": _EVIDENCE_LIST,
                },
                "required": ["dataInicio", "titulo"],
            },
        },
    },
    "required": ["gruposGerais", "gruposEspecificos"],
}


# --- step 3: syllabus topics for a set of disciplines ----------------------

CONTEUDO_INSTRUCTION = (
    "Você recebe também uma lista de disciplinas e um cargo. Para CADA disciplina "
    "da lista, extraia do conteúdo programático os tópicos correspondentes ÀQUELE "
    "cargo, um tópico por item, mantendo a redação do edital.\n"
    "NÃO resuma nem modernize leis, versões ou tecnologias.\n"
    "NÃO misture o conteúdo de cargos diferentes.\n"
    "Retorne itens: [{ disciplina (exatamente como na lista), topicos: [string], "
    "evidence: [{ physicalPage, snippet }] }]. Se não achar o conteúdo de uma "
    "disciplina, devolva topicos: []."
)

CONTEUDO_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "itens": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "disciplina": {"type": "string"},
                    "topicos": {"type": "array", "items": {"type": "string"}},
                    "evidence": _EVIDENCE_LIST,
                },
                "required": ["disciplina", "topicos"],
            },
        }
    },
    "required": ["itens"],
}
