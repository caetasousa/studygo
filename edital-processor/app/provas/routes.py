"""Rotas internas de provas.

Sem estado de fila e sem banco: o backend decide o que processar e quando, e
manda só identificadores de arquivos do volume compartilhado — nunca caminhos.
"""

from __future__ import annotations

from fastapi import APIRouter, Depends
from fastapi.concurrency import run_in_threadpool
from pydantic import BaseModel

from app.api.routes import get_provider
from app.core.config import Settings, get_settings
from app.core.security import require_service_token
from app.provas import pipeline
from app.provas.documentos import recortar, regioes
from app.provas.schemas import Origem, QuestaoParaClassificar
from app.providers.base import LLMProvider

router = APIRouter(prefix="/internal/provas", dependencies=[Depends(require_service_token)])


class PedidoDocumento(BaseModel):
    documento: str


class PedidoGabarito(PedidoDocumento):
    # O caderno da prova: a relação da FCC traz todos os tipos num arquivo.
    caderno: str = ""
    # O cargo da prova e, com ele, a folha de alterações de gabarito: ela traz
    # vários cargos, e só as questões que mudaram.
    cargo: str = ""
    alteracoes: bool = False


class PedidoRegiao(BaseModel):
    documento: str
    origem: Origem


class PedidoLeitura(PedidoRegiao):
    # A releitura de uma questão diz qual: o processador a acha pelo número.
    questao: int = 0
    # O trecho marcado em volta de um texto de apoio: lê só o texto.
    apoio: bool = False


@router.post("/preparar")
async def preparar(body: PedidoDocumento, settings: Settings = Depends(get_settings)) -> object:
    result = await run_in_threadpool(regioes, settings.provas_dir, body.documento, settings)
    return [o.model_dump(by_alias=True) for o in result]


@router.post("/metadados")
async def metadados(
    body: PedidoRegiao,
    settings: Settings = Depends(get_settings),
    provider: LLMProvider = Depends(get_provider),
) -> object:
    result = await pipeline.metadados(
        settings.provas_dir, body.documento, body.origem, provider, settings
    )
    return result.model_dump(by_alias=True)


@router.post("/extrair")
async def extrair(
    body: PedidoLeitura,
    settings: Settings = Depends(get_settings),
    provider: LLMProvider = Depends(get_provider),
) -> object:
    if body.apoio:
        lido = await pipeline.texto_de_apoio(
            settings.provas_dir, body.documento, body.origem, provider, settings
        )
        return lido.model_dump(by_alias=True)
    result = await pipeline.extrair(
        settings.provas_dir, body.documento, body.origem, provider, settings, questao=body.questao
    )
    return result.model_dump(by_alias=True)


class PedidoClassificacao(BaseModel):
    questoes: list[QuestaoParaClassificar]


@router.post("/classificar")
async def classificar(
    body: PedidoClassificacao,
    settings: Settings = Depends(get_settings),
    provider: LLMProvider = Depends(get_provider),
) -> object:
    result = await pipeline.classificar(body.questoes, provider, settings)
    return result.model_dump(by_alias=True)


@router.post("/recortar")
async def recortar_rota(
    body: PedidoRegiao, settings: Settings = Depends(get_settings)
) -> dict[str, str]:
    asset = await run_in_threadpool(
        recortar, settings.provas_dir, body.documento, body.origem, settings
    )
    return {"arquivo": asset}


@router.post("/gabarito")
async def gabarito(
    body: PedidoGabarito,
    settings: Settings = Depends(get_settings),
    provider: LLMProvider = Depends(get_provider),
) -> object:
    if body.alteracoes:
        alteradas = await run_in_threadpool(
            pipeline.ler_alteracoes, settings.provas_dir, body.documento, body.cargo, body.caderno
        )
        return alteradas.model_dump(by_alias=True)
    result = await pipeline.gabarito(
        settings.provas_dir, body.documento, body.caderno, provider, settings
    )
    return result.model_dump(by_alias=True)
