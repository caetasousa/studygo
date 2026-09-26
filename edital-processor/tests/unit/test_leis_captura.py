"""Captura de leis — cada teste cita o id de app/leis/README.md que cobre.

Os trechos de HTML imitam o que o Planalto e a Casa Civil de Goiás publicam de
verdade (setembro de 2026): parágrafos partidos em linhas, `&nbsp;`, riscado
por `<strike>` e por estilo, notas como link.
"""

from __future__ import annotations

import asyncio
import json

import pytest

from app.leis.captura import Captura, capturar
from app.leis.classificar import Classe, combinar, por_gemini, por_regras
from app.leis.fontes import Fonte, FonteInvalida, Resposta, baixar, decodificar_html, fonte_do_link
from app.leis.limpeza import paragrafos_de_html, paragrafos_de_pdf, texto_visivel
from app.leis.montar import montar
from app.leis.verificar import verificar

PLANALTO = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"
GOIAS = "https://legisla.casacivil.go.gov.br/pesquisa_legislacao/{id}"


def _fonte(link: str = PLANALTO) -> Fonte:
    return fonte_do_link(link)


def _capturar(http: object, provider: object = None) -> Captura:
    return asyncio.run(capturar(_fonte(), http, provider))  # type: ignore[arg-type]


def _html(corpo: str) -> str:
    return f"<html><body>{corpo}</body></html>"


def _textos(corpo: str) -> list[str]:
    return [p.texto for p in paragrafos_de_html(_html(corpo)) if not p.anterior]


def _dispositivos(corpo: str) -> list[dict[str, object]]:
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert montagem.problemas == []
    return [d.model_dump() for d in montagem.dispositivos]


LEI_PEQUENA = """
<p>Presidência da República</p>
<p><a href="x">Texto compilado</a></p>
<p>LEI Nº 1, DE 1º DE JANEIRO DE 2020</p>
<p>O PRESIDENTE DA REPÚBLICA Faço saber que o Congresso Nacional decreta:</p>
<p>CAPÍTULO I<br>DISPOSIÇÕES PRELIMINARES</p>
<p><a name="art1"></a>Art.&nbsp;1º Esta Lei dispõe sobre
    o teste.</p>
<p>Parágrafo único. O teste é
    obrigatório.</p>
<p>Art. 2º Compete ao órgão:</p>
<p>I - julgar;</p>
<p>II – apreciar:</p>
<p>a) as contas;</p>
<p>b) os atos.</p>
<p>§ 1º Primeiro parágrafo.</p>
<p>§ 2º Segundo parágrafo.</p>
<p>Brasília, 1º de janeiro de 2020.</p>
"""


# ---------------------------------------------------------------- baixar


def test_k1_recusa_link_fora_do_dominio_da_fonte() -> None:
    with pytest.raises(FonteInvalida, match="fonte oficial"):
        _fonte("https://www.exemplo.com/lei.htm")


def test_k1_casacivil_go_so_consulta_a_api_do_estado() -> None:
    chamadas: list[str] = []

    def http(url: str) -> Resposta:
        chamadas.append(url)
        corpo = json.dumps({"conteudo": "<p>Art. 1º Texto.</p>"}).encode()
        return Resposta(200, "application/json", corpo)

    original = baixar(_fonte(GOIAS.format(id="86708")), http)
    assert chamadas == ["https://legisla.casacivil.go.gov.br/api/v2/pesquisa/legislacoes/86708"]
    assert original.html == "<p>Art. 1º Texto.</p>"
    with pytest.raises(FonteInvalida):
        _fonte(GOIAS.format(id="../86708"))


@pytest.mark.parametrize(
    ("resposta", "motivo"),
    [
        (Resposta(404, "text/html", b"<p>Art. 1</p>"), "404"),
        (Resposta(200, "text/html", b"<html><p>Erro interno</p></html>"), "nenhum artigo"),
    ],
)
def test_k2_recusa_resposta_que_nao_e_a_lei(resposta: Resposta, motivo: str) -> None:
    with pytest.raises(FonteInvalida, match=motivo):
        baixar(_fonte(), lambda url: resposta)


def test_k2_json_sem_conteudo_e_recusado() -> None:
    corpo = json.dumps({"ementa": "x"}).encode()
    with pytest.raises(FonteInvalida, match="conteudo"):
        baixar(
            _fonte(GOIAS.format(id="1")),
            lambda url: Resposta(200, "application/json", corpo),
        )


def test_k2_original_guarda_o_hash_dos_bytes_baixados() -> None:
    corpo = "<p>Art. 1º Função.</p>".encode("cp1252")
    original = baixar(_fonte(), lambda url: Resposta(200, "text/html", corpo))
    import hashlib

    assert original.sha256 == hashlib.sha256(corpo).hexdigest()
    assert original.bruto == corpo


