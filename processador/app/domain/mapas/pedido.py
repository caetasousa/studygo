"""O pedido de mapa e o que volta dele.

O backend entrega o PDF de uma aula (o Trabalho); o Claude escreve o mapa numa
pasta (a Saida); o backend responde à entrega (a Resposta).
"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class Trabalho:
    """Um PDF que o backend entregou para virar mapa."""

    pedido: str
    dono: str
    arquivo: str
    pdf: bytes
    materia: str = ""
    temas: tuple[str, ...] = ()
    slugs_existentes: tuple[str, ...] = ()
    token_claude: str = ""


@dataclass(frozen=True)
class Resposta:
    """O que o backend respondeu à entrega: o status e a mensagem."""

    status: int
    mensagem: str


@dataclass(frozen=True)
class Saida:
    """O que o Claude escreveu na pasta de saída, pronto para o backend."""

    mapa: str
    questoes: dict[str, object] | None
    temas: list[str]
    relatorio: str
    imagens: list[tuple[str, bytes]] = field(default_factory=list)


class ClaudeFalhou(Exception):
    """O Claude terminou com erro: o motivo vai para o pedido."""
