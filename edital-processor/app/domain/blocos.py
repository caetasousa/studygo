"""Content blocks.

A page carries one or more of these. Classification (Phase 2) assigns them from
cheap surface signals; the LLM step only ever sees pages whose blocks are
relevant, delimited and labelled as untrusted data.
"""

from __future__ import annotations

from enum import StrEnum


class Bloco(StrEnum):
    DADOS_GERAIS = "dados_gerais"
    CARGOS_VAGAS = "cargos_vagas"
    ESTRUTURA_PROVAS = "estrutura_provas"
    CONTEUDO_PROGRAMATICO = "conteudo_programatico"
    CRONOGRAMA = "cronograma"
    IRRELEVANTE = "irrelevante"


# Blocks worth sending to the model. IRRELEVANTE is everything else and is never
# forwarded.
RELEVANTES: frozenset[Bloco] = frozenset(
    {
        Bloco.DADOS_GERAIS,
        Bloco.CARGOS_VAGAS,
        Bloco.ESTRUTURA_PROVAS,
        Bloco.CONTEUDO_PROGRAMATICO,
        Bloco.CRONOGRAMA,
    }
)
