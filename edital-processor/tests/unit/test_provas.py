"""Provas sem chamada externa: geometria das regiões, recortes, gabarito e o
contrato com o Gemini, incluindo a recusa por recitação."""

from __future__ import annotations

import uuid
from itertools import pairwise
from pathlib import Path
from typing import Any

import pymupdf
import pytest

from app.core.config import Settings
from app.core.errors import (
    DocumentNotFound,
    InvalidPDF,
    OCRUnavailable,
    ProviderRefused,
    RenderLimitExceeded,
)
from app.provas import pipeline
from app.provas.documentos import arquivo, recortar, regioes, render
from app.provas.schemas import Metadados, Origem
from app.providers.base import StructuredRequest
from app.services.ocr import LinhaOCR


def pdf(root: Path, height: int = 800, rotation: int = 0) -> str:
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        p = doc.new_page(width=595, height=height)
        p.insert_text((30, 60), "Texto original")
        p.draw_rect(pymupdf.Rect(100, 100, 200, 200), color=(1, 0, 0))
        p.set_rotation(rotation)
        doc.save(arquivo(root, id, "pdf"))
    return id


def test_regioes_longas_limitadas(tmp_path: Path) -> None:
    """A prova de exemplo é uma página só, com 8.772 pt de altura: nenhuma
    renderização pode pegá-la inteira."""
    settings = Settings(provas_dir=tmp_path)
    id = pdf(tmp_path, 8772)
    regions = regioes(tmp_path, id, settings)
    assert len(regions) > 10
    assert regions[0].retangulo[1] == 0
    assert regions[-1].retangulo[3] == 8772
    for previous, current in pairwise(regions):
        assert current.retangulo[1] < previous.retangulo[3], "regiões sem sobreposição"
    for origin in regions:
        image, _ = render(tmp_path, id, origin, settings)
        pix = pymupdf.Pixmap(image)
        assert pix.width * pix.height <= settings.provas_region_pixels + 10000
    with pytest.raises(RenderLimitExceeded):
        regioes(tmp_path, id, Settings(provas_max_regions=1))


@pytest.mark.parametrize("rotation", [0, 90, 180, 270])
def test_recorte_preserva_pixels_e_rotacao(tmp_path: Path, rotation: int) -> None:
    id = pdf(tmp_path, rotation=rotation)
    settings = Settings(provas_dir=tmp_path)
    region = regioes(tmp_path, id, settings)[0]
    image, _ = render(tmp_path, id, region, settings)
    asset = recortar(tmp_path, id, region, settings)
    assert arquivo(tmp_path, asset, "png").read_bytes() == image
    with pytest.raises(InvalidPDF):
        render(tmp_path, id, Origem(pagina=1, retangulo=[-1, 0, 100, 100]), settings)


def test_recorte_no_pe_da_pagina_arredondado(tmp_path: Path) -> None:
    """O pé da folha A4 é 841,9199…, e a tela manda 841,92: o trecho da 60 do
    TRT-15, no pé da página, era recusado como fora dela."""
    settings = Settings(provas_dir=tmp_path)
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        doc.new_page(width=595.44, height=841.92)
        doc.save(arquivo(tmp_path, id, "pdf"))
    with pymupdf.open(arquivo(tmp_path, id, "pdf")) as doc:
        pe = doc[0].rect.y1

    png, _ = render(
        tmp_path, id, Origem(pagina=1, retangulo=[40, 704, 580, round(pe, 2) + 0.01]), settings
    )

    assert png
    with pytest.raises(InvalidPDF):
        render(tmp_path, id, Origem(pagina=1, retangulo=[40, 704, 580, pe + 5]), settings)


def test_recorte_repetido_nao_grava_outro_arquivo(tmp_path: Path) -> None:
    """A tela pede a prévia da região a cada visita; com id aleatório, cada
    visita deixava um PNG permanente no volume."""
    id = pdf(tmp_path)
    settings = Settings(provas_dir=tmp_path)
    region = regioes(tmp_path, id, settings)[0]

    primeiro = recortar(tmp_path, id, region, settings)
    segundo = recortar(tmp_path, id, region, settings)
    outro = recortar(tmp_path, id, Origem(pagina=1, retangulo=[0, 0, 300, 300]), settings)

    assert primeiro == segundo != outro
    assert len(list(tmp_path.glob("*.png"))) == 2


def test_pdf_ausente_nao_e_erro_transitorio(tmp_path: Path) -> None:
    with pytest.raises(DocumentNotFound):
        regioes(tmp_path, str(uuid.uuid4()), Settings(provas_dir=tmp_path))


def _gabarito(root: Path, cabecalho: str, cargo: str = "Cargo: E05\nTipo de Gabarito: 4") -> str:
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        p = doc.new_page()
        p.insert_text(
            (20, 30),
            f"{cabecalho}\n{cargo}\n1\nE\nGabarito sem alteração\n2\nX\nAnulada",
        )
        doc.save(arquivo(root, id, "pdf"))
    return id


def test_gabarito_fcc_preliminar(tmp_path: Path) -> None:
    g, _ = pipeline.ler_gabarito(tmp_path, _gabarito(tmp_path, "GABARITO PRELIMINAR"))
    assert g.cargo == "E05" and g.caderno == "4" and g.tipo == "preliminar"
    assert g.respostas == {"1": "E", "2": ""}
    assert g.situacoes["2"] == "Anulada"


def test_gabarito_com_codigo_de_cargo_so_de_numeros(tmp_path: Path) -> None:
    # Como no TRT-15: o código vem colado ao nome abreviado, na mesma linha
    # do tipo de gabarito.
    cargo = "Cargo: 24 - AN JUD - AREA APOIO ESP - ESP TEC DA INFORMACAO. Tipo de Gabarito: 1"
    g, _ = pipeline.ler_gabarito(tmp_path, _gabarito(tmp_path, "PRELIMINAR", cargo))
    assert g.cargo == "24" and g.caderno == "1"
    assert g.respostas == {"1": "E", "2": ""}


def test_gabarito_escaneado_com_as_trocas_do_ocr(tmp_path: Path) -> None:
    """O do TRT-6 é imagem com texto de OCR: "Cargo: EO5" e a resposta C como
    "c". O código ficava vazio e as questões de resposta C, sem resposta."""
    cargo = "Cargo: EO5 - AN JUD - AREA APOIO ESP - ESP TEC DA INFORMAGAO. Tipo de Gabarito: 5"
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        doc.new_page().insert_text(
            (20, 30),
            f"PRELIMINAR\n{cargo}\n1 \nB \nGabarito sem alteração\n2 \nc \nGabarito sem alteragdo\n"
            "3 \nx \nAnulada",
        )
        doc.save(arquivo(tmp_path, id, "pdf"))

    g, _ = pipeline.ler_gabarito(tmp_path, id)

    assert g.cargo == "E05" and g.caderno == "5"
    assert g.respostas == {"1": "B", "2": "C", "3": ""}
    assert g.situacoes["3"] == "Anulada"


