"""Baixar a lei da fonte pública, guardando os bytes exatos que chegaram."""

from __future__ import annotations

import hashlib
import json
import re
import urllib.error
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from urllib.parse import urlparse

from app.leis.catalogo import Norma
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
_TEM_ARTIGO = re.compile(r"\bArt\.?\s*\d")


def http_padrao(url: str) -> Resposta:
    pedido = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (studygo; leis)"})
    try:
        with urllib.request.urlopen(pedido, timeout=60) as r:
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


def _url(norma: Norma) -> tuple[str, str]:
    if norma.fonte == "casacivil-go":
        if not re.fullmatch(r"\d+", norma.link):
            raise FonteInvalida(f"casacivil-go espera o id numérico da lei, não {norma.link!r}")
        return _API_GO.format(id=norma.link), _PAGINA_GO.format(id=norma.link)
    partes = urlparse(norma.link)
    if partes.scheme != "https" or not partes.hostname:
        raise FonteInvalida(f"link inválido: {norma.link!r} (precisa ser https)")
    if norma.fonte == "planalto" and not (
        partes.hostname == "planalto.gov.br" or partes.hostname.endswith(".planalto.gov.br")
    ):
        raise FonteInvalida(f"a fonte planalto só aceita links de planalto.gov.br: {norma.link}")
    return norma.link, norma.link


def baixar(norma: Norma, http: Http) -> Original:
    if not norma.link:
        raise FonteInvalida("a norma ainda não tem link em normas.toml")
    url, publica = _url(norma)
    resposta = http(url)
    if resposta.status != 200:
        raise FonteInvalida(f"a fonte respondeu {resposta.status} para {url}")
    bruto = resposta.corpo
    sha = hashlib.sha256(bruto).hexdigest()

    if norma.fonte == "casacivil-go":
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
        return Original(url, publica, bruto, sha, None, True, "pdf")
    else:
        html = decodificar_html(bruto)
        extensao = "htm"

    if not _TEM_ARTIGO.search(texto_visivel(html)):
        raise FonteInvalida(f"a página baixada não tem nenhum artigo: {url}")
    return Original(url, publica, bruto, sha, html, False, extensao)