def test_k3_windows_1252_decodificado_sem_mojibake() -> None:
    assert decodificar_html("<p>Art. 1º Função pública.</p>".encode("cp1252")) == (
        "<p>Art. 1º Função pública.</p>"
    )
    assert decodificar_html("<p>Art. 1º Função.</p>".encode()) == "<p>Art. 1º Função.</p>"


# ---------------------------------------------------------------- limpar


def test_k4_entidades_viram_texto() -> None:
    assert _textos("<p>Art.&nbsp;9&ordm; A&nbsp;&nbsp;lei.</p>") == ["Art. 9º A lei."]


def test_k5_linhas_partidas_viram_um_paragrafo_sem_colar_palavras() -> None:
    corpo = "<p>Art. 70. A fiscalização contábil, \n\tfinanceira, orçamentária.</p>"
    assert _textos(corpo) == ["Art. 70. A fiscalização contábil, financeira, orçamentária."]


@pytest.mark.parametrize(
    "riscado",
    [
        "<p><strike>I - texto antigo;</strike></p>",
        "<p><s>I - texto antigo;</s></p>",
        "<p><del>I - texto antigo;</del></p>",
        '<p><span style="color: black; text-decoration:line-through">I - texto antigo;</span></p>',
        '<p style="text-decoration: line-through">I - texto antigo;</p>',
        '<p class="inciso conteudo-revogado">I - texto antigo;</p>',
        '<p><strike><a name="x"></a> <span>I - texto antigo;</span> </strike></p>',
    ],
)
def test_k6_riscado_e_redacao_anterior(riscado: str) -> None:
    paragrafos = paragrafos_de_html(_html(riscado + "<p>I - texto novo;</p>"))
    assert [(p.texto, p.anterior) for p in paragrafos] == [
        ("I - texto antigo;", True),
        ("I - texto novo;", False),
    ]


def test_k6_o_riscado_nao_vaza_para_o_paragrafo_seguinte() -> None:
    # HTML real do Planalto fecha <strike> fora de ordem com frequência.
    corpo = "<p><strike>I - antigo;</p><p>I - novo;</p>"
    assert _textos(corpo) == ["I - novo;"]


def test_k6b_marca_de_situacao_sozinha_vira_nota() -> None:
    ds = _dispositivos("<p>Art. 1º A.</p><p>(Rejeitada)</p><p>Art. 2º B.</p>")
    assert [d["ref"] for d in ds] == ["art1", "art2"]
    assert ds[0]["notas"] == ["(Rejeitada)"]


def test_k6b_marca_de_situacao_fora_do_risco_nao_faz_o_riscado_vigente() -> None:
    corpo = (
        "<p><strike>VII - aplicações de internet - o conjunto;</strike> (Rejeitada) "
        '<a href="mpv.htm">(Redação dada pela Medida Provisória nº 1.068, de 2021)</a></p>'
        "<p>VII - aplicações de internet: o conjunto;</p>"
    )
    antigo, novo = paragrafos_de_html(_html(corpo))
    assert antigo.anterior
    assert antigo.texto == "VII - aplicações de internet - o conjunto;"
    assert antigo.notas == (
        "(Rejeitada)",
        "(Redação dada pela Medida Provisória nº 1.068, de 2021)",
    )
    assert not novo.anterior


@pytest.mark.parametrize(
    "corpo",
    [
        "<p>I - <strike>impostos sobre:</strike></p>",
        "<p>c<strike>) os nascidos no estrangeiro;</strike></p>",
        "<p>§ 2º <strike>O imposto previsto.</strike></p>",
        "<p>c) <strike>os nascidos no estrangeiro</strike>;</p>",
        "<p>A<strike>rt. 109. No caso de descumprimento.</strike></p>",
        "<p>Art. 155. <strike>Compete aos Estados.</strike></p>",
    ],
)
def test_k6c_rotulo_fora_do_risco_nao_faz_o_riscado_vigente(corpo: str) -> None:
    [p] = paragrafos_de_html(_html(corpo))
    assert p.anterior


def test_k7_nota_de_redacao_em_link_vira_nota() -> None:
    corpo = (
        "<p>Parágrafo único. Prestará contas qualquer pessoa.&nbsp;&nbsp;"
        '<a href="Emendas/emc19.htm">(Redação dada pela Emenda Constitucional nº 19, de 1998)'
        "</a></p>"
    )
    [p] = paragrafos_de_html(_html(corpo))
    assert p.texto == "Parágrafo único. Prestará contas qualquer pessoa."
    assert p.notas == ("(Redação dada pela Emenda Constitucional nº 19, de 1998)",)


