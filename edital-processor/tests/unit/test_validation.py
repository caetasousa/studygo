from __future__ import annotations

import pymupdf
import pytest

from app.core.config import Settings
from app.core.errors import (
    CorruptedPDF,
    EncryptedPDF,
    InvalidPDF,
    PageLimitExceeded,
    UploadTooLarge,
)
from app.services.validation import validate_upload


def test_accepts_a_real_text_pdf(text_pdf: bytes, settings: Settings) -> None:
    result = validate_upload(text_pdf, "application/pdf", settings)
    assert result.page_count == 2


def test_rejects_empty_upload(settings: Settings) -> None:
    with pytest.raises(InvalidPDF):
        validate_upload(b"", "application/pdf", settings)


def test_rejects_non_pdf_bytes(settings: Settings) -> None:
    with pytest.raises(InvalidPDF):
        validate_upload(b"not a pdf at all, just text", "application/pdf", settings)


def test_rejects_wrong_declared_mime(text_pdf: bytes, settings: Settings) -> None:
    with pytest.raises(InvalidPDF):
        validate_upload(text_pdf, "image/png", settings)


def test_octet_stream_is_allowed_when_bytes_are_pdf(text_pdf: bytes, settings: Settings) -> None:
    result = validate_upload(text_pdf, "application/octet-stream", settings)
    assert result.page_count == 2


def test_rejects_oversized_upload(text_pdf: bytes, settings: Settings) -> None:
    tiny = settings.model_copy(update={"max_upload_bytes": 10})
    with pytest.raises(UploadTooLarge):
        validate_upload(text_pdf, "application/pdf", tiny)


def test_rejects_too_many_pages(settings: Settings) -> None:
    doc = pymupdf.open()
    for _ in range(5):
        doc.new_page()
    data = doc.tobytes()
    doc.close()
    capped = settings.model_copy(update={"max_pages": 3})
    with pytest.raises(PageLimitExceeded):
        validate_upload(data, "application/pdf", capped)


def test_rejects_encrypted_pdf(settings: Settings) -> None:
    doc = pymupdf.open()
    doc.new_page()
    data = doc.tobytes(encryption=pymupdf.PDF_ENCRYPT_AES_256, owner_pw="x", user_pw="secret")
    doc.close()
    with pytest.raises(EncryptedPDF):
        validate_upload(data, "application/pdf", settings)


def test_rejects_truncated_pdf(text_pdf: bytes, settings: Settings) -> None:
    with pytest.raises((CorruptedPDF, InvalidPDF)):
        validate_upload(text_pdf[:200], "application/pdf", settings)
