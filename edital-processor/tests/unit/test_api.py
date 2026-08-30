from __future__ import annotations

import io
from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient

from app.core.config import Settings, get_settings
from app.main import create_app


@pytest.fixture
def client(tmp_path: object) -> Iterator[TestClient]:
    def _settings() -> Settings:
        return Settings(
            service_token="s3cr3t",
            gemini_api_key="",
            work_dir=tmp_path / "work",  # type: ignore[operator]
        )

    app = create_app()
    app.dependency_overrides[get_settings] = _settings
    with TestClient(app) as c:
        yield c
    app.dependency_overrides.clear()


def test_healthz_ok(client: TestClient) -> None:
    r = client.get("/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok", "gemini": False}


def test_analisar_requires_service_token(client: TestClient, text_pdf: bytes) -> None:
    r = client.post(
        "/internal/editais/analisar",
        files={"file": ("e.pdf", io.BytesIO(text_pdf), "application/pdf")},
        headers={"x-owner-ref": "user-1"},
    )
    assert r.status_code == 401
    assert r.json()["code"] == "unauthorized"


def test_analisar_requires_owner_ref(client: TestClient, text_pdf: bytes) -> None:
    r = client.post(
        "/internal/editais/analisar",
        files={"file": ("e.pdf", io.BytesIO(text_pdf), "application/pdf")},
        headers={"authorization": "Bearer s3cr3t"},
    )
    assert r.status_code == 401


def test_analisar_happy_path(client: TestClient, text_pdf: bytes) -> None:
    r = client.post(
        "/internal/editais/analisar",
        files={"file": ("edital.pdf", io.BytesIO(text_pdf), "application/pdf")},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "user-1"},
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["totalPages"] == 2
    assert body["documentId"]
    assert len(body["sha256"]) == 64


def test_analisar_rejects_non_pdf(client: TestClient) -> None:
    r = client.post(
        "/internal/editais/analisar",
        files={"file": ("x.pdf", io.BytesIO(b"just text"), "application/pdf")},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "user-1"},
    )
    assert r.status_code == 400
    assert r.json()["code"] == "invalid_pdf"


def test_analisar_accepts_pasted_text(client: TestClient) -> None:
    r = client.post(
        "/internal/editais/analisar",
        json={"texto": "TRIBUNAL DE CONTAS. EDITAL No 01/2026. Codigo de Opcao B02."},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "user-1"},
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["totalPages"] == 1
    assert body["ocrPages"] == 0
    assert body["documentId"]


def test_analisar_rejects_empty_text(client: TestClient) -> None:
    r = client.post(
        "/internal/editais/analisar",
        json={"texto": "   "},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "user-1"},
    )
    assert r.status_code == 400
    assert r.json()["code"] == "invalid_pdf"
