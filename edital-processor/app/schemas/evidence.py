"""Evidence and confidence (spec §11).

Every critical fact points back to where it came from: a stable field path, the
physical PDF page, how it was read, and a confidence the formula in
``app.services.confidence`` computes — never a number the model reports about
itself.
"""

from __future__ import annotations

from enum import StrEnum

from pydantic import Field

from app.schemas.base import WireModel


class ExtractionMethod(StrEnum):
    NATIVE_TEXT = "native_text"
    OCR = "ocr"
    TABLE = "table"
    LLM = "llm"
    # combinations the pipeline actually produces
    NATIVE_TEXT_LLM = "native_text+llm"
    OCR_LLM = "ocr+llm"
    TABLE_LLM = "table+llm"


class BoundingBox(WireModel):
    x0: float
    y0: float
    x1: float
    y1: float


class Evidence(WireModel):
    # JSON Pointer into the ExtractionResult, e.g. "/cargos/1/totalVagas".
    field: str = Field(pattern=r"^/.*")
    # Physical PDF page, 1-based.
    physical_page: int = Field(ge=1)
    # The number printed on the page, when it differs from the physical index.
    printed_page: str | None = None
    section: str | None = None
    # A short quote from the normalized page text; long enough to locate, short
    # enough not to leak the document.
    snippet: str = Field(max_length=400)
    method: ExtractionMethod
    bbox: BoundingBox | None = None
    # 0..1, or null when no method is defensible.
    confidence: float | None = Field(default=None, ge=0.0, le=1.0)
