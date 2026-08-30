"""Page block classification (spec §7.13).

Two passes:

1. Per-page keyword signals — headings and stock phrases, kept tight so legal
   boilerplate does not match a content block.
2. A section sweep — the syllabus (Anexo II) and the schedule (Anexo III) run
   across several pages under one heading, so once a section starts its block
   carries forward until another section heading appears.

Only used to decide which pages the LLM step sees. Never a factual claim.
"""

from __future__ import annotations

import re

from app.domain.blocos import Bloco

# Per-page signals. A page matches a block when any pattern hits.
_SIGNALS: dict[Bloco, tuple[re.Pattern[str], ...]] = {
    Bloco.DADOS_GERAIS: (
        re.compile(r"\bedital\s+n[º°o]?\s*0?1/2026\b", re.I),
        re.compile(r"\babertura\s+de\s+inscri[çc][õo]es\b", re.I),
        re.compile(r"\binstru[çc][õo]es\s+especiais\b", re.I),
        re.compile(r"\bdisposi[çc][õo]es\s+preliminares\b", re.I),
    ),
    Bloco.CARGOS_VAGAS: (
        re.compile(r"\bcargo\s*/?\s*especialidade\b", re.I),
        re.compile(r"\bc[óo]digo\s+de\s+op[çc][ãa]o\b", re.I),
        re.compile(r"\bn[º°o]?\s*total\s+de\s+vagas\b", re.I),
        re.compile(r"\bescolaridade\s*/?\s*pr[ée]-?requisito\b", re.I),
        re.compile(r"\bvencimento\s+inicial\b", re.I),
        re.compile(r"\bt[ée]cnico\s+de\s+controle\s+externo\b.{0,60}\bvagas\b", re.I),
    ),
    Bloco.ESTRUTURA_PROVAS: (
        re.compile(r"(^|\n)\s*\d+\.?\s+das\s+provas\b", re.I),
        re.compile(r"\bprovas?\s+objetivas?\s*:", re.I),
        re.compile(r"\bconhecimentos\s+(gerais|espec[íi]ficos)\b.{0,40}\bquest[õo]es\b", re.I),
        re.compile(r"\bprova\s+discursiva\s*[-]\s*(reda[çc][ãa]o|estudo\s+de\s+caso)\b", re.I),
        re.compile(r"\bcar[áa]ter\s+e\s+dura[çc][ãa]o\s+das?\s+provas?\b", re.I),
        re.compile(r"\bn[úu]mero\s+de\s+quest[õo]es\b.{0,30}\bpeso\b", re.I),
    ),
    Bloco.CRONOGRAMA: (
        re.compile(r"\bcronograma\s+das\s+provas\s+e\s+publica[çc][õo]es\b", re.I),
        re.compile(r"\bdatas?\s+previstas?\b", re.I),
    ),
}

# Section starts. Matched only against the first few lines of a page, and only
# as a standalone heading — "ANEXO II" on its own line, not "Anexo II deste
# Edital" buried in a sentence. These begin a run that carries forward.
_HEAD_LINES = 4
_SECTION_START: dict[Bloco, tuple[re.Pattern[str], ...]] = {
    Bloco.CONTEUDO_PROGRAMATICO: (
        re.compile(r"^\s*anexo\s+i{2}\s*$", re.I | re.M),
        re.compile(r"^\s*conte[úu]do\s+program[áa]tico\s*$", re.I | re.M),
    ),
    Bloco.CRONOGRAMA: (
        re.compile(r"^\s*anexo\s+i{3}\s*$", re.I | re.M),
        re.compile(r"^\s*cronograma\s+das\s+provas\s+e\s+publica[çc][õo]es\s*$", re.I | re.M),
    ),
}

# Section ends. A standalone anexo heading of any number, or the signature
# block, closes an open run.
_SECTION_END = (
    re.compile(r"^\s*anexo\s+[iv]{1,4}\s*$", re.I | re.M),
    re.compile(r"\bgoi[âa]nia,?\s+\d{1,2}\s+de\s+\w+\s+de\s+20\d\d\b", re.I),
    re.compile(r"\bpresidente\s+da\s+comiss[ãa]o\s+do\s+concurso\b", re.I),
)


def _head(text: str) -> str:
    return "\n".join(text.splitlines()[:_HEAD_LINES])


def classify_page(text: str) -> list[Bloco]:
    """Per-page signals only. Use ``classify_document`` for the full result."""
    if not text.strip():
        return [Bloco.IRRELEVANTE]
    hits = [b for b, ps in _SIGNALS.items() if any(p.search(text) for p in ps)]
    return hits or [Bloco.IRRELEVANTE]


def classify_document(page_texts: list[str]) -> list[list[Bloco]]:
    """Classify every page, applying the section sweep."""
    result: list[set[Bloco]] = []
    open_section: Bloco | None = None

    for text in page_texts:
        stripped = text.strip()
        if not stripped:
            result.append(set())
            open_section = None
            continue

        # A standalone anexo heading near the top of the page starts a run; any
        # other anexo heading (or the signature block) closes one.
        head = _head(stripped)
        starts_here: Bloco | None = None
        for block, patterns in _SECTION_START.items():
            if any(p.search(head) for p in patterns):
                starts_here = block
                break
        if starts_here is not None:
            open_section = starts_here
        elif any(p.search(stripped) for p in _SECTION_END):
            open_section = None

        page_blocks = {b for b, ps in _SIGNALS.items() if any(p.search(stripped) for p in ps)}
        if open_section is not None:
            page_blocks.add(open_section)

        result.append(page_blocks)

    return [sorted(bs) if bs else [Bloco.IRRELEVANTE] for bs in result]
