"""A porta do OCR: reconhecer o texto das páginas escaneadas."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Protocol


@dataclass(frozen=True)
class WordBox:
    text: str
    x0: float
    y0: float
    x1: float
    y1: float
    conf: float  # 0..1


@dataclass
class OCRPage:
    physical_page: int
    text: str
    words: list[WordBox] = field(default_factory=list)
    mean_conf: float | None = None


class Ocr(Protocol):
    def reconhecer(self, data: bytes, page_numbers: list[int]) -> list[OCRPage]:
        """As páginas pedidas, em ordem. Sem o motor de OCR, OCRUnavailable;
        uma página que estoura o tempo fica de fora, e as outras seguem."""
        ...
