"""FastAPI application entrypoint."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.routes import install_error_handler, router
from app.core.logging import configure_logging, get_logger

_log = get_logger("main")


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    # The work dir is created by ArtifactStore on first use (which honours a
    # dependency override in tests); nothing to set up here but logging.
    configure_logging()
    _log.info("edital-processor starting", extra={"stage": "startup"})
    yield
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
    install_error_handler(app)
    return app


app = create_app()
