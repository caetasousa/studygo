"""Which pages go to OCR (spec §7.9).

OCR is expensive and lossy, so it runs only where the native layer is unusable:
too few characters outright, or a quality score below the configured floor.
"""

from __future__ import annotations

from app.core.config import Settings
from app.schemas.document import PageText


def pages_needing_ocr(pages: list[PageText], settings: Settings) -> list[int]:
    """Return the physical page numbers (1-based) that should be OCR'd."""
    needing: list[int] = []
    for page in pages:
        chars = len(page.text.strip())
        if chars < settings.min_text_chars or page.text_score < settings.min_text_score:
            needing.append(page.physical_page)
    return needing
