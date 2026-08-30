from __future__ import annotations

from app.schemas.alerts import AlertCode, AlertSeverity
from app.schemas.extraction import (
    DisciplinaExtraida,
    DocumentInfo,
    ExtractionResult,
    GroupKind,
    GrupoConhecimento,
    WeightScope,
)
from app.services.validation_rules import validate


def _result(**kw: object) -> ExtractionResult:
    base: dict[str, object] = {
        "document": DocumentInfo(filename="e.pdf", sha256="a" * 64, total_pages=26),
    }
    base.update(kw)
    return ExtractionResult(**base)  # type: ignore[arg-type]


def _codes(result: ExtractionResult) -> set[AlertCode]:
    return {a.code for a in validate(result)}


def test_missing_disciplines_is_a_blocker() -> None:
    alerts = validate(_result())
    assert any(
        a.code == AlertCode.MISSING_DISCIPLINES and a.severity == AlertSeverity.BLOCKER
        for a in alerts
    )


def test_group_total_without_breakdown_blocks() -> None:
    result = _result(
        grupos_gerais=[
            GrupoConhecimento(
                kind=GroupKind.GERAL,
                rotulo="Conhecimentos Gerais",
                total_questoes=25,
                disciplinas=[
                    DisciplinaExtraida(nome="Lingua Portuguesa"),
                    DisciplinaExtraida(nome="Matematica"),
                ],
            )
        ],
        data_prova="2027-01-17",
    )
    alerts = validate(result)
    blocker = next(a for a in alerts if a.code == AlertCode.QUESTIONS_NOT_BROKEN_DOWN)
    assert blocker.severity == AlertSeverity.BLOCKER
    assert blocker.field == "/gruposGerais/0"


def test_question_sum_mismatch_warns() -> None:
    result = _result(
        grupos_especificos=[
            GrupoConhecimento(
                kind=GroupKind.ESPECIFICO,
                rotulo="Especificos",
                total_questoes=45,
                disciplinas=[
                    DisciplinaExtraida(nome="A", numero_questoes=20),
                    DisciplinaExtraida(nome="B", numero_questoes=20),
                ],
            )
        ],
        data_prova="2027-01-17",
    )
    assert AlertCode.QUESTION_SUM_MISMATCH in _codes(result)


def test_matching_sum_produces_no_mismatch_alert() -> None:
    result = _result(
        grupos_especificos=[
            GrupoConhecimento(
                kind=GroupKind.ESPECIFICO,
                rotulo="Especificos",
                total_questoes=45,
                disciplinas=[
                    DisciplinaExtraida(nome="A", numero_questoes=25),
                    DisciplinaExtraida(nome="B", numero_questoes=20),
                ],
            )
        ],
        data_prova="2027-01-17",
    )
    assert AlertCode.QUESTION_SUM_MISMATCH not in _codes(result)


def test_outro_group_is_blocker() -> None:
    result = _result(
        grupos_gerais=[
            GrupoConhecimento(
                kind=GroupKind.OUTRO,
                rotulo="Prova de Titulos",
                disciplinas=[DisciplinaExtraida(nome="X", numero_questoes=1)],
            )
        ],
        data_prova="2027-01-17",
    )
    assert AlertCode.GROUP_NOT_MAPPABLE in _codes(result)


def test_group_scoped_weight_is_info() -> None:
    result = _result(
        grupos_especificos=[
            GrupoConhecimento(
                kind=GroupKind.ESPECIFICO,
                rotulo="Especificos",
                peso=2.0,
                peso_scope=WeightScope.GROUP,
                total_questoes=45,
                disciplinas=[DisciplinaExtraida(nome="A", numero_questoes=45)],
            )
        ],
        data_prova="2027-01-17",
    )
    alert = next(a for a in validate(result) if a.code == AlertCode.WEIGHT_SCOPE_GROUP_ONLY)
    assert alert.severity == AlertSeverity.INFO


def test_missing_exam_date_warns() -> None:
    result = _result(
        grupos_gerais=[
            GrupoConhecimento(
                kind=GroupKind.GERAL,
                rotulo="G",
                total_questoes=25,
                disciplinas=[DisciplinaExtraida(nome="A", numero_questoes=25)],
            )
        ]
    )
    assert AlertCode.MISSING_EXAM_DATE in _codes(result)


def test_duplicate_cargo_code_warns() -> None:
    from app.schemas.extraction import Cargo

    result = _result(
        cargos=[Cargo(codigo="A01", nome="X"), Cargo(codigo="A01", nome="Y")],
        grupos_gerais=[
            GrupoConhecimento(
                kind=GroupKind.GERAL,
                rotulo="G",
                total_questoes=25,
                disciplinas=[DisciplinaExtraida(nome="A", numero_questoes=25)],
            )
        ],
        data_prova="2027-01-17",
    )
    assert AlertCode.DUPLICATE_CARGO_CODE in _codes(result)
