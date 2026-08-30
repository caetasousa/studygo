from __future__ import annotations

from app.core.config import Settings
from app.schemas.document import PageText
from app.services.ocr_decision import pages_needing_ocr


def _page(n: int, *, text: str, score: float) -> PageText:
    return PageText(physical_page=n, text=text, text_score=score, source="native_text")


def test_good_pages_are_not_ocrd(settings: Settings) -> None:
    pages = [
        _page(1, text="conteudo real e legivel " * 20, score=0.9),
        _page(2, text="mais texto de boa qualidade " * 20, score=0.8),
    ]
    assert pages_needing_ocr(pages, settings) == []


def test_empty_page_is_ocrd(settings: Settings) -> None:
    pages = [_page(1, text="", score=0.0)]
    assert pages_needing_ocr(pages, settings) == [1]


def test_low_score_page_is_ocrd(settings: Settings) -> None:
    pages = [
        _page(1, text="conteudo bom " * 30, score=0.9),
        _page(2, text="g4rbl3 \x00 ilegivel " * 30, score=0.3),
    ]
    assert pages_needing_ocr(pages, settings) == [2]


def test_short_but_present_text_below_char_floor_is_ocrd(settings: Settings) -> None:
    pages = [_page(1, text="Anexo", score=0.9)]
    assert pages_needing_ocr(pages, settings) == [1]


def test_thresholds_are_configurable(settings: Settings) -> None:
    pages = [_page(1, text="x" * 100, score=0.6)]
    strict = settings.model_copy(update={"min_text_score": 0.7})
    assert pages_needing_ocr(pages, settings) == []
    assert pages_needing_ocr(pages, strict) == [1]
