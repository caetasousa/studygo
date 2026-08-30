from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.alerts import Alert, AlertCode, AlertSeverity
from app.schemas.evidence import Evidence, ExtractionMethod
from app.schemas.extraction import (
    DisciplinaExtraida,
    DocumentInfo,
    ExtractionResult,
    GroupKind,
    GrupoConhecimento,
    WeightScope,
)


def test_absent_question_count_is_none_not_zero() -> None:
    d = DisciplinaExtraida(nome="Lingua Portuguesa")
    assert d.numero_questoes is None
    # zero is representable but distinct from absence
    d0 = DisciplinaExtraida(nome="X", numero_questoes=0)
    assert d0.numero_questoes == 0


def test_group_carries_verbatim_label_and_kind() -> None:
    g = GrupoConhecimento(
        kind=GroupKind.GERAL,
        rotulo="Conhecimentos Gerais",
        total_questoes=25,
        peso=1.0,
        peso_scope=WeightScope.GROUP,
    )
    assert g.kind == "ger"
    assert g.rotulo == "Conhecimentos Gerais"


def test_evidence_field_must_be_json_pointer() -> None:
    with pytest.raises(ValidationError):
        Evidence(
            field="cargos.0.nome",
            physical_page=1,
            snippet="x",
            method=ExtractionMethod.NATIVE_TEXT,
        )
    ok = Evidence(
        field="/cargos/0/nome",
        physical_page=1,
        snippet="Tecnico Administrativo",
        method=ExtractionMethod.OCR,
        confidence=0.8,
    )
    assert ok.physical_page == 1


def test_evidence_page_is_one_based() -> None:
    with pytest.raises(ValidationError):
        Evidence(
            field="/x",
            physical_page=0,
            snippet="x",
            method=ExtractionMethod.LLM,
        )


def test_minimal_result_validates() -> None:
    r = ExtractionResult(
        document=DocumentInfo(filename="e.pdf", sha256="a" * 64, total_pages=26),
    )
    assert r.confidence is None
    assert r.cargos == []


def test_alert_field_optional_pointer() -> None:
    a = Alert(
        code=AlertCode.QUESTIONS_NOT_BROKEN_DOWN,
        severity=AlertSeverity.BLOCKER,
        message="informe a estimativa por disciplina",
        field="/grupos_gerais/0",
    )
    assert a.severity == "blocker"
