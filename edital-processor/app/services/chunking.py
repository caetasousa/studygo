"""Chunk selection (spec §7.14-7.15).

Turns the normalized pages into the delimited units the LLM step consumes.
Only pages carrying a relevant block are included. Each chunk has a stable id
(``p{page}#{ordinal}``) and records its physical page and source method, so
every fact the model returns can be traced back.
"""

from __future__ import annotations

from dataclasses import dataclass

from app.domain.blocos import RELEVANTES, Bloco
from app.schemas.document import NormalizedDocument

# A chunk is capped so a single page cannot dominate the prompt; long pages are
# split on paragraph boundaries.
_MAX_CHUNK_CHARS = 2500


@dataclass(frozen=True)
class Chunk:
    id: str
    physical_page: int
    printed_page: str | None
    source: str  # "native_text" | "ocr"
    blocks: tuple[Bloco, ...]
    text: str


def _split_page(text: str, limit: int) -> list[str]:
    if len(text) <= limit:
        return [text]
    parts: list[str] = []
    current: list[str] = []
    size = 0
    for para in text.split("\n\n"):
        piece = para.strip()
        if not piece:
            continue
        if size + len(piece) > limit and current:
            parts.append("\n\n".join(current))
            current, size = [], 0
        current.append(piece)
        size += len(piece) + 2
    if current:
        parts.append("\n\n".join(current))
    return parts


def build_chunks(doc: NormalizedDocument) -> list[Chunk]:
    chunks: list[Chunk] = []
    for page in doc.pages:
        relevant = tuple(b for b in page.blocks if b in RELEVANTES)
        if not relevant or not page.text.strip():
            continue
        for ordinal, piece in enumerate(_split_page(page.text, _MAX_CHUNK_CHARS)):
            chunks.append(
                Chunk(
                    id=f"p{page.physical_page}#{ordinal}",
                    physical_page=page.physical_page,
                    printed_page=page.printed_page,
                    source=page.source,
                    blocks=relevant,
                    text=piece,
                )
            )
    return chunks


def chunks_for_blocks(chunks: list[Chunk], wanted: set[Bloco]) -> list[Chunk]:
    """Subset carrying at least one of ``wanted`` — lets each wizard step send
    only the pages it needs (cargos vs. structure vs. syllabus)."""
    return [c for c in chunks if any(b in wanted for b in c.blocks)]


def render_for_prompt(chunks: list[Chunk]) -> list[str]:
    """One delimited string per chunk. The delimiter and the id let the model
    cite a source; the wrapper text tells it this is data, not instructions."""
    rendered: list[str] = []
    for chunk in chunks:
        header = (
            f"<<<CHUNK id={chunk.id} pagina_fisica={chunk.physical_page} fonte={chunk.source}>>>"
        )
        rendered.append(f"{header}\n{chunk.text}\n<<<FIM CHUNK {chunk.id}>>>")
    return rendered
