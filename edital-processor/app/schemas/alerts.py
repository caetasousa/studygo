"""Alerts (spec §12).

A deterministic check that fails, a disagreement between a local rule and the
LLM, a missing critical field, an unmappable group — each becomes one of these.
The Go backend surfaces them to the user before anything is saved.
"""

from __future__ import annotations

from enum import StrEnum

from pydantic import Field

from app.schemas.base import WireModel


class AlertSeverity(StrEnum):
    INFO = "info"
    WARNING = "warning"
    BLOCKER = "blocker"  # user must resolve before saving


class AlertCode(StrEnum):
    MISSING_EXAM_DATE = "missing_exam_date"
    MISSING_DISCIPLINES = "missing_disciplines"
    QUESTIONS_NOT_BROKEN_DOWN = "questions_not_broken_down"
    QUESTION_SUM_MISMATCH = "question_sum_mismatch"
    DUPLICATE_CARGO_CODE = "duplicate_cargo_code"
    GROUP_NOT_MAPPABLE = "group_not_mappable"
    DURATION_SCOPE_UNCLEAR = "duration_scope_unclear"
    WEIGHT_SCOPE_GROUP_ONLY = "weight_scope_group_only"
    PAGE_CONFLICT = "page_conflict"
    RULE_LLM_DISAGREEMENT = "rule_llm_disagreement"
    EVIDENCE_PAGE_INVALID = "evidence_page_invalid"
    LOW_CONFIDENCE = "low_confidence"


class Alert(WireModel):
    code: AlertCode
    severity: AlertSeverity
    message: str
    # JSON Pointer to the field this is about, when it is about one field.
    field: str | None = Field(default=None, pattern=r"^/.*")
