"""Which pages go to OCR (spec §7.9).

OCR is expensive and lossy, so it runs only where the native layer is unusable:
too few characters outright, or a quality score below the configured floor.
"""

from __future__ import annotations

from app.domain.edital.document import PageText


def pages_needing_ocr(pages: list[PageText], min_chars: int, min_score: float) -> list[int]:
    """Return the physical page numbers (1-based) that should be OCR'd.

    Os limiares vêm da configuração (min_text_chars, min_text_score), lida
    pelo caso de uso."""
    needing: list[int] = []
    for page in pages:
        chars = len(page.text.strip())
        if chars < min_chars or page.text_score < min_score:
            needing.append(page.physical_page)
    return needing
