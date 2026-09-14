"""The LLM seam.

A small, injectable interface. Phase 1 ships only the protocol and a null
implementation; the real Gemini adapter lands in Phase 3. No registry, no
dynamic selection — one provider.
"""

from __future__ import annotations

from typing import Protocol

from pydantic import BaseModel, Field


class ImageInput(BaseModel):
    data: bytes
    mime: str = "image/png"


class StructuredRequest(BaseModel):
    # A short instruction plus the delimited, untrusted document chunks. The
    # provider never receives the whole PDF.
    system: str
    chunks: list[str]
    # JSON schema the response must conform to.
    response_schema: dict[str, object]

    # --- tarefas multimodais (provas) ----------------------------------------
    # Os editais não usam nada daqui, e é isso que mantém o comportamento deles.
    #
    # `instruction` troca o preâmbulo dos editais pela instrução da tarefa, e o
    # schema passa a ir como JSON Schema completo. Com ela, a resposta também
    # traz `_modelo`, `_entrada` e `_saida`: o consumo da chamada, para medir o
    # custo real por prova.
    images: list[ImageInput] = Field(default_factory=list)
    instruction: str | None = None
    # Uma tentativa por modelo, e um timeout encerra a cadeia: quem repete é a
    # fila do backend, com espera, e não este processo segurando a conexão.
    # Sobrecarga (503) e cota (429) ainda passam ao próximo modelo, porque
    # voltam em segundos, e cada modelo tem capacidade e cota próprias.
    single_attempt: bool = False
    # Teto desta chamada; sem ele vale o dos editais, curto demais para uma
    # região com dez questões.
    timeout_seconds: float | None = None


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
