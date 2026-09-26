"""As capturas em andamento: o link colado na tela vira uma tarefa em segundo
plano, consultada pelo id até ficar pronta.

A Constituição leva mais de um minuto com o Gemini — mais do que o proxy e o
túnel deixam uma requisição aberta. Por isso a captura não responde na mesma
chamada: ela roda no event loop do processador (o cliente do Gemini é
assíncrono e preso a ele) e o resultado fica aqui, em memória, até expirar.
Perder as capturas num reinício é aceitável: a pessoa cola o link de novo.
"""

from __future__ import annotations

import asyncio
import secrets
import time
from collections.abc import Callable
from dataclasses import dataclass, field

from app.core.errors import CapturaNaoEncontrada, CapturasDemais, LinkDeLeiInvalido
from app.core.logging import get_logger
from app.leis.captura import Captura, capturar
from app.leis.fontes import FonteInvalida, Http, fonte_do_link
from app.providers.base import LLMProvider

__all__ = ["CapturaNaoEncontrada", "Capturas", "CapturasDemais"]

_log = get_logger("leis")

_ERRO_INTERNO = (
    "A captura falhou por um erro interno do processador. Tente de novo; se "
    "repetir, veja o log do edital-processor."
)


@dataclass
class _Andamento:
    dono: str
    estado: str = "rodando"
    etapa: str = "na fila"
    feitos: int = 0
    total: int = 0
    captura: Captura | None = None
    erro: str = ""
    terminou_em: float | None = None
    tarefa: asyncio.Task[None] | None = field(default=None, repr=False)


class Capturas:
    def __init__(
        self,
        http: Http,
        provider: LLMProvider | None,
        simultaneas: int = 2,
        validade: float = 3600.0,
        relogio: Callable[[], float] = time.monotonic,
    ) -> None:
        self._http = http
        self._provider = provider
        # Cada Constituição segura alguns MB e ocupa o Gemini por um minuto:
        # duas por vez atendem quem testa sem deixar o processador cair (K33).
        self._simultaneas = simultaneas
        self._validade = validade
        self._relogio = relogio
        self._andamentos: dict[str, _Andamento] = {}

    def iniciar(self, link: str, dono: str) -> str:
        self._varrer()
        try:
            fonte = fonte_do_link(link)
        except FonteInvalida as exc:
            raise LinkDeLeiInvalido(str(exc)) from exc
        rodando = sum(1 for a in self._andamentos.values() if a.estado == "rodando")
        if rodando >= self._simultaneas:
            raise CapturasDemais("outras capturas estão rodando; tente de novo em um minuto")

        cid = secrets.token_urlsafe(16)
        andamento = _Andamento(dono=dono)
        self._andamentos[cid] = andamento
        andamento.tarefa = asyncio.get_running_loop().create_task(
            self._rodar(cid, andamento, fonte)
        )
        return cid

    async def _rodar(self, cid: str, andamento: _Andamento, fonte: object) -> None:
        def progresso(etapa: str, feitos: int, total: int) -> None:
            andamento.etapa, andamento.feitos, andamento.total = etapa, feitos, total

        try:
            andamento.captura = await capturar(
                fonte,  # type: ignore[arg-type]
                self._http,
                self._provider,
                progresso,
            )
            andamento.estado = "pronta"
        except Exception:
            # Qualquer exceção termina a captura (K32): "rodando" para sempre
            # deixaria a tela esperando sem fim. O detalhe vai para o log, não
            # para a tela.
            _log.exception("captura de lei falhou", extra={"stage": "leis", "captura": cid})
            andamento.estado = "falhou"
            andamento.erro = _ERRO_INTERNO
        finally:
            andamento.terminou_em = self._relogio()

    def consultar(self, cid: str, dono: str) -> dict[str, object]:
        self._varrer()
        andamento = self._andamentos.get(cid)
        # A de outra conta responde como inexistente (K31): nem confirmar que o
        # id existe.
        if andamento is None or andamento.dono != dono:
            raise CapturaNaoEncontrada("captura não encontrada ou expirada")
        corpo: dict[str, object] = {
            "id": cid,
            "estado": andamento.estado,
            "etapa": andamento.etapa,
            "progresso": {"feitos": andamento.feitos, "total": andamento.total},
        }
        if andamento.erro:
            corpo["erro"] = andamento.erro
        if andamento.captura is not None:
            corpo["resultado"] = _resultado(andamento.captura)
        return corpo

    def _varrer(self) -> None:
        """A captura terminada vale por `validade` segundos (K37)."""
        agora = self._relogio()
        vencidas = [
            cid
            for cid, a in self._andamentos.items()
            if a.terminou_em is not None and agora - a.terminou_em > self._validade
        ]
        for cid in vencidas:
            del self._andamentos[cid]


def _resultado(c: Captura) -> dict[str, object]:
    return {
        "fonte": c.fonte,
        "gemini": c.gemini,
        "versao": c.versao,
        "originalSha256": c.original_sha256,
        "paragrafos": c.paragrafos,
        "publicavel": c.publicavel,
        "dispositivos": c.dispositivos,
        "bloqueios": c.bloqueios,
        "avisos": [{"id": a.id, "texto": a.texto, "trecho": a.trecho} for a in c.avisos],
        "resumo": c.resumo,
    }
