"""Baixar a lei da fonte oficial, guardando os bytes exatos que chegaram.

A fonte sai do link que a pessoa cola na tela. Como é o servidor que baixa, só
passa link de fonte oficial (governo, legislativo, judiciário), por https, sem
porta nem credencial — e um redirecionamento para fora delas não é seguido.
"""

from __future__ import annotations

import hashlib
import ipaddress
import json
import re
import urllib.error
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from typing import Literal
from urllib.parse import urlparse

from app.leis.limpeza import texto_visivel


class FonteInvalida(Exception):
    """A fonte não entregou a lei: o motivo vai para o captura.md."""


@dataclass(frozen=True)
class Resposta:
    status: int
    tipo: str
    corpo: bytes


Http = Callable[[str], Resposta]


@dataclass(frozen=True)
class Original:
    url: str
    # O endereço que uma pessoa abre no navegador. Na Casa Civil de Goiás é
    # outro que o da API de onde o texto veio.
    url_publica: str
    bruto: bytes
    sha256: str
    html: str | None
    pdf: bool
    extensao: str


_API_GO = "https://legisla.casacivil.go.gov.br/api/v2/pesquisa/legislacoes/{id}"
_PAGINA_GO = "https://legisla.casacivil.go.gov.br/pesquisa_legislacao/{id}"
_HOST_GO = "legisla.casacivil.go.gov.br"
_ID_GO = re.compile(r"^/(?:pesquisa_legislacao|api/v2/pesquisa/legislacoes)/(\d+)(?:/|$)")
_TEM_ARTIGO = re.compile(r"\bArt\.?\s*\d")
# Os domínios que publicam lei. Um sufixo, não uma lista de sites: a norma do
# TCE-GO está num portal, a de um tribunal em outro, e todos são .gov.br ou
# .jus.br.
_OFICIAIS = (".gov.br", ".leg.br", ".jus.br")

TipoDeFonte = Literal["planalto", "casacivil-go", "link"]


@dataclass(frozen=True)
class Fonte:
    tipo: TipoDeFonte
    # De onde o texto vem. Na Casa Civil de Goiás é a API, não a página.
    url: str
    # O endereço que uma pessoa abre no navegador.
    url_publica: str


def _host_oficial(link: str) -> str:
    """O host do link, se ele puder ser baixado pelo servidor; senão, o motivo."""
    try:
        partes = urlparse(link.strip())
        porta = partes.port
    except ValueError as exc:
        raise FonteInvalida(f"link inválido: {link!r}") from exc
    if partes.scheme != "https":
        raise FonteInvalida("o link precisa começar com https://")
    host = (partes.hostname or "").lower()
    if not host or partes.username or partes.password or porta not in (None, 443):
        raise FonteInvalida(f"link inválido: {link!r}")
    try:
        ipaddress.ip_address(host)
    except ValueError:
        pass
    else:
        raise FonteInvalida("use o endereço da fonte oficial, não um IP")
    if not host.endswith(_OFICIAIS):
        raise FonteInvalida(
            f"{host} não é uma fonte oficial: use o link do Planalto, da Casa Civil de "
            "Goiás ou de outro site .gov.br, .leg.br ou .jus.br"
        )
    return host


def fonte_do_link(link: str) -> Fonte:
    host = _host_oficial(link)
    partes = urlparse(link.strip())
    if host == _HOST_GO:
        m = _ID_GO.match(partes.path)
        if not m:
            raise FonteInvalida(
                "da Casa Civil de Goiás, use o link da lei "
                "(legisla.casacivil.go.gov.br/pesquisa_legislacao/<número>/…)"
            )
        return Fonte("casacivil-go", _API_GO.format(id=m[1]), _PAGINA_GO.format(id=m[1]))
    url = partes._replace(fragment="").geturl()
    if host == "planalto.gov.br" or host.endswith(".planalto.gov.br"):
        return Fonte("planalto", url, url)
    return Fonte("link", url, url)


def redirecionamento_permitido(destino: str) -> bool:
    try:
        _host_oficial(destino)
    except FonteInvalida:
        return False
    return True


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


def decodificar_html(bruto: bytes) -> str:
    # O Planalto publica em Windows-1252 e nem sempre declara. UTF-8 estrito
    # falha nos acentos de um arquivo 1252, então tentar UTF-8 primeiro não
    # confunde os dois.
    try:
        return bruto.decode("utf-8")
    except UnicodeDecodeError:
        return bruto.decode("cp1252", errors="replace")


def baixar(fonte: Fonte, http: Http) -> Original:
    resposta = http(fonte.url)
    if resposta.status != 200:
        raise FonteInvalida(f"a fonte respondeu {resposta.status} para {fonte.url}")
    bruto = resposta.corpo
    sha = hashlib.sha256(bruto).hexdigest()

    if fonte.tipo == "casacivil-go":
        try:
            dados = json.loads(bruto)
        except json.JSONDecodeError as exc:
            raise FonteInvalida("a API de Goiás não respondeu JSON") from exc
        conteudo = dados.get("conteudo") if isinstance(dados, dict) else None
        if not isinstance(conteudo, str) or not conteudo.strip():
            raise FonteInvalida("a API de Goiás respondeu sem `conteudo`")
        html = conteudo
        extensao = "json"
    elif bruto.startswith(b"%PDF") or "pdf" in resposta.tipo.lower():
        return Original(fonte.url, fonte.url_publica, bruto, sha, None, True, "pdf")
    else:
        html = decodificar_html(bruto)
        extensao = "htm"

    if not _TEM_ARTIGO.search(texto_visivel(html)):
        raise FonteInvalida(f"a página baixada não tem nenhum artigo: {fonte.url}")
    return Original(fonte.url, fonte.url_publica, bruto, sha, html, False, extensao)
