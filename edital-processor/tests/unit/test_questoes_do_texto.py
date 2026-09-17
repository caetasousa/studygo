"""O leitor de questões do OCR, caso a caso. Cada caso é um desenho que a
bateria com os cadernos reais (TJCE, TRF-4, TRT-1, 6, 15, 18 e 21, ALERR e
SCGE-PE) mostrou; os textos são inventados.
"""

from __future__ import annotations

import pytest

from app.provas.questoes_do_texto import QuestaoDoTexto, questoes_das_linhas
from app.services.ocr import LinhaOCR

ALTURA = 0.012


def linhas(*partes: tuple[float, str]) -> list[LinhaOCR]:
    """(topo, texto) → linhas do OCR com a altura de uma linha comum."""
    return [LinhaOCR(texto, topo, topo + ALTURA) for topo, texto in partes]


def questao(numero: int, topo: float, enunciado: str = "Enunciado") -> list[tuple[float, str]]:
    """Uma questão com o número colado no enunciado e uma linha por alternativa."""
    return [
        (topo, f"{numero}. {enunciado} {numero}" if numero else f"{enunciado}"),
        *(
            (topo + 0.02 * (k + 1), f"({letra}) alternativa {letra}")
            for k, letra in enumerate("ABCDE")
        ),
    ]


def resumo(lidas: list[QuestaoDoTexto]) -> list[tuple[int, bool]]:
    return [(q.numero, q.completa) for q in lidas]


def test_duas_questoes_numeradas() -> None:
    lidas = questoes_das_linhas(linhas(*questao(27, 0.05), *questao(28, 0.20)))

    assert resumo(lidas) == [(27, True), (28, True)]
    assert lidas[0].enunciado == "Enunciado 27"
    assert lidas[1].alternativas[2] == ("C", "alternativa C")
    assert lidas[0].topo == pytest.approx(0.05) and lidas[0].pe == pytest.approx(0.162)


@pytest.mark.parametrize(
    "numero_e_enunciado",
    [
        # O número à parte antes do enunciado, na mesma altura.
        [(0.050, "18."), (0.050, "Uma pesquisa com estudantes revelou")],
        # E depois: o OCR põe a caixa da margem atrás da primeira linha.
        [(0.050, "Uma pesquisa com estudantes revelou"), (0.052, "18.")],
    ],
)
def test_numero_numa_linha_a_parte(numero_e_enunciado: list[tuple[float, str]]) -> None:
    alternativas = [(0.07 + 0.02 * k, f"({letra}) {k}") for k, letra in enumerate("ABCDE")]

    lidas = questoes_das_linhas(linhas(*numero_e_enunciado, *alternativas))

    assert resumo(lidas) == [(18, True)]
    assert lidas[0].enunciado == "Uma pesquisa com estudantes revelou"


def test_sem_numero_a_questao_sai_da_vizinha() -> None:
    lidas = questoes_das_linhas(
        linhas(*questao(0, 0.05, "Sem número"), *questao(11, 0.20), *questao(0, 0.35, "Outra"))
    )

    assert resumo(lidas) == [(10, True), (11, True), (12, True)]


def test_sem_nenhum_numero_fica_zero() -> None:
    assert resumo(questoes_das_linhas(linhas(*questao(0, 0.05)))) == [(0, True)]


@pytest.mark.parametrize(
    "marcas",
    [
        ["(AJ", "(B]", "(Cj", "(Dj)", "(E)"],
        ["(4)", "(B)", "(Cc)", "{D)", "(E) |"],
        ["A)", "B)", "C)", "D)", "E)"],
    ],
)
def test_as_trocas_do_ocr_na_letra(marcas: list[str]) -> None:
    partes = [(0.05, "5. Qual é o termo?")]
    partes += [(0.07 + 0.02 * k, f"{marca} opção {k}") for k, marca in enumerate(marcas)]

    lidas = questoes_das_linhas(linhas(*partes))

    assert resumo(lidas) == [(5, True)]
    assert [texto for _, texto in lidas[0].alternativas] == [f"opção {k}" for k in range(5)]


def test_parentese_de_abrir_numa_linha_a_parte() -> None:
    partes = [(0.05, "12. Pode ser considerada paradoxal a expressão:")]
    for k, letra in enumerate("ABCDE"):
        partes += [(0.07 + 0.02 * k, f"{letra}) termo {k}."), (0.0705 + 0.02 * k, "(")]

    assert resumo(questoes_das_linhas(linhas(*partes))) == [(12, True)]


def test_alternativas_lado_a_lado() -> None:
    lidas = questoes_das_linhas(
        linhas(
            (0.05, "3. O presente é"),
            (0.07, "(A) certo. (B) breve. (C) duvidoso."),
            (0.09, "(D) fixo. (E) longo."),
        )
    )

    assert resumo(lidas) == [(3, True)]
    assert lidas[0].alternativas == [
        ("A", "certo."),
        ("B", "breve."),
        ("C", "duvidoso."),
        ("D", "fixo."),
        ("E", "longo."),
    ]


