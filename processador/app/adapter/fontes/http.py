"""O GET das fontes oficiais de lei, por urllib.

O redirecionamento só é seguido se o destino também é fonte oficial: o link é
de quem estuda, e é o servidor que baixa.
"""

from __future__ import annotations

import urllib.error
import urllib.request

from app.domain.leis.fontes import FonteInvalida, redirecionamento_permitido
from app.port.fontes import Resposta


class _SoFontesOficiais(urllib.request.HTTPRedirectHandler):
    def redirect_request(  # type: ignore[no-untyped-def]
        self, req, fp, code, msg, headers, newurl
    ):
        if not redirecionamento_permitido(newurl):
            raise FonteInvalida(f"a fonte redirecionou para fora das fontes oficiais: {newurl}")
        return super().redirect_request(req, fp, code, msg, headers, newurl)


_abrir = urllib.request.build_opener(_SoFontesOficiais()).open


def http_padrao(url: str) -> Resposta:
    pedido = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (studygo; leis)"})
    try:
        with _abrir(pedido, timeout=60) as r:
            return Resposta(r.status, r.headers.get("Content-Type", ""), r.read())
    except urllib.error.HTTPError as exc:
        return Resposta(exc.code, exc.headers.get("Content-Type", ""), b"")
