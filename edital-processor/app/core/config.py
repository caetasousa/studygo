"""Runtime configuration.

Every operational limit the pipeline enforces lives here so it can be tuned per
environment without a code change. Loaded once at import time from the process
environment (prefix ``EP_``) or an optional ``.env`` file.
"""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="EP_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # --- internal auth -----------------------------------------------------
    # Shared secret the Go backend sends on every internal call. The service is
    # never exposed publicly; this is defence in depth for the compose network.
    service_token: str = Field(default="", min_length=0)

    # --- upload / PDF limits --------------------------------------------------
    max_upload_bytes: int = 25 * 1024 * 1024
    max_pages: int = 80
    max_pixels_per_page: int = 40_000_000  # ~ 6500 x 6150; guards image bombs
    max_page_dimension: int = 20_000  # pt; a page larger than this is suspect

    # --- text quality ------------------------------------------------------
    # A page whose native-text score is below this is sent to OCR.
    min_text_score: float = 0.55
    # Below this many characters a page is treated as having no usable text at
    # all, regardless of score.
    min_text_chars: int = 40

    # --- OCR -------------------------------------------------------------------
    ocr_language: str = "por"
    ocr_dpi: int = 300
    ocr_timeout_seconds: float = 40.0
    # Measured on a 26-page scanned edital: 2 -> ~40s, 4 -> ~24s, 8 -> ~41s
    # (Tesseract is itself threaded, so oversubscribing costs more than it buys).
    ocr_max_concurrency: int = 4

    # --- Gemini ----------------------------------------------------------------
    gemini_api_key: str = Field(default="", min_length=0)
    # Fallback chain, tried in order. Mirrors the chain the Go adapter carried.
    gemini_models: tuple[str, ...] = (
        "gemini-flash-lite-latest",
        "gemini-3.5-flash-lite",
        "gemini-3.1-flash-lite",
        "gemini-flash-latest",
    )
    # A flash-lite call over our chunk budget answers in ~10-25s; past this it is
    # not going to land, and waiting only burns the caller's patience (and the Go
    # client's 4-minute budget, which covers the whole wizard step).
    gemini_timeout_seconds: float = 45.0
    # Ceiling for the whole fallback chain. The Go client allows 4 minutes per
    # wizard step, so give up before that and return a typed error instead of
    # letting the caller time out on us.
    gemini_total_budget_seconds: float = 200.0
    # Per model. The chain below is the real redundancy — retrying one overloaded
    # model many times mostly stalls, so keep this small and move on.
    gemini_max_attempts: int = 2

    # --- confidence ----------------------------------------------------------
    min_confidence: float = 0.0

    # --- temporary artifacts -----------------------------------------------
    work_dir: Path = Path("/var/lib/edital-processor/work")
    artifact_ttl_seconds: int = 3600

    # --- ops -----------------------------------------------------------------
    debug_endpoint_enabled: bool = False

    @property
    def gemini_enabled(self) -> bool:
        return bool(self.gemini_api_key)


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
