from __future__ import annotations

from app.services.normalize import normalize_text, snippet_matches_page


def test_collapses_whitespace_and_blank_runs() -> None:
    raw = "linha   um\n\n\n\nlinha    dois   \n"
    assert normalize_text(raw) == "linha um\n\nlinha dois"


def test_dehyphenates_line_wrapped_words() -> None:
    assert normalize_text("concur-\nso publico") == "concurso publico"


def test_crlf_normalized() -> None:
    assert normalize_text("a\r\nb") == "a\nb"


def test_snippet_matches_native_text_exactly() -> None:
    page = "O Concurso Publico realizar-se-a sob a responsabilidade da FCC."
    assert snippet_matches_page("responsabilidade da FCC", page, ocr=False)
    assert not snippet_matches_page("responsabilidade da banca", page, ocr=False)


def test_ocr_snippet_tolerates_spacing_noise() -> None:
    page = "Conhecimentos Especificos 45 questoes peso 2"
    noisy = "Conhecimentos  Espec ificos 45 questoes"
    assert snippet_matches_page(noisy, page, ocr=True)
    assert not snippet_matches_page(noisy, page, ocr=False)
