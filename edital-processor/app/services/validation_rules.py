"""Deterministic cross-validation (spec §12).

Runs over the assembled ExtractionResult and produces alerts. It never mutates
the result — the user resolves the alerts during review.
"""

from __future__ import annotations

from datetime import date

from app.schemas.alerts import Alert, AlertCode, AlertSeverity
from app.schemas.evidence import Evidence
from app.schemas.extraction import ExtractionResult, GroupKind, GrupoConhecimento

_MAX_PAGE_SLACK = 0


def _iso_ok(value: str | None) -> bool:
    if not value:
        return True
    try:
        date.fromisoformat(value)
        return True
    except ValueError:
        return False


def validate(result: ExtractionResult) -> list[Alert]:
    alerts: list[Alert] = []
    alerts += _cargo_codes_unique(result)
    alerts += _exam_date(result)
    alerts += _disciplines_present(result)
    alerts += _question_breakdown(result)
    alerts += _group_mappable(result)
    alerts += _duration_scope(result)
    alerts += _dates_iso(result)
    alerts += _evidence_pages(result)
    return alerts


def _cargo_codes_unique(result: ExtractionResult) -> list[Alert]:
    seen: set[str] = set()
    dupes: set[str] = set()
    for cargo in result.cargos:
        if cargo.codigo in seen:
            dupes.add(cargo.codigo)
        seen.add(cargo.codigo)
    return [
        Alert(
            code=AlertCode.DUPLICATE_CARGO_CODE,
            severity=AlertSeverity.WARNING,
            message=f"código de cargo repetido: {code}",
        )
        for code in sorted(dupes)
    ]


def _exam_date(result: ExtractionResult) -> list[Alert]:
    if result.data_prova:
        return []
    return [
        Alert(
            code=AlertCode.MISSING_EXAM_DATE,
            severity=AlertSeverity.WARNING,
            message="o edital não trazia a data da prova — preencha antes de salvar",
            field="/dataProva",
        )
    ]


def _disciplines_present(result: ExtractionResult) -> list[Alert]:
    groups = result.grupos_gerais + result.grupos_especificos
    if any(g.disciplinas for g in groups):
        return []
    return [
        Alert(
            code=AlertCode.MISSING_DISCIPLINES,
            severity=AlertSeverity.BLOCKER,
            message="não foi possível identificar as disciplinas — ajuste manualmente",
        )
    ]


def _question_breakdown(result: ExtractionResult) -> list[Alert]:
    alerts: list[Alert] = []
    for scope, groups in (
        ("gruposGerais", result.grupos_gerais),
        ("gruposEspecificos", result.grupos_especificos),
    ):
        for i, group in enumerate(groups):
            field = f"/{scope}/{i}"
            counts = [d.numero_questoes for d in group.disciplinas]
            has_any = any(c is not None for c in counts)
            has_all = all(c is not None for c in counts) and bool(counts)

            if group.disciplinas and not has_any:
                alerts.append(
                    Alert(
                        code=AlertCode.QUESTIONS_NOT_BROKEN_DOWN,
                        severity=AlertSeverity.BLOCKER,
                        message=(
                            f"o edital informou apenas o total do grupo "
                            f"({group.total_questoes}) — informe a estimativa por "
                            f"disciplina ou rateie explicitamente"
                        ),
                        field=field,
                    )
                )
            elif has_all and group.total_questoes is not None:
                total = sum(c for c in counts if c is not None)
                if total != group.total_questoes:
                    alerts.append(
                        Alert(
                            code=AlertCode.QUESTION_SUM_MISMATCH,
                            severity=AlertSeverity.WARNING,
                            message=(
                                f"a soma das questões por disciplina ({total}) não "
                                f"bate com o total do grupo ({group.total_questoes})"
                            ),
                            field=field,
                        )
                    )
    return alerts


def _group_mappable(result: ExtractionResult) -> list[Alert]:
    alerts: list[Alert] = []
    for scope, groups in (
        ("gruposGerais", result.grupos_gerais),
        ("gruposEspecificos", result.grupos_especificos),
    ):
        for i, group in enumerate(groups):
            if group.kind == GroupKind.OUTRO:
                alerts.append(
                    Alert(
                        code=AlertCode.GROUP_NOT_MAPPABLE,
                        severity=AlertSeverity.BLOCKER,
                        message=(
                            f'o grupo "{group.rotulo}" não é Conhecimentos Gerais '
                            f"nem Específicos — classifique manualmente"
                        ),
                        field=f"/{scope}/{i}",
                    )
                )
            if _weight_group_only(group):
                alerts.append(
                    Alert(
                        code=AlertCode.WEIGHT_SCOPE_GROUP_ONLY,
                        severity=AlertSeverity.INFO,
                        message=(
                            f'o peso de "{group.rotulo}" foi informado para o grupo, '
                            f"não por disciplina"
                        ),
                        field=f"/{scope}/{i}",
                    )
                )
    return alerts


def _weight_group_only(group: GrupoConhecimento) -> bool:
    return (
        group.peso is not None
        and group.peso_scope is not None
        and group.peso_scope.value == "group"
        and not any(d.peso is not None for d in group.disciplinas)
    )


def _duration_scope(result: ExtractionResult) -> list[Alert]:
    if result.duracao is None:
        return []
    if result.duracao.scope.value == "unknown":
        return [
            Alert(
                code=AlertCode.DURATION_SCOPE_UNCLEAR,
                severity=AlertSeverity.WARNING,
                message="não ficou claro se a duração cobre o conjunto de provas ou uma prova só",
                field="/duracao",
            )
        ]
    return []


def _dates_iso(result: ExtractionResult) -> list[Alert]:
    bad: list[str] = []
    if not _iso_ok(result.data_prova):
        bad.append("/dataProva")
    for i, marco in enumerate(result.cronograma):
        if not _iso_ok(marco.data_inicio) or not _iso_ok(marco.data_fim):
            bad.append(f"/cronograma/{i}")
    return [
        Alert(
            code=AlertCode.EVIDENCE_PAGE_INVALID,
            severity=AlertSeverity.WARNING,
            message=f"data em formato inválido em {field}",
            field=field,
        )
        for field in bad
    ]


def _evidence_pages(result: ExtractionResult) -> list[Alert]:
    total = result.document.total_pages
    bad: set[str] = set()
    for ev in _all_evidence(result):
        if ev.physical_page < 1 or ev.physical_page > total:
            bad.add(ev.field)
    return [
        Alert(
            code=AlertCode.EVIDENCE_PAGE_INVALID,
            severity=AlertSeverity.WARNING,
            message=f"evidência aponta para uma página inexistente: {field}",
            field=field,
        )
        for field in sorted(bad)
    ]


def _all_evidence(result: ExtractionResult) -> list[Evidence]:
    out: list[Evidence] = list(result.evidence)
    for cargo in result.cargos:
        out += cargo.evidence
    for group in result.grupos_gerais + result.grupos_especificos:
        out += group.evidence
        for disc in group.disciplinas:
            out += disc.evidence
    for pd in result.prova_discursiva:
        out += pd.evidence
    if result.duracao:
        out += result.duracao.evidence
    for marco in result.cronograma:
        out += marco.evidence
    for cont in result.conteudo_programatico:
        for item in cont.itens:
            out += item.evidence
    return out
