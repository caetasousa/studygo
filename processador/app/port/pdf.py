"""A porta do leitor de PDF: validar o arquivo hostil e tirar dele o texto.

O PDF chega de quem estuda, então é tratado como hostil: a validação vem antes
de qualquer página ser lida. A implementação (adapter/pdf) usa o PyMuPDF e o
pdfplumber; o caso de uso só vê páginas e tabelas.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Protocol

from app.domain.edital.document import ExtractedTable, PageText


@dataclass(frozen=True)
class ValidatedPDF:
    data: bytes
    page_count: int


@dataclass
class ExtractionOutput:
    pages: list[PageText]
    tables: list[ExtractedTable] = field(default_factory=list)


class LeitorDePdf(Protocol):
    def validar(self, data: bytes, declared_mime: str | None) -> ValidatedPDF:
        """Recusa com um ProcessorError tipado: vazio, grande demais, cifrado,
        corrompido, páginas ou imagens acima do limite."""
        ...

    def extrair(self, data: bytes) -> ExtractionOutput:
        """O texto nativo de cada página, normalizado e pontuado, e as tabelas.
        Não faz OCR."""
        ...

    def paginas(self, data: bytes) -> list[str]:
        """O texto de cada página, cru, na ordem. Para a lei em PDF."""
        ...
