"""Temporary document artifacts (spec §5).

A processed edital is stored under a random UUID, bound to an ``owner_ref``, with
a TTL. Later wizard steps load it by id, and the load fails unless the same
owner_ref is presented. Paths are always derived here from the UUID — never from
anything the client sent — so there is no path traversal surface.
"""

from __future__ import annotations

import json
import os
import tempfile
import time
import uuid
from pathlib import Path

from app.core.config import Settings
from app.core.errors import DocumentExpired, DocumentNotFound, DocumentOwnerMismatch
from app.schemas.document import NormalizedDocument

_UUID_RE = uuid.UUID  # parse-guard for ids read back in


class ArtifactStore:
    def __init__(self, settings: Settings) -> None:
        self._root = settings.work_dir
        self._ttl = settings.artifact_ttl_seconds
        self._root.mkdir(parents=True, exist_ok=True)

    def _path_for(self, document_id: str) -> Path:
        # Reject anything that is not a plain UUID before it touches the fs.
        try:
            parsed = _UUID_RE(document_id)
        except (ValueError, AttributeError, TypeError) as exc:
            raise DocumentNotFound("malformed document id") from exc
        return self._root / f"{parsed}.json"

    def create_id(self) -> str:
        return str(uuid.uuid4())

    def save(self, doc: NormalizedDocument) -> None:
        path = self._path_for(doc.document_id)
        payload = doc.model_dump_json()
        # Atomic write: temp file in the same dir, then rename.
        fd, tmp_name = tempfile.mkstemp(dir=self._root, suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as fh:
                fh.write(payload)
            Path(tmp_name).replace(path)
        except BaseException:
            Path(tmp_name).unlink(missing_ok=True)
            raise

    def load(self, document_id: str, owner_ref: str) -> NormalizedDocument:
        path = self._path_for(document_id)
        try:
            raw = path.read_text(encoding="utf-8")
        except FileNotFoundError as exc:
            raise DocumentNotFound("no such document") from exc

        doc = NormalizedDocument.model_validate(json.loads(raw))

        if doc.is_expired:
            path.unlink(missing_ok=True)
            raise DocumentExpired("document artifact has expired")
        if doc.owner_ref != owner_ref:
            raise DocumentOwnerMismatch("document belongs to another user")
        return doc

    def delete(self, document_id: str) -> None:
        self._path_for(document_id).unlink(missing_ok=True)

    def sweep_expired(self) -> int:
        """Remove expired artifacts. Returns how many were deleted."""
        removed = 0
        now = time.time()
        for path in self._root.glob("*.json"):
            try:
                created = json.loads(path.read_text(encoding="utf-8")).get("created_at", 0)
                ttl = json.loads(path.read_text(encoding="utf-8")).get("ttl_seconds", self._ttl)
            except (OSError, json.JSONDecodeError):
                path.unlink(missing_ok=True)
                removed += 1
                continue
            if now - created > ttl:
                path.unlink(missing_ok=True)
                removed += 1
        return removed
