"""Composition root: aqui cada adapter é ligado ao seu caso de uso.

É o único lugar que conhece as implementações. As rotas pedem o caso de uso
pronto (adapter/httpapi/dependencias.py); os casos de uso só veem portas.

Cada peça nasce no primeiro uso, não na importação: o ``app`` do módulo é criado
quando o uvicorn importa este arquivo, e criar ali o store de artefatos já
escreveria no work_dir — em teste, fora do tmp_path. A fila dos mapas também
precisa nascer dentro do event loop que vai atendê-la.
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from functools import cached_property
from pathlib import Path

from fastapi import FastAPI

from app.adapter.backend.cliente import Backend
from app.adapter.claude.cli import rodar_claude_de_verdade
from app.adapter.claude.conexao import Conexoes
from app.adapter.disco.artefatos import ArtifactStore
from app.adapter.fontes.http import http_padrao
from app.adapter.httpapi.editais import install_error_handler, router
from app.adapter.httpapi.leis import router as router_leis
from app.adapter.httpapi.mapas import router as router_mapas
from app.adapter.httpapi.mapas import router_claude
from app.adapter.llm.nulo import NullProvider
from app.adapter.ocr.tesseract import OcrTesseract
from app.adapter.pdf.leitor import LeitorPyMuPDF
from app.platform.config import Settings, get_settings
from app.platform.logging import configure_logging, get_logger
from app.port.backend import Avisar, Entregar
from app.port.claude import RodarClaude
from app.port.fontes import Http
from app.port.llm import LLMProvider
from app.service.edital.editais import Editais
from app.service.leis.capturas import Capturas
from app.service.mapas.fila import Mapas

_log = get_logger("main")


class Componentes:
    """As peças do serviço, montadas sob demanda.

    Os testes trocam só a fronteira de fora — o LLM, as fontes oficiais de lei,
    o backend, o Claude — passando a substituta aqui; o resto é o de verdade.
    """

    def __init__(
        self,
        settings: Settings | None = None,
        *,
        llm: LLMProvider | None = None,
        http_leis: Http | None = None,
        entregar: Entregar | None = None,
        avisar_falha: Avisar | None = None,
        claude: RodarClaude | None = None,
    ) -> None:
        self._settings = settings
        self._llm = llm
        self._http_leis = http_leis
        self._entregar = entregar
        self._avisar_falha = avisar_falha
        self._claude = claude

    def montado(self, nome: str) -> bool:
        """Se a peça já nasceu. A faxina e o desligamento não criam uma só
        para poder limpá-la."""
        return nome in self.__dict__

    @cached_property
    def settings(self) -> Settings:
        return self._settings or get_settings()

    @cached_property
    def llm(self) -> LLMProvider:
        if self._llm is not None:
            return self._llm
        if self.settings.gemini_enabled:
            # O SDK do Gemini é pesado: só carrega quando há chave.
            from app.adapter.llm.gemini import GeminiProvider

            return GeminiProvider(self.settings)
        return NullProvider()

    @cached_property
    def artefatos(self) -> ArtifactStore:
        return ArtifactStore(self.settings)

    @cached_property
    def pdf(self) -> LeitorPyMuPDF:
        return LeitorPyMuPDF(self.settings)

    @cached_property
    def editais(self) -> Editais:
        return Editais(
            self.settings,
            self.artefatos,
            self.pdf,
            OcrTesseract(self.settings),
            self.llm,
        )

    @cached_property
    def capturas(self) -> Capturas:
        return Capturas(
            http=self._http_leis or http_padrao,
            provider=self.llm if self.llm.available() else None,
            pdf=self.pdf,
        )

    @cached_property
    def backend(self) -> Backend:
        return Backend(self.settings)

    @cached_property
    def conexoes(self) -> Conexoes:
        return Conexoes(self.settings)

    @cached_property
    def mapas(self) -> Mapas:
        settings = self.settings

        async def rodar(
            args: list[str], mensagem: str, pasta: Path, ambiente: dict[str, str]
        ) -> tuple[int, str]:
            return await rodar_claude_de_verdade(
                args, mensagem, pasta, ambiente, settings.mapas_timeout_seconds
            )

        return Mapas(
            settings,
            self._entregar or self.backend.entregar,
            self._avisar_falha or self.backend.avisar_falha,
            self._claude or rodar,
            self.conexoes,
        )

    async def pedir_redespacho(self) -> None:
        # Com o backend substituído (testes), não há a quem pedir.
        if self._entregar is None:
            await self.backend.pedir_redespacho()

    async def parar(self) -> None:
        """No desligamento: interrompe o Claude em andamento (o redespacho o
        retoma) e encerra os logins pela metade."""
        if self.montado("mapas"):
            await self.mapas.parar()
        if self.montado("conexoes"):
            self.conexoes.encerrar_tudo()


async def _varrer_uma_vez(componentes: Componentes) -> int:
    """Apaga os artefatos vencidos. Devolve quantos saíram.

    Sem store criado ainda não há artefato, então não há o que varrer — a
    varredura não constrói um só para poder passar.

    O sweep toca no disco, então vai para uma thread: bloquear o event loop
    para fazer faxina seria trocar um problema por outro.
    """
    if not componentes.montado("artefatos"):
        return 0

    return await asyncio.to_thread(componentes.editais.varrer_vencidos)


async def _faxina_periodica(componentes: Componentes, intervalo: float) -> None:
    """Varre os artefatos vencidos de tempos em tempos, para sempre.

    O artefato só era apagado quando alguém tentava carregá-lo depois do prazo —
    e um assistente abandonado no passo 1 nunca tem esse alguém. O arquivo ficava
    no volume até o disco acabar. `sweep_expired` já existia e era testado; o que
    faltava era alguém chamá-lo.

    Uma falha aqui não pode derrubar o serviço: a próxima passada tenta de novo.
    """
    while True:
        await asyncio.sleep(intervalo)

        # Except amplo de propósito: disco cheio, permissão, arquivo sumindo no
        # meio da varredura. Nenhum deles justifica matar a tarefa.
        try:
            removidos = await _varrer_uma_vez(componentes)
        except Exception:
            _log.warning("falha varrendo artefatos vencidos", extra={"stage": "sweep"})
            continue

        if removidos:
            _log.info(
                "artefatos vencidos removidos",
                extra={"stage": "sweep", "removed": removidos},
            )


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    componentes: Componentes = app.state.componentes
    # O work dir é criado pelo ArtifactStore no primeiro uso; aqui não há o que
    # preparar além do log e da faxina.
    configure_logging()
    _log.info("edital-processor starting", extra={"stage": "startup"})

    faxina = asyncio.create_task(
        _faxina_periodica(componentes, componentes.settings.artifact_sweep_seconds)
    )
    # Um reinício perde os mapas que estavam na fila (em memória): pede ao
    # backend que os entregue de novo. Em segundo plano, porque o backend pode
    # ainda estar subindo.
    redespacho = asyncio.create_task(componentes.pedir_redespacho())

    try:
        yield
    finally:
        faxina.cancel()
        redespacho.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await faxina
        with contextlib.suppress(asyncio.CancelledError):
            await redespacho
        await componentes.parar()

        _log.info("edital-processor stopping", extra={"stage": "shutdown"})


def create_app(componentes: Componentes | None = None) -> FastAPI:
    app = FastAPI(
        title="studygo edital-processor",
        version="0.1.0",
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.state.componentes = componentes or Componentes()
    app.include_router(router)
    app.include_router(router_leis)
    app.include_router(router_mapas)
    app.include_router(router_claude)
    install_error_handler(app)
    return app


app = create_app()
