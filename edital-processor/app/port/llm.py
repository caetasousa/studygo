"""A porta do LLM: o que os casos de uso pedem a um modelo.

Uma implementação de verdade (adapter/llm/gemini.py) e a nula, para quando não
há chave (adapter/llm/nulo.py). Quem escolhe é a montagem (app/main.py).
"""

from __future__ import annotations

from typing import Protocol

from pydantic import BaseModel


class StructuredRequest(BaseModel):
    # A short instruction plus the delimited, untrusted document chunks. The
    # provider never receives the whole PDF.
    system: str
    chunks: list[str]
    # JSON schema the response must conform to.
    response_schema: dict[str, object]
    # Instrução de sistema própria do pedido. Sem ela vale o preâmbulo de
    # edital do provider; a captura de leis precisa de outra.
    instruction: str | None = None


class LLMProvider(Protocol):
    def available(self) -> bool: ...

    async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
        """Return the parsed JSON object. Raises a typed ProcessorError on
        rate limit, timeout, upstream failure, or an unparseable response."""
        ...
