from __future__ import annotations

import pytest

from app.core.config import Settings
from app.services.artifacts import ArtifactStore
from app.services.pipeline import analyse


def test_analyse_text_pdf_persists_artifact(
    text_pdf: bytes, settings: Settings, store: ArtifactStore
) -> None:
    outcome = analyse(
        data=text_pdf,
        declared_mime="application/pdf",
        filename="edital.pdf",
        owner_ref="user-1",
        settings=settings,
        store=store,
    )
    assert outcome.document.total_pages == 2
    assert outcome.document.sha256 != ""
    reloaded = store.load(outcome.document.document_id, "user-1")
    assert len(reloaded.pages) == 2
    # a text PDF needs no OCR at all
    assert outcome.ocr_page_numbers == []
    assert outcome.ocr_ran is False


def test_analyse_flags_scanned_pages_even_without_tesseract(
    scanned_pdf: bytes, settings: Settings, store: ArtifactStore
) -> None:
    # The decision is independent of whether Tesseract is installed.
    outcome = analyse(
        data=scanned_pdf,
        declared_mime="application/pdf",
        filename="scan.pdf",
        owner_ref="user-1",
        settings=settings,
        store=store,
    )
    assert outcome.ocr_page_numbers == [1, 2, 3]
    # The document is still stored whether or not OCR could run.
    assert store.load(outcome.document.document_id, "user-1").total_pages == 3


def test_analyse_mixed_pdf_flags_only_the_blank_page(
    mixed_pdf: bytes, settings: Settings, store: ArtifactStore
) -> None:
    outcome = analyse(
        data=mixed_pdf,
        declared_mime="application/pdf",
        filename="mixed.pdf",
        owner_ref="user-1",
        settings=settings,
        store=store,
    )
    assert outcome.ocr_page_numbers == [2]


def test_ocr_merged_into_pages_when_it_runs(
    monkeypatch: pytest.MonkeyPatch,
    scanned_pdf: bytes,
    settings: Settings,
    store: ArtifactStore,
) -> None:
    from app.services import pipeline
    from app.services.ocr import OCRPage, WordBox

    def fake_run_ocr(data: bytes, page_numbers: list[int], s: Settings) -> list[OCRPage]:
        return [
            OCRPage(
                physical_page=n,
                text=f"texto reconhecido da pagina {n} conteudo programatico",
                words=[WordBox(text="texto", x0=1, y0=2, x1=3, y1=4, conf=0.9)],
                mean_conf=0.9,
            )
            for n in page_numbers
        ]

    monkeypatch.setattr(pipeline, "run_ocr", fake_run_ocr)

    outcome = analyse(
        data=scanned_pdf,
        declared_mime="application/pdf",
        filename="scan.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
    )
    assert outcome.ocr_ran is True
    doc = store.load(outcome.document.document_id, "u")
    assert all(p.source == "ocr" for p in doc.pages)
    assert all(p.text for p in doc.pages)
    assert doc.pages[0].ocr_confidence == 0.9
    assert doc.pages[0].has_word_boxes


def test_ocr_failure_is_swallowed(
    monkeypatch: pytest.MonkeyPatch,
    scanned_pdf: bytes,
    settings: Settings,
    store: ArtifactStore,
) -> None:
    from app.core.errors import OCRUnavailable
    from app.services import pipeline

    def boom(*args: object, **kwargs: object) -> object:
        raise OCRUnavailable("no tesseract")

    monkeypatch.setattr(pipeline, "run_ocr", boom)

    outcome = analyse(
        data=scanned_pdf,
        declared_mime="application/pdf",
        filename="scan.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
    )
    assert outcome.ocr_ran is False
    doc = store.load(outcome.document.document_id, "u")
    assert all(p.source == "none" for p in doc.pages)


def test_sha256_is_stable_for_same_bytes(
    text_pdf: bytes, settings: Settings, store: ArtifactStore
) -> None:
    a = analyse(
        data=text_pdf,
        declared_mime="application/pdf",
        filename="e.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
    )
    b = analyse(
        data=text_pdf,
        declared_mime="application/pdf",
        filename="e.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
    )
    assert a.document.sha256 == b.document.sha256
    assert a.document.document_id != b.document.document_id