def test_k7_nota_de_goias_e_vide_viram_nota() -> None:
    corpo = (
        '<p class="inciso">IV – apreciar os atos;<br> <nota><a href="/p/1">'
        "- Redação dada pela Lei nº 20.122, de 11-06-2018, art. 1º.</a></nota></p>"
        '<p class="preambulo"><vide>- Vide Lei nº 23.122</vide></p>'
    )
    paragrafos = paragrafos_de_html(_html(corpo))
    assert paragrafos[0].texto == "IV – apreciar os atos;"
    assert paragrafos[0].notas == ("- Redação dada pela Lei nº 20.122, de 11-06-2018, art. 1º.",)
    assert paragrafos[1].texto == ""
    assert paragrafos[1].notas == ("- Vide Lei nº 23.122",)


def test_k7_link_que_e_parte_do_texto_continua_no_texto() -> None:
    corpo = '<p>Art. 3º Nos termos da <a href="lei.htm">Lei nº 8.666, de 1993</a>, aplica-se.</p>'
    assert _textos(corpo) == ["Art. 3º Nos termos da Lei nº 8.666, de 1993, aplica-se."]


def test_k7_revogado_sem_texto_fica_marcado() -> None:
    corpo = '<p>Art. 1º Texto.</p><p>§ 3º <a href="x">(Revogado pela Lei nº 9, de 2001)</a></p>'
    [_, par] = _dispositivos(corpo)
    assert par["texto"] == "§ 3º"
    assert par["revogado"] is True
    assert par["notas"] == ["(Revogado pela Lei nº 9, de 2001)"]


def test_k8_riscado_parcial_sai_do_texto_vigente_e_fica_como_anterior() -> None:
    corpo = "<p>Art. 1º O prazo é de <strike>trinta</strike> sessenta dias.</p>"
    [p] = paragrafos_de_html(_html(corpo))
    assert p.texto == "Art. 1º O prazo é de sessenta dias."
    assert p.riscado == ("trinta",)


def test_k9_cabecalho_e_nome_no_mesmo_paragrafo_ou_em_dois() -> None:
    juntos = _dispositivos("<p>CAPÍTULO I<br>DISPOSIÇÕES PRELIMINARES</p><p>Art. 1º Texto.</p>")
    separados = _dispositivos(
        "<p>CAPÍTULO I</p><p>DISPOSIÇÕES PRELIMINARES</p><p>Art. 1º Texto.</p>"
    )
    for d in (juntos, separados):
        assert (d[0]["tipo"], d[0]["rotulo"], d[0]["nome"]) == (
            "capitulo",
            "CAPÍTULO I",
            "DISPOSIÇÕES PRELIMINARES",
        )
        assert d[1]["pai"] == d[0]["ref"]


def test_k9_capitulo_com_letra() -> None:
    ds = _dispositivos("<p>Art. 1º A.</p><p>CAPÍTULO IV-A</p><p>DO RECURSO</p><p>Art. 2º B.</p>")
    assert ds[1]["ref"] == "cap4-a"


def test_k9_nome_em_varias_linhas_de_goias() -> None:
    ds = _dispositivos(
        '<p class="agrupamento"><strong>CAPÍTULO III</strong></p>'
        '<p class="filho-agrupamento">DO PRESIDENTE, DO VICE-PRESIDENTE,</p>'
        '<p class="filho-agrupamento">DO OUVIDOR E</p>'
        '<p class="filho-agrupamento">DO DIRETOR-GERAL</p>'
        '<p class="sub-agrupamento">SUBSEÇÃO I</p>'
        '<p class="filho-sub-agrupamento">DA FISCALIZAÇÃO</p>'
        '<p class="artigo">Art. 1º A.</p>'
    )
    assert ds[0]["nome"] == "DO PRESIDENTE, DO VICE-PRESIDENTE, DO OUVIDOR E DO DIRETOR-GERAL"
    assert (ds[1]["rotulo"], ds[1]["nome"]) == ("SUBSEÇÃO I", "DA FISCALIZAÇÃO")


def test_k9_nome_da_divisao_em_caixa_normal() -> None:
    ds = _dispositivos(
        "<p>Art. 1º A.</p><p>Seção II</p><p>Do Conselho Nacional</p><p>Art. 2º B.</p>"
    )
    assert (ds[1]["rotulo"], ds[1]["nome"]) == ("Seção II", "Do Conselho Nacional")


def test_k10b_sumario_de_links_nao_e_lei_nem_abre_o_adct() -> None:
    corpo = (
        '<p><a href="c.htm">CONSTITUIÇÃO DA REPÚBLICA FEDERATIVA DO BRASIL DE 1988</a></p>'
        '<p><a href="emc.htm">Emendas Constitucionais</a></p>'
        '<p><a href="#adct">Ato das Disposições Constitucionais Transitórias</a></p>'
        "<p>Art. 1º Corpo.</p>"
    )
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert montagem.descartados == [
        "Emendas Constitucionais",
        "Ato das Disposições Constitucionais Transitórias",
    ]
    assert [d.ref for d in montagem.dispositivos] == ["preambulo1", "art1"]


