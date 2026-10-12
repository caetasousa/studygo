"""Per-page text quality score (spec §7.8).

A number in [0, 1]. Low means the native text layer is missing or garbled and
the page should go to OCR. The weights are deliberate and documented; tune them
in config, not here.

Signals, each mapped to [0, 1]:
  * length      - characters relative to a "full page" reference
  * alnum_ratio - share of alphanumeric / whitespace / common punctuation
  * invalid     - share of replacement chars and control bytes (inverted)
  * words       - share of tokens that look like real words (>=2 letters)
  * repetition  - share of unique lines (inverted duplication)
"""

from __future__ import annotations

import re
import unicodedata
from dataclasses import dataclass

_FULL_PAGE_CHARS = 1800.0  # a dense A4 text page
# A "word" is three or more letters in a row. Two-letter fragments ("th", "nt")
# are what OCR garble produces, so they do not count.
_WORD_RE = re.compile(r"[^\W\d_]{3,}", re.UNICODE)
_TOKEN_RE = re.compile(r"\S+")
_LETTER_RE = re.compile(r"[^\W\d_]", re.UNICODE)
_DIGIT_RE = re.compile(r"\d")
_ALLOWED_PUNCT = set(" \t\n\r.,;:!?()[]{}'\"-/%")

_WEIGHTS = {
    "length": 0.15,
    "alnum_ratio": 0.20,
    "invalid": 0.20,
    "words": 0.30,
    "repetition": 0.15,
}


@dataclass(frozen=True)
class QualityBreakdown:
    score: float
    length: float
    alnum_ratio: float
    invalid: float
    words: float
    repetition: float
    char_count: int


def _clamp(value: float) -> float:
    return max(0.0, min(1.0, value))


def score_page_text(text: str) -> QualityBreakdown:
    stripped = text.strip()
    n = len(stripped)
    if n == 0:
        return QualityBreakdown(0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0)

    length = _clamp(n / _FULL_PAGE_CHARS)

    good = 0
    invalid = 0
    for ch in stripped:
        if ch == "�" or (unicodedata.category(ch).startswith("C") and ch not in "\n\r\t"):
            invalid += 1
        elif ch.isalnum() or ch in _ALLOWED_PUNCT:
            good += 1
    alnum_ratio = good / n
    invalid_ratio = 1.0 - (invalid / n)

    tokens = _TOKEN_RE.findall(stripped)
    words = 0.0
    if tokens:
        word_hits = len(_WORD_RE.findall(stripped)) / len(tokens)
        # A real Portuguese page is overwhelmingly letters; garble is speckled
        # with stray digits. Weight the word signal down when digits dominate.
        letters = len(_LETTER_RE.findall(stripped))
        digits = len(_DIGIT_RE.findall(stripped))
        letter_purity = letters / (letters + digits) if (letters + digits) else 0.0
        words = _clamp(word_hits * letter_purity)

    lines = [ln.strip() for ln in stripped.splitlines() if ln.strip()]
    repetition = 1.0
    if lines:
        repetition = len(set(lines)) / len(lines)

    score = (
        _WEIGHTS["length"] * length
        + _WEIGHTS["alnum_ratio"] * alnum_ratio
        + _WEIGHTS["invalid"] * invalid_ratio
        + _WEIGHTS["words"] * words
        + _WEIGHTS["repetition"] * repetition
    )

    return QualityBreakdown(
        score=_clamp(score),
        length=length,
        alnum_ratio=alnum_ratio,
        invalid=invalid_ratio,
        words=words,
        repetition=repetition,
        char_count=n,
    )
