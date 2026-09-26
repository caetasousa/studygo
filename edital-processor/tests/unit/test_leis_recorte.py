"""Pesquisa pelo tópico e captura do recorte — cada teste cita o id de
app/leis/README.md que cobre.

O HTTP é uma função que responde por URL: o que não está no mapa é 404, como
um endereço do Planalto que não existe.
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest

from app.leis.captura import capturar
from app.leis.fontes import Resposta, descobrir_fonte, fonte_do_link
from app.leis.pesquisa import pesquisar

PLANALTO = "https://www.planalto.gov.br/ccivil_03"
CF = f"{PLANALTO}/constituicao/constituicao.htm"
LGPD = f"{PLANALTO}/_ato2015-2018/2018/lei/l13709.htm"
MARCO = f"{PLANALTO}/_ato2011-2014/2014/lei/l12965.htm"
BUSCA_GO = "https://legisla.casacivil.go.gov.br/api/v2/pesquisa/legislacoes?numero="

LEI = """<html><body>
<p>LEI Nº 1, DE 1º DE JANEIRO DE 2020</p>
<p>CAPÍTULO I<br>DISPOSIÇÕES PRELIMINARES</p>
<p>Art. 1º Esta Lei dispõe sobre o teste.</p>
<p>CAPÍTULO II<br>DO CONTROLE</p>
<p>Art. 2º Compete ao órgão:</p>
<p>I - julgar;</p>
<p>II - apreciar.</p>
<p>Art. 3º O controle é externo.</p>
<p>CAPÍTULO III<br>DAS DISPOSIÇÕES FINAIS</p>
<p>Art. 4º Esta Lei entra em vigor na data de sua publicação.</p>
<p>Brasília, 1º de janeiro de 2020.</p>
</body></html>"""

ARTIGO = b"<html><p>Art. 1\xba Texto.</p></html>"


def _http(mapa: dict[str, Any]) -> Any:
    chamadas: list[str] = []

    def http(url: str) -> Resposta:
        chamadas.append(url)
        corpo = mapa.get(url)
        if corpo is None:
            return Resposta(404, "text/html", b"")
        if isinstance(corpo, (dict, list)):
            return Resposta(200, "application/json", json.dumps(corpo).encode())
        return Resposta(
            200, "text/html", corpo if isinstance(corpo, bytes) else corpo.encode("cp1252")
        )

    http.chamadas = chamadas  # type: ignore[attr-defined]
    return http


def _go(*leis: dict[str, object]) -> dict[str, object]:
    return {"total_resultados": len(leis), "resultados": list(leis)}


def _lei_go(id_: int, numero: str, ano: int, tipo: str) -> dict[str, object]:
    return {"id": id_, "numero": numero, "ano": ano, "tipo_legislacao": {"nome": tipo}}


# ---------------------------------------------------------------- K38, K39: a fonte


def test_k38_constituicao_federal_pelo_nome() -> None:
    http = _http({CF: ARTIGO})
    tema = "Constituição da República Federativa do Brasil de 1988: Administração Pública"
    fonte = descobrir_fonte(tema, http)
    assert fonte is not None and fonte.url == CF


@pytest.mark.parametrize(
    ("tema", "url"),
    [
        ("Lei Geral de Proteção de Dados Pessoais (Lei nº 13.709/2018): minimização", LGPD),
        ("Marco Civil da Internet (Lei nº 12.965/2014): princípios", MARCO),
        ("Lei nº 12.965, de 23 de abril de 2014", MARCO),
    ],
)
def test_k38_lei_federal_pelo_numero_e_ano(tema: str, url: str) -> None:
    http = _http({url: ARTIGO})
    fonte = descobrir_fonte(tema, http)
    assert fonte is not None and fonte.url == url
    # Conferiu que o endereço existe antes de dizer que achou.
    assert url in http.chamadas


def test_k38_lei_federal_que_o_planalto_nao_tem_e_nao_achei() -> None:
    assert descobrir_fonte("Lei nº 13.709/2018", _http({})) is None


def test_k38_lei_estadual_de_goias_pela_casa_civil() -> None:
    http = _http({BUSCA_GO + "16168": _go(_lei_go(86708, "16.168", 2007, "Lei Ordinária"))})
    tema = (
        "Lei Orgânica do Tribunal de Contas do Estado de Goiás "
        "(Lei Estadual nº 16.168, de 11/12/2007)"
    )
    fonte = descobrir_fonte(tema, http)
    assert fonte is not None and fonte.tipo == "casacivil-go"
    assert fonte.url_publica.endswith("/pesquisa_legislacao/86708")
    # Estadual não é procurada no Planalto.
    assert not any("planalto" in u for u in http.chamadas)


def test_k38_complementar_nao_e_ordinaria_e_o_ano_decide() -> None:
    http = _http(
        {
            BUSCA_GO + "205": _go(
                _lei_go(1, "205", 1990, "Lei Ordinária"),
                _lei_go(2, "205", 2025, "Lei Complementar"),
                _lei_go(3, "205", 2001, "Lei Complementar"),
            )
        }
    )
    fonte = descobrir_fonte("Lei Complementar estadual nº 205, de 19 de maio de 2025", http)
    assert fonte is not None and fonte.url_publica.endswith("/pesquisa_legislacao/2")


def test_k38_estadual_ambigua_e_nao_achei() -> None:
    http = _http(
        {
            BUSCA_GO + "205": _go(
                _lei_go(2, "205", 2025, "Lei Complementar"),
                _lei_go(3, "205", 2001, "Lei Complementar"),
            )
        }
    )
    # Sem ano, duas complementares 205: não chuta.
    assert descobrir_fonte("Lei Complementar estadual nº 205", http) is None


@pytest.mark.parametrize(
    "tema",
    [
        "Constituição do Estado de Goiás: disposições relativas ao Tribunal de Contas",
        "Resolução Administrativa nº 17/2024, que dispõe sobre a Política de Segurança",
        "Certificação digital e sua aplicação em sistemas informatizados",
        "Lei nº 13.709",
    ],
)
def test_k39_sem_fonte_conhecida_e_nao_achei(tema: str) -> None:
    assert descobrir_fonte(tema, _http({})) is None


# ---------------------------------------------------------------- pesquisa


def test_pesquisa_devolve_a_estrutura_para_ler_o_topico() -> None:
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"
    p = asyncio.run(pesquisar("Lei nº 1: controle", link, _http({link: LEI})))
    assert p.fonte == link
    assert p.epigrafe.startswith("LEI Nº 1")
    assert p.estrutura[0]["texto"].startswith("LEI Nº 1")  # a epígrafe vai inteira
    tipos = {d["ref"]: d["tipo"] for d in p.estrutura}
    assert tipos == {
        "preambulo1": "preambulo",
        "cap1": "capitulo",
        "art1": "artigo",
        "cap2": "capitulo",
        "art2": "artigo",
        "art3": "artigo",
        "cap3": "capitulo",
        "art4": "artigo",
    }


# ---------------------------------------------------------------- K40–K44: a captura do recorte


def _capturar(recorte: list[str] | None, provider: object = None) -> Any:
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"
    return asyncio.run(
        capturar(fonte_do_link(link), _http({link: LEI}), provider, recorte=recorte)  # type: ignore[arg-type]
    )


def _refs(c: Any) -> list[str]:
    assert c.dispositivos is not None, c.bloqueios
    return [d["ref"] for d in c.dispositivos]


def test_k40_k41_recorte_guarda_a_divisao_inteira_e_os_pais() -> None:
    c = _capturar(["cap2"])
    assert _refs(c) == ["preambulo1", "cap2", "art2", "art2.inc1", "art2.inc2", "art3"]
    assert c.recorte == ["cap2"]


def test_k40_artigo_avulso_leva_o_capitulo_acima() -> None:
    assert _refs(_capturar(["art3"])) == ["preambulo1", "cap2", "art3"]


def test_k41_sem_recorte_e_a_lei_inteira() -> None:
    assert "art4" in _refs(_capturar(None))


def test_k42_ref_que_a_lei_nao_tem_bloqueia() -> None:
    c = _capturar(["art2", "art99"])
    assert any("art99" in b for b in c.bloqueios)
    assert not c.publicavel


def test_k43_o_gemini_confere_so_o_recorte() -> None:
    enviados: list[str] = []

    class Conta:
        def available(self) -> bool:
            return True

        async def extract_structured(self, request: Any) -> dict[str, object]:
            itens = []
            for chunk in request.chunks:
                pid, texto = chunk.split(": ", 1)
                enviados.append(texto)
                itens.append({"id": pid, "tipo": _tipo(texto)})
            return {"itens": itens}

    _capturar(["cap2"], Conta())
    assert any(t.startswith("Art. 2º") for t in enviados)
    assert not any(t.startswith(("Art. 1º", "Art. 4º")) for t in enviados)


def _tipo(texto: str) -> str:
    if texto.startswith("Art."):
        return "artigo"
    if texto.startswith("CAPÍTULO"):
        return "capitulo"
    if texto[:3] in ("I -", "II "):
        return "inciso"
    return "solto"


def test_k44_aviso_de_fora_do_recorte_nao_aparece() -> None:
    lei = LEI.replace("Art. 1º Esta Lei", "Art. 5º Esta Lei")  # salto no capítulo I
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"
    fora = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None, recorte=["cap3"]))
    assert not [a for a in fora.avisos if a.id.startswith("salto")]
    inteira = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None))
    assert [a for a in inteira.avisos if a.id.startswith("salto")]


# ---------------------------------------------------------------- K45, K46

RESOLUCAO_COM_ANEXO = """<html><body>
<p>RESOLUÇÃO Nº 22, DE 4 DE SETEMBRO DE 2008</p>
<p>O TRIBUNAL DE CONTAS, no uso de suas atribuições, RESOLVE</p>
<p>Art. 1º Aprovar o Regimento Interno, cujo teor consta do anexo.</p>
<p>Art. 2º Esta Resolução entra em vigor na data de sua publicação.</p>
<p>REGIMENTO INTERNO</p>
<p>TÍTULO I<br>DA NATUREZA</p>
<p>Art. 1º O Tribunal de Contas é órgão de controle externo.</p>
<p>Art. 2º O controle externo é exercido com o auxílio do Tribunal.</p>
<p>TÍTULO II<br>DA ORGANIZAÇÃO</p>
<p>Art. 3º O Tribunal tem sede na Capital.</p>
</body></html>"""


def test_k45_anexo_que_recomeca_a_numeracao_tem_refs_proprias() -> None:
    link = "https://gnoi.tce.go.gov.br/atoNormativo/Publicado/1"
    c = asyncio.run(capturar(fonte_do_link(link), _http({link: RESOLUCAO_COM_ANEXO}), None))
    assert c.bloqueios == []
    refs = [d["ref"] for d in c.dispositivos or []]
    # O regimento é o texto que se estuda: fica com as refs limpas (art1…);
    # os artigos da resolução que o aprova ganham o prefixo.
    assert "resolucao.art1" in refs and "resolucao.art2" in refs
    assert "tit1" in refs and "art1" in refs and "art3" in refs
    assert refs.index("resolucao.art2") < refs.index("art1")


def test_k46_problema_fora_do_recorte_nao_bloqueia() -> None:
    # Uma alínea sem inciso no capítulo III é problema — mas só o capítulo II
    # foi pedido.
    lei = LEI.replace(
        "<p>Art. 4º Esta Lei entra em vigor na data de sua publicação.</p>",
        "<p>Art. 4º Esta Lei entra em vigor.</p><p>a) na data da publicação.</p>",
    )
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"
    fora = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None, recorte=["cap2"]))
    assert fora.bloqueios == [], fora.bloqueios
    assert fora.publicavel
    inteira = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None))
    assert inteira.bloqueios
    dentro = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None, recorte=["cap3"]))
    assert dentro.bloqueios


RESOLUCAO_COM_DOIS_ANEXOS = """<html><body>
<p>RESOLUÇÃO ADMINISTRATIVA Nº 1/2014</p>
<p>Art. 1º Ficam aprovados os Códigos de Ética dos anexos I e II.</p>
<p>Art. 2º Esta Resolução entra em vigor na data de sua publicação.</p>
<p>ANEXO I</p>
<p>CAPÍTULO I<br>DOS MEMBROS</p>
<p>Art. 1º Este Código se aplica aos Conselheiros.</p>
<p>ANEXO II</p>
<p>CAPÍTULO I<br>DOS SERVIDORES</p>
<p>Art. 1º Este Código se aplica aos servidores.</p>
<p>Art. 2º O servidor zela pela ética.</p>
</body></html>"""


def test_k45b_dois_anexos_tem_refs_proprias() -> None:
    link = "https://gnoi.tce.go.gov.br/atoNormativo/Publicado/2"
    http = _http({link: RESOLUCAO_COM_DOIS_ANEXOS})
    c = asyncio.run(capturar(fonte_do_link(link), http, None))
    assert c.bloqueios == [], c.bloqueios
    refs = [d["ref"] for d in c.dispositivos or []]
    assert {"resolucao.art1", "anexo1.cap1", "anexo1.art1", "anexo2.cap1", "anexo2.art1"} <= set(
        refs
    )
    # O Anexo II sozinho, como o edital pede.
    so = asyncio.run(capturar(fonte_do_link(link), http, None, recorte=["anexo2.cap1"]))
    assert [d["ref"] for d in so.dispositivos or []] == [
        "preambulo1",
        "anexo2.cap1",
        "anexo2.art1",
        "anexo2.art2",
    ]


def test_k47_descartado_com_o_mesmo_texto_de_um_paragrafo_do_corpo() -> None:
    # O portal do TCE-GO anota a publicação no cabeçalho e no corpo com o
    # mesmo texto; só a do cabeçalho é descartada.
    vide = (
        '<p><a href="https://gnoi.tce.go.gov.br/atoNormativo/Publicado/9">-Vide Portaria 830</a>,'
        ' <a href="https://dec.tce.go.gov.br/d">DEC 2-12-2024.</a></p>'
    )
    ato = """<html><body>
