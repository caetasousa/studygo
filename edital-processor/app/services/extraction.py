"""Native text and table extraction (spec §7.6-7.7, §7.11-7.12).

PyMuPDF gives the per-page text layer; pdfplumber reconstructs tables. Physical
page numbers are 1-based. This stage does NOT do OCR — it produces the page list
with quality scores so ``ocr_decision`` can pick which pages need it.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import pdfplumber
import pymupdf

from app.schemas.document import ExtractedTable, PageText
from app.services.normalize import normalize_text
from app.services.quality import score_page_text


@dataclass
class ExtractionOutput:
    pages: list[PageText]
    tables: list[ExtractedTable] = field(default_factory=list)


def _printed_page_label(page: pymupdf.Page) -> str | None:
    # PyMuPDF exposes the page label ("iv", "12", ...) when the PDF defines one.
    label = page.get_label() if hasattr(page, "get_label") else ""
    label = (label or "").strip()
    return label or None


def extract_native(data: bytes) -> ExtractionOutput:
    pages: list[PageText] = []

    doc = pymupdf.open(stream=data, filetype="pdf")
    try:
        for index in range(doc.page_count):
            page = doc.load_page(index)
            raw = page.get_text("text") or ""
            text = normalize_text(raw)
            quality = score_page_text(text)
            pages.append(
                PageText(
                    physical_page=index + 1,
                    printed_page=_printed_page_label(page),
                    text=text,
                    source="native_text" if text else "none",
                    text_score=quality.score,
                    width=float(page.rect.width),
                    height=float(page.rect.height),
                )
            )
    finally:
        doc.close()

    tables = _extract_tables(data)
    return ExtractionOutput(pages=pages, tables=tables)


def _extract_tables(data: bytes) -> list[ExtractedTable]:
    import io

    out: list[ExtractedTable] = []
    try:
        with pdfplumber.open(io.BytesIO(data)) as pdf:
            for i, page in enumerate(pdf.pages):
                for table in page.extract_tables() or []:
                    rows = [
                        [(_clean_cell(cell)) for cell in row]
                        for row in table
                        if any(cell not in (None, "") for cell in row)
                    ]
                    if rows:
                        out.append(ExtractedTable(physical_page=i + 1, rows=rows))
    except Exception:
        # pdfplumber is best-effort support; a failure here is not fatal.
        return out
    return out


def _clean_cell(cell: str | None) -> str:
    if cell is None:
        return ""
    return normalize_text(cell).replace("\n", " ").strip()
