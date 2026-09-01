"""OCR unit tests.

The real Tesseract path is exercised only in the container (integration mark).
Here we test the parts that do not need the binary: the unavailable-guard, and
the word-box geometry conversion via a fake pytesseract.
"""

from __future__ import annotations

import sys
import types

import pytest

from app.core.config import Settings
from app.core.errors import OCRUnavailable
from app.services import ocr


def test_run_ocr_empty_target_list_is_noop() -> None:
    assert ocr.run_ocr(b"", [], Settings()) == []


def test_missing_tesseract_raises_unavailable(monkeypatch: pytest.MonkeyPatch) -> None:
    fake = types.ModuleType("pytesseract")

    def _boom() -> None:
        raise RuntimeError("tesseract is not installed or it's not in your PATH")

    fake.get_tesseract_version = _boom  # type: ignore[attr-defined]
    monkeypatch.setitem(sys.modules, "pytesseract", fake)

    with pytest.raises(OCRUnavailable):
        ocr._import_tesseract()


def test_one_page_timeout_does_not_discard_the_rest(
    monkeypatch: pytest.MonkeyPatch, scanned_pdf: bytes
) -> None:
    """A single slow page used to take the whole batch down with it: run_ocr
    raised out of the whole function on the first per-page timeout, so the
    caller discarded every result — including pages that had already
    finished fine. _ocr_one is the seam that actually raises and the only one
    that knows which physical page it is running, so it is patched directly
    rather than pytesseract itself."""
    fake = types.ModuleType("pytesseract")
    fake.get_tesseract_version = lambda: "5.5.0"  # type: ignore[attr-defined]
    monkeypatch.setitem(sys.modules, "pytesseract", fake)

    def flaky(png: bytes, physical_page: int, settings: Settings, tess: object) -> ocr.OCRPage:
        if physical_page == 2:
            raise RuntimeError("Tesseract process timeout")
        return ocr.OCRPage(physical_page=physical_page, text="ok")

    monkeypatch.setattr(ocr, "_ocr_one", flaky)

    # scanned_pdf has 3 pages; page 2 always times out, 1 and 3 succeed.
    pages = ocr.run_ocr(scanned_pdf, [1, 2, 3], Settings())

    got_pages = sorted(p.physical_page for p in pages)
    assert got_pages == [1, 3], "a página que travou não devia levar as outras junto"


def test_word_boxes_convert_px_to_points(
    monkeypatch: pytest.MonkeyPatch, scanned_pdf: bytes
) -> None:
    fake = types.ModuleType("pytesseract")
    fake.get_tesseract_version = lambda: "5.5.0"  # type: ignore[attr-defined]
    fake.Output = types.SimpleNamespace(DICT="dict")  # type: ignore[attr-defined]

    def image_to_data(image: object, **kwargs: object) -> dict[str, list[object]]:
        # one word, at 300 dpi: 300px in => 72pt out
        return {
            "text": ["", "EDITAL"],
            "conf": [-1, 96],
            "left": [0, 300],
            "top": [0, 0],
            "width": [0, 300],
            "height": [0, 30],
            "block_num": [0, 1],
            "par_num": [0, 1],
            "line_num": [0, 1],
        }

    fake.image_to_data = image_to_data  # type: ignore[attr-defined]
    monkeypatch.setitem(sys.modules, "pytesseract", fake)

    settings = Settings(ocr_dpi=300)
    pages = ocr.run_ocr(scanned_pdf, [1], settings)
    assert len(pages) == 1
    box = pages[0].words[0]
    assert box.text == "EDITAL"
    assert box.x0 == pytest.approx(72.0, abs=0.5)
    assert box.x1 == pytest.approx(144.0, abs=0.5)
    assert box.conf == pytest.approx(0.96)
    assert pages[0].text == "EDITAL"
    assert pages[0].mean_conf == pytest.approx(0.96)
