"""The structured extraction (spec §8).

Models what studygo consumes plus what a reviewer needs to check the extraction.
Absent data is ``None`` — never ``0``, never an empty list standing in for
"unknown". "Conhecimentos Gerais/Específicos" are groups, never disciplines.
"""

from __future__ import annotations

from enum import StrEnum

from pydantic import Field, model_validator

from app.schemas.alerts import Alert
from app.schemas.base import WireModel
from app.schemas.evidence import Evidence


class GroupKind(StrEnum):
    GERAL = "ger"
    ESPECIFICO = "esp"
    # A group the edital defines that is neither — mapped only after manual
    # review, never silently. Carries the edital's own label.
    OUTRO = "outro"


class DiscursiveKind(StrEnum):
    REDACAO = "redacao"
    ESTUDO_DE_CASO = "estudo_de_caso"
    OUTRO = "outro"


class DurationScope(StrEnum):
    # 4h30 covers the whole objective+discursive sitting.
    EXAM_SET = "exam_set"
    # a single prova has its own limit
    SINGLE_PROVA = "single_prova"
    UNKNOWN = "unknown"


class WeightScope(StrEnum):
    # the weight was stated for the group as a whole
    GROUP = "group"
    # the weight was stated per discipline
    DISCIPLINE = "discipline"


class DocumentInfo(WireModel):
    filename: str
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    total_pages: int = Field(ge=1)


class Cargo(WireModel):
    codigo: str
    nome: str
    especialidade: str | None = None
    escolaridade: str | None = None
    total_vagas: int | None = Field(default=None, ge=0)
    evidence: list[Evidence] = Field(default_factory=list)


class DisciplinaExtraida(WireModel):
    nome: str
    # Only when the edital actually breaks the group's questions down by
    # discipline. Never derived, never defaulted.
    numero_questoes: int | None = Field(default=None, ge=0)
    # Present only when a weight is stated per discipline (WeightScope.DISCIPLINE).
    peso: float | None = Field(default=None, gt=0)
    evidence: list[Evidence] = Field(default_factory=list)


class GrupoConhecimento(WireModel):
    kind: GroupKind
    # The edital's own heading — kept verbatim, needed for OUTRO and for review.
    rotulo: str
    total_questoes: int | None = Field(default=None, ge=0)
    peso: float | None = Field(default=None, gt=0)
    peso_scope: WeightScope | None = None
    disciplinas: list[DisciplinaExtraida] = Field(default_factory=list)
    evidence: list[Evidence] = Field(default_factory=list)


class ProvaDiscursiva(WireModel):
    modalidade: DiscursiveKind
    rotulo: str
    questoes: int | None = Field(default=None, ge=1)
    evidence: list[Evidence] = Field(default_factory=list)


class DuracaoProva(WireModel):
    minutos: int = Field(gt=0)
    scope: DurationScope
    evidence: list[Evidence] = Field(default_factory=list)


class MarcoCronograma(WireModel):
    data_inicio: str = Field(pattern=r"^\d{4}-\d{2}-\d{2}$")
    data_fim: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")
    titulo: str
    exige_acao: bool = False
    evidence: list[Evidence] = Field(default_factory=list)


class ItemConteudo(WireModel):
    # Kept verbatim — laws, versions, technologies are never modernized or
    # summarized (spec §8.4).
    texto: str
    evidence: list[Evidence] = Field(default_factory=list)


class ConteudoDisciplina(WireModel):
    cargo_codigo: str
    grupo_kind: GroupKind
    disciplina: str
    itens: list[ItemConteudo] = Field(default_factory=list)
    # True when this syllabus block is explicitly shared across cargos.
    compartilhado: bool = False


class ExtractionResult(WireModel):
    document: DocumentInfo
    banca: str | None = None
    data_prova: str | None = Field(default=None, pattern=r"^\d{4}-\d{2}-\d{2}$")

    cargos: list[Cargo] = Field(default_factory=list)
    grupos_gerais: list[GrupoConhecimento] = Field(default_factory=list)
    grupos_especificos: list[GrupoConhecimento] = Field(default_factory=list)

    prova_discursiva: list[ProvaDiscursiva] = Field(default_factory=list)
    duracao: DuracaoProva | None = None

    cronograma: list[MarcoCronograma] = Field(default_factory=list)
    conteudo_programatico: list[ConteudoDisciplina] = Field(default_factory=list)

    evidence: list[Evidence] = Field(default_factory=list)
    alerts: list[Alert] = Field(default_factory=list)
    # 0..1 overall, or null when the document was too degraded to score.
    confidence: float | None = Field(default=None, ge=0.0, le=1.0)

    @model_validator(mode="after")
    def _no_zero_for_absent(self) -> ExtractionResult:
        # Belt-and-braces for the §8 rule: a group with 0 questions is almost
        # always "unknown" mis-encoded. It is allowed but must not pass silently.
        return self