def test_gabarito_definitivo_que_cita_o_preliminar(tmp_path: Path) -> None:
    cabecalho = "GABARITO DEFINITIVO (após recursos contra o preliminar)"
    g, _ = pipeline.ler_gabarito(tmp_path, _gabarito(tmp_path, cabecalho))
    assert g.tipo == "definitivo"


def test_gabarito_em_tabela_questao_alternativa(tmp_path: Path) -> None:
    """O da SCGE-PE: "Questão / Alternativa", número e letra linha a linha,
    sem situação. Lido como o da FCC, a letra seguinte virava a situação e
    ficava uma questão sim, outra não — e com o zero, que não é a questão 1."""
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        doc.new_page().insert_text(
            (20, 30),
            "Secretaria da Controladoria Geral do Estado - SCGE (PE)\n"
            "CARGO: Gestor Governamental - Tecnologia da Informação\n"
            "EXAME: CPU/PE - Janeiro/2026 | CADERNO: Tipo 004\n"
            "Questão\nAlternativa\n01 \nE \n02 \nA \n03 \nX \n10 \nD ",
        )
        doc.save(arquivo(tmp_path, id, "pdf"))

    g, _ = pipeline.ler_gabarito(tmp_path, id, "004")

    assert g.respostas == {"1": "E", "2": "A", "3": "", "10": "D"}
    assert g.situacoes == {}
    assert g.caderno == "004" and g.cargo == "" and g.tipo == "nao_informado"


def _relacao(root: Path) -> str:
    """A "Relação dos gabaritos" impressa do site da FCC, como a do TRT-18: um
    tipo embaixo do outro, as respostas em colunas lado a lado — e, no PDF,
    todas as respostas gravadas antes de qualquer cabeçalho."""
    id = str(uuid.uuid4())
    tipos = {100: ["001 - A", "002 - B", "003 - C"], 250: ["001 - D", "002 - E", "003 - X"]}
    with pymupdf.open() as doc:
        p = doc.new_page()
        for y, respostas in tipos.items():
            for k, resposta in enumerate(respostas):
                p.insert_text((60 + 90 * (k % 2), y + 40 + 15 * (k // 2)), resposta)
        for n, y in enumerate(tipos, start=1):
            p.insert_text(
                (30, y),
                "Cargo ou opção L12 - TÉCNICO JUD - APOIO ESP - ESP TEC DA INFORMAÇÃO\n"
                f"Tipo gabarito {n}",
            )
        doc.save(arquivo(root, id, "pdf"))
    return id


def test_relacao_de_gabaritos_le_o_tipo_do_caderno_da_prova(tmp_path: Path) -> None:
    g, _ = pipeline.ler_gabarito(tmp_path, _relacao(tmp_path), "TIPO-002")
    assert g.cargo == "L12" and g.caderno == "2" and g.tipo == "nao_informado"
    assert g.respostas == {"1": "D", "2": "E", "3": ""}


@pytest.mark.parametrize("caderno", ["", "9"])
def test_relacao_sem_o_tipo_do_caderno_fica_com_o_primeiro(tmp_path: Path, caderno: str) -> None:
    # O backend compara o tipo com o caderno e mostra a diferença ao curador.
    g, _ = pipeline.ler_gabarito(tmp_path, _relacao(tmp_path), caderno)
    assert g.caderno == "1"
    assert g.respostas == {"1": "A", "2": "B", "3": "C"}


class FakeProvider:
    """Responde com o que a fila de respostas mandar; uma exceção na fila é
    levantada no lugar da resposta."""

    def __init__(self, *respostas: Any) -> None:
        self.respostas = list(respostas)
        self.pedidos: list[StructuredRequest] = []

    def available(self) -> bool:
        return True

    async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
        assert request.images and request.single_attempt and request.instruction
        self.pedidos.append(request)
        resposta = self.respostas.pop(0)
        if isinstance(resposta, Exception):
            raise resposta
        return dict(resposta)


QUESTAO_44 = {
    "Numero": 44,
    "Blocos": [
        {"Tipo": "texto", "Texto": "Considere o esquema."},
        {"Tipo": "imagem", "Descricao": "diagrama", "Retangulo": [200, 100, 700, 600]},
    ],
    "Alternativas": [{"Letra": letra, "Blocos": [{"Texto": letra}]} for letra in "ABCDE"],
    # Campos que o schema não pede e o modelo inventou: não podem vazar.
    "Resposta": "A",
    "Revisada": True,
}


async def test_modelo_nao_confirma_revisao_nem_inventa_resposta(tmp_path: Path) -> None:
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 100, 500, 600], regiao="7")
    provider = FakeProvider({"Questoes": [QUESTAO_44], "_modelo": "m", "_saida": 900})

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    q = result.questoes[0]
    assert not q.revisada and q.resposta == "" and q.origens == [origin]
    figura = q.blocos[1]
    # [ymin, xmin, ymax, xmax] sobre a região, mais 2% de margem.
    assert figura.origem is not None and figura.origem.retangulo == [40, 190, 310, 460]
    assert not figura.revisado and arquivo(tmp_path, figura.arquivo, "png").exists()
    assert result.extracoes[0].tokens_saida == 900
    assert result.extracoes[0].prompt == pipeline.PROMPT


async def test_figura_sem_coordenadas_vira_pendencia(tmp_path: Path) -> None:
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    questao = dict(QUESTAO_44, Blocos=[{"Tipo": "imagem", "Retangulo": [700, 100, 200, 600]}])

    result = await pipeline.extrair(
        tmp_path, id, origin, FakeProvider({"Questoes": [questao]}), Settings(provas_dir=tmp_path)
    )

    assert result.questoes[0].blocos[0].arquivo == ""
    assert "recorte à mão" in result.alertas[0]


async def test_recitacao_pede_a_estrutura_e_le_o_apoio_por_ocr(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """O texto de Sêneca das questões 1 a 10 é trecho de livro, e o Gemini se
    recusa a reproduzi-lo. A segunda tentativa pede só onde ele está; o texto
    vem do OCR daquele retângulo, já ligado às questões."""
    monkeypatch.setattr(pipeline, "ocr_image", lambda png, s: "A vida divide-se em três períodos")
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    estrutura = {
        "Apoios": [
            {
                "Id": "t1",
                "Questoes": [1, 2],
                "Blocos": [{"Tipo": "imagem", "Retangulo": [100, 50, 400, 950]}],
            }
        ],
        "Questoes": [dict(QUESTAO_44, Numero=1, Blocos=[{"Texto": "Enunciado"}])],
    }
    provider = FakeProvider(ProviderRefused("RECITATION"), estrutura)

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    assert provider.pedidos[1].instruction == pipeline.INSTRUCAO_ESTRUTURA
    apoio = result.apoios[0]
    assert apoio.id == "r0-t1" and apoio.questoes == [1, 2]
    assert apoio.blocos[0].tipo == "texto" and apoio.blocos[0].texto.startswith("A vida")
    assert result.questoes[0].apoios == ["r0-t1"]
    assert any("OCR" in a for a in result.alertas)
    assert result.extracoes[0].prompt.endswith("-estrutura")


def _pagina_com_texto(root: Path, texto: str) -> str:
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        doc.new_page(width=595, height=800).insert_textbox(pymupdf.Rect(30, 30, 565, 770), texto)
        doc.save(arquivo(root, id, "pdf"))
    return id


CRONICA = (
    "Atenção: Para responder às questões de números 14 a 17, baseie-se no texto abaixo:\n"
    "No voo da caneta\n"
    "Numa das cartas ao seu amigo Mário de Andrade, assegurava-lhe o poeta que era com uma "
    "caneta na mão que costumava viver as suas maiores emoções.\n"
    "Comentando isso numa das minhas aulas de Literatura, atentei para a reação de um aluno.\n"
    "(Adaptado de: Aldair Rômulo Siqueira)"
)


async def test_trecho_de_apoio_le_so_o_texto(tmp_path: Path) -> None:
    """O texto das questões 14 a 17 do TRT-18 veio só com o primeiro parágrafo; o
    curador marca o texto inteiro, e o trecho não traz questão, nem a da ponta."""
    id = _pagina_com_texto(tmp_path, CRONICA)
    trecho = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="ta:r3-t1")
    provider = FakeProvider(
        {
            "Apoios": [{"Id": "t1", "Questoes": [14, 15, 16, 17], "Blocos": [{"Texto": CRONICA}]}],
            "Questoes": [dict(QUESTAO_44, Numero=14)],
        }
    )

    result = await pipeline.texto_de_apoio(
        tmp_path, id, trecho, provider, Settings(provas_dir=tmp_path)
    )

    assert provider.pedidos[0].instruction == pipeline.INSTRUCAO_TEXTO_DE_APOIO
    assert result.questoes == [] and len(result.apoios) == 1
    apoio = result.apoios[0]
    assert apoio.questoes == [14, 15, 16, 17] and "14 a 17" in apoio.aviso
    assert apoio.blocos[0].texto.startswith("No voo da caneta")
    assert "maiores emoções" in apoio.blocos[0].texto and apoio.origens == [trecho]
    assert result.alertas == []


async def test_trecho_de_apoio_recusado_vem_do_texto_do_pdf(tmp_path: Path) -> None:
    id = _pagina_com_texto(tmp_path, CRONICA)
    trecho = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="ta:r3-t1")

    result = await pipeline.texto_de_apoio(
        tmp_path,
        id,
        trecho,
        FakeProvider(ProviderRefused("RECITATION")),
        Settings(provas_dir=tmp_path),
    )

    apoio = result.apoios[0]
    assert "Comentando isso" in apoio.blocos[0].texto and apoio.questoes == [14, 15, 16, 17]
    assert "texto do PDF" in result.alertas[0] and result.extracoes[0].modelo == "pdf"


