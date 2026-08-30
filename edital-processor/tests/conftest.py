"""Shared fixtures.

PDFs are generated in-process so the hermetic suite needs no binary fixtures.
The real edital, if present at ``tests/fixtures/edital_real.pdf``, is picked up
by the ``realpdf``-marked tests only.
"""

from __future__ import annotations

from pathlib import Path

import pymupdf
import pytest

from app.core.config import Settings
from app.services.artifacts import ArtifactStore

FIXTURES = Path(__file__).parent / "fixtures"
REAL_PDF = FIXTURES / "edital_real.pdf"


@pytest.fixture
def settings(tmp_path: Path) -> Settings:
    return Settings(
        service_token="",
        gemini_api_key="",
        work_dir=tmp_path / "work",
        artifact_ttl_seconds=3600,
    )


@pytest.fixture
def store(settings: Settings) -> ArtifactStore:
    return ArtifactStore(settings)


def _text_pdf(pages: list[str]) -> bytes:
    doc = pymupdf.open()
    for body in pages:
        page = doc.new_page(width=595, height=842)
        page.insert_text((56, 72), body, fontsize=11)
    data: bytes = doc.tobytes()
    doc.close()
    return data


def _image_pdf(pages: int = 2) -> bytes:
    """A scanned-style PDF: each page is a blank raster, zero text layer."""
    doc = pymupdf.open()
    pix = pymupdf.Pixmap(pymupdf.csRGB, pymupdf.IRect(0, 0, 620, 877))
    pix.clear_with(250)
    for _ in range(pages):
        page = doc.new_page(width=595, height=842)
        page.insert_image(page.rect, pixmap=pix)
    data: bytes = doc.tobytes()
    doc.close()
    return data


@pytest.fixture
def text_pdf() -> bytes:
    return _text_pdf(
        [
            "TRIBUNAL DE CONTAS DO ESTADO DE GOIAS\n"
            "CONCURSO PUBLICO EDITAL 01/2026\n"
            "Banca: Fundacao Carlos Chagas\n"
            "Cargo A01 Tecnico Administrativo 6 vagas\n"
            "Cargo B02 Tecnologia da Informacao 10 vagas\n"
            + ("conteudo programatico ementa disciplinas provas questoes peso " * 30),
            "ANEXO II CONTEUDO PROGRAMATICO\n"
            "Lingua Portuguesa: crase, concordancia verbal, regencia.\n"
            + ("topicos e subtopicos do edital detalhados por disciplina " * 30),
        ]
    )


@pytest.fixture
def scanned_pdf() -> bytes:
    return _image_pdf(3)


@pytest.fixture
def mixed_pdf() -> bytes:
    """One good text page, one blank raster page."""
    doc = pymupdf.open()
    p1 = doc.new_page(width=595, height=842)
    p1.insert_text((56, 72), "pagina com texto nativo " * 40, fontsize=11)
    p2 = doc.new_page(width=595, height=842)
    pix = pymupdf.Pixmap(pymupdf.csRGB, pymupdf.IRect(0, 0, 620, 877))
    pix.clear_with(250)
    p2.insert_image(p2.rect, pixmap=pix)
    data: bytes = doc.tobytes()
    doc.close()
    return data


def make_text_pdf(pages: list[str]) -> bytes:
    return _text_pdf(pages)
