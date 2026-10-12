"""A porta dos artefatos: o documento normalizado guardado entre os passos.

Só guarda e devolve. Quem decide se o artefato venceu ou se é de outra pessoa é
o caso de uso (service/edital/editais.py).
"""

from __future__ import annotations

from typing import Protocol

from app.domain.edital.document import NormalizedDocument


class Artefatos(Protocol):
    def create_id(self) -> str: ...

    def save(self, doc: NormalizedDocument) -> None: ...

    def load(self, document_id: str) -> NormalizedDocument:
        """DocumentNotFound se não existe ou se o id não é um id."""
        ...

    def delete(self, document_id: str) -> None: ...

    def sweep_expired(self) -> int:
        """Apaga os vencidos e os ilegíveis; devolve quantos saíram."""
        ...
