from __future__ import annotations

from app.services.extraction import extract_native


def test_extracts_text_pages_with_physical_numbers(text_pdf: bytes) -> None:
    out = extract_native(text_pdf)
    assert [p.physical_page for p in out.pages] == [1, 2]
    assert "TRIBUNAL DE CONTAS" in out.pages[0].text
    assert out.pages[0].source == "native_text"
    assert out.pages[0].text_score > 0.5


def test_scanned_pdf_yields_empty_pages(scanned_pdf: bytes) -> None:
    out = extract_native(scanned_pdf)
    assert len(out.pages) == 3
    for page in out.pages:
        assert page.text == ""
        assert page.source == "none"
        assert page.text_score == 0.0


def test_mixed_pdf_scores_pages_independently(mixed_pdf: bytes) -> None:
    out = extract_native(mixed_pdf)
    assert out.pages[0].source == "native_text"
    assert out.pages[0].text_score > 0.4
    assert out.pages[1].source == "none"
    assert out.pages[1].text_score == 0.0


def test_table_reconstruction_from_ruled_table() -> None:
    import pymupdf

    doc = pymupdf.open()
    page = doc.new_page(width=595, height=842)
    # draw a 2x2 grid and label the cells
    page.draw_rect(pymupdf.Rect(50, 50, 250, 150))
    page.draw_line(pymupdf.Point(150, 50), pymupdf.Point(150, 150))
    page.draw_line(pymupdf.Point(50, 100), pymupdf.Point(250, 100))
    page.insert_text((60, 80), "Cargo")
    page.insert_text((160, 80), "Vagas")
    page.insert_text((60, 130), "A01")
    page.insert_text((160, 130), "6")
    data = doc.tobytes()
    doc.close()

    out = extract_native(data)
    assert out.tables, "expected at least one reconstructed table"
    flat = [cell for row in out.tables[0].rows for cell in row]
    assert "Cargo" in flat and "A01" in flat
