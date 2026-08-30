"""Real Gemini extraction against the real edital.

Runs only when GEMINI_API_KEY is set AND tests/fixtures/edital_real.pdf exists.
Never part of the default suite. Asserts facts about the real TCE-GO Edital
01/2026 — none of these values are hard-coded in the implementation.
"""

from __future__ import annotations

import os
from pathlib import Path

import pytest

from app.core.config import Settings
from app.services.artifacts import ArtifactStore
from app.services.extract_llm import extract_cargos, extract_conteudo, extract_estrutura
from app.services.pipeline import analyse
from tests.conftest import REAL_PDF

pytestmark = [
    pytest.mark.gemini,
    pytest.mark.realpdf,
    pytest.mark.skipif(not REAL_PDF.exists(), reason="tests/fixtures/edital_real.pdf missing"),
    pytest.mark.skipif(not os.getenv("GEMINI_API_KEY"), reason="GEMINI_API_KEY not set"),
]


@pytest.fixture(scope="module")
def analysed() -> tuple[Settings, ArtifactStore, str]:
    settings = Settings(
        gemini_api_key=os.environ["GEMINI_API_KEY"],
        work_dir=Path("/tmp/ep-itest-work"),
        ocr_dpi=220,
    )
    store = ArtifactStore(settings)
    outcome = analyse(
        data=REAL_PDF.read_bytes(),
        declared_mime="application/pdf",
        filename="edital.pdf",
        owner_ref="itest",
        settings=settings,
        store=store,
    )
    assert outcome.ocr_ran, "OCR must run for this test"
    return settings, store, outcome.document.document_id


async def test_extracts_banca_and_both_cargos(
    analysed: tuple[Settings, ArtifactStore, str],
) -> None:
    settings, store, doc_id = analysed
    from app.providers.gemini import GeminiProvider

    doc = store.load(doc_id, "itest")
    banca, cargos = await extract_cargos(doc, GeminiProvider(settings))

    assert banca is not None and "carlos chagas" in banca.lower()
    codes = {c.codigo.upper() for c in cargos}
    assert {"A01", "B02"} <= codes


async def test_b02_structure_has_25_45_split_and_estudo_de_caso(
    analysed: tuple[Settings, ArtifactStore, str],
) -> None:
    settings, store, doc_id = analysed
    from app.providers.gemini import GeminiProvider

    doc = store.load(doc_id, "itest")
    result = await extract_estrutura(doc, "B02", GeminiProvider(settings))

    gerais = result.grupos_gerais
    especificos = result.grupos_especificos
    assert gerais and especificos

    assert any(g.total_questoes == 25 for g in gerais)
    assert any(g.total_questoes == 45 for g in especificos)
    # weights 1 and 2
    assert any(g.peso == 1 for g in gerais)
    assert any(g.peso == 2 for g in especificos)
    # B02's discursive is Estudo de Caso, not Redação
    assert any(pd.modalidade.value == "estudo_de_caso" for pd in result.prova_discursiva)
    # duration 4h30 = 270 min, covering the exam set
    assert result.duracao is not None and result.duracao.minutos == 270


async def test_engenharia_de_software_is_b02_specific_content(
    analysed: tuple[Settings, ArtifactStore, str],
) -> None:
    settings, store, doc_id = analysed
    from app.providers.gemini import GeminiProvider

    doc = store.load(doc_id, "itest")
    result = await extract_conteudo(
        doc, "B02", ["Engenharia de Software", "Lingua Portuguesa"], GeminiProvider(settings)
    )
    by_disc = {i.disciplina: i for i in result.itens}
    eng = by_disc["Engenharia de Software"]
    assert eng.itens, "Engenharia de Software should have topics"
    joined = " ".join(eng.itens).lower()
    assert "engenharia de software" in joined or "ciclo de vida" in joined
    # evidence points at real pages
    for ev in eng.evidence:
        assert 1 <= ev.physical_page <= doc.total_pages
