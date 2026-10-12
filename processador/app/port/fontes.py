"""A porta das fontes oficiais de lei: um GET que devolve status, tipo e bytes.

Um erro HTTP volta como resposta (status != 200), não como exceção: o caso de
uso decide o que um 404 quer dizer. A implementação recusa redirecionamento
para fora das fontes oficiais (adapter/fontes/http.py).
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass


@dataclass(frozen=True)
class Resposta:
    status: int
    tipo: str
    corpo: bytes


Http = Callable[[str], Resposta]
