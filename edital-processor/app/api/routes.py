"""Internal HTTP routes.

Public shapes stay in the Go backend; these are what Go calls internally. The
browser never sees text, a provider URI, or anything but the opaque
``documentId``.
"""

from __future__ import annotations

from fastapi import APIRouter, Depends, Request, UploadFile
from fastapi.responses import JSONResponse

from app.core.config import Settings, get_settings
from app.core.errors import InvalidPDF, ProcessorError, UploadTooLarge
from app.core.security import owner_ref, request_id, require_service_token
from app.providers.base import LLMProvider, NullProvider
from app.schemas.api import (
    AnalisarResponse,
    ConteudoRequest,
    ConteudoResponse,
    EstruturaRequest,
    EstruturaResponse,
)
from app.services.artifacts import ArtifactStore
from app.services.extract_llm import extract_cargos, extract_conteudo, extract_estrutura
from app.services.pipeline import AnalyseOutcome, analyse, analyse_text
from app.services.validation_rules import validate as run_rules

router = APIRouter()

_store: ArtifactStore | None = None
_provider: LLMProvider | None = None


def get_store(settings: Settings = Depends(get_settings)) -> ArtifactStore:
    global _store
    if _store is None:
        _store = ArtifactStore(settings)
    return _store


def get_provider(settings: Settings = Depends(get_settings)) -> LLMProvider:
    global _provider
    if _provider is None:
        if settings.gemini_enabled:
            from app.providers.gemini import GeminiProvider

            _provider = GeminiProvider(settings)
        else:
            _provider = NullProvider()
    return _provider


@router.get("/healthz")
async def healthz(
    settings: Settings = Depends(get_settings),
    provider: LLMProvider = Depends(get_provider),
) -> dict[str, object]:
    return {"status": "ok", "gemini": provider.available()}


@router.post(
    "/internal/editais/analisar",
    response_model=AnalisarResponse,
    dependencies=[Depends(require_service_token)],
)
async def analisar(
    request: Request,
    file: UploadFile | None = None,
    settings: Settings = Depends(get_settings),
    store: ArtifactStore = Depends(get_store),
    provider: LLMProvider = Depends(get_provider),
) -> AnalisarResponse:
    ref = owner_ref(request)
    rid = request_id(request)

    outcome = await _analyse_request(request, file, ref, settings, store, rid)

    from app.schemas.extraction import Cargo

    banca: str | None = None
    cargos: list[Cargo] = []
    if provider.available():
        banca, cargos = await extract_cargos(outcome.document, provider)

    return AnalisarResponse(
        document_id=outcome.document.document_id,
        filename=outcome.document.filename,
        sha256=outcome.document.sha256,
        total_pages=outcome.document.total_pages,
        ocr_pages=len(outcome.ocr_page_numbers) if outcome.ocr_ran else 0,
        banca=banca,
        cargos=cargos,
        alerts=[],
    )


async def _analyse_request(
    request: Request,
    file: UploadFile | None,
    ref: str,
    settings: Settings,
    store: ArtifactStore,
    rid: str | None,
) -> AnalyseOutcome:
    """Dispatch on how the edital arrived: a file, or pasted text in a JSON body
    or a multipart 'texto' field."""
    if file is not None:
        data = await file.read(settings.max_upload_bytes + 1)
        if len(data) > settings.max_upload_bytes:
            raise UploadTooLarge("upload exceeds the configured limit")
        return analyse(
            data=data,
            declared_mime=file.content_type,
            filename=file.filename or "edital.pdf",
            owner_ref=ref,
            settings=settings,
            store=store,
            request_id=rid,
        )

    texto = ""
    ctype = request.headers.get("content-type", "")
    if ctype.startswith("application/json"):
        body = await request.json()
        texto = str(body.get("texto", "")).strip()
    elif ctype.startswith("multipart/form-data"):
        form = await request.form()
        texto = str(form.get("texto", "")).strip()

    if not texto:
        raise InvalidPDF("no file and no text in the request")

    return analyse_text(
        text=texto,
        filename="edital-colado.txt",
        owner_ref=ref,
        settings=settings,
        store=store,
        request_id=rid,
    )


@router.post(
    "/internal/editais/estrutura",
    response_model=EstruturaResponse,
    dependencies=[Depends(require_service_token)],
)
async def estrutura(
    request: Request,
    body: EstruturaRequest,
    store: ArtifactStore = Depends(get_store),
    provider: LLMProvider = Depends(get_provider),
) -> EstruturaResponse:
    doc = store.load(body.document_id, owner_ref(request))
    result = await extract_estrutura(doc, body.cargo, provider)

    # Deterministic checks run over an ExtractionResult; assemble a partial one.
    from app.schemas.extraction import DocumentInfo, ExtractionResult

    partial = ExtractionResult(
        document=DocumentInfo(
            filename=doc.filename, sha256=doc.sha256, total_pages=doc.total_pages
        ),
        data_prova=result.data_prova,
        grupos_gerais=result.grupos_gerais,
        grupos_especificos=result.grupos_especificos,
        prova_discursiva=result.prova_discursiva,
        duracao=result.duracao,
    )
    result.alerts = run_rules(partial)
    return result


@router.post(
    "/internal/editais/conteudo",
    response_model=ConteudoResponse,
    dependencies=[Depends(require_service_token)],
)
async def conteudo(
    request: Request,
    body: ConteudoRequest,
    store: ArtifactStore = Depends(get_store),
    provider: LLMProvider = Depends(get_provider),
) -> ConteudoResponse:
    doc = store.load(body.document_id, owner_ref(request))
    return await extract_conteudo(doc, body.cargo, body.disciplinas, provider)


def install_error_handler(app: object) -> None:
    from fastapi import FastAPI

    assert isinstance(app, FastAPI)

    @app.exception_handler(ProcessorError)
    async def _handle(request: Request, exc: ProcessorError) -> JSONResponse:
        rid = request.headers.get("x-request-id")
        return JSONResponse(status_code=exc.http_status, content=exc.as_payload(rid))