def test_k10_cabecalho_do_site_fica_fora_da_lei() -> None:
    paragrafos = paragrafos_de_html(_html(LEI_PEQUENA))
    montagem = montar(paragrafos, por_regras(paragrafos))
    textos = [d.texto for d in montagem.dispositivos]
    assert "Presidência da República" not in textos
    assert "Texto compilado" not in textos
    assert montagem.descartados == ["Presidência da República", "Texto compilado"]
    assert montagem.dispositivos[0].tipo == "preambulo"


# ---------------------------------------------------------------- classificar


@pytest.mark.parametrize(
    ("texto", "rotulo", "ref"),
    [
        ("Art. 1º Texto.", "Art. 1º", "art1"),
        ("Art. 1º-A. Texto.", "Art. 1º-A", "art1-a"),
        ("Art. 9º -A Texto.", "Art. 9º-A", "art9-a"),
        ("Art. 5o Texto.", "Art. 5º", "art5"),
        ("Art. 10. Texto.", "Art. 10", "art10"),
        ("Art. 5 º Texto.", "Art. 5º", "art5"),
        ("Art. 104-B. Texto.", "Art. 104-B", "art104-b"),
        # K11b: erro de digitação da própria fonte (LGPD, 2026).
        ("Art. 5 7. (VETADO).", "Art. 57", "art57"),
    ],
)
def test_k11_rotulo_de_artigo(texto: str, rotulo: str, ref: str) -> None:
    [d] = _dispositivos(f"<p>{texto}</p>")
    assert (d["tipo"], d["rotulo"], d["ref"]) == ("artigo", rotulo, ref)


def test_k11_sup_do_ordinal_nao_quebra_o_artigo() -> None:
    [d] = _dispositivos("<p>Art. 5<sup>º</sup> Todo aquele.</p>")
    assert d["texto"] == "Art. 5º Todo aquele."
    assert d["ref"] == "art5"


def test_k12_paragrafo_unico_e_paragrafo_com_letra() -> None:
    ds = _dispositivos(
        "<p>Art. 1º Texto.</p><p>Parágrafo único. Um.</p>"
        "<p>Art. 2º Texto.</p><p>§ 1º Um.</p><p>§ 1º-A. Um-A.</p>"
    )
    assert [d["ref"] for d in ds] == ["art1", "art1.parunico", "art2", "art2.par1", "art2.par1-a"]
    assert ds[1]["rotulo"] == "Parágrafo único"


@pytest.mark.parametrize("traco", ["-", "–", "—"])
def test_k13_inciso_com_qualquer_traco(traco: str) -> None:
    ds = _dispositivos(f"<p>Art. 1º Compete:</p><p>IV {traco} julgar;</p><p>IV-A {traco} ver;</p>")
    assert [d["ref"] for d in ds[1:]] == ["art1.inc4", "art1.inc4-a"]
    assert [d["tipo"] for d in ds[1:]] == ["inciso", "inciso"]


def test_k14_alinea_sob_inciso_e_item_sob_alinea() -> None:
    ds = _dispositivos(
        "<p>Art. 1º Compete:</p><p>I - julgar:</p><p>a) as contas:</p><p>1. anuais;</p>"
        "<p>§ 1º Parágrafo:</p><p>II - ver.</p>"
    )
    assert [d["ref"] for d in ds] == [
        "art1",
        "art1.inc1",
        "art1.inc1.alia",
        "art1.inc1.alia.item1",
        "art1.par1",
        "art1.par1.inc2",
    ]


def test_k13_inciso_com_letra_sem_traco() -> None:
    ds = _dispositivos("<p>Art. 92. São órgãos:</p><p>I - o STF;</p><p>I-A o CNJ;</p>")
    assert [d["ref"] for d in ds] == ["art92", "art92.inc1", "art92.inc1-a"]


def test_k13_inciso_sem_traco_da_emenda_45() -> None:
    ds = _dispositivos(
        "<p>Art. 114. Compete:</p><p>I as ações oriundas;</p><p>II as ações que envolvam;</p>"
    )
    assert [d["ref"] for d in ds] == ["art114", "art114.inc1", "art114.inc2"]


def test_k14_alinea_com_espaco_antes_do_parentese() -> None:
    ds = _dispositivos(
        "<p>Art. 77. Até 2004:</p><p>I - no caso da União:</p><p>a ) no ano 2000;</p>"
    )
    assert ds[-1]["ref"] == "art77.inc1.alia"


