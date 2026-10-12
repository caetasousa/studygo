"""Como uma rota chega ao caso de uso: pela montagem que o app guardou.

A montagem (app/main.py, ``Componentes``) cria cada peça no primeiro uso e a
guarda no ``app.state``. As rotas não constroem nada: pedem aqui.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Protocol, cast

from fastapi import Request

from app.platform.config import Settings

if TYPE_CHECKING:
    from app.port.claude import ContasDoClaude
    from app.service.edital.editais import Editais
    from app.service.leis.capturas import Capturas
    from app.service.mapas.fila import Mapas


class Montagem(Protocol):
    @property
    def settings(self) -> Settings: ...

    @property
    def editais(self) -> Editais: ...

    @property
    def capturas(self) -> Capturas: ...

    @property
    def mapas(self) -> Mapas: ...

    @property
    def conexoes(self) -> ContasDoClaude: ...


def montagem(request: Request) -> Montagem:
    return cast("Montagem", request.app.state.componentes)


def settings(request: Request) -> Settings:
    return montagem(request).settings


def editais(request: Request) -> Editais:
    return montagem(request).editais


def capturas(request: Request) -> Capturas:
    return montagem(request).capturas


def mapas(request: Request) -> Mapas:
    return montagem(request).mapas


def conexoes(request: Request) -> ContasDoClaude:
    return montagem(request).conexoes
