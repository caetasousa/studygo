"""Internal HTTP contract between the Go backend and this service.

The public ``/api/editais/*`` shapes stay in Go; these are what Go sends and
receives internally. The browser never sees text, a provider URI, or anything
but the opaque ``documentId``.
"""

from __future__ import annotations

from pydantic import Field

from app.domain.edital.alerts import Alert
from app.domain.edital.base import WireModel
from app.domain.edital.extraction import Cargo


class AnalisarResponse(WireModel):
    """Step 1 result: the document handle plus the cheap top-level facts."""

    document_id: str
    filename: str
    sha256: str
    total_pages: int
    ocr_pages: int
    banca: str | None = None
    cargos: list[Cargo] = Field(default_factory=list)
    alerts: list[Alert] = Field(default_factory=list)


class EstruturaRequest(WireModel):
    document_id: str
    # The stable cargo code from step 1 when the client has it; the display name
    # otherwise.
    cargo: str


class ConteudoRequest(WireModel):
    document_id: str
    cargo: str
    disciplinas: list[str] = Field(min_length=1)
