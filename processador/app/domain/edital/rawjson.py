"""Typed access to a decoded-JSON dict.

The provider returns ``dict[str, object]``. These helpers narrow the members so
the mapping code stays type-clean without ``# type: ignore`` at every access.
Anything that does not match the requested shape comes back as the default.
"""

from __future__ import annotations

from typing import Any


def as_dict(value: object) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def as_list(value: object) -> list[Any]:
    return value if isinstance(value, list) else []


def as_dict_list(value: object) -> list[dict[str, Any]]:
    return [item for item in as_list(value) if isinstance(item, dict)]


def get_str(source: dict[str, Any], key: str) -> str | None:
    value = source.get(key)
    if isinstance(value, str) and value.strip():
        return value.strip()
    return None


def get_int(source: dict[str, Any], key: str) -> int | None:
    value = source.get(key)
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value.is_integer():
        return int(value)
    return None


def get_float(source: dict[str, Any], key: str) -> float | None:
    value = source.get(key)
    if isinstance(value, bool):
        return None
    if isinstance(value, int | float):
        return float(value)
    return None


def get_str_list(source: dict[str, Any], key: str) -> list[str]:
    return [
        item.strip() for item in as_list(source.get(key)) if isinstance(item, str) and item.strip()
    ]
