"""Confidence scoring (spec §11).

A confidence is never a number the model reports about itself. It is computed
from measurable signals:

  * source_quality  — native text > clean OCR > low-confidence OCR
  * snippet_valid   — the evidence snippet is actually on the cited page
  * rule_agreement  — a deterministic local check agrees with the LLM
  * arithmetic      — the numbers are internally consistent
  * page_conflict   — the same fact is not contradicted on another page (inverted)

The result is the weighted mean of the signals that apply. When no signal is
defensible the confidence is ``None``.
"""

from __future__ import annotations

from dataclasses import dataclass

_WEIGHTS = {
    "source_quality": 0.30,
    "snippet_valid": 0.25,
    "rule_agreement": 0.25,
    "arithmetic": 0.10,
    "page_conflict": 0.10,
}


@dataclass
class Signals:
    source_quality: float | None = None
    snippet_valid: bool | None = None
    rule_agreement: bool | None = None
    arithmetic_ok: bool | None = None
    page_conflict: bool | None = None  # True == there IS a conflict


def source_quality_for(method: str, ocr_confidence: float | None) -> float:
    if method == "native_text":
        return 1.0
    if method == "ocr":
        return 0.6 if ocr_confidence is None else 0.4 + 0.5 * min(1.0, ocr_confidence)
    if method == "table":
        return 0.8
    return 0.5  # llm-only, no anchored source


def score(signals: Signals) -> float | None:
    contributions: list[tuple[float, float]] = []

    if signals.source_quality is not None:
        contributions.append((_WEIGHTS["source_quality"], signals.source_quality))
    if signals.snippet_valid is not None:
        contributions.append((_WEIGHTS["snippet_valid"], 1.0 if signals.snippet_valid else 0.0))
    if signals.rule_agreement is not None:
        contributions.append((_WEIGHTS["rule_agreement"], 1.0 if signals.rule_agreement else 0.0))
    if signals.arithmetic_ok is not None:
        contributions.append((_WEIGHTS["arithmetic"], 1.0 if signals.arithmetic_ok else 0.0))
    if signals.page_conflict is not None:
        contributions.append((_WEIGHTS["page_conflict"], 0.0 if signals.page_conflict else 1.0))

    if not contributions:
        return None

    total_weight = sum(w for w, _ in contributions)
    return round(sum(w * v for w, v in contributions) / total_weight, 3)


def overall(confidences: list[float | None]) -> float | None:
    present = [c for c in confidences if c is not None]
    if not present:
        return None
    return round(sum(present) / len(present), 3)