def test_k14_alinea_sem_inciso_nem_paragrafo_e_problema() -> None:
    paragrafos = paragrafos_de_html(_html("<p>Art. 1º Compete:</p><p>a) as contas.</p>"))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert any("alínea" in p for p in montagem.problemas)


def test_k14b_texto_citado_entre_aspas_nao_e_dispositivo_desta_lei() -> None:
    ds = _dispositivos(
        "<p>Art. 60. A Lei nº 12.965 passa a vigorar com as seguintes alterações:</p>"
        "<p>“Art. 7º .......</p><p>X - exclusão definitiva dos dados;</p>"
        "<p>.........” (NR)</p><p>Art. 61. Vigência.</p>"
    )
    assert [(d["ref"], d["tipo"]) for d in ds] == [
        ("art60", "artigo"),
        ("art60.txt1", "solto"),
        ("art60.txt2", "solto"),
        ("art60.txt3", "solto"),
        ("art61", "artigo"),
    ]


def test_k15_adct_tem_refs_proprias() -> None:
    ds = _dispositivos(
        "<p>Art. 1º Corpo.</p><p>Art. 2º Corpo.</p>"
        "<p>ATO DAS DISPOSIÇÕES CONSTITUCIONAIS TRANSITÓRIAS</p>"
        "<p>Art. 1º Transitório.</p><p>Parágrafo único. Transitório.</p>"
    )
    assert [d["ref"] for d in ds] == ["art1", "art2", "adct", "adct.art1", "adct.art1.parunico"]


def test_k15b_fecho_do_corpo_antes_do_adct_nao_engole_o_adct() -> None:
    ds = _dispositivos(
        "<p>Art. 1º Corpo.</p><p>Brasília, 5 de outubro de 1988.</p><p>ULYSSES GUIMARÃES</p>"
        "<p>ATO DAS DISPOSIÇÕES CONSTITUCIONAIS TRANSITÓRIAS</p>"
        "<p>Art. 1º Transitório:</p><p>I - no caso:</p><p>a ) no ano 2000;</p>"
    )
    assert [d["ref"] for d in ds] == [
        "art1",
        "fecho1",
        "fecho2",
        "adct",
        "adct.art1",
        "adct.art1.inc1",
        "adct.art1.inc1.alia",
    ]


class _ProviderFalso:
    def __init__(self, respostas: list[dict[str, object]]) -> None:
        self.respostas = respostas
        self.pedidos: list[object] = []

    def available(self) -> bool:
        return True

    async def extract_structured(self, request: object) -> dict[str, object]:
        self.pedidos.append(request)
        return self.respostas.pop(0)


async def test_k16_gemini_com_id_estranho_faltando_ou_repetido_falha() -> None:
    paragrafos = paragrafos_de_html(_html("<p>Art. 1º A.</p><p>Art. 2º B.</p>"))
    ids = [p.id for p in paragrafos]
    ruins = [
        [{"id": ids[0], "tipo": "artigo"}],
        [{"id": ids[0], "tipo": "artigo"}, {"id": "p9999", "tipo": "artigo"}],
        [{"id": ids[0], "tipo": "artigo"}, {"id": ids[0], "tipo": "artigo"}],
        [{"id": ids[1], "tipo": "artigo"}, {"id": ids[0], "tipo": "artigo"}],
    ]
    for itens in ruins:
        with pytest.raises(ValueError, match="Gemini"):
            # O lote ruim é pedido de novo em metades (K16b), e erra de novo.
            await por_gemini(paragrafos, _ProviderFalso([{"itens": itens}] * 3))


class _PulaNoPrimeiro(_ProviderFalso):
    """Pula um id no primeiro pedido e acerta os seguintes, ecoando os ids."""

    def __init__(self, pular_sempre_o_lote_inteiro: bool = False) -> None:
        super().__init__([])
        self.chamadas = 0
        self.sempre = pular_sempre_o_lote_inteiro

    async def extract_structured(self, request: object) -> dict[str, object]:
        self.chamadas += 1
        ids = [c.split(": ", 1)[0] for c in request.chunks]  # type: ignore[attr-defined]
        if self.chamadas == 1 or (self.sempre and len(ids) > 1):
            ids = ids[:-1]
        return {"itens": [{"id": i, "tipo": "artigo"} for i in ids]}