async def test_trecho_de_apoio_escaneado_recusado_vem_do_ocr(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(pipeline, "ocr_image", lambda png, s: CRONICA)
    id = pdf(tmp_path)  # só "Texto original": pouco texto para ser o do apoio
    trecho = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="ta:r3-t1")

    result = await pipeline.texto_de_apoio(
        tmp_path,
        id,
        trecho,
        FakeProvider(ProviderRefused("RECITATION")),
        Settings(provas_dir=tmp_path),
    )

    assert result.apoios[0].blocos[0].texto.startswith("No voo da caneta")
    assert "OCR" in result.alertas[0] and result.extracoes[0].modelo == "ocr"


async def test_sem_tesseract_o_apoio_fica_como_recorte(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    def sem_ocr(png: bytes, settings: Settings) -> str:
        raise OCRUnavailable("sem tesseract")

    monkeypatch.setattr(pipeline, "ocr_image", sem_ocr)
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    estrutura = {
        "Apoios": [{"Id": "t1", "Blocos": [{"Tipo": "imagem", "Retangulo": [100, 50, 400, 950]}]}]
    }

    result = await pipeline.extrair(
        tmp_path,
        id,
        origin,
        FakeProvider(ProviderRefused("RECITATION"), estrutura),
        Settings(provas_dir=tmp_path),
    )

    bloco = result.apoios[0].blocos[0]
    assert bloco.tipo == "imagem" and arquivo(tmp_path, bloco.arquivo, "png").exists()


async def test_recusa_dupla_nao_perde_a_regiao_em_silencio(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(pipeline, "ocr_image_com_linhas", lambda png, s: ("texto da região", []))
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="3")
    provider = FakeProvider(ProviderRefused("RECITATION"), ProviderRefused("RECITATION"))

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    assert result.questoes == []
    assert result.apoios[0].blocos[0].texto == "texto da região"
    assert "região 3" in result.alertas[0]


async def test_metadados_vem_da_capa(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    def sem_ocr(png: bytes, s: Settings) -> str:
        raise AssertionError("com o código lido, a capa não passa por OCR")

    monkeypatch.setattr(pipeline, "ocr_image", sem_ocr)
    id = pdf(tmp_path)
    capa = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    provider = FakeProvider(
        {
            "Orgao": "TJCE",
            "Ano": 2026,
            "Cargo": "E05",
            "CargoNome": "Analista Judiciário",
            "Caderno": "004",
            "Total": 60,
        }
    )

    m = await pipeline.metadados(tmp_path, id, capa, provider, Settings(provas_dir=tmp_path))

    assert (m.orgao, m.ano, m.cargo, m.caderno, m.total) == ("TJCE", 2026, "E05", "004", 60)
    assert m.cargo_nome == "Analista Judiciário"
    assert provider.pedidos[0].instruction == pipeline.INSTRUCAO_CAPA


NOME_SI = "Analista Judiciário \u2013 Área Técnico Administrativa Especialidade: Sistemas"


@pytest.mark.parametrize(
    ("lido", "texto", "esperado"),
    [
        # A leitura certa passa como veio.
        ({"cargo": "F06", "cargo_nome": NOME_SI}, "", ("F06", NOME_SI)),
        # O nome no lugar do código: vai para o nome, e o código fica para o
        # curador, que a pendência manda buscar na capa.
        ({"cargo": NOME_SI}, "", ("", NOME_SI)),
        ({"cargo": "F06 - " + NOME_SI}, "", ("F06", "F06 - " + NOME_SI)),
        # Com texto nativo, o código sai do quadro do candidato, seja o que for
        # que a IA leu.
        (
            {"cargo": NOME_SI},
            "Nome do Candidato\nCaderno de Prova \u2018f06\u2019, Tipo 004",
            ("F06", NOME_SI),
        ),
        # Há código só de números (TRT-15, TRF-4), lido na capa ou no texto.
        ({"cargo": "24", "cargo_nome": NOME_SI}, "", ("24", NOME_SI)),
        ({"cargo": NOME_SI}, "Nome do Candidato\nCaderno de Prova '03', Tipo 001", ("03", NOME_SI)),
        # Mas um número no meio do nome não vira código.
        (
            {"cargo": "Técnico Judiciário - TRT 15"},
            "",
            ("", "Técnico Judiciário - TRT 15"),
        ),
        # O texto que vem de OCR, como saiu das capas do TJCE, TRT-1 e TRT-15:
        # letras trocadas, dígito lido como letra e a aspa de fechar perdida.
        ({"cargo": NOME_SI}, "Nº do Caderno [Gadero de Prova 'EOS; Tipo 004", ("E05", NOME_SI)),
        ({"cargo": NOME_SI}, "Caderno de Prova 'FO6', Tipo 004 | MODELO", ("F06", NOME_SI)),
        ({"cargo": NOME_SI}, "íCadernu de Prova 'Q17, Tipo 004", ("Q17", NOME_SI)),
        ({"cargo": NOME_SI}, 'Cademo de Prova "28, Tipo 001', ("28", NOME_SI)),
        # O que o OCR não deixa virar código fica com a leitura da IA.
        ({"cargo": "F06"}, "Caderno de Prova 'F0X', Tipo 004", ("F06", "")),
    ],
)
def test_acertar_cargo(lido: dict[str, str], texto: str, esperado: tuple[str, str]) -> None:
    m = pipeline.acertar_cargo(Metadados(**lido), texto)

    assert (m.cargo, m.cargo_nome) == esperado


async def test_capa_recusada_deixa_a_identificacao_para_o_curador(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(pipeline, "ocr_image", lambda png, s: "Caderno de Prova 'FO6', Tipo 004")
    id = pdf(tmp_path)
    capa = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")

    m = await pipeline.metadados(
        tmp_path, id, capa, FakeProvider(ProviderRefused("SAFETY")), Settings(provas_dir=tmp_path)
    )

    # O resto fica para o curador; o código do cargo ainda sai do OCR.
    assert m.orgao == "" and m.total == 0 and m.cargo == "F06"


async def test_capa_escaneada_tira_o_codigo_do_ocr(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A capa do TJCE é imagem pura, e a IA devolveu o nome no lugar do
    código: o código ficava vazio. O OCR do quadro do candidato o traz."""
    monkeypatch.setattr(
        pipeline, "ocr_image", lambda png, s: "Nº do Caderno [Gadero de Prova 'EOS; Tipo 004"
    )
    id = pdf(tmp_path)
    capa = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    provider = FakeProvider({"Orgao": "TJCE", "Cargo": NOME_SI, "Caderno": "004"})

    m = await pipeline.metadados(tmp_path, id, capa, provider, Settings(provas_dir=tmp_path))

    assert (m.cargo, m.cargo_nome) == ("E05", NOME_SI)


@pytest.mark.parametrize(
    ("aviso", "numeros"),
    [
        (
            "Considere o texto do filósofo Sêneca para responder às questões de 1 a 10.",
            set(range(1, 11)),
        ),
        ("Atenção: as questões de números 11 a 13 referem-se ao texto.", {11, 12, 13}),
        ("Para responder às questões 21 e 22, considere a tabela.", {21, 22}),
        ("Texto sem aviso nenhum.", set()),
    ],
)
def test_questoes_citadas_no_aviso(aviso: str, numeros: set[int]) -> None:
    """O modelo lista só as questões que vê na região; o aviso diz a faixa
    inteira, e é ela que liga o texto às questões das regiões seguintes."""
    assert pipeline._questoes_citadas(aviso) == numeros


async def test_alternativas_no_enunciado_viram_alerta(tmp_path: Path) -> None:
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    repetida = " ".join(f"({letra}) opção {letra}." for letra in "ABCDE")
    questao = dict(QUESTAO_44, Blocos=[{"Texto": "Enunciado. " + repetida}])

    result = await pipeline.extrair(
        tmp_path, id, origin, FakeProvider({"Questoes": [questao]}), Settings(provas_dir=tmp_path)
    )

    assert any("repetir as alternativas" in a for a in result.alertas)
    assert result.questoes[0].blocos[0].texto.startswith("Enunciado.")


def test_ocr_da_prova_junta_as_linhas_do_paragrafo() -> None:
    """O caderno quebra a linha no meio da frase; o texto publicado não pode."""
    from app.services.ocr import texto_por_paragrafo

    linhas = [
        (1, 1, 1, "A vida divide-se em três"),
        (1, 1, 2, "períodos: o que se foi, o que es-"),
        (1, 1, 3, "tá sendo e o que há de vir."),
        (1, 2, 1, "(Adaptado de: Sêneca.)"),
    ]
    data: dict[str, list[object]] = {
        k: [] for k in ("text", "conf", "block_num", "par_num", "line_num")
    }
    for bloco, par, linha, frase in linhas:
        for palavra in frase.split():
            data["text"].append(palavra)
            data["conf"].append("91")
            data["block_num"].append(bloco)
            data["par_num"].append(par)
            data["line_num"].append(linha)

    assert texto_por_paragrafo(data) == (
        "A vida divide-se em três períodos: o que se foi, o que está sendo e o que há de vir.\n"
        "(Adaptado de: Sêneca.)"
    )


SENECA = (
    "A vida divide-se em três períodos: o que se foi, o que está sendo e o que há de vir.\n"
    "(Adaptado de: Sêneca. Sobre a brevidade da vida. São Paulo: Companhia das Letras, 2017)"
)


def test_apoio_do_ocr_fica_so_com_texto_e_fonte() -> None:
    """O retângulo pega o título da seção, o lixo do código de barras e a
    primeira linha da questão 1; nada disso é o texto."""
    from app.provas.schemas import Bloco

    ocr = (
        "breed scrtelrihos\nCONHECIMENTOS GERAIS Língua Portuguesa Atenção: Considere o texto "
        "do filósofo latino Sêneca para responder às questões de 1 a 10.\n"
        + SENECA
        + "\n1. não lhes sobra tempo para examinar o passado."
    )

    assert [b.texto for b in pipeline.so_o_texto([Bloco(texto=ocr)])] == [SENECA]


def test_apoio_do_gemini_perde_o_aviso_e_mantem_a_fonte() -> None:
    from app.provas.schemas import Bloco

    blocos = [
        Bloco(texto="Atenção: Considere o texto abaixo para responder às questões de 1 a 10."),
        Bloco(texto="A vida divide-se em três períodos.", formato="italico"),
        Bloco(texto="(Adaptado de: Sêneca.)"),
    ]

    limpos = pipeline.so_o_texto(blocos)

    assert [b.texto for b in limpos] == [
        "A vida divide-se em três períodos.",
        "(Adaptado de: Sêneca.)",
    ]
    assert limpos[0].formato == "italico"


def test_apoio_sem_aviso_nao_muda() -> None:
    from app.provas.schemas import Bloco

    blocos = [Bloco(texto="Tabela de alíquotas vigentes."), Bloco(tipo="imagem", arquivo="x")]

    assert pipeline.so_o_texto(blocos) == blocos


def test_faixa_de_questoes_no_meio_do_texto_nao_e_aviso() -> None:
    from app.provas.schemas import Bloco

    texto = "Um texto longo. " * 40 + "Como se viu nas questões de 3 a 5, a regra muda."

    assert pipeline.so_o_texto([Bloco(texto=texto)])[0].texto == texto


async def test_classificacao_manda_todas_as_questoes_juntas() -> None:
    """Numa chamada só: é o que faz a mesma matéria ter o mesmo nome na prova toda."""
    from app.provas.schemas import QuestaoParaClassificar

    class Classificador:
        def __init__(self) -> None:
            self.pedidos: list[StructuredRequest] = []

        def available(self) -> bool:
            return True

        async def extract_structured(self, request: StructuredRequest) -> dict[str, object]:
            self.pedidos.append(request)
            return {"Materias": [{"Numero": 21, "Materia": "Redes de Computadores"}]}

    provider = Classificador()
    questoes = [
        QuestaoParaClassificar(numero=1, secao="Língua Portuguesa", texto="Em relação à oração"),
        QuestaoParaClassificar(numero=21, secao="CONHECIMENTOS ESPECÍFICOS", texto="VLAN e trunk"),
    ]

    result = await pipeline.classificar(questoes, provider, Settings())

    assert result.materias[0].numero == 21 and result.materias[0].materia == "Redes de Computadores"
    assert len(provider.pedidos) == 1
    assert (
        "Questão 21 — seção: CONHECIMENTOS ESPECÍFICOS — VLAN e trunk"
        in provider.pedidos[0].chunks[0]
    )
    assert provider.pedidos[0].instruction == pipeline.INSTRUCAO_MATERIAS


async def test_questao_guarda_a_propria_area_no_original(tmp_path: Path) -> None:
    """A revisão mostra a questão recortada ao lado do rascunho; sem a área, a
    região inteira."""
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 100, 500, 600], regiao="4")
    com_area = dict(QUESTAO_44, Retangulo=[200, 100, 700, 600])
    sem_area = dict(QUESTAO_44, Numero=45)

    result = await pipeline.extrair(
        tmp_path,
        id,
        origin,
        FakeProvider({"Questoes": [com_area, sem_area]}),
        Settings(provas_dir=tmp_path),
    )

    area = result.questoes[0].origens[0]
    assert area.regiao == "4" and area.retangulo == [40, 190, 310, 460]
    assert result.questoes[1].origens == [origin]


# A questão 22 do TJCE, como o Gemini a devolveu: o SQL como prosa, partido pela
# lacuna sublinhada.
RESTORE = (
    "RESTORE DATABASE ProcessoJudicial\n"
    "FROM DISK = 'C:\\Backup\\ProcessoJudicial_FULL.bak'\n"
    "WITH NORECOVERY;\n"
    "RESTORE LOG ProcessoJudicial\n"
    "FROM DISK = 'C:\\Backup\\ProcessoJudicial_LOG.trn'\n"
    "WITH "
)


async def test_codigo_lido_como_texto_vira_bloco_de_codigo_com_a_lacuna(tmp_path: Path) -> None:
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="3")
    questao = dict(
        QUESTAO_44,
        Numero=22,
        Blocos=[
            {"Texto": "O DBA iniciou o seguinte processo:\n" + RESTORE},
            {"Texto": "I", "Formato": "sublinhado"},
            {"Texto": "\nA lacuna I deve ser completada com"},
        ],
    )

    result = await pipeline.extrair(
        tmp_path, id, origin, FakeProvider({"Questoes": [questao]}), Settings(provas_dir=tmp_path)
    )

    blocos = result.questoes[0].blocos
    assert [(b.tipo, b.texto) for b in blocos] == [
        ("texto", "O DBA iniciou o seguinte processo:"),
        ("codigo", RESTORE + "___I___"),
        ("texto", "A lacuna I deve ser completada com"),
    ]


def test_lacuna_logo_depois_do_bloco_de_codigo_entra_nele() -> None:
    from app.provas.schemas import Bloco

    blocos = [
        Bloco(texto="Considere:"),
        Bloco(tipo="codigo", texto="SELECT nome\nFROM clientes\nWHERE uf = "),
        Bloco(texto="II", formato="sublinhado"),
        Bloco(texto="\nA lacuna II deve ser preenchida com"),
    ]

    assert [(b.tipo, b.texto) for b in pipeline.codigo_solto(blocos)] == [
        ("texto", "Considere:"),
        ("codigo", "SELECT nome\nFROM clientes\nWHERE uf = ___II___"),
        ("texto", "A lacuna II deve ser preenchida com"),
    ]


def test_trecho_de_programa_com_chaves_vira_codigo() -> None:
    from app.provas.schemas import Bloco

    java = "public class A {\n    int x = 10;\n    System.out.println(x);\n}"
    blocos = [Bloco(texto=f"Considere o programa:\n{java}\nAo executá-lo, imprime-se")]

    assert [(b.tipo, b.texto) for b in pipeline.codigo_solto(blocos)] == [
        ("texto", "Considere o programa:"),
        ("codigo", java),
        ("texto", "Ao executá-lo, imprime-se"),
    ]


@pytest.mark.parametrize(
    "texto",
    [
        # Itens da FCC terminam em ponto e vírgula e continuam sendo prosa.
        "Considere:\nI. o modo de configuração do switch;\nII. a sigla (IEEE 802.1Q);\n"
        "III. o protocolo usado.",
        # Uma linha só com cara de SQL pode ser frase em maiúsculas.
        "O comando abaixo foi executado:\nDROP TABLE clientes;\nApós a execução, a tabela",
        "I. C++ é compilada;\nII. C++ tem herança múltipla;",
        "Sabendo que\nx = 3\ny = 4\ncalcule x + y.",
        # Já marcado com crases triplas pelo curador ou pelo modelo.
        "Considere:\n```\nSELECT *\nFROM t;\n```",
    ],
)
def test_prosa_e_codigo_ja_marcado_ficam_como_vieram(texto: str) -> None:
    from app.provas.schemas import Bloco

    blocos = [Bloco(texto=texto), Bloco(tipo="imagem", arquivo="x")]

    assert pipeline.codigo_solto(blocos) == blocos


def test_bloco_de_codigo_do_modelo_fica_intacto() -> None:
    from app.provas.schemas import Bloco

    blocos = [
        Bloco(texto="Execute:"),
        Bloco(tipo="codigo", texto="ls -l"),
        Bloco(texto="O comando"),
    ]

    assert pipeline.codigo_solto(blocos) == blocos


# O OCR da região 0 do TJCE quando o Gemini recusou tudo: capa e instruções,
# o aviso, o texto com a fonte e o começo da questão 1.
CAPA_E_TEXTO = (
    "EE, colégio Sala Ordem Pigs M0001 || 0001\n"
    "TRIBUNAL DE JUSTIÇA DO ESTADO DO CEARÁ Concurso Público\n"
    "- Verifique se este caderno contém 60 questões numeradas de 1 a 60.\n"
    "Para cada questão existe apenas UMA resposta certa.\n"
    "CONHECIMENTOS GERAIS Língua Portuguesa Atenção: Considere o texto do filósofo latino "
    "Sêneca para responder às questões de 1 a 10.\n" + SENECA + "\n"
    "1. não lhes sobra tempo para examinar o passado.\n(A) concessão.\n"
)


def test_ocr_da_regiao_separa_o_texto_pelo_aviso() -> None:
    """A faixa das instruções ("60 questões numeradas de 1 a 60") não é aviso de
    texto; o texto vai do aviso até a fonte, e liga às questões que ele cita."""
    [(aviso, corpo, questoes)] = pipeline.textos_do_ocr(CAPA_E_TEXTO)

    assert (
        aviso == "Considere o texto do filósofo latino Sêneca para responder às questões de 1 a 10."
    )
    assert corpo == SENECA
    assert questoes == set(range(1, 11))


def test_ocr_com_dois_textos_liga_cada_um_as_suas_questoes() -> None:
    texto = (
        "Atenção: Considere o texto I para responder às questões de 1 a 5.\nTexto um.\n"
        "(Adaptado de: A.)\n1. Primeira.\n"
        "Atenção: Considere o texto II para responder às questões de números 6 a 10.\n"
        "Texto dois.\n6. Sexta.\n"
    )

    textos = pipeline.textos_do_ocr(texto)

    assert [(c, sorted(q)) for _, c, q in textos] == [
        ("Texto um.\n(Adaptado de: A.)", [1, 2, 3, 4, 5]),
        ("Texto dois.", [6, 7, 8, 9, 10]),
    ]


async def test_recusa_dupla_com_aviso_entrega_o_texto_ja_ligado(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(pipeline, "ocr_image_com_linhas", lambda png, s: (CAPA_E_TEXTO, []))
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    provider = FakeProvider(ProviderRefused("RECITATION"), ProviderRefused("RECITATION"))

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    [apoio] = result.apoios
    assert apoio.id == "r0-ocr1" and apoio.questoes == list(range(1, 11))
    assert apoio.blocos[0].texto == SENECA and apoio.aviso.startswith("Considere o texto")
    assert apoio.origens == [origin]


def _linhas(*pares: tuple[float, str]) -> list[LinhaOCR]:
    """Linhas do OCR a partir de (topo, texto); cada uma com 1% de altura."""
    return [LinhaOCR(texto, topo, topo + 0.01) for topo, texto in pares]


QUESTAO_1 = {
    "Numero": 1,
    "Blocos": [{"Tipo": "texto", "Texto": "não lhes sobra tempo para examinar o passado."}],
    "Alternativas": [{"Letra": letra, "Blocos": [{"Texto": letra}]} for letra in "ABCDE"],
    "Completa": True,
}

# A página 2 do caderno F06: aviso, texto do Sêneca, fonte, questões 1 a 5.
PAGINA_DO_SENECA = _linhas(
    (0.02, "Caderno de Prova 'F06', Tipo 004"),
    (0.05, "CONHECIMENTOS GERAIS"),
    (0.07, "Língua Portuguesa"),
    (0.09, "Atenção: Considere o texto do filósofo Sêneca para responder às questões de 1 a 10."),
    (0.11, "A vida divide-se em três períodos: o que se foi, o que está sendo e o que há de vir."),
    (0.30, "a vida deles desaparece num abismo, e, tal como de nada adianta verter líquido"),
    (0.34, "(Adaptado de: Sêneca. Sobre a brevidade da vida. São Paulo: Companhia das Letras)"),
    (0.37, "1. não lhes sobra tempo para examinar o passado."),
    (0.40, "(A) concessão."),
    (0.86, "5. Sêneca lança mão da figura de linguagem denominada antítese no seguinte trecho:"),
    (0.97, "2 TJUCE-Conhec.Gerais"),
)


@pytest.mark.parametrize(
    ("linhas", "esperado"),
    [
        # As questões vêm depois da fonte, até o pé da região.
        (PAGINA_DO_SENECA, [(0.35, 1.0)]),
        # Dois textos: as questões do primeiro ficam até o aviso do segundo.
        (
            _linhas(
                (0.05, "Considere o texto abaixo para responder às questões de 1 a 3."),
                (0.10, "Texto um."),
                (0.20, "(Adaptado de: A.)"),
                (0.25, "1. Primeira."),
                (0.50, "Considere o texto abaixo para responder às questões de 4 e 5."),
                (0.55, "Texto dois."),
                (0.70, "(Adaptado de: B.)"),
                (0.75, "4. Quarta."),
            ),
            [(0.21, 0.50), (0.71, 1.0)],
        ),
        # O aviso e a fonte quebrados em duas linhas.
        (
            _linhas(
                (0.10, "Atenção: Considere o texto abaixo para responder às"),
                (0.12, "questões de 1 a 3."),
                (0.20, "Texto."),
                (0.30, "(Adaptado de: Fulano. Livro. São Paulo:"),
                (0.32, "Editora, 2017)"),
                (0.40, "1. Primeira."),
            ),
            [(0.33, 1.0)],
        ),
        # Texto sem fonte: vai até a primeira questão.
        (
            _linhas(
                (0.10, "Considere o texto para responder às questões 1 e 2."),
                (0.20, "Texto sem fonte."),
                (0.40, "1. Primeira."),
            ),
            [(0.40, 1.0)],
        ),
        # Sem texto de apoio, a recusa foi por outra coisa: nada a reler.
        (_linhas((0.10, "1. Primeira."), (0.50, "2. Segunda.")), []),
    ],
)
def test_faixas_de_questoes(linhas: list[LinhaOCR], esperado: list[tuple[float, float]]) -> None:
    got = pipeline.faixas_de_questoes(linhas)

    assert [(round(a, 2), round(b, 2)) for a, b in got] == esperado


async def test_regiao_recusada_rele_as_questoes_sem_o_texto(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A IA recusa a página do Sêneca inteira — o texto e as questões que o
    citam; lida só a faixa abaixo da fonte, as questões vêm. Antes, as cinco se
    perdiam e o curador tinha de digitá-las."""
    monkeypatch.setattr(
        pipeline, "ocr_image_com_linhas", lambda png, s: (CAPA_E_TEXTO, PAGINA_DO_SENECA)
    )
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="1")
    provider = FakeProvider(
        ProviderRefused("RECITATION"),
        ProviderRefused("RECITATION"),
        {"Questoes": [QUESTAO_1]},
    )

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    assert len(provider.pedidos) == 3
    assert [q.numero for q in result.questoes] == [1]
    assert result.questoes[0].apoios == ["r1-ocr1"]
    assert result.apoios[0].id == "r1-ocr1" and result.apoios[0].questoes == list(range(1, 11))
    assert "as questões 1 foram lidas à parte" in result.alertas[0]


async def test_faixa_recusada_de_novo_fica_para_o_curador(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(
        pipeline, "ocr_image_com_linhas", lambda png, s: (CAPA_E_TEXTO, PAGINA_DO_SENECA)
    )
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="1")
    recusa = ProviderRefused("RECITATION")
    provider = FakeProvider(recusa, recusa, recusa, recusa)

    result = await pipeline.extrair(tmp_path, id, origin, provider, Settings(provas_dir=tmp_path))

    assert len(provider.pedidos) == 4  # região inteira duas vezes, a faixa duas
    # A questão 1 que a faixa não trouxe sai do OCR, com o (A) que ele leu, e
    # fica incompleta para a releitura; o texto continua o de apoio.
    q = result.questoes[0]
    assert (q.numero, q.lida_por_ocr, q.completa) == (1, True, False)
    assert [a.letra for a in q.alternativas] == ["A"] and result.apoios[0].id == "r1-ocr1"
    assert "as questões 1 vieram do OCR" in result.alertas[0]


async def test_apoio_guarda_o_aviso_e_a_area_no_original(tmp_path: Path) -> None:
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 100, 500, 600], regiao="0")
    apoio = {
        "Id": "t1",
        "Blocos": [
            {"Texto": "Atenção: Considere o texto abaixo para responder às questões de 1 a 3."},
            {"Texto": "A vida divide-se em três períodos."},
        ],
        "Questoes": [1],
        "Retangulo": [200, 100, 700, 600],
    }

    result = await pipeline.extrair(
        tmp_path, id, origin, FakeProvider({"Apoios": [apoio]}), Settings(provas_dir=tmp_path)
    )

    [lido] = result.apoios
    assert lido.aviso == "Considere o texto abaixo para responder às questões de 1 a 3."
    assert lido.questoes == [1, 2, 3]
    assert [b.texto for b in lido.blocos] == ["A vida divide-se em três períodos."]
    assert lido.origens[0].retangulo == [40, 190, 310, 460]


def pagina_com_grafico(root: Path) -> str:
    """Uma coluna de prosa com um gráfico no meio, como nas provas: título
    centrado, moldura com barras e rótulos embaixo."""
    id = str(uuid.uuid4())
    linha = ("considere o grafico abaixo para responder a questao sobre a media " * 3)[:100]
    with pymupdf.open() as doc:
        p = doc.new_page(width=595, height=842)
        for y in (230, 242, 254, 460, 472):
            p.insert_text((60, y), linha, fontsize=9)
        p.insert_text((265, 300), "Gols por jogo", fontsize=10)
        p.draw_rect(pymupdf.Rect(220, 310, 380, 420), color=(0, 0, 0), width=1)
        for i, altura in enumerate((60, 90, 40)):
            x = 240 + i * 45
            p.draw_rect(
                pymupdf.Rect(x, 420 - altura, x + 25, 420), color=(0, 0, 0), fill=(0.2, 0.2, 0.2)
            )
        p.insert_text((245, 432), "Ana   Bia   Caio", fontsize=8)
        doc.save(arquivo(root, id, "pdf"))
    return id


PAGINA = [0, 0, 595, 842]


def test_ajuste_tira_a_prosa_que_o_modelo_pegou(tmp_path: Path) -> None:
    """A caixa do modelo pegou linhas de texto em cima e embaixo; o recorte
    fica com o título, o gráfico e os rótulos."""
    from app.provas.documentos import ajustar_figura

    id = pagina_com_grafico(tmp_path)
    grande = Origem(pagina=1, retangulo=[150, 222, 450, 478], regiao="0")

    x0, y0, x1, y1 = ajustar_figura(tmp_path, id, grande, PAGINA, Settings()).retangulo

    assert 256 < y0 < 292, "o topo fica entre a última linha de texto e o título"
    assert 433 < y1 < 451, "o fundo fica entre os rótulos e a linha de texto de baixo"
    assert x0 < 222 and x1 > 378, "a moldura inteira"


def test_ajuste_devolve_o_titulo_que_o_modelo_cortou(tmp_path: Path) -> None:
    from app.provas.documentos import ajustar_figura

    id = pagina_com_grafico(tmp_path)
    cortada = Origem(pagina=1, retangulo=[215, 315, 385, 425], regiao="0")

    _, y0, _, y1 = ajustar_figura(tmp_path, id, cortada, PAGINA, Settings()).retangulo

    assert y0 < 292, "o título volta"
    assert 433 < y1 < 451, "os rótulos entram, a prosa não"


def test_ajuste_sem_figura_mantem_a_caixa(tmp_path: Path) -> None:
    from app.provas.documentos import ajustar_figura

    id = pagina_com_grafico(tmp_path)
    vazia = Origem(pagina=1, retangulo=[300, 600, 500, 800], regiao="0")

    assert ajustar_figura(tmp_path, id, vazia, PAGINA, Settings()) == vazia


async def test_numero_sem_questao_nao_entra(tmp_path: Path) -> None:
    """Na região do texto de apoio, o modelo listava as questões que o texto
    cita como questões vazias; marcadas como completas, venciam a leitura de
    verdade da região seguinte."""
    id = pdf(tmp_path)
    origin = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="0")
    vazias = [{"Numero": n, "Blocos": [], "Alternativas": []} for n in (9, 10)]

    result = await pipeline.extrair(
        tmp_path,
        id,
        origin,
        FakeProvider({"Questoes": [QUESTAO_44, *vazias]}),
        Settings(provas_dir=tmp_path),
    )

    assert [q.numero for q in result.questoes] == [44]


