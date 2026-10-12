"""Internal HTTP routes.

Public shapes stay in the Go backend; these are what Go calls internally. The
browser never sees text, a provider URI, or anything but the opaque
``documentId``.
"""

from __future__ import annotations

from fastapi import APIRouter, Depends, FastAPI, Request, UploadFile
from fastapi.responses import JSONResponse

from app.adapter.httpapi import dependencias
from app.adapter.httpapi.dto import AnalisarResponse, ConteudoRequest, EstruturaRequest
from app.adapter.httpapi.seguranca import owner_ref, request_id, require_service_token
from app.domain.edital.extraction import Conteudo, Estrutura
from app.platform.config import Settings
from app.platform.errors import InvalidPDF, ProcessorError, UploadTooLarge
from app.service.edital.editais import Analise, Editais

router = APIRouter()


@router.get("/healthz")
async def healthz(editais: Editais = Depends(dependencias.editais)) -> dict[str, object]:
    return {"status": "ok", "gemini": editais.llm_disponivel()}


@router.post(
    "/internal/editais/analisar",
    response_model=AnalisarResponse,
    dependencies=[Depends(require_service_token)],
)
async def analisar(
    request: Request,
    file: UploadFile | None = None,
    settings: Settings = Depends(dependencias.settings),
    editais: Editais = Depends(dependencias.editais),
) -> AnalisarResponse:
    analise = await _analisar(request, file, settings, editais)
    return AnalisarResponse(
        document_id=analise.document.document_id,
        filename=analise.document.filename,
        sha256=analise.document.sha256,
        total_pages=analise.document.total_pages,
        ocr_pages=analise.ocr_pages,
        banca=analise.banca,
        cargos=analise.cargos,
        alerts=[],
    )


async def _analisar(
    request: Request, file: UploadFile | None, settings: Settings, editais: Editais
) -> Analise:
    """Dispatch on how the edital arrived: a file, or pasted text in a JSON body
    or a multipart 'texto' field."""
    ref = owner_ref(request)
    rid = request_id(request)
    if file is not None:
        data = await file.read(settings.max_upload_bytes + 1)
        if len(data) > settings.max_upload_bytes:
            raise UploadTooLarge("upload exceeds the configured limit")
        return await editais.analisar_pdf(
            data, file.content_type, file.filename or "edital.pdf", ref, rid
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
    return await editais.analisar_texto(texto, ref, rid)


@router.post(
    "/internal/editais/estrutura",
    response_model=Estrutura,
    dependencies=[Depends(require_service_token)],
)
async def estrutura(
    request: Request,
    body: EstruturaRequest,
    editais: Editais = Depends(dependencias.editais),
) -> Estrutura:
    return await editais.estrutura(body.document_id, owner_ref(request), body.cargo)


@router.post(
    "/internal/editais/conteudo",
    response_model=Conteudo,
    dependencies=[Depends(require_service_token)],
)
async def conteudo(
    request: Request,
    body: ConteudoRequest,
    editais: Editais = Depends(dependencias.editais),
) -> Conteudo:
    return await editais.conteudo(
        body.document_id, owner_ref(request), body.cargo, body.disciplinas
    )


def install_error_handler(app: FastAPI) -> None:
    @app.exception_handler(ProcessorError)
    async def _handle(request: Request, exc: ProcessorError) -> JSONResponse:
        rid = request.headers.get("x-request-id")
        return JSONResponse(status_code=exc.http_status, content=exc.as_payload(rid))
