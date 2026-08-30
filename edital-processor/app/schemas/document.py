"""The normalized document artifact.

Produced by the pipeline's extraction stage, stored under a random UUID with a
TTL, and reused by the later wizard steps so the PDF is processed once. It never
contains a client-supplied path or an external provider URI.
"""

from __future__ import annotations

import time

from pydantic import BaseModel, Field

from app.domain.blocos import Bloco


class WordBoxModel(BaseModel):
    text: str
    x0: float
    y0: float
    x1: float
    y1: float
    conf: float = Field(ge=0.0, le=1.0)


class PageText(BaseModel):
    physical_page: int = Field(ge=1)
    printed_page: str | None = None
    # The page's text, normalized. Empty when the page had no usable text and
    # OCR has not (yet) run.
    text: str = ""
    # How ``text`` was obtained.
    source: str = "none"  # "native_text" | "ocr" | "none"
    # 0..1 native-text quality; drives the OCR decision.
    text_score: float = 0.0
    # 0..1 mean OCR confidence, when source == "ocr".
    ocr_confidence: float | None = None
    width: float = 0.0
    height: float = 0.0
    blocks: list[Bloco] = Field(default_factory=list)
    # Word boxes from image_to_data, in page points. Empty for native text.
    word_boxes: list[WordBoxModel] = Field(default_factory=list)

    @property
    def has_word_boxes(self) -> bool:
        return bool(self.word_boxes)


class ExtractedTable(BaseModel):
    physical_page: int = Field(ge=1)
    # Rows of cell strings; merged cells are expanded to the spanning value.
    rows: list[list[str]] = Field(default_factory=list)


class NormalizedDocument(BaseModel):
    document_id: str
    owner_ref: str
    filename: str
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    total_pages: int = Field(ge=1)
    created_at: float = Field(default_factory=time.time)
    ttl_seconds: int
    pages: list[PageText] = Field(default_factory=list)
    tables: list[ExtractedTable] = Field(default_factory=list)

    @property
    def is_expired(self) -> bool:
        return time.time() - self.created_at > self.ttl_seconds
