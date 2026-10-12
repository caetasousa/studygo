"""As portas do Claude Code: rodar o `claude -p` e a conexão de cada conta.

O processo é a fronteira de fora: o caso de uso monta os argumentos e a
mensagem (service/mapas/sessao.py), a implementação só roda
(adapter/claude/cli.py). A conexão é o login da assinatura de cada conta, num
pseudo-terminal (adapter/claude/conexao.py).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

# (argumentos, mensagem, pasta, ambiente) -> (código de saída, saída)
RodarClaude = Callable[[list[str], str, Path, dict[str, str]], Awaitable[tuple[int, str]]]


@dataclass(frozen=True)
class Situacao:
    conectado: bool
    email: str = ""
    plano: str = ""


class ContasDoClaude(Protocol):
    async def situacao(self, dono: str) -> Situacao: ...

    def ambiente(self, dono: str) -> dict[str, str]:
        """O ambiente do Claude desta conta: a pasta dela, e nenhum token de fora."""
        ...

    async def iniciar(self, dono: str) -> str:
        """O link de autorização da assinatura."""
        ...

    async def concluir(self, dono: str, codigo: str) -> Situacao: ...

    async def desconectar(self, dono: str) -> None: ...

    def encerrar_tudo(self) -> None: ...
