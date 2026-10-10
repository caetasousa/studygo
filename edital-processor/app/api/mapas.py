"""Rotas internas dos mapas mentais e da conexão do Claude de cada conta.

Quem chama é o backend, nunca o navegador.
"""

from __future__ import annotations

import json
from pathlib import Path

from fastapi import APIRouter, Depends, Form, Request, Response, UploadFile
from pydantic import BaseModel, Field, ValidationError, field_validator

from app.core.config import Settings, get_settings
from app.core.errors import InvalidPDF, UploadTooLarge
from app.core.security import owner_ref, require_service_token
from app.mapas.backend import Backend
from app.mapas.conexao import Conexoes, Situacao
from app.mapas.servico import Mapas, Trabalho, rodar_claude_de_verdade

router = APIRouter(prefix="/internal/mapas", dependencies=[Depends(require_service_token)])

router_claude = APIRouter(prefix="/internal/claude", dependencies=[Depends(require_service_token)])

_mapas: Mapas | None = None
_conexoes: Conexoes | None = None


def get_conexoes(settings: Settings = Depends(get_settings)) -> Conexoes:
    global _conexoes
    if _conexoes is None:
        _conexoes = Conexoes(settings)
    return _conexoes


def get_mapas(
    settings: Settings = Depends(get_settings), conexoes: Conexoes = Depends(get_conexoes)
) -> Mapas:
    global _mapas
    if _mapas is None:
        backend = Backend(settings)

        async def rodar(
            args: list[str], mensagem: str, pasta: Path, ambiente: dict[str, str]
        ) -> tuple[int, str]:
            return await rodar_claude_de_verdade(
                args,
                mensagem,
                pasta,
                ambiente,
                settings.mapas_timeout_seconds,
            )

        _mapas = Mapas(settings, backend.entregar, backend.avisar_falha, rodar, conexoes)
    return _mapas


class DadosDoTrabalho(BaseModel):
    pedido: str = Field(min_length=1, max_length=64)
    arquivo: str = Field(default="aula.pdf", max_length=200)
    materia: str = Field(default="", max_length=300)
    temas: list[str] = Field(default_factory=list, max_length=500)
    slugs_existentes: list[str] = Field(
        default_factory=list, alias="slugsExistentes", max_length=5000
    )
    token_claude: str = Field(default="", alias="tokenClaude", max_length=2000)

    @field_validator("temas", "slugs_existentes", mode="before")
    @classmethod
    def _nulo_e_vazio(cls, v: object) -> object:
        # A lista vazia do Go chega como null.
        return [] if v is None else v


@router.post("/processamentos", status_code=202)
async def processar(
    request: Request,
    dados: str = Form(...),
    pdf: UploadFile | None = None,
    settings: Settings = Depends(get_settings),
    mapas: Mapas = Depends(get_mapas),
) -> dict[str, str]:
    dono = owner_ref(request)
    try:
        d = DadosDoTrabalho.model_validate(json.loads(dados))
    except (ValueError, ValidationError) as exc:
        raise InvalidPDF(f"dados do pedido inválidos: {exc}") from exc
    if pdf is None:
        raise InvalidPDF("faltou o PDF da aula")
    conteudo = await pdf.read(settings.mapas_max_pdf_bytes + 1)
    if len(conteudo) > settings.mapas_max_pdf_bytes:
        raise UploadTooLarge("o PDF passa do limite dos mapas")
    if not conteudo.startswith(b"%PDF-"):
        raise InvalidPDF("o arquivo não é um PDF")

    mapas.iniciar(
        Trabalho(
            pedido=d.pedido,
            dono=dono,
            arquivo=d.arquivo,
            pdf=conteudo,
            materia=d.materia,
            temas=tuple(d.temas),
            slugs_existentes=tuple(d.slugs_existentes),
            token_claude=d.token_claude,
        )
    )
    return {"pedido": d.pedido}


async def parar_mapas() -> None:
    """No desligamento: interrompe o Claude em andamento (o redespacho o retoma)."""
    if _mapas is not None:
        await _mapas.parar()
    if _conexoes is not None:
        _conexoes.encerrar_tudo()


# --- a conexão do Claude da conta ----------------------------------------------


class CodigoDoClaude(BaseModel):
    codigo: str = Field(min_length=1, max_length=500)


def _situacao(s: Situacao) -> dict[str, object]:
    return {"conectado": s.conectado, "email": s.email, "plano": s.plano}


@router_claude.get("")
async def situacao_do_claude(
    request: Request, conexoes: Conexoes = Depends(get_conexoes)
) -> dict[str, object]:
    return _situacao(await conexoes.situacao(owner_ref(request)))


@router_claude.post("/conexao")
async def conectar_o_claude(
    request: Request, conexoes: Conexoes = Depends(get_conexoes)
) -> dict[str, str]:
    return {"url": await conexoes.iniciar(owner_ref(request))}


@router_claude.post("/conexao/codigo")
async def concluir_a_conexao(
    corpo: CodigoDoClaude, request: Request, conexoes: Conexoes = Depends(get_conexoes)
) -> dict[str, object]:
    return _situacao(await conexoes.concluir(owner_ref(request), corpo.codigo.strip()))


@router_claude.delete("", status_code=204)
async def desconectar_o_claude(
    request: Request, conexoes: Conexoes = Depends(get_conexoes)
) -> Response:
    await conexoes.desconectar(owner_ref(request))
    return Response(status_code=204)