<p>RESOLUÇÃO ADMINISTRATIVA Nº 3/2024</p>
{VIDE}
<p>CAPÍTULO I<br>DAS DISPOSIÇÕES GERAIS</p>
<p>Art. 1º Esta Resolução institui a política.</p>
{VIDE}
<p>Art. 2º Esta Resolução entra em vigor na data de sua publicação.</p>
</body></html>""".replace("{VIDE}", vide)
    link = "https://gnoi.tce.go.gov.br/atoNormativo/Publicado/3"
    c = asyncio.run(capturar(fonte_do_link(link), _http({link: ato}), None))
    assert c.bloqueios == [], c.bloqueios


def test_k48_rotulo_repetido_na_fonte_vira_aviso_e_ref_propria() -> None:
    lei = LEI.replace(
        "<p>CAPÍTULO II<br>DO CONTROLE</p>",
        "<p>CAPÍTULO II<br>DO CONTROLE</p><p>Seção I<br>Da jornada</p>",
    ).replace(
        "<p>Art. 3º O controle é externo.</p>",
        "<p>Seção I<br>Da frequência</p><p>Art. 3º O controle é externo.</p>",
    )
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0002.htm"
    c = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None))
    assert c.bloqueios == [], c.bloqueios
    por_ref = {d["ref"]: d for d in c.dispositivos or []}
    assert por_ref["cap2.sec1"]["nome"] == "Da jornada"
    assert por_ref["cap2.sec1-2"]["nome"] == "Da frequência"
    assert por_ref["art3"]["pai"] == "cap2.sec1-2"
    # A pessoa confere: pode ser erro da fonte, pode ser um sumário lido como corpo.
    assert "repetido: cap2.sec1" in [a.id for a in c.avisos]


def test_k49_alineas_sob_o_caput_que_as_anuncia() -> None:
    lei = LEI.replace(
        "<p>Art. 3º O controle é externo.</p>",
        "<p>Art. 3º Ao servidor são assegurados os seguintes direitos:</p>"
        "<p>a) de ser representado;</p><p>b) de se associar.</p>",
    )
    link = "https://www.planalto.gov.br/ccivil_03/leis/l0003.htm"
    c = asyncio.run(capturar(fonte_do_link(link), _http({link: lei}), None))
    assert c.bloqueios == [], c.bloqueios
    por_ref = {d["ref"]: d for d in c.dispositivos or []}
    assert por_ref["art3.alia"]["pai"] == "art3" and por_ref["art3.alib"]["pai"] == "art3"
    assert [a.id for a in c.avisos if a.id.startswith("alinea")] == ["alinea-no-caput: art3"]
    # Sem os dois-pontos que anunciam a lista, alínea sob artigo continua errada.
    torta = LEI.replace(
        "<p>Art. 3º O controle é externo.</p>",
        "<p>Art. 3º O controle é externo.</p><p>a) de ser representado.</p>",
    )
    c = asyncio.run(capturar(fonte_do_link(link), _http({link: torta}), None))
    assert c.bloqueios
