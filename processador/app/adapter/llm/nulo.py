"""O LLM de quando não há chave configurada: toda chamada diz que não há."""

from __future__ import annotations

from app.platform.errors import ProviderUnavailable
from app.port.llm import StructuredRequest


class NullProvider:
    def available(self) -> bool:
        return False

    async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
        raise ProviderUnavailable("no LLM provider configured")