# A página 11 do caderno F06: a 41 no meio, a 42 embaixo, até o rodapé.
PAGINA_DA_42 = _linhas(
    (0.02, "Caderno de Prova 'F06', Tipo 004"),
    (0.49, "41. Um Tribunal de Justiça mantém uma consulta pública em Spring Boot"),
    (0.60, "(A) configurar @BatchSize nas coleções"),
    (0.74, "42. Em um Tribunal de Justiça, um portal de serviços digitais permite"),
    (0.80, "1. uma lista dentro da questão não fecha a questão"),
    (0.84, "(A) expor Circuit Breaker como catálogo de serviços"),
    (0.97, "TJUCE-An.Jud.-Tec.Inf.-Sistemas-F06 11"),
)


def test_area_da_questao() -> None:
    assert pipeline.area_da_questao(PAGINA_DA_42, 42) == (0.74, 1.0)
    assert pipeline.area_da_questao(PAGINA_DA_42, 41) == (0.49, 0.74)
    assert pipeline.area_da_questao(PAGINA_DA_42, 43) is None


def test_releitura_acha_a_questao_na_pagina_vizinha(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A releitura da 42 foi mandada à página 12, onde ela não está: o número
    impresso a acha na 11, e a região vira só a questão."""
    id = str(uuid.uuid4())
    with pymupdf.open() as doc:
        for _ in range(3):
            doc.new_page(width=600, height=1000)
        doc.save(arquivo(tmp_path, id, "pdf"))
    lidas = iter([_linhas((0.1, "43. Outra questão")), PAGINA_DA_42])
    monkeypatch.setattr(pipeline, "ocr_image_com_linhas", lambda png, s: ("", next(lidas)))
    pedida = Origem(pagina=3, retangulo=[0, 0, 600, 1000], regiao="q42")

    achada = pipeline.localizar_questao(tmp_path, id, pedida, 42, Settings(provas_dir=tmp_path))

    assert achada.pagina == 2 and achada.regiao == "q42"
    assert [round(v) for v in achada.retangulo] == [0, 732, 600, 996]


async def test_releitura_le_so_a_questao(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    id = pdf(tmp_path)
    so_a_questao = Origem(pagina=1, retangulo=[0, 400, 595, 800], regiao="q42")
    monkeypatch.setattr(pipeline, "localizar_questao", lambda *a: so_a_questao)
    pedida = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="q42")

    result = await pipeline.extrair(
        tmp_path,
        id,
        pedida,
        FakeProvider({"Questoes": [QUESTAO_1]}),
        Settings(provas_dir=tmp_path),
        questao=42,
    )

    assert result.questoes[0].origens == [so_a_questao]


# A página que a IA recusou inteira, sem texto de apoio: antes, o OCR virava um
# texto de apoio e as três questões faltavam.
PAGINA_RECUSADA = _linhas(
    (0.02, "Caderno de Prova 'E05', Tipo 004"),
    (0.05, "27. Um órgão usa uma solução de segurança. Uma implicação é"),
    (0.09, "(A) centralizar."),
    (0.12, "(B) isolar."),
    (0.15, "(C) duplicar."),
    (0.18, "(D) remover."),
    (0.21, "(E) auditar."),
    (0.30, "28. Considere a política de cópias. Ela deve"),
    (0.34, "(A) ser diária."),
    (0.37, "(B) ser semanal."),
    (0.40, "(C) ser mensal."),
    (0.43, "(D) ser anual."),
    (0.46, "(E) ser dispensada."),
)


async def test_recusa_dupla_sem_texto_de_apoio_tira_as_questoes_do_ocr(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(pipeline, "ocr_image_com_linhas", lambda png, s: ("OCR", PAGINA_RECUSADA))
    id = pdf(tmp_path)
    origem = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="5")
    recusa = ProviderRefused("RECITATION")

    result = await pipeline.extrair(
        tmp_path, id, origem, FakeProvider(recusa, recusa), Settings(provas_dir=tmp_path)
    )

    assert [(q.numero, q.completa, q.lida_por_ocr) for q in result.questoes] == [
        (27, True, True),
        (28, True, True),
    ]
    assert result.questoes[1].alternativas[4].blocos[0].texto == "ser dispensada."
    # A área de cada questão é a dela na região, não a página inteira.
    y0, y1 = result.questoes[0].origens[0].retangulo[1::2]
    assert 30 < y0 < 60 and 160 < y1 < 200
    # O OCR era das questões: não vira texto de apoio.
    assert result.apoios == []
    assert "as questões 27-28 vieram do OCR" in result.alertas[0]


async def test_releitura_recusada_fica_com_a_questao_do_ocr(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A releitura da questão 28 é a região só dela: o número na caixa da margem
    não sai no OCR, e a leitura sem número é a pedida."""
    so_a_28 = [linha for linha in PAGINA_RECUSADA if linha.topo >= 0.30]
    so_a_28[0] = LinhaOCR("Considere a política de cópias. Ela deve", 0.30, 0.32)
    monkeypatch.setattr(pipeline, "ocr_image_com_linhas", lambda png, s: ("OCR", so_a_28))
    monkeypatch.setattr(pipeline, "localizar_questao", lambda root, doc, origem, n, s: origem)
    id = pdf(tmp_path)
    origem = Origem(pagina=1, retangulo=[0, 0, 595, 800], regiao="q28")
    recusa = ProviderRefused("RECITATION")

    result = await pipeline.extrair(
        tmp_path,
        id,
        origem,
        FakeProvider(recusa, recusa),
        Settings(provas_dir=tmp_path),
        questao=28,
    )

    assert [(q.numero, q.completa, q.lida_por_ocr) for q in result.questoes] == [(28, True, True)]
