"""LLM extraction steps (spec §16-20).

Each step selects the chunks its blocks need, calls the provider once, maps the
raw JSON onto the typed schema, attaches evidence anchored to real pages, and
leaves the deterministic checks to ``validation_rules``. The provider never sees
the whole document.
"""

from __future__ import annotations

from datetime import date

from app.domain.blocos import Bloco
from app.prompts import edital as prompts
from app.providers.base import LLMProvider, StructuredRequest
from app.schemas.api import (
    ConteudoDisciplinaOut,
    ConteudoResponse,
    EstruturaResponse,
)
from app.schemas.document import NormalizedDocument
from app.schemas.evidence import Evidence, ExtractionMethod
from app.schemas.extraction import (
    Cargo,
    DisciplinaExtraida,
    DiscursiveKind,
    DuracaoProva,
    DurationScope,
    GroupKind,
    GrupoConhecimento,
    MarcoCronograma,
    ProvaDiscursiva,
    WeightScope,
)
from app.services.chunking import build_chunks, chunks_for_blocks, render_for_prompt
from app.services.normalize import snippet_matches_page
from app.services.rawjson import (
    as_dict,
    as_dict_list,
    get_float,
    get_int,
    get_str,
    get_str_list,
)


def _page_source(doc: NormalizedDocument, page: int) -> str:
    for candidate in doc.pages:
        if candidate.physical_page == page:
            return candidate.source
    return "none"


def _method_for(source: str) -> ExtractionMethod:
    if source == "ocr":
        return ExtractionMethod.OCR_LLM
    if source == "native_text":
        return ExtractionMethod.NATIVE_TEXT_LLM
    return ExtractionMethod.LLM


def _evidence(doc: NormalizedDocument, field: str, raw_ev: object) -> list[Evidence]:
    out: list[Evidence] = []
    page_text = {p.physical_page: p.text for p in doc.pages}
    for entry in as_dict_list(raw_ev):
        page = get_int(entry, "physicalPage") or 0
        snippet = (get_str(entry, "snippet") or "")[:400]
        if page < 1 or page > doc.total_pages or not snippet:
            continue
        source = _page_source(doc, page)
        valid = snippet_matches_page(snippet, page_text.get(page, ""), ocr=(source == "ocr"))
        out.append(
            Evidence(
                field=field,
                physical_page=page,
                snippet=snippet,
                method=_method_for(source),
                confidence=0.75 if valid else None,
            )
        )
    return out


# --- step 1: banca + cargos --------------------------------------------------


async def extract_cargos(
    doc: NormalizedDocument, provider: LLMProvider
) -> tuple[str | None, list[Cargo]]:
    chunks = chunks_for_blocks(build_chunks(doc), {Bloco.DADOS_GERAIS, Bloco.CARGOS_VAGAS})
    if not chunks:
        return None, []

    raw = await provider.extract_structured(
        StructuredRequest(
            system=prompts.CARGOS_INSTRUCTION,
            chunks=render_for_prompt(chunks),
            response_schema=prompts.CARGOS_SCHEMA,
        )
    )

    cargos: list[Cargo] = []
    for i, item in enumerate(as_dict_list(raw.get("cargos"))):
        codigo = get_str(item, "codigo")
        if not codigo:
            continue
        cargos.append(
            Cargo(
                codigo=codigo,
                nome=get_str(item, "nome") or "",
                especialidade=get_str(item, "especialidade"),
                escolaridade=get_str(item, "escolaridade"),
                total_vagas=get_int(item, "totalVagas"),
                evidence=_evidence(doc, f"/cargos/{i}", item.get("evidence")),
            )
        )
    return get_str(raw, "banca"), cargos


# --- step 2: exam structure ------------------------------------------------


async def extract_estrutura(
    doc: NormalizedDocument, cargo: str, provider: LLMProvider
) -> EstruturaResponse:
    # The structure table gives the group totals; the discipline NAMES live only in
    # the syllabus (Anexo II), so that block has to come along or there is nothing
    # to name the disciplines from.
    chunks = chunks_for_blocks(
        build_chunks(doc),
        {
            Bloco.ESTRUTURA_PROVAS,
            Bloco.CARGOS_VAGAS,
            Bloco.CONTEUDO_PROGRAMATICO,
            Bloco.CRONOGRAMA,
        },
    )
    if not chunks:
        return EstruturaResponse()

    raw = await provider.extract_structured(
        StructuredRequest(
            system=f"{prompts.ESTRUTURA_INSTRUCTION}\n\nCARGO ESCOLHIDO: {cargo}",
            chunks=render_for_prompt(chunks),
            response_schema=prompts.ESTRUTURA_SCHEMA,
        )
    )

    return EstruturaResponse(
        nome_sugerido=get_str(raw, "nomeSugerido"),
        data_prova=get_str(raw, "dataProva"),
        grupos_gerais=_groups(doc, raw.get("gruposGerais"), "gruposGerais"),
        grupos_especificos=_groups(doc, raw.get("gruposEspecificos"), "gruposEspecificos"),
        prova_discursiva=_discursivas(doc, raw.get("provaDiscursiva")),
        duracao=_duracao(doc, raw.get("duracao")),
        cronograma=_cronograma(doc, raw.get("cronograma")),
    )


