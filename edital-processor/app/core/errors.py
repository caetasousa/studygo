"""Typed errors.

Every failure the pipeline can raise is one of these. Each carries a stable
``code`` (the Go backend maps it to the public contract), a safe message (no PDF
text, no paths, no secrets), whether a retry could plausibly succeed, and the
HTTP status the internal API should return.
"""

from __future__ import annotations

from fastapi import status


class ProcessorError(Exception):
    code: str = "internal_error"
    http_status: int = status.HTTP_500_INTERNAL_SERVER_ERROR
    transient: bool = False

    def __init__(self, message: str | None = None) -> None:
        super().__init__(message or self.__class__.__name__)
        self.message = message or self.code

    def as_payload(self, request_id: str | None) -> dict[str, object]:
        return {
            "code": self.code,
            "message": self.message,
            "transient": self.transient,
            "requestId": request_id,
        }


# --- client / document problems (400-ish, not transient) -----------------------


class InvalidPDF(ProcessorError):
    code = "invalid_pdf"
    http_status = status.HTTP_400_BAD_REQUEST


class EncryptedPDF(ProcessorError):
    code = "encrypted_pdf"
    http_status = status.HTTP_400_BAD_REQUEST


class CorruptedPDF(ProcessorError):
    code = "corrupted_pdf"
    http_status = status.HTTP_400_BAD_REQUEST


class UploadTooLarge(ProcessorError):
    code = "upload_too_large"
    http_status = status.HTTP_413_CONTENT_TOO_LARGE


class PageLimitExceeded(ProcessorError):
    code = "page_limit_exceeded"
    http_status = status.HTTP_422_UNPROCESSABLE_CONTENT


class RenderLimitExceeded(ProcessorError):
    code = "render_limit_exceeded"
    http_status = status.HTTP_422_UNPROCESSABLE_CONTENT


class ValidationFailed(ProcessorError):
    code = "validation_failed"
    http_status = status.HTTP_422_UNPROCESSABLE_CONTENT


# --- artifact lifecycle -------------------------------------------------------


class DocumentNotFound(ProcessorError):
    code = "document_not_found"
    http_status = status.HTTP_404_NOT_FOUND


class DocumentExpired(ProcessorError):
    code = "document_expired"
    http_status = status.HTTP_410_GONE


class DocumentOwnerMismatch(ProcessorError):
    code = "document_owner_mismatch"
    http_status = status.HTTP_403_FORBIDDEN


# --- OCR ---------------------------------------------------------------------


class OCRUnavailable(ProcessorError):
    code = "ocr_unavailable"
    http_status = status.HTTP_503_SERVICE_UNAVAILABLE
    transient = True


class OCRTimeout(ProcessorError):
    code = "ocr_timeout"
    http_status = status.HTTP_504_GATEWAY_TIMEOUT
    transient = True


# --- provider (Gemini) -------------------------------------------------------


class ProviderRateLimited(ProcessorError):
    code = "provider_rate_limited"
    http_status = status.HTTP_429_TOO_MANY_REQUESTS
    transient = True


class ProviderUnavailable(ProcessorError):
    code = "provider_unavailable"
    http_status = status.HTTP_503_SERVICE_UNAVAILABLE
    transient = True


class ProviderTimeout(ProcessorError):
    code = "provider_timeout"
    http_status = status.HTTP_504_GATEWAY_TIMEOUT
    transient = True


class InvalidProviderResponse(ProcessorError):
    code = "invalid_provider_response"
    http_status = status.HTTP_502_BAD_GATEWAY
    transient = False


# --- auth --------------------------------------------------------------------


class Unauthorized(ProcessorError):
    code = "unauthorized"
    http_status = status.HTTP_401_UNAUTHORIZED
