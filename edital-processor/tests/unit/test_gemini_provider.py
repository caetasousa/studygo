"""Gemini provider: retry, model fallback, timeout, bad-response handling.

The real API is never hit here — a fake ``google.genai`` module is injected.
The opt-in real test lives in ``tests/integration`` behind the ``gemini`` mark.
"""

from __future__ import annotations

import asyncio
import sys
import types
from typing import Any

import pytest

from app.core.config import Settings
from app.core.errors import (
    InvalidProviderResponse,
    ProviderRateLimited,
    ProviderTimeout,
    ProviderUnavailable,
)
from app.providers.base import StructuredRequest


class _FakeClientError(Exception):
    def __init__(self, code: int) -> None:
        super().__init__(f"client error {code}")
        self.code = code


class _FakeServerError(Exception):
    def __init__(self, code: int = 503) -> None:
        super().__init__(f"server error {code}")
        self.code = code


class _FakeAPIError(Exception):
    pass


def _install_fake_genai(monkeypatch: pytest.MonkeyPatch, behaviour: Any) -> list[str]:
    """Wire a fake google.genai whose generate_content runs ``behaviour``.
    Returns a list that records which models were tried."""
    tried: list[str] = []

    class _Models:
        async def generate_content(self, *, model: str, contents: str, config: Any) -> Any:
            tried.append(model)
            return await behaviour(model, contents, config)

    class _Aio:
        models = _Models()

    class _Client:
        def __init__(self, *, api_key: str) -> None:
            self.aio = _Aio()

    genai_mod = types.ModuleType("google.genai")
    genai_mod.Client = _Client  # type: ignore[attr-defined]

    types_mod = types.ModuleType("google.genai.types")

    class _Cfg:
        def __init__(self, **kw: Any) -> None: ...

    types_mod.GenerateContentConfig = _Cfg  # type: ignore[attr-defined]

    errors_mod = types.ModuleType("google.genai.errors")
    errors_mod.ClientError = _FakeClientError  # type: ignore[attr-defined]
    errors_mod.ServerError = _FakeServerError  # type: ignore[attr-defined]
    errors_mod.APIError = _FakeAPIError  # type: ignore[attr-defined]

    google_mod = types.ModuleType("google")
    google_mod.genai = genai_mod  # type: ignore[attr-defined]

    monkeypatch.setitem(sys.modules, "google", google_mod)
    monkeypatch.setitem(sys.modules, "google.genai", genai_mod)
    monkeypatch.setitem(sys.modules, "google.genai.types", types_mod)
    monkeypatch.setitem(sys.modules, "google.genai.errors", errors_mod)
    return tried


def _settings() -> Settings:
    return Settings(
        gemini_api_key="fake",
        gemini_models=("m1", "m2", "m3"),
        gemini_max_attempts=2,
        gemini_timeout_seconds=0.2,
    )


def _request() -> StructuredRequest:
    return StructuredRequest(
        system="s", chunks=["<<<CHUNK id=p1#0>>>x<<<FIM CHUNK p1#0>>>"], response_schema={}
    )


@pytest.fixture
def provider(monkeypatch: pytest.MonkeyPatch):  # type: ignore[no-untyped-def]
    from app.providers.gemini import GeminiProvider

    return lambda behaviour: (
        _install_fake_genai(monkeypatch, behaviour),
        GeminiProvider(_settings()),
    )


async def test_returns_parsed_json_on_success(provider: Any) -> None:
    async def ok(model: str, contents: str, config: Any) -> Any:
        return types.SimpleNamespace(text='{"banca": "FCC", "cargos": []}')

    tried, gp = provider(ok)
    result = await gp.extract_structured(_request())
    assert result == {"banca": "FCC", "cargos": []}
    assert tried == ["m1"]


async def test_gives_up_when_the_total_budget_is_spent(provider: Any) -> None:
    """The Go client allows ~4 minutes per wizard step. The chain must give up
    before that and raise a typed error, rather than let the caller time out."""

    async def slow(model: str, contents: str, config: Any) -> Any:
        # Outlive the per-call timeout so every attempt ends as a TimeoutError.
        await asyncio.sleep(10)
        raise AssertionError("should not be reached")

    tried, gp = provider(slow)
    # Budget smaller than one full call: the chain must stop after the first
    # model instead of walking all four.
    gp._settings.gemini_timeout_seconds = 0.05  # type: ignore[misc]
    gp._settings.gemini_total_budget_seconds = 0.05  # type: ignore[misc]

    with pytest.raises(ProviderTimeout):
        await gp.extract_structured(_request())

    assert tried == ["m1"]


async def test_falls_back_to_next_model_on_server_error(provider: Any) -> None:
    async def flaky(model: str, contents: str, config: Any) -> Any:
        if model in {"m1", "m2"}:
            raise _FakeServerError(503)
        return types.SimpleNamespace(text="{}")

    tried, gp = provider(flaky)
    result = await gp.extract_structured(_request())
    assert result == {}
    # A 503 is that model being out of capacity, so we move to the next one
    # immediately instead of retrying a model we know is overloaded.
    assert tried == ["m1", "m2", "m3"]


async def test_rate_limit_exhausts_chain_then_raises(provider: Any) -> None:
    async def limited(model: str, contents: str, config: Any) -> Any:
        raise _FakeClientError(429)

    tried, gp = provider(limited)
    with pytest.raises(ProviderRateLimited):
        await gp.extract_structured(_request())
    assert tried == ["m1", "m1", "m2", "m2", "m3", "m3"]


async def test_non_429_client_error_is_not_retried(provider: Any) -> None:
    async def bad_request(model: str, contents: str, config: Any) -> Any:
        raise _FakeClientError(400)

    tried, gp = provider(bad_request)
    with pytest.raises(InvalidProviderResponse):
        await gp.extract_structured(_request())
    assert tried == ["m1"]


async def test_timeout_moves_to_next_model_then_raises(provider: Any) -> None:
    import asyncio

    async def slow(model: str, contents: str, config: Any) -> Any:
        await asyncio.sleep(5)

    tried, gp = provider(slow)
    with pytest.raises(ProviderTimeout):
        await gp.extract_structured(_request())
    assert tried == ["m1", "m2", "m3"]


async def test_empty_response_is_invalid(provider: Any) -> None:
    async def empty(model: str, contents: str, config: Any) -> Any:
        return types.SimpleNamespace(text="")

    _, gp = provider(empty)
    with pytest.raises(InvalidProviderResponse):
        await gp.extract_structured(_request())


async def test_non_object_json_is_invalid(provider: Any) -> None:
    async def arr(model: str, contents: str, config: Any) -> Any:
        return types.SimpleNamespace(text="[1, 2, 3]")

    _, gp = provider(arr)
    with pytest.raises(InvalidProviderResponse):
        await gp.extract_structured(_request())


async def test_no_key_reports_unavailable() -> None:
    from app.providers.gemini import GeminiProvider

    gp = GeminiProvider(Settings(gemini_api_key=""))
    assert gp.available() is False
    with pytest.raises(ProviderUnavailable):
        await gp.extract_structured(_request())
