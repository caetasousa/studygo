"""FastAPI application entrypoint."""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.concurrency import run_in_threadpool

from app.api.leis import router as router_leis
from app.api.mapas import parar_mapas, router_claude
from app.api.mapas import router as router_mapas
from app.api.routes import install_error_handler, router, store_em_uso
from app.core.config import get_settings
from app.core.logging import configure_logging, get_logger
from app.mapas.backend import Backend

_log = get_logger("main")


async def _varrer_uma_vez() -> int:
    """Apaga os artefatos vencidos. Devolve quantos saíram.

    Sem store criado ainda não há artefato, então não há o que varrer — a
    varredura não constrói um só para poder passar (ver store_em_uso).

    O sweep toca no disco, então vai para a thread pool: bloquear o event loop
    para fazer faxina seria trocar um problema por outro.
    """
    store = store_em_uso()
    if store is None:
        return 0

    return await run_in_threadpool(store.sweep_expired)


async def _faxina_periodica(intervalo: float) -> None:
    """Varre os artefatos vencidos de tempos em tempos, para sempre.

    O artefato só era apagado quando alguém tentava carregá-lo depois do prazo —
    e um assistente abandonado no passo 1 nunca tem esse alguém. O arquivo ficava
    no volume até o disco acabar. `sweep_expired` já existia e era testado; o que
    faltava era alguém chamá-lo.

    Uma falha aqui não pode derrubar o serviço: a próxima passada tenta de novo.
    """
    while True:
        await asyncio.sleep(intervalo)

        # Except amplo de propósito: disco cheio, permissão, arquivo sumindo no
        # meio da varredura. Nenhum deles justifica matar a tarefa.
        try:
            removidos = await _varrer_uma_vez()
        except Exception:
            _log.warning("falha varrendo artefatos vencidos", extra={"stage": "sweep"})
            continue

        if removidos:
            _log.info(
                "artefatos vencidos removidos",
                extra={"stage": "sweep", "removed": removidos},
            )


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    # O work dir é criado pelo ArtifactStore no primeiro uso (que respeita o
    # override de dependência nos testes); aqui não há o que preparar além do
    # log e da faxina.
    configure_logging()
    _log.info("edital-processor starting", extra={"stage": "startup"})

    faxina = asyncio.create_task(_faxina_periodica(get_settings().artifact_sweep_seconds))
    # Um reinício perde os mapas que estavam na fila (em memória): pede ao
    # backend que os entregue de novo. Em segundo plano, porque o backend pode
    # ainda estar subindo.
    redespacho = asyncio.create_task(Backend(get_settings()).pedir_redespacho())

    try:
        yield
    finally:
        faxina.cancel()
        redespacho.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await faxina
        with contextlib.suppress(asyncio.CancelledError):
            await redespacho
        await parar_mapas()

        _log.info("edital-processor stopping", extra={"stage": "shutdown"})


def create_app() -> FastAPI:
    app = FastAPI(
        title="studygo edital-processor",
        version="0.1.0",
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.include_router(router)
    app.include_router(router_leis)
    app.include_router(router_mapas)
    app.include_router(router_claude)
    install_error_handler(app)
    return app


app = create_app()
