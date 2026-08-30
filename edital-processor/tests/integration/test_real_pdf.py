"""Real-PDF checks (spec §14, "Teste real").

Runs only when tests/fixtures/edital_real.pdf exists. Nothing here is hard-coded
into the implementation — these assertions describe the real TCE-GO Edital
01/2026 and would need to change for a different document.
"""

from __future__ import annotations

import pytest

from app.core.config import Settings
from app.services.extraction import extract_native
from app.services.ocr_decision import pages_needing_ocr
from app.services.validation import validate_upload
from tests.conftest import REAL_PDF

pytestmark = [
    pytest.mark.realpdf,
    pytest.mark.skipif(not REAL_PDF.exists(), reason="tests/fixtures/edital_real.pdf not present"),
]


@pytest.fixture
def real_bytes() -> bytes:
    return REAL_PDF.read_bytes()


def test_validates(real_bytes: bytes) -> None:
    result = validate_upload(real_bytes, "application/pdf", Settings())
    assert result.page_count == 26


def test_is_a_fully_scanned_pdf(real_bytes: bytes) -> None:
    out = extract_native(real_bytes)
    assert len(out.pages) == 26
    # This edital has no text layer at all — the whole point of the OCR path.
    assert all(p.source == "none" for p in out.pages)
    assert all(p.text == "" for p in out.pages)


def test_every_page_is_flagged_for_ocr(real_bytes: bytes) -> None:
    out = extract_native(real_bytes)
    settings = Settings()
    assert pages_needing_ocr(out.pages, settings) == list(range(1, 27))


def test_page_geometry_is_a4_portrait(real_bytes: bytes) -> None:
    out = extract_native(real_bytes)
    for page in out.pages:
        assert 580 < page.width < 610  # ~595 pt
        assert 830 < page.height < 850  # ~842 pt


# --- Phase 2: OCR + classification (needs Tesseract, hence integration) ---


@pytest.mark.integration
def test_ocr_reads_the_scanned_edital(real_bytes: bytes, tmp_path: object) -> None:
    from app.core.errors import OCRUnavailable
    from app.services.ocr import run_ocr

    settings = Settings(ocr_dpi=200)  # lower dpi keeps the test quick
    try:
        pages = run_ocr(real_bytes, [1], settings)
    except OCRUnavailable:
        pytest.skip("tesseract not installed on this host")

    assert len(pages) == 1
    text = pages[0].text.upper()
    assert "TRIBUNAL DE CONTAS" in text
    assert "FUNDA" in text and "CARLOS CHAGAS" in text
    assert pages[0].mean_conf is not None and pages[0].mean_conf > 0.7
    assert pages[0].words  # word boxes preserved


@pytest.mark.integration
def test_full_pipeline_classifies_the_real_edital(real_bytes: bytes, tmp_path: object) -> None:
    from app.domain.blocos import Bloco
    from app.services.artifacts import ArtifactStore
    from app.services.pipeline import analyse

    settings = Settings(work_dir=tmp_path / "w", ocr_dpi=200)  # type: ignore[operator]
    store = ArtifactStore(settings)
    outcome = analyse(
        data=real_bytes,
        declared_mime="application/pdf",
        filename="edital.pdf",
        owner_ref="u",
        settings=settings,
        store=store,
    )
    if not outcome.ocr_ran:
        pytest.skip("tesseract not installed on this host")
    doc = store.load(outcome.document.document_id, "u")

    def pages_with(block: Bloco) -> list[int]:
        return [p.physical_page for p in doc.pages if block in p.blocks]

    # Anexo II is pages 19-22 in this edital.
    syllabus = pages_with(Bloco.CONTEUDO_PROGRAMATICO)
    assert set(syllabus) >= {19, 20, 21, 22}
    assert 3 not in syllabus  # legal boilerplate stays out

    # Exam structure is on the "DAS PROVAS" pages.
    assert set(pages_with(Bloco.ESTRUTURA_PROVAS)) & {8, 12, 13}

    # A good chunk of the 26 pages is irrelevant and excluded.
    irrelevant = [p.physical_page for p in doc.pages if p.blocks == [Bloco.IRRELEVANTE]]
    assert len(irrelevant) >= 6
