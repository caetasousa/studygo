"""A porta do backend, no sentido contrário: entregar o mapa pronto ou avisar
que o pedido falhou. A implementação fala com a porta interna dele
(adapter/backend/cliente.py)."""

from __future__ import annotations

from collections.abc import Awaitable, Callable

from app.domain.mapas.pedido import Resposta, Saida

Entregar = Callable[[str, Saida], Awaitable[Resposta]]
Avisar = Callable[[str, str], Awaitable[None]]