async def test_k16b_lote_com_id_pulado_e_pedido_de_novo_em_duas_metades() -> None:
    # Com temperatura 0 o mesmo pedido repete o mesmo erro: a segunda chance
    # tem de ser um pedido diferente — o lote em duas metades.
    corpo = "".join(f"<p>Art. {i}º T.</p>" for i in range(1, 5))
    paragrafos = paragrafos_de_html(_html(corpo))
    provider = _PulaNoPrimeiro()
    classes = await por_gemini(paragrafos, provider)
    assert [c.tipo for c in classes if c] == ["artigo"] * 4
    assert provider.chamadas == 3
    with pytest.raises(ValueError, match="Gemini"):
        await por_gemini(paragrafos, _PulaNoPrimeiro(pular_sempre_o_lote_inteiro=True))


async def test_k16_gemini_nunca_recebe_pedido_de_texto_e_classifica_em_lotes() -> None:
    corpo = "".join(f"<p>Art. {i}º Texto {i}.</p>" for i in range(1, 6))
    paragrafos = paragrafos_de_html(_html(corpo))
    lotes = [
        {"itens": [{"id": p.id, "tipo": "artigo"} for p in paragrafos[:3]]},
        {"itens": [{"id": p.id, "tipo": "artigo"} for p in paragrafos[3:]]},
    ]
    provider = _ProviderFalso(lotes)
    classes = await por_gemini(paragrafos, provider, lote=3)
    assert [c.tipo for c in classes] == ["artigo"] * 5
    assert len(provider.pedidos) == 2
    schema = json.dumps(provider.pedidos[0].response_schema)  # type: ignore[attr-defined]
    assert "texto" not in schema


def test_k17_divergencia_onde_a_estrutura_esta_em_jogo_bloqueia() -> None:
    paragrafos = paragrafos_de_html(
        _html(
            "<p>Art. 1º Texto.</p><p>DISPOSIÇÕES</p>"
            "<p>“Art. 7º .......</p><p>X - citado;</p><p>.......” (NR)</p>"
        )
    )
    regras = por_regras(paragrafos)
    gemini = [Classe("solto"), Classe("solto"), Classe("solto"), Classe("inciso"), Classe("solto")]
    finais, divergencias = combinar(regras, gemini, paragrafos)
    assert [c.tipo for c in finais] == [c.tipo for c in regras]
    # Artigo que o Gemini acha solto, e texto citado que ele acha inciso: os
    # dois mudariam a árvore, e a regra pode estar errada.
    assert divergencias == [
        "p0001: a regra diz artigo, o Gemini diz solto",
        "p0004: a regra diz solto, o Gemini diz inciso",
    ]


def test_k17c_palpite_impossivel_ou_sem_efeito_na_arvore_nao_bloqueia() -> None:
    paragrafos = paragrafos_de_html(
        _html(
            '<p><a href="e.htm">Emendas Constitucionais</a></p>'
            "<p>Art. 1º Texto.</p><p>CAPÍTULO I</p><p>DAS CONTAS</p><p>Art. 2º B.</p>"
        )
    )
    regras = por_regras(paragrafos)
    # descartar × solto não muda a árvore; "inciso" para "Art. 1º" e "titulo"
    # para "DAS CONTAS" são impossíveis: não há rótulo de inciso nem de título.
    gemini = [Classe("solto"), Classe("inciso"), Classe("capitulo"), Classe("titulo"), None]
    finais, divergencias = combinar(regras, gemini, paragrafos)
    assert divergencias == []
    assert [c.tipo for c in finais] == [c.tipo for c in regras]


def test_k17b_divergencia_vira_aviso_com_id_so_da_regra() -> None:
    corpo = _html(LEI_PEQUENA).encode("cp1252")
    paragrafos = paragrafos_de_html(_html(LEI_PEQUENA))
    regras = {p.id: c.tipo for p, c in zip(paragrafos, por_regras(paragrafos), strict=True)}
    pid = next(p.id for p in paragrafos if p.texto.startswith("Art. 2º"))

    def discorda(palpite: str) -> _ProviderFalso:
        class _Discorda(_ProviderFalso):
            async def extract_structured(self, request: object) -> dict[str, object]:
                itens = []
                for chunk in request.chunks:  # type: ignore[attr-defined]
                    id_, texto = chunk.split(": ", 1)
                    tipo = palpite if texto.startswith("Art. 2º") else regras[id_]
                    itens.append({"id": id_, "tipo": tipo})
                return {"itens": itens}

        return _Discorda([])

    def http(url: str) -> Resposta:
        return Resposta(200, "text/html", corpo)

    solto = _capturar(http, discorda("solto"))
    assert solto.bloqueios == []
    [aviso] = solto.avisos
    assert aviso.id == f"{pid}: a regra diz artigo"
    assert aviso.texto == f"{pid}: a regra diz artigo, o Gemini diz solto"
    assert aviso.trecho.startswith("Art. 2º Compete ao órgão:")
    # O Gemini muda de palpite entre execuções: o aviso que a pessoa revisou é
    # do parágrafo e do tipo que a regra deu, qualquer que seja o palpite.
    assert [a.id for a in _capturar(http, discorda("nome")).avisos] == [aviso.id]


