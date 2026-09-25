"""O catálogo `conteudo/leis/normas.toml`: o que baixar e de onde."""

from __future__ import annotations

import tomllib
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field


class Norma(BaseModel):
    slug: str = Field(pattern=r"^[a-z0-9]+(-[a-z0-9]+)*$")
    nome: str
    curto: str
    disciplina: str
    prioridade: Literal["A", "B", "C"]
    fonte: Literal["planalto", "casacivil-go", "link"]
    link: str = ""
    reconhecer: list[str] = []
    recorte: list[str] = []
    questoes: bool = False
    # Saltos de numeração que existem de verdade na lei ("art7 → art9"): sem
    # declarar aqui, a verificação trata o salto como artigo perdido.
    aceitar: list[str] = []


def ler_normas(caminho: Path) -> list[Norma]:
    dados = tomllib.loads(caminho.read_text(encoding="utf-8"))
    normas = [Norma.model_validate(n) for n in dados.get("norma", [])]
    slugs = [n.slug for n in normas]
    repetidos = {s for s in slugs if slugs.count(s) > 1}
    if repetidos:
        raise ValueError(f"slug repetido em {caminho.name}: {', '.join(sorted(repetidos))}")
    return normas
