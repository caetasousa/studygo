"""Shared model base.

The internal HTTP contract is JSON with camelCase keys — that is what the Go
backend marshals and unmarshals. Python code uses snake_case throughout; this
base does the translation on the wire, both directions.
"""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict
from pydantic.alias_generators import to_camel


class WireModel(BaseModel):
    model_config = ConfigDict(
        alias_generator=to_camel,
        validate_by_name=True,
        validate_by_alias=True,
        serialize_by_alias=True,
    )
