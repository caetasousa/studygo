"""Baixar a lei da fonte oficial, guardando os bytes exatos que chegaram, e
achar a fonte pelo tópico do edital.

Tudo pela porta das fontes (port/fontes.py): o que vale como fonte oficial é do
domínio (domain/leis/fontes.py).
"""

from __future__ import annotations

import hashlib
import json
import re

from app.domain.leis.fontes import (
    API_GO,
    PAGINA_GO,
    TEM_ARTIGO,
    Fonte,
    FonteInvalida,
    Original,
    decodificar_html,
)
from app.domain.leis.limpeza import texto_visivel
from app.port.fontes import Http


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

    if not TEM_ARTIGO.search(texto_visivel(html)):
        raise FonteInvalida(f"a página baixada não tem nenhum artigo: {fonte.url}")
    return Original(fonte.url, fonte.url_publica, bruto, sha, html, False, extensao)


# --- descobrir a fonte pelo tópico do edital -----------------------------------

_PLANALTO = "https://www.planalto.gov.br/ccivil_03"
_CF = f"{_PLANALTO}/constituicao/constituicao.htm"
_BUSCA_GO = "https://legisla.casacivil.go.gov.br/api/v2/pesquisa/legislacoes?numero={numero}"
# O Planalto guarda as leis de 2004 em diante em pastas de quatro em quatro anos.
_PASTAS_DO_PLANALTO = (
    (2004, 2006),
    (2007, 2010),
    (2011, 2014),
    (2015, 2018),
    (2019, 2022),
    (2023, 2026),
)

_CONSTITUICAO_FEDERAL = re.compile(
    r"constitui[çc][ãa]o\s+(da\s+rep[úu]blica\s+federativa\s+do\s+brasil|federal)", re.I
)
_LEI = re.compile(
    r"\blei\s+(?P<complementar>complementar\s+)?(?:(?P<estadual>estadual|federal)\s+)?"
    r"n[º°o.]*\s*(?P<numero>\d{1,3}(?:\.\d{3})*|\d+)"
    r"(?:\s*/\s*(?P<ano_barra>\d{4})|[^()]*?\bde\s+(?:\d{1,2}[º°]?\s+de\s+\w+\s+de\s+|\d{1,2}/\d{1,2}/)(?P<ano_data>\d{4}))?",
    re.I,
)
_DECRETO = re.compile(
    r"\bdecreto\s+(?:federal\s+)?n[º°o.]*\s*(?P<numero>\d{1,3}(?:\.\d{3})*|\d+)"
    r"(?:\s*/\s*(?P<ano_barra>\d{4})|[^()]*?\bde\s+(?:\d{1,2}[º°]?\s+de\s+\w+\s+de\s+|\d{1,2}/\d{1,2}/)(?P<ano_data>\d{4}))?",
    re.I,
)
_ESTADUAL = re.compile(r"\bestadua(l|is)\b|estado de goi[áa]s|\bgoi[áa]s\b", re.I)


def _existe(url: str, http: Http) -> bool:
    resposta = http(url)
    return resposta.status == 200 and bool(
        TEM_ARTIGO.search(texto_visivel(decodificar_html(resposta.corpo)))
    )


def _federal(numero: int, ano: int | None, complementar: bool, http: Http) -> Fonte | None:
    if complementar:
        candidatos = [f"{_PLANALTO}/leis/lcp/lcp{numero}.htm"]
    elif ano is None:
        return None  # sem o ano, a pasta do Planalto é chute (K38)
    elif ano < 2004:
        candidatos = [f"{_PLANALTO}/leis/l{numero}.htm", f"{_PLANALTO}/leis/l{numero}cons.htm"]
    else:
        pasta = next((p for p in _PASTAS_DO_PLANALTO if p[0] <= ano <= p[1]), None)
        if pasta is None:
            return None
        candidatos = [f"{_PLANALTO}/_ato{pasta[0]}-{pasta[1]}/{ano}/lei/l{numero}.htm"]
    for url in candidatos:
        if _existe(url, http):
            return Fonte("planalto", url, url)
    return None


def _decreto_federal(numero: int, ano: int | None, http: Http) -> Fonte | None:
    # Só a pasta por período (2004 em diante) é previsível; antes, cada ano
    # tem a sua convenção no Planalto.
    pasta = next((p for p in _PASTAS_DO_PLANALTO if ano and p[0] <= ano <= p[1]), None)
    if pasta is None:
        return None
    base = f"{_PLANALTO}/_ato{pasta[0]}-{pasta[1]}/{ano}/decreto"
    for url in (f"{base}/D{numero}.htm", f"{base}/d{numero}.htm"):
        if _existe(url, http):
            return Fonte("planalto", url, url)
    return None


def _goias(numero: int, ano: int | None, complementar: bool, http: Http) -> Fonte | None:
    resposta = http(_BUSCA_GO.format(numero=numero))
    if resposta.status != 200:
        return None
    try:
        dados = json.loads(resposta.corpo)
    except json.JSONDecodeError:
        return None
    achadas = []
    for lei in dados.get("resultados", []) if isinstance(dados, dict) else []:
        tipo = str((lei.get("tipo_legislacao") or {}).get("nome", "")).lower()
        if re.sub(r"\D", "", str(lei.get("numero", ""))) != str(numero):
            continue
        if complementar != ("complementar" in tipo) or not tipo.startswith("lei"):
            continue
        if ano is not None and lei.get("ano") != ano:
            continue
        achadas.append(lei)
    # Mais de uma que serve (sem ano, duas complementares 205): não chuta (K38).
    if len(achadas) != 1:
        return None
    id_ = achadas[0]["id"]
    return Fonte("casacivil-go", API_GO.format(id=id_), PAGINA_GO.format(id=id_))


def descobrir_fonte(tema: str, http: Http) -> Fonte | None:
    """A fonte oficial da norma que o tópico cita, ou None se não dá para ter
    certeza (K39): aí quem estuda cola o link."""
    if _CONSTITUICAO_FEDERAL.search(tema):
        return Fonte("planalto", _CF, _CF) if _existe(_CF, http) else None
    m = _LEI.search(tema)
    d = _DECRETO.search(tema)
    # O que o tópico cita primeiro é a norma dele: "Decreto nº X, que
    # regulamenta a Lei nº Y" é o decreto.
    if d and (not m or d.start() < m.start()):
        if _ESTADUAL.search(tema):
            return None  # decreto de Goiás: a Casa Civil não é consultada por ele
        ano_d = d["ano_barra"] or d["ano_data"]
        return _decreto_federal(
            int(d["numero"].replace(".", "")), int(ano_d) if ano_d else None, http
        )
    if not m:
        return None
    numero = int(m["numero"].replace(".", ""))
    ano_txt = m["ano_barra"] or m["ano_data"]
    ano = int(ano_txt) if ano_txt else None
    complementar = bool(m["complementar"])
    estadual = (m["estadual"] or "").lower() == "estadual" or bool(_ESTADUAL.search(tema))
    if estadual:
        return _goias(numero, ano, complementar, http)
    return _federal(numero, ano, complementar, http)
