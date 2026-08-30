"""PDF validation (spec §7.1-7.5).

Treats the upload as hostile: checks the declared type against the real bytes,
opens the file with a real parser, and rejects encryption, corruption, and
resource bombs *before* any page is rendered.
"""

from __future__ import annotations

from dataclasses import dataclass

import pymupdf

from app.core.config import Settings
from app.core.errors import (
    CorruptedPDF,
    EncryptedPDF,
    InvalidPDF,
    PageLimitExceeded,
    RenderLimitExceeded,
    UploadTooLarge,
)

_PDF_MAGIC = b"%PDF-"


@dataclass(frozen=True)
class ValidatedPDF:
    data: bytes
    page_count: int


def validate_upload(data: bytes, declared_mime: str | None, settings: Settings) -> ValidatedPDF:
    if len(data) == 0:
        raise InvalidPDF("empty upload")
    if len(data) > settings.max_upload_bytes:
        raise UploadTooLarge(f"upload is {len(data)} bytes, limit is {settings.max_upload_bytes}")

    if declared_mime and declared_mime.split(";")[0].strip() not in {
        "application/pdf",
        "application/x-pdf",
        "application/octet-stream",
    }:
        raise InvalidPDF(f"declared content type is not a PDF: {declared_mime}")

    # The real bytes, not the declared type, decide.
    if not data[:1024].lstrip().startswith(_PDF_MAGIC):
        raise InvalidPDF("bytes do not start with a PDF header")

    try:
        doc = pymupdf.open(stream=data, filetype="pdf")
    except Exception as exc:  # pymupdf raises a bare Exception subclass
        raise CorruptedPDF("the PDF could not be parsed") from exc

    try:
        if doc.is_encrypted and not doc.authenticate(""):
            raise EncryptedPDF("the PDF is password protected")

        page_count = doc.page_count
        if page_count < 1:
            raise CorruptedPDF("the PDF has no pages")
        if page_count > settings.max_pages:
            raise PageLimitExceeded(f"{page_count} pages, limit is {settings.max_pages}")

        for index in range(page_count):
            page = doc.load_page(index)
            rect = page.rect
            if (
                rect.width > settings.max_page_dimension
                or rect.height > settings.max_page_dimension
            ):
                raise RenderLimitExceeded(f"page {index + 1} is abnormally large")
            # Guard against a page whose embedded image would decode to a huge
            # bitmap.
            for img in page.get_images(full=True):
                info = doc.extract_image(img[0])
                if info["width"] * info["height"] > settings.max_pixels_per_page:
                    raise RenderLimitExceeded(
                        f"page {index + 1} carries an image over the pixel limit"
                    )
    finally:
        doc.close()

    return ValidatedPDF(data=data, page_count=page_count)