def test_k18_sem_gemini_a_captura_diz_que_nao_foi_conferida() -> None:
    resultado = _capturar(
        lambda url: Resposta(200, "text/html", _html(LEI_PEQUENA).encode("cp1252"))
    )
    assert resultado.publicavel
    assert resultado.gemini is False
    assert [a.id for a in resultado.avisos] == ["sem-gemini"]


# ---------------------------------------------------------------- montar e verificar


def test_k19_paragrafo_ou_inciso_sem_artigo_e_problema() -> None:
    for corpo in ("<p>CAPÍTULO I</p><p>§ 1º Solto.</p>", "<p>I - solto;</p><p>Art. 1º X.</p>"):
        paragrafos = paragrafos_de_html(_html(corpo))
        assert montar(paragrafos, por_regras(paragrafos)).problemas


def test_k20_artigo_fora_de_sequencia_e_problema() -> None:
    paragrafos = paragrafos_de_html(_html("<p>Art. 1º A.</p><p>Art. 3º C.</p>"))
    montagem = montar(paragrafos, por_regras(paragrafos))
    problemas = verificar(_html("<p>Art. 1º A.</p><p>Art. 3º C.</p>"), paragrafos, montagem)
    assert any("art1 → art3" in p for p in problemas)


def test_k20_sequencia_com_letra_e_aceita() -> None:
    corpo = "<p>Art. 1º A.</p><p>Art. 1º-A. B.</p><p>Art. 1º-B. B.</p><p>Art. 2º C.</p>"
    paragrafos = paragrafos_de_html(_html(corpo))
    assert verificar(_html(corpo), paragrafos, montar(paragrafos, por_regras(paragrafos))) == []


def test_k21_ref_repetida_e_problema() -> None:
    corpo = "<p>Art. 1º A.</p><p>I - um;</p><p>I - de novo;</p>"
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert any("art1.inc1" in p and "repetida" in p for p in montagem.problemas)


def test_k22_texto_que_nao_esta_no_original_e_problema() -> None:
    corpo = "<p>Art. 1º O prazo é de sessenta dias.</p>"
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    montagem.dispositivos[0].texto = "Art. 1º O prazo é de noventa dias."
    problemas = verificar(_html(corpo), paragrafos, montagem)
    assert any("remontado" in p for p in problemas)


def test_k22_redacao_anterior_com_nota_no_meio_confere_com_o_original() -> None:
    corpo = (
        "<p>Art. 1º A.</p>"
        '<p><strike>Art. 2º Antigo. <a href="e.htm">(Redação dada pela EC 24)</a> }</strike></p>'
        "<p>Art. 2º Novo.</p>"
    )
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert verificar(_html(corpo), paragrafos, montagem) == []


def test_k22_texto_visivel_ignora_tags_e_entidades() -> None:
    assert texto_visivel("<p>Art.&nbsp;1º <b>A</b>\n lei.</p>") == "Art. 1º A lei."


def test_k23_redacao_anterior_vai_para_o_dispositivo_certo() -> None:
    corpo = (
        "<p>Art. 70. Texto.</p>"
        "<p><strike>Parágrafo único. Versão antiga.</strike></p>"
        "<p>Parágrafo único. Versão nova.</p>"
        "<p>Art. 71. Compete:</p><p>I - um;</p><p>II - dois;</p>"
        "<p><s>II - dois antigo;</s></p>"
    )
    ds = {d["ref"]: d for d in _dispositivos(corpo)}
    assert ds["art70.parunico"]["anteriores"] == ["Parágrafo único. Versão antiga."]
    assert ds["art71.inc2"]["anteriores"] == ["II - dois antigo;"]
    assert ds["art71.inc1"]["anteriores"] == []


def test_k23_redacao_anterior_sem_vigente_vira_dispositivo_revogado() -> None:
    corpo = "<p>Art. 1º A.</p><p><strike>Art. 2º Revogado inteiro.</strike></p><p>Art. 3º C.</p>"
    ds = _dispositivos(corpo)
    assert [(d["ref"], d["revogado"]) for d in ds] == [
        ("art1", False),
        ("art2", True),
        ("art3", False),
    ]
    assert ds[1]["texto"] == ""
    assert ds[1]["anteriores"] == ["Art. 2º Revogado inteiro."]


