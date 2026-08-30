"""The analyse stage.

Phase 2 scope: validate -> hash -> native extract -> score -> selective OCR ->
classify -> persist the normalized artifact. The LLM extraction (Phase 3) reads
the artifact this produces.

OCR is best-effort: if Tesseract is unavailable the pages that needed it keep
their empty text and the document is still stored (the LLM step will have less
to work with, and confidence will reflect that).
"""

from __future__ import annotations

import time
from dataclasses import dataclass

from app.core.config import Settings
from app.core.errors import OCRTimeout, OCRUnavailable
from app.core.logging import get_logger
from app.schemas.document import NormalizedDocument, PageText, WordBoxModel
from app.services.artifacts import ArtifactStore
from app.services.classification import classify_document
from app.services.extraction import extract_native
from app.services.hashing import sha256_hex
from app.services.normalize import normalize_text
from app.services.ocr import run_ocr
from app.services.ocr_decision import pages_needing_ocr
from app.services.quality import score_page_text
from app.services.validation import validate_upload

_log = get_logger("pipeline")


@dataclass
class AnalyseOutcome:
    document: NormalizedDocument
    ocr_page_numbers: list[int]
    ocr_ran: bool


def analyse(
    *,
    data: bytes,
    declared_mime: str | None,
    filename: str,
    owner_ref: str,
    settings: Settings,
    store: ArtifactStore,
    request_id: str | None = None,
) -> AnalyseOutcome:
    started = time.monotonic()

    validated = validate_upload(data, declared_mime, settings)
    digest = sha256_hex(validated.data)

    extraction = extract_native(validated.data)
    ocr_targets = pages_needing_ocr(extraction.pages, settings)

    ocr_ran = False
    if ocr_targets:
        ocr_ran = _apply_ocr(extraction.pages, validated.data, ocr_targets, settings, request_id)

    # Classify every page from whatever text it now has, with the section sweep.
    page_blocks = classify_document([p.text for p in extraction.pages])
    for page, blocks in zip(extraction.pages, page_blocks, strict=True):
        page.blocks = blocks

    doc = NormalizedDocument(
        document_id=store.create_id(),
        owner_ref=owner_ref,
        filename=filename,
        sha256=digest,
        total_pages=validated.page_count,
        ttl_seconds=settings.artifact_ttl_seconds,
        pages=extraction.pages,
        tables=extraction.tables,
    )
    store.save(doc)

    _log.info(
        "edital analysed",
        extra={
            "request_id": request_id,
            "stage": "analyse",
            "duration_ms": round((time.monotonic() - started) * 1000),
            "page_count": validated.page_count,
            "ocr_pages": len(ocr_targets) if ocr_ran else 0,
            "document_id": doc.document_id,
            "bytes": len(validated.data),
        },
    )
    return AnalyseOutcome(document=doc, ocr_page_numbers=ocr_targets, ocr_ran=ocr_ran)


def analyse_text(
    *,
    text: str,
    filename: str,
    owner_ref: str,
    settings: Settings,
    store: ArtifactStore,
    request_id: str | None = None,
) -> AnalyseOutcome:
    """Pasted-text path: no PDF, no OCR. The text becomes a single-page
    document, classified and chunked like any other."""
    started = time.monotonic()

    normalized = normalize_text(text)
    digest = sha256_hex(normalized.encode("utf-8"))
    quality = score_page_text(normalized)

    page = PageText(
        physical_page=1,
        text=normalized,
        source="native_text",
        text_score=quality.score,
    )
    page.blocks = classify_document([normalized])[0]

    doc = NormalizedDocument(
        document_id=store.create_id(),
        owner_ref=owner_ref,
        filename=filename,
        sha256=digest,
        total_pages=1,
        ttl_seconds=settings.artifact_ttl_seconds,
        pages=[page],
    )
    store.save(doc)

    _log.info(
        "edital text analysed",
        extra={
            "request_id": request_id,
            "stage": "analyse",
            "duration_ms": round((time.monotonic() - started) * 1000),
            "page_count": 1,
            "document_id": doc.document_id,
            "bytes": len(normalized),
        },
    )
    return AnalyseOutcome(document=doc, ocr_page_numbers=[], ocr_ran=False)


def _apply_ocr(
    pages: list[PageText],
    data: bytes,
    targets: list[int],
    settings: Settings,
    request_id: str | None,
) -> bool:
    """Run OCR on ``targets`` and merge the results into ``pages`` in place.
    Returns whether OCR actually ran."""
    try:
        ocr_results = run_ocr(data, targets, settings)
    except (OCRUnavailable, OCRTimeout) as exc:
        _log.warning(
            "OCR skipped",
            extra={"request_id": request_id, "stage": "ocr", "error_code": exc.code},
        )
        return False

    by_page = {r.physical_page: r for r in ocr_results}
    for page in pages:
        result = by_page.get(page.physical_page)
        if result is None:
            continue
        page.text = result.text
        page.source = "ocr"
        page.ocr_confidence = result.mean_conf
        page.word_boxes = [
            WordBoxModel(text=w.text, x0=w.x0, y0=w.y0, x1=w.x1, y1=w.y1, conf=w.conf)
            for w in result.words
        ]
    return True
