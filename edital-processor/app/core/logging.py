"""Structured logging.

Metadata only. Never a line of PDF text, a prompt, a token, a key, or a client
path. The formatter emits one JSON object per record so the compose log driver
and any aggregator can parse it.
"""

from __future__ import annotations

import json
import logging
import sys
from typing import Any

_SAFE_EXTRA_KEYS = frozenset(
    {
        "request_id",
        "stage",
        "duration_ms",
        "page_count",
        "ocr_pages",
        "model",
        "attempts",
        "error_code",
        "document_id",
        "bytes",
    }
)


class JSONFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "level": record.levelname,
            "logger": record.name,
            "msg": record.getMessage(),
        }
        for key in _SAFE_EXTRA_KEYS:
            if key in record.__dict__:
                payload[key] = record.__dict__[key]
        if record.exc_info:
            payload["exc"] = self.formatException(record.exc_info).splitlines()[-1]
        return json.dumps(payload, ensure_ascii=False)


def configure_logging(level: int = logging.INFO) -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JSONFormatter())
    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(level)


def get_logger(name: str) -> logging.Logger:
    return logging.getLogger(name)