def test_k23b_bloco_de_redacoes_antigas_casa_com_o_bloco_vigente() -> None:
    corpo = (
        "<p>Art. 26. Texto:</p><p>III - três;</p>"
        "<p><strike>IV - quatro antigo;</strike></p><p><strike>V - cinco antigo;</strike></p>"
        "<p>IV - quatro;</p><p>V - cinco;</p>"
        "<p><strike>VI - seis antigo;</strike></p>"
        "<p>§ 1º Parágrafo.</p>"
    )
    ds = {d["ref"]: d for d in _dispositivos(corpo)}
    assert ds["art26.inc4"]["anteriores"] == ["IV - quatro antigo;"]
    assert ds["art26.inc5"]["anteriores"] == ["V - cinco antigo;"]
    assert ds["art26.inc6"]["revogado"] is True
    assert list(ds) == [
        "art26",
        "art26.inc3",
        "art26.inc4",
        "art26.inc5",
        "art26.inc6",
        "art26.par1",
    ]


def test_k23c_artigo_revogado_fora_de_ordem_nao_e_salto() -> None:
    corpo = (
        "<p>Art. 1º A.</p><p><strike>Art. 1º-K. Incluído e revogado.</strike></p>"
        "<p>Art. 1º-A. B.</p><p>Art. 2º C.</p>"
    )
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert montagem.problemas == []
    assert verificar(_html(corpo), paragrafos, montagem) == []


def test_k20_salto_coberto_por_revogado_nao_e_problema() -> None:
    corpo = "<p>Art. 1º A.</p><p>Art. 3º C.</p><p><strike>Art. 2º Revogado.</strike></p>"
    paragrafos = paragrafos_de_html(_html(corpo))
    montagem = montar(paragrafos, por_regras(paragrafos))
    assert verificar(_html(corpo), paragrafos, montagem) == []


# ---------------------------------------------------------------- versão


def test_k34_verificacao_falhou_nao_devolve_a_lei() -> None:
    # Um dispositivo sem artigo: a árvore não monta, e a prévia não traz lei.
    corpo = _html("<p>CAPÍTULO I</p><p>§ 1º Solto.</p>").encode()
    resultado = _capturar(lambda url: Resposta(200, "text/html", corpo))
    assert resultado.bloqueios
    assert resultado.dispositivos is None and resultado.versao is None
    assert not resultado.publicavel


def test_k26_recapturar_a_mesma_fonte_mantem_a_versao() -> None:
    corpo = _html(LEI_PEQUENA).encode("cp1252")

    def http(url: str) -> Resposta:
        return Resposta(200, "text/html", corpo)

    primeira, segunda = _capturar(http), _capturar(http)
    assert primeira.versao == segunda.versao
    assert primeira.dispositivos == segunda.dispositivos


def test_dispositivos_tem_o_formato_do_pacote() -> None:
    corpo = _html(LEI_PEQUENA).encode("cp1252")
    lei = _capturar(lambda url: Resposta(200, "text/html", corpo))
    assert lei.fonte == PLANALTO
    assert lei.dispositivos is not None
    refs = [d["ref"] for d in lei.dispositivos]
    assert refs == [
        "preambulo1",
        "preambulo2",
        "cap1",
        "art1",
        "art1.parunico",
        "art2",
        "art2.inc1",
        "art2.inc2",
        "art2.inc2.alia",
        "art2.inc2.alib",
        "art2.par1",
        "art2.par2",
        "fecho1",
    ]
    assert set(lei.dispositivos[0]) == {
        "ref",
        "pai",
        "tipo",
        "rotulo",
        "nome",
        "texto",
        "notas",
        "anteriores",
        "revogado",
    }


# ---------------------------------------------------------------- PDF


def test_k27_pdf_junta_linhas_e_hifenizacao() -> None:
    paginas = [
        "Art. 1º O Tribunal de Contas, órgão de adminis-\n"
        "tração, compete:\n"
        "I - julgar as contas dos\n"
        "responsáveis;\n"
        "II - apreciar.\n"
    ]
    paragrafos = paragrafos_de_pdf(paginas)
    assert [p.texto for p in paragrafos] == [
        "Art. 1º O Tribunal de Contas, órgão de administração, compete:",
        "I - julgar as contas dos responsáveis;",
        "II - apreciar.",
    ]
    assert paragrafos[0].juncoes == ("adminis-|tração",)


def test_k28_pdf_remove_cabecalho_e_rodape_repetidos() -> None:
    paginas = [
        f"RESOLUÇÃO Nº 22/2008\nArt. {i}º Texto {i}.\nPágina {i} de 3\n" for i in range(1, 4)
    ]
    # O cabeçalho idêntico fica só na primeira página (é a epígrafe); a
    # paginação, que muda de página a página, sai inteira.
    textos = [p.texto for p in paragrafos_de_pdf(paginas)]
    assert textos == [
        "RESOLUÇÃO Nº 22/2008",
        "Art. 1º Texto 1.",
        "Art. 2º Texto 2.",
        "Art. 3º Texto 3.",
    ]