def _groups(doc: NormalizedDocument, raw_groups: object, scope: str) -> list[GrupoConhecimento]:
    out: list[GrupoConhecimento] = []
    for i, group in enumerate(as_dict_list(raw_groups)):
        rotulo = get_str(group, "rotulo")
        if not rotulo:
            continue
        kind = get_str(group, "kind") or "outro"
        peso_scope = get_str(group, "pesoScope")
        out.append(
            GrupoConhecimento(
                kind=GroupKind(kind) if kind in {"ger", "esp", "outro"} else GroupKind.OUTRO,
                rotulo=rotulo,
                total_questoes=get_int(group, "totalQuestoes"),
                peso=get_float(group, "peso"),
                peso_scope=(
                    WeightScope(peso_scope) if peso_scope in {"group", "discipline"} else None
                ),
                disciplinas=_disciplinas(group.get("disciplinas")),
                evidence=_evidence(doc, f"/{scope}/{i}", group.get("evidence")),
            )
        )
    return out


def _disciplinas(raw: object) -> list[DisciplinaExtraida]:
    out: list[DisciplinaExtraida] = []
    for disc in as_dict_list(raw):
        nome = get_str(disc, "nome")
        if not nome:
            continue
        out.append(
            DisciplinaExtraida(
                nome=nome,
                numero_questoes=get_int(disc, "numeroQuestoes"),
                peso=get_float(disc, "peso"),
            )
        )
    return out


def _discursivas(doc: NormalizedDocument, raw: object) -> list[ProvaDiscursiva]:
    out: list[ProvaDiscursiva] = []
    for i, item in enumerate(as_dict_list(raw)):
        mod = get_str(item, "modalidade")
        if not mod:
            continue
        out.append(
            ProvaDiscursiva(
                modalidade=(
                    DiscursiveKind(mod)
                    if mod in {"redacao", "estudo_de_caso", "outro"}
                    else DiscursiveKind.OUTRO
                ),
                rotulo=get_str(item, "rotulo") or "",
                questoes=get_int(item, "questoes"),
                evidence=_evidence(doc, f"/provaDiscursiva/{i}", item.get("evidence")),
            )
        )
    return out


def _iso_date(value: str | None) -> str | None:
    """Keep only a well-formed YYYY-MM-DD. The schema enforces the shape, so a
    malformed date is dropped rather than allowed to fail the whole response."""
    if not value:
        return None
    try:
        date.fromisoformat(value)
    except ValueError:
        return None
    return value


def _cronograma(doc: NormalizedDocument, raw: object) -> list[MarcoCronograma]:
    out: list[MarcoCronograma] = []
    for i, item in enumerate(as_dict_list(raw)):
        titulo = get_str(item, "titulo")
        inicio = _iso_date(get_str(item, "dataInicio"))
        if not titulo or not inicio:
            continue
        fim = _iso_date(get_str(item, "dataFim"))
        # An end before the start is the model mis-reading a range; drop the end
        # rather than persist a backwards interval.
        if fim is not None and fim < inicio:
            fim = None
        out.append(
            MarcoCronograma(
                data_inicio=inicio,
                data_fim=fim,
                titulo=titulo,
                exige_acao=bool(item.get("exigeAcao")),
                evidence=_evidence(doc, f"/cronograma/{i}", item.get("evidence")),
            )
        )
    return out


def _duracao(doc: NormalizedDocument, raw: object) -> DuracaoProva | None:
    data = as_dict(raw)
    minutos = get_int(data, "minutos")
    if not minutos:
        return None
    scope = get_str(data, "scope") or "unknown"
    return DuracaoProva(
        minutos=minutos,
        scope=(
            DurationScope(scope)
            if scope in {"exam_set", "single_prova", "unknown"}
            else DurationScope.UNKNOWN
        ),
        evidence=_evidence(doc, "/duracao", data.get("evidence")),
    )


# --- step 3: syllabus topics ----------------------------------------------


async def extract_conteudo(
    doc: NormalizedDocument, cargo: str, disciplinas: list[str], provider: LLMProvider
) -> ConteudoResponse:
    chunks = chunks_for_blocks(build_chunks(doc), {Bloco.CONTEUDO_PROGRAMATICO})
    if not chunks:
        return ConteudoResponse(itens=[ConteudoDisciplinaOut(disciplina=d) for d in disciplinas])

    disc_list = "\n".join(f"- {d}" for d in disciplinas)
    raw = await provider.extract_structured(
        StructuredRequest(
            system=f"{prompts.CONTEUDO_INSTRUCTION}\n\nCARGO: {cargo}\nDISCIPLINAS:\n{disc_list}",
            chunks=render_for_prompt(chunks),
            response_schema=prompts.CONTEUDO_SCHEMA,
        )
    )

    by_name = {d.strip().lower(): d for d in disciplinas}
    seen: set[str] = set()
    itens: list[ConteudoDisciplinaOut] = []
    for i, item in enumerate(as_dict_list(raw.get("itens"))):
        key = (get_str(item, "disciplina") or "").lower()
        canonical = by_name.get(key)
        if canonical is None or canonical in seen:
            continue
        seen.add(canonical)
        itens.append(
            ConteudoDisciplinaOut(
                disciplina=canonical,
                itens=get_str_list(item, "topicos"),
                evidence=_evidence(doc, f"/itens/{i}", item.get("evidence")),
            )
        )
    for missing in disciplinas:
        if missing not in seen:
            itens.append(ConteudoDisciplinaOut(disciplina=missing))
    return ConteudoResponse(itens=itens)