def test_a_alternativa_e_continua_na_linha_seguinte() -> None:
    lidas = questoes_das_linhas(
        linhas(
            *questao(7, 0.05)[:-1],
            (0.15, "(E) a alternativa que não cabe"),
            (0.162, "numa linha só."),
            *questao(8, 0.20),
        )
    )

    assert resumo(lidas) == [(7, True), (8, True)]
    assert lidas[0].alternativas[4] == ("E", "a alternativa que não cabe numa linha só.")


def test_questao_sem_numero_colada_na_e_da_anterior() -> None:
    """Sem o espaço de parágrafo depois da (E), o (A) seguinte mostra que outra
    questão começou — ela ficava dentro da (E) e faltava."""
    lidas = questoes_das_linhas(
        linhas(
            *questao(2, 0.05)[:-1],
            (0.150, "(E) há de vir"),
            (0.164, "No texto, o presente é"),
            (0.18, "(A) breve."),
            (0.20, "(B) certo."),
            (0.22, "(C) fixo."),
            (0.24, "(D) longo."),
            (0.26, "(E) duvidoso."),
        )
    )

    assert resumo(lidas) == [(2, True), (3, True)]
    assert lidas[0].alternativas[4] == ("E", "há de vir")
    assert lidas[1].enunciado == "No texto, o presente é"


def test_texto_de_apoio_nao_entra_no_enunciado() -> None:
    lidas = questoes_das_linhas(
        linhas(
            (0.02, "Atenção: Para responder às questões de números 11 a 20, leia o texto."),
            (0.05, "Uma vela para Dario"),
            (0.08, "Dario vem apressado pela rua."),
            (0.11, "(Adaptado de: Autor. Contos. Editora, 1970)"),
            *questao(11, 0.15, "Segundo o texto,"),
        )
    )

    assert resumo(lidas) == [(11, True)]
    assert lidas[0].enunciado == "Segundo o texto, 11"


def test_texto_de_apoio_sem_fonte_fica_com_o_ultimo_paragrafo() -> None:
    lidas = questoes_das_linhas(
        linhas(
            (0.02, "Considere o texto para responder às questões 4 e 5."),
            (0.05, "Primeiro parágrafo do texto."),
            (0.10, "Segundo o texto, é correto afirmar:"),
            (0.13, "(A) um."),
            (0.15, "(B) dois."),
            (0.17, "(C) três."),
            (0.19, "(D) quatro."),
            (0.21, "(E) cinco."),
        )
    )

    assert [q.enunciado for q in lidas] == ["Segundo o texto, é correto afirmar:"]


def test_lista_numerada_dentro_do_enunciado() -> None:
    lidas = questoes_das_linhas(
        linhas(
            (0.02, "25. Em um Tribunal, três situações ocorreram:"),
            (0.04, "1. instabilidade no sistema;"),
            (0.06, "2. falha na rede;"),
            (0.08, "3. perda de dados."),
            *((0.10 + 0.02 * k, f"({letra}) {k}") for k, letra in enumerate("ABCDE")),
        )
    )

    assert resumo(lidas) == [(25, True)]
    assert "2. falha na rede;" in lidas[0].enunciado


def test_questao_cortada_no_alto_da_regiao_fica_de_fora() -> None:
    lidas = questoes_das_linhas(
        linhas((0.02, "(D) o fim da questão anterior."), (0.04, "(E) cortada."), *questao(9, 0.10))
    )

    assert resumo(lidas) == [(9, True)]
    assert lidas[0].enunciado == "Enunciado 9"


def test_questao_cortada_no_pe_da_regiao_fica_incompleta() -> None:
    assert resumo(questoes_das_linhas(linhas(*questao(30, 0.80)[:4]))) == [(30, False)]


def test_cabecalho_e_lixo_do_ocr_saem() -> None:
    lidas = questoes_das_linhas(
        linhas(
            (0.01, "RR casemo se prova 26, po 00:"),
            (0.02, "Caderno de Prova '24', Tipo 001"),
            (0.03, "|"),
            *questao(4, 0.05),
        )
    )

    assert resumo(lidas) == [(4, True)]
    assert lidas[0].enunciado == "Enunciado 4"


def test_numero_fora_de_ordem_e_lixo() -> None:
    """Um "7." perdido depois da 27 não é a questão 7: a questão seguinte, sem
    número legível, é a 28."""
    lidas = questoes_das_linhas(
        linhas(*questao(27, 0.05), *questao(7, 0.20, "Outra questão"), *questao(29, 0.35))
    )

    assert [q.numero for q in lidas] == [27, 28, 29]


def test_o_um_que_o_ocr_le_como_letra() -> None:
    lidas = questoes_das_linhas(linhas((0.05, "t0. Em sua argumentação,"), *questao(0, 0.05)[1:]))

    assert resumo(lidas) == [(10, True)]
    assert lidas[0].enunciado == "Em sua argumentação,"


def test_regiao_sem_questao() -> None:
    assert questoes_das_linhas(linhas((0.1, "Texto corrido sem alternativas."))) == []
    assert questoes_das_linhas([]) == []
