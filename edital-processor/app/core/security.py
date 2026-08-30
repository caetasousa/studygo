"""Internal-call authentication.

The service never sees the user's JWT. The Go backend validates that, checks
ownership, and calls here with:

  * ``Authorization: Bearer <service token>`` — a shared secret for the compose
    network; compared in constant time.
  * ``X-Owner-Ref`` — an opaque, stable per-user handle. The service stores it on
    the artifact and refuses later steps that present a different one. It is
    never logged and carries no meaning here beyond equality.
  * ``X-Request-Id`` — propagated from the Go request for correlated logs.
"""

from __future__ import annotations

import secrets

from fastapi import Depends, Header, Request

from app.core.config import Settings, get_settings
from app.core.errors import Unauthorized

OWNER_REF_HEADER = "x-owner-ref"
REQUEST_ID_HEADER = "x-request-id"


def require_service_token(
    authorization: str | None = Header(default=None),
    settings: Settings = Depends(get_settings),
) -> None:
    """FastAPI dependency: reject a call without the shared service token.

    When no token is configured the guard is disabled — useful for local unit
    runs, never for a deployed service (compose always sets it).
    """
    expected = settings.service_token
    if not expected:
        return

    presented = ""
    if authorization and authorization.lower().startswith("bearer "):
        presented = authorization[7:]

    if not secrets.compare_digest(presented, expected):
        raise Unauthorized("invalid or missing service token")


def owner_ref(request: Request) -> str:
    ref = request.headers.get(OWNER_REF_HEADER, "").strip()
    if not ref:
        raise Unauthorized("missing owner reference")
    return ref


def request_id(request: Request) -> str | None:
    return request.headers.get(REQUEST_ID_HEADER) or None
