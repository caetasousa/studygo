"""The LLM seam.

A small, injectable interface. Phase 1 ships only the protocol and a null
implementation; the real Gemini adapter lands in Phase 3. No registry, no
dynamic selection — one provider.
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


class LLMProvider(Protocol):
    def available(self) -> bool: ...

    async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
        """Return the parsed JSON object. Raises a typed ProcessorError on
        rate limit, timeout, upstream failure, or an unparseable response."""
        ...


class NullProvider:
    """Used when no API key is configured. Every call reports unavailable."""

    def available(self) -> bool:
        return False

    async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
        from app.core.errors import ProviderUnavailable

        raise ProviderUnavailable("no LLM provider configured")
