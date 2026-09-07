"""A faxina de artefatos e o event loop.

Dois problemas que eram invisíveis de fora: o disco enchendo devagar, e o
serviço parando de responder enquanto trabalhava. Nenhum dos dois aparece num
teste de resposta HTTP — daí estes.
"""

from __future__ import annotations

import asyncio
import io
import json
import time
from collections.abc import Iterator
from pathlib import Path

import httpx
import pytest
from fastapi.testclient import TestClient

import app.api.routes as routes
import app.services.pipeline as pipeline
from app.core.config import Settings, get_settings
from app.main import _varrer_uma_vez, create_app
from app.schemas.document import NormalizedDocument
from app.services.artifacts import ArtifactStore


@pytest.fixture(autouse=True)
def store_limpo() -> Iterator[None]:
    """O store do módulo é global; devolvê-lo ao estado anterior evita que um
    teste enxergue o artefato do outro."""
    anterior = routes._store
    routes._store = None
    yield
    routes._store = anterior


async def test_varrer_sem_store_nao_cria_nada(tmp_path: Path) -> None:
    """Sem requisição nenhuma não há artefato, e a faxina não constrói um store
    só para poder passar — construir criaria o work_dir padrão, fora do tmp."""
    assert await _varrer_uma_vez() == 0
    assert not (tmp_path / "work").exists()


async def test_varrer_remove_o_vencido_e_preserva_o_vivo(tmp_path: Path) -> None:
    settings = Settings(service_token="", gemini_api_key="", work_dir=tmp_path / "work")
    store = ArtifactStore(settings)
    routes._store = store

    vivo = NormalizedDocument(
        document_id=store.create_id(),
        owner_ref="user-1",
        filename="vivo.pdf",
        sha256="a" * 64,
        total_pages=1,
        ttl_seconds=3600,
        pages=[],
        tables=[],
    )
    store.save(vivo)

    vencido = NormalizedDocument(
        document_id=store.create_id(),
        owner_ref="user-1",
        filename="vencido.pdf",
        sha256="b" * 64,
        total_pages=1,
        ttl_seconds=1,
        pages=[],
        tables=[],
    )
    store.save(vencido)

    # Envelhece o segundo no disco, sem esperar de verdade.
    caminho = tmp_path / "work" / f"{vencido.document_id}.json"
    payload = json.loads(caminho.read_text(encoding="utf-8"))
    payload["created_at"] = time.time() - 10
    caminho.write_text(json.dumps(payload), encoding="utf-8")

    assert await _varrer_uma_vez() == 1

    restantes = {p.stem for p in (tmp_path / "work").glob("*.json")}
    assert restantes == {vivo.document_id}


async def test_varrer_duas_vezes_nao_apaga_o_que_ja_saiu(tmp_path: Path) -> None:
    settings = Settings(service_token="", gemini_api_key="", work_dir=tmp_path / "work")
    routes._store = ArtifactStore(settings)

    assert await _varrer_uma_vez() == 0
    assert await _varrer_uma_vez() == 0


async def test_analisar_nao_bloqueia_o_event_loop(
    tmp_path: Path, text_pdf: bytes, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A regressão que motivou o run_in_threadpool.

    `analyse` é síncrona e demora: PDF, OCR, classificação. Chamada direto de
    dentro de um `async def`, ela segura o event loop e NADA mais é atendido —
    nem o /healthz de 3 segundos do healthcheck do container, que passava a
    falhar durante toda importação.

    O teste não mede tempo (isso seria instável): ele conta quantas vezes uma
    tarefa concorrente conseguiu acordar enquanto a análise rodava. Com o loop
    bloqueado esse número é zero.
    """
    real = pipeline.analyse

    def analyse_lenta(**kwargs: object) -> object:
        time.sleep(0.3)  # bloqueante de propósito: é o que a de verdade faz

        return real(**kwargs)  # type: ignore[arg-type]

    monkeypatch.setattr(routes, "analyse", analyse_lenta)

    def _settings() -> Settings:
        return Settings(
            service_token="s3cr3t", gemini_api_key="", work_dir=tmp_path / "work"
        )

    aplicativo = create_app()
    aplicativo.dependency_overrides[get_settings] = _settings

    batidas = 0

    async def pulsar() -> None:
        nonlocal batidas
        while True:
            await asyncio.sleep(0.01)
            batidas += 1

    transporte = httpx.ASGITransport(app=aplicativo)

    async with httpx.AsyncClient(transport=transporte, base_url="http://proc") as cliente:
        pulso = asyncio.create_task(pulsar())

        resposta = await cliente.post(
            "/internal/editais/analisar",
            files={"file": ("e.pdf", io.BytesIO(text_pdf), "application/pdf")},
            headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "user-1"},
        )

        pulso.cancel()

    assert resposta.status_code == 200, resposta.text
    assert batidas > 0, "o event loop ficou parado durante a análise"


def test_o_ciclo_de_vida_sobe_e_desce_limpo(tmp_path: Path) -> None:
    """A tarefa de faxina é cancelada no shutdown — sem exceção vazando e sem
    tarefa órfã."""

    def _settings() -> Settings:
        return Settings(service_token="", gemini_api_key="", work_dir=tmp_path / "work")

    aplicativo = create_app()
    aplicativo.dependency_overrides[get_settings] = _settings

    with TestClient(aplicativo) as cliente:
        assert cliente.get("/healthz").status_code == 200
