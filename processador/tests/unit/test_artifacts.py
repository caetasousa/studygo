from __future__ import annotations

import time

import pytest

from app.adapter.disco.artefatos import ArtifactStore
from app.adapter.llm.nulo import NullProvider
from app.adapter.ocr.tesseract import OcrTesseract
from app.adapter.pdf.leitor import LeitorPyMuPDF
from app.domain.edital.document import NormalizedDocument, PageText
from app.platform.config import Settings
from app.platform.errors import DocumentExpired, DocumentNotFound, DocumentOwnerMismatch
from app.service.edital.editais import Editais


def _doc(store: ArtifactStore, *, owner: str = "user-1", ttl: int = 3600) -> NormalizedDocument:
    return NormalizedDocument(
        document_id=store.create_id(),
        owner_ref=owner,
        filename="edital.pdf",
        sha256="a" * 64,
        total_pages=1,
        ttl_seconds=ttl,
        pages=[PageText(physical_page=1, text="x")],
    )


def _editais(settings: Settings, store: ArtifactStore) -> Editais:
    return Editais(settings, store, LeitorPyMuPDF(settings), OcrTesseract(settings), NullProvider())


def test_save_then_load_roundtrips(store: ArtifactStore) -> None:
    doc = _doc(store)
    store.save(doc)
    loaded = store.load(doc.document_id)
    assert loaded.document_id == doc.document_id
    assert loaded.pages[0].text == "x"


def test_load_unknown_id_raises(store: ArtifactStore) -> None:
    with pytest.raises(DocumentNotFound):
        store.load(store.create_id())


def test_malformed_id_is_rejected_before_fs(store: ArtifactStore) -> None:
    with pytest.raises(DocumentNotFound):
        store.load("../../etc/passwd")


def test_owner_mismatch_raises(settings: Settings, store: ArtifactStore) -> None:
    doc = _doc(store, owner="user-1")
    store.save(doc)
    with pytest.raises(DocumentOwnerMismatch):
        _editais(settings, store).carregar(doc.document_id, "user-2")


def test_expired_artifact_raises_and_is_removed(settings: Settings, store: ArtifactStore) -> None:
    doc = _doc(store, ttl=0)
    store.save(doc)
    time.sleep(0.01)
    editais = _editais(settings, store)
    with pytest.raises(DocumentExpired):
        editais.carregar(doc.document_id, "user-1")
    with pytest.raises(DocumentNotFound):
        editais.carregar(doc.document_id, "user-1")


def test_sweep_removes_only_expired(store: ArtifactStore) -> None:
    fresh = _doc(store, ttl=3600)
    stale = _doc(store, ttl=0)
    store.save(fresh)
    store.save(stale)
    time.sleep(0.01)
    assert store.sweep_expired() == 1
    assert store.load(fresh.document_id).document_id == fresh.document_id


def test_save_is_atomic_no_partial_file(store: ArtifactStore, tmp_path: object) -> None:
    doc = _doc(store)
    store.save(doc)
    # no .tmp files left behind
    leftovers = list(store._root.glob("*.tmp"))  # type: ignore[attr-defined]
    assert leftovers == []
