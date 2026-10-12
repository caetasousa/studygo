"""A conferência da classificação pelo Gemini, em lotes.

Numa lei do tamanho da Constituição são milhares de parágrafos: vão em lotes, e
um lote que volta torto ganha uma segunda chance em duas metades.
"""

from __future__ import annotations

from collections.abc import Callable

from app.domain.leis.classificar import CORTE, INSTRUCAO, PEDIDO, TIPOS, Classe, esquema
from app.domain.leis.limpeza import Paragrafo
from app.port.llm import LLMProvider, StructuredRequest


def _pedido(grupo: list[Paragrafo]) -> StructuredRequest:
    return StructuredRequest(
        system=PEDIDO,
        chunks=[f"{p.id}: {p.texto[:CORTE]}" for p in grupo],
        response_schema=esquema(),
        instruction=INSTRUCAO,
    )


async def _uma_vez(
    provider: LLMProvider, grupo: list[Paragrafo]
) -> tuple[list[dict[str, object]], str]:
    resposta = await provider.extract_structured(_pedido(grupo))
    itens = resposta.get("itens")
    if not isinstance(itens, list):
        return [], "o Gemini respondeu sem `itens`"
    ids = [str(i.get("id")) if isinstance(i, dict) else "" for i in itens]
    esperados = [p.id for p in grupo]
    if ids == esperados:
        return [i for i in itens if isinstance(i, dict)], ""
    faltam = sorted(set(esperados) - set(ids))
    sobram = sorted(set(ids) - set(esperados))
    return [], (
        f"o Gemini não devolveu os ids enviados, na ordem: faltam {faltam[:5]}, sobram {sobram[:5]}"
    )


async def _pedir_lote(provider: LLMProvider, grupo: list[Paragrafo]) -> list[dict[str, object]]:
    # Numa lei do tamanho da CF, o modelo às vezes pula um id. Com temperatura
    # 0, repetir o mesmo pedido repete o mesmo erro; a segunda chance é o lote
    # em duas metades. Se uma metade ainda falha, é sintoma de outra coisa.
    itens, erro = await _uma_vez(provider, grupo)
    if not erro:
        return itens
    if len(grupo) < 2:
        raise ValueError(erro)
    meio = len(grupo) // 2
    juntos: list[dict[str, object]] = []
    for metade in (grupo[:meio], grupo[meio:]):
        parte, erro = await _uma_vez(provider, metade)
        if erro:
            raise ValueError(erro)
        juntos.extend(parte)
    return juntos


async def por_gemini(
    paragrafos: list[Paragrafo],
    provider: LLMProvider,
    lote: int = 150,
    ao_lote: Callable[[int, int], None] | None = None,
) -> list[Classe | None]:
    """A classe que o Gemini dá a cada parágrafo vigente; None nos demais."""
    enviados = [p for p in paragrafos if not p.anterior and p.texto]
    tipos: dict[str, str] = {}
    total = -(-len(enviados) // lote)
    for n, inicio in enumerate(range(0, len(enviados), lote)):
        if ao_lote is not None:
            ao_lote(n, total)
        grupo = enviados[inicio : inicio + lote]
        itens = await _pedir_lote(provider, grupo)
        for item in itens:
            tipo = str(item.get("tipo"))
            if tipo not in TIPOS:
                raise ValueError(f"o Gemini devolveu um tipo desconhecido: {tipo}")
            tipos[str(item["id"])] = tipo
    return [Classe(tipos[p.id]) if p.id in tipos else None for p in paragrafos]
