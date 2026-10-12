from __future__ import annotations

from app.adapter.disco.artefatos import ArtifactStore
from app.adapter.ocr.tesseract import OcrTesseract
from app.adapter.pdf.leitor import LeitorPyMuPDF
from app.platform.config import Settings
from app.port.ocr import OCRPage, WordBox
from app.service.edital.analise import analyse


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
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrTesseract(settings),
    )
    assert outcome.document.total_pages == 2
    assert outcome.document.sha256 != ""
    reloaded = store.load(outcome.document.document_id)
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
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrTesseract(settings),
    )
    assert outcome.ocr_page_numbers == [1, 2, 3]
    # The document is still stored whether or not OCR could run.
    assert store.load(outcome.document.document_id).total_pages == 3


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
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrTesseract(settings),
    )
    assert outcome.ocr_page_numbers == [2]


def test_ocr_merged_into_pages_when_it_runs(
    scanned_pdf: bytes,
    settings: Settings,
    store: ArtifactStore,
) -> None:
    class OcrQueReconhece:
        def reconhecer(self, data: bytes, page_numbers: list[int]) -> list[OCRPage]:
            return [
                OCRPage(
                    physical_page=n,
                    text=f"texto reconhecido da pagina {n} conteudo programatico",
                    words=[WordBox(text="texto", x0=1, y0=2, x1=3, y1=4, conf=0.9)],
                    mean_conf=0.9,
                )
                for n in page_numbers
            ]

    outcome = analyse(
        data=scanned_pdf,
        declared_mime="application/pdf",
        filename="scan.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrQueReconhece(),
    )
    assert outcome.ocr_ran is True
    doc = store.load(outcome.document.document_id)
    assert all(p.source == "ocr" for p in doc.pages)
    assert all(p.text for p in doc.pages)
    assert doc.pages[0].ocr_confidence == 0.9
    assert doc.pages[0].has_word_boxes


def test_ocr_failure_is_swallowed(
    scanned_pdf: bytes,
    settings: Settings,
    store: ArtifactStore,
) -> None:
    from app.platform.errors import OCRUnavailable

    class OcrSemTesseract:
        def reconhecer(self, data: bytes, page_numbers: list[int]) -> list[OCRPage]:
            raise OCRUnavailable("no tesseract")

    outcome = analyse(
        data=scanned_pdf,
        declared_mime="application/pdf",
        filename="scan.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrSemTesseract(),
    )
    assert outcome.ocr_ran is False
    doc = store.load(outcome.document.document_id)
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
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrTesseract(settings),
    )
    b = analyse(
        data=text_pdf,
        declared_mime="application/pdf",
        filename="e.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
        pdf=LeitorPyMuPDF(settings),
        ocr=OcrTesseract(settings),
    )
    assert a.document.sha256 == b.document.sha256
    assert a.document.document_id != b.document.document_id
