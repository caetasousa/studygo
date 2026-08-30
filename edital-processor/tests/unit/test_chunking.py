from __future__ import annotations

from app.domain.blocos import Bloco
from app.schemas.document import NormalizedDocument, PageText
from app.services.chunking import (
    build_chunks,
    chunks_for_blocks,
    render_for_prompt,
)


def _doc(pages: list[PageText]) -> NormalizedDocument:
    return NormalizedDocument(
        document_id="d",
        owner_ref="u",
        filename="e.pdf",
        sha256="a" * 64,
        total_pages=len(pages),
        ttl_seconds=3600,
        pages=pages,
    )


def test_only_relevant_pages_become_chunks() -> None:
    doc = _doc(
        [
            PageText(physical_page=1, text="capa", source="ocr", blocks=[Bloco.DADOS_GERAIS]),
            PageText(physical_page=2, text="prosa legal", source="ocr", blocks=[Bloco.IRRELEVANTE]),
            PageText(
                physical_page=3,
                text="conteudo programatico",
                source="ocr",
                blocks=[Bloco.CONTEUDO_PROGRAMATICO],
            ),
        ]
    )
    chunks = build_chunks(doc)
    assert [c.physical_page for c in chunks] == [1, 3]


def test_chunk_ids_are_stable_and_page_anchored() -> None:
    doc = _doc(
        [
            PageText(
                physical_page=7, text="x" * 10, source="native_text", blocks=[Bloco.CARGOS_VAGAS]
            )
        ]
    )
    chunks = build_chunks(doc)
    assert chunks[0].id == "p7#0"
    assert chunks[0].physical_page == 7


def test_long_page_splits_on_paragraphs() -> None:
    body = "\n\n".join(f"paragrafo {i} " + "palavra " * 60 for i in range(10))
    doc = _doc(
        [
            PageText(
                physical_page=1,
                text=body,
                source="ocr",
                blocks=[Bloco.CONTEUDO_PROGRAMATICO],
            )
        ]
    )
    chunks = build_chunks(doc)
    assert len(chunks) > 1
    assert [c.id for c in chunks] == [f"p1#{i}" for i in range(len(chunks))]
    assert all(len(c.text) <= 2600 for c in chunks)


def test_chunks_for_blocks_filters() -> None:
    doc = _doc(
        [
            PageText(physical_page=1, text="a", source="ocr", blocks=[Bloco.CARGOS_VAGAS]),
            PageText(physical_page=2, text="b", source="ocr", blocks=[Bloco.CONTEUDO_PROGRAMATICO]),
        ]
    )
    chunks = build_chunks(doc)
    only_syllabus = chunks_for_blocks(chunks, {Bloco.CONTEUDO_PROGRAMATICO})
    assert [c.physical_page for c in only_syllabus] == [2]


def test_render_wraps_with_delimiters_and_id() -> None:
    doc = _doc([PageText(physical_page=3, text="linha", source="ocr", blocks=[Bloco.CRONOGRAMA])])
    rendered = render_for_prompt(build_chunks(doc))
    assert rendered[0].startswith("<<<CHUNK id=p3#0 pagina_fisica=3 fonte=ocr>>>")
    assert rendered[0].endswith("<<<FIM CHUNK p3#0>>>")
    assert "linha" in rendered[0]
