"""A ponte com o backend: entregar o mapa, avisar a falha, pedir o redespacho.

O backend escuta numa porta interna (8081), que não é publicada nem passa pelo
nginx; a autenticação é o mesmo token de serviço que o backend usa para chamar
este processador. A biblioteca padrão basta (numa thread, para não travar o
event loop): são poucas chamadas, e o processador não ganha dependência nova.
"""

from __future__ import annotations

import asyncio
import json
import urllib.error
import urllib.request
import uuid
from dataclasses import asdict

from app.core.config import Settings
from app.core.logging import get_logger
from app.mapas.servico import Resposta, Saida

_log = get_logger("mapas")

# O backend pode estar reiniciando (deploy) bem na hora da entrega: o mapa de
# meia hora de Claude não se perde por isso.
_TENTATIVAS_DE_REDE = 10
_ESPERA_DE_REDE = 30.0


class Backend:
    def __init__(self, settings: Settings, espera: float = _ESPERA_DE_REDE) -> None:
        self._url = settings.backend_internal_url.rstrip("/")
        self._token = settings.service_token
        self._espera = espera

    async def entregar(self, pedido: str, saida: Saida) -> Resposta:
        corpo, tipo = _multipart(saida)
        return await self._com_insistencia(
            f"/internal/pedidos-de-mapa/{pedido}/resultado", corpo, tipo
        )

    async def avisar_falha(self, pedido: str, relatorio: str) -> None:
        corpo = json.dumps({"relatorio": relatorio[:19000]}).encode()
        resposta = await self._com_insistencia(
            f"/internal/pedidos-de-mapa/{pedido}/falha", corpo, "application/json"
        )
        if resposta.status >= 400 and resposta.status not in (404, 409):
            _log.warning("o backend não registrou a falha", extra={"stage": "mapas"})

    async def pedir_redespacho(self) -> None:
        """Ao subir: pede de volta o que estava processando antes do reinício."""
        if not self._url:
            return
        resposta = await self._com_insistencia(
            "/internal/pedidos-de-mapa/redespacho", b"{}", "application/json"
        )
        _log.info("redespacho pedido", extra={"stage": "mapas", "status": resposta.status})

    async def _com_insistencia(self, caminho: str, corpo: bytes, tipo: str) -> Resposta:
        ultima = Resposta(0, "o backend interno não está configurado (EP_BACKEND_INTERNAL_URL)")
        if not self._url:
            return ultima
        for _ in range(_TENTATIVAS_DE_REDE):
            try:
                return await asyncio.to_thread(self._post, caminho, corpo, tipo)
            except (urllib.error.URLError, OSError) as exc:
                ultima = Resposta(0, f"o backend não respondeu: {exc}")
                await asyncio.sleep(self._espera)
        return ultima

    def _post(self, caminho: str, corpo: bytes, tipo: str) -> Resposta:
        req = urllib.request.Request(
            self._url + caminho,
            data=corpo,
            method="POST",
            headers={"Content-Type": tipo, "Authorization": f"Bearer {self._token}"},
        )
        try:
            with urllib.request.urlopen(req, timeout=300) as r:
                return Resposta(r.status, r.read().decode(errors="replace"))
        except urllib.error.HTTPError as e:
            texto = e.read().decode(errors="replace")
            try:
                mensagem = str(json.loads(texto).get("erro") or texto)
            except ValueError:
                mensagem = texto[:500]
            return Resposta(e.code, mensagem)


def _multipart(saida: Saida) -> tuple[bytes, str]:
    """O resultado como o backend o lê: os dados em JSON e as imagens como arquivos."""
    fronteira = uuid.uuid4().hex
    dados = asdict(saida)
    dados.pop("imagens")
    partes = [
        (f'--{fronteira}\r\nContent-Disposition: form-data; name="dados"\r\n\r\n').encode()
        + json.dumps(dados, ensure_ascii=False).encode()
        + b"\r\n"
    ]
    for nome, conteudo in saida.imagens:
        partes.append(
            (
                f"--{fronteira}\r\n"
                f'Content-Disposition: form-data; name="imagens"; filename="{nome}"\r\n'
                "Content-Type: application/octet-stream\r\n\r\n"
            ).encode()
            + conteudo
            + b"\r\n"
        )
    corpo = b"".join(partes) + f"--{fronteira}--\r\n".encode()
    return corpo, f"multipart/form-data; boundary={fronteira}"
