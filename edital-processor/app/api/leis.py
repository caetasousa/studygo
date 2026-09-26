"""Rotas internas da captura de leis. Quem chama é o backend, nunca o navegador."""

from __future__ import annotations

from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel, Field

from app.api.routes import get_provider
from app.core.security import owner_ref, require_service_token
from app.leis.fontes import http_padrao
from app.leis.servico import Capturas
from app.providers.base import LLMProvider

router = APIRouter(prefix="/internal/leis", dependencies=[Depends(require_service_token)])

_capturas: Capturas | None = None


def get_capturas(provider: LLMProvider = Depends(get_provider)) -> Capturas:
    global _capturas
    if _capturas is None:
        _capturas = Capturas(http=http_padrao, provider=provider if provider.available() else None)
    return _capturas


class CapturaRequest(BaseModel):
    link: str = Field(max_length=2048)
    # As raízes do que guardar ("tit3.cap7", "art37"); sem recorte, a lei inteira.
    recorte: list[str] | None = Field(default=None, max_length=500)


class PesquisaRequest(BaseModel):
    tema: str = Field(max_length=2000)
    link: str | None = Field(default=None, max_length=2048)


@router.post("/capturas", status_code=202)
async def iniciar(
    request: Request, body: CapturaRequest, capturas: Capturas = Depends(get_capturas)
) -> dict[str, str]:
    return {"id": capturas.iniciar(body.link, owner_ref(request), body.recorte)}


@router.post("/pesquisas")
async def pesquisar(
    request: Request, body: PesquisaRequest, capturas: Capturas = Depends(get_capturas)
) -> dict[str, object]:
    owner_ref(request)
    return await capturas.pesquisar(body.tema, body.link)


@router.get("/capturas/{cid}")
async def consultar(
    request: Request, cid: str, capturas: Capturas = Depends(get_capturas)
) -> dict[str, object]:
    return capturas.consultar(cid, owner_ref(request))
