"""Text normalization.

One canonical form for page text so evidence snippets can be matched against it,
whether the text came from the PDF layer or from OCR. Deliberately light: it
must not destroy the words a reviewer would search for.
"""

from __future__ import annotations

import re
import unicodedata

_WS_RUN = re.compile(r"[ \t]{2,}")
_BLANK_RUN = re.compile(r"\n{3,}")
_TRAILING_WS = re.compile(r"[ \t]+\n")


def normalize_text(text: str) -> str:
    text = unicodedata.normalize("NFC", text)
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = text.replace("\u00a0", " ")
    # de-hyphenate words split across line breaks: "concur-\nso" -> "concurso"
    text = re.sub(r"(\w)-\n(\w)", r"\1\2", text)
    text = _TRAILING_WS.sub("\n", text)
    text = _WS_RUN.sub(" ", text)
    text = _BLANK_RUN.sub("\n\n", text)
    return text.strip()


def snippet_matches_page(snippet: str, page_text: str, *, ocr: bool) -> bool:
    """Whether an evidence snippet can be found in a page's normalized text.

    Exact for native text; for OCR, tolerant of the small substitutions the
    engine makes (spacing, a stray char) by comparing on a squashed form.
    """
    s = normalize_text(snippet)
    p = normalize_text(page_text)
    if s in p:
        return True
    if not ocr:
        return False
    return _squash(s) in _squash(p)


def _squash(text: str) -> str:
    return re.sub(r"[^0-9a-zçãõáéíóúâêô]", "", text.lower())
