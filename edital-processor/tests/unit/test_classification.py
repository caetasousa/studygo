from __future__ import annotations

from app.domain.blocos import Bloco
from app.services.classification import classify_document, classify_page


def test_empty_page_is_irrelevant() -> None:
    assert classify_page("") == [Bloco.IRRELEVANTE]
    assert classify_page("   \n ") == [Bloco.IRRELEVANTE]


def test_cover_page_is_dados_gerais() -> None:
    text = (
        "TRIBUNAL DE CONTAS DO ESTADO DE GOIAS\n"
        "EDITAL Nº 01/2026 - ABERTURA DE INSCRICOES\n"
        "INSTRUCOES ESPECIAIS\n"
        "1. DAS DISPOSICOES PRELIMINARES"
    )
    assert Bloco.DADOS_GERAIS in classify_page(text)


def test_vagas_table_page_is_cargos_vagas() -> None:
    text = (
        "Codigo de Opcao | Cargo/Especialidade | Escolaridade/Pre-requisito | "
        "Nº Total de Vagas\n"
        "A01 | Tecnico de Controle Externo - Especialidade: Tecnico Administrativo | 6\n"
        "Vencimento inicial: R$ 11.862,19"
    )
    assert Bloco.CARGOS_VAGAS in classify_page(text)


def test_provas_page_is_estrutura_provas() -> None:
    text = (
        "7. DAS PROVAS\n"
        "Provas Objetivas: Conhecimentos Gerais 25 questoes Conhecimentos "
        "Especificos 45 questoes\n"
        "Prova Discursiva - Redacao"
    )
    assert Bloco.ESTRUTURA_PROVAS in classify_page(text)


def test_legal_boilerplate_mentioning_concurso_is_not_dados_gerais() -> None:
    # Real page 3: numbered sub-items and payment prose. "concurso publico"
    # appears inside a URL — must not match.
    text = (
        "internet ou por meio dos caixas eletronicos, os pagamentos realizados "
        "foram desses horarios. Nao serao consideradas as inscricoes cujo "
        "pagamento tenha sido efetuado por meio do Boleto Bancario gerado fora "
        "do endereco eletronico (www.concursosfcc.com.br) ou fora do prazo."
    )
    assert classify_page(text) == [Bloco.IRRELEVANTE]


def test_syllabus_sweep_carries_across_pages_under_one_heading() -> None:
    pages = [
        "1. DAS DISPOSICOES PRELIMINARES\nprosa legal do edital",  # 1
        "ANEXO II\nCONTEUDO PROGRAMATICO\nLingua Portuguesa\nRedacao Oficial.",  # 2 start
        "Matematica e Raciocinio Logico\nNumeros inteiros e racionais.",  # 3 carried
        "Engenharia de Software\nFundamentos da engenharia de software.",  # 4 carried
        "ANEXO III\nCRONOGRAMA DAS PROVAS E PUBLICACOES\nDatas Previstas",  # 5 new section
        "Item Atividades\n05/10/2026 a 06/11/2026 Periodo das inscricoes",  # 6 carried
    ]
    blocks = classify_document(pages)
    assert Bloco.CONTEUDO_PROGRAMATICO not in blocks[0]
    assert Bloco.CONTEUDO_PROGRAMATICO in blocks[1]
    assert Bloco.CONTEUDO_PROGRAMATICO in blocks[2]
    assert Bloco.CONTEUDO_PROGRAMATICO in blocks[3]
    # the Anexo III heading closes the syllabus run
    assert Bloco.CONTEUDO_PROGRAMATICO not in blocks[4]
    assert Bloco.CRONOGRAMA in blocks[4]
    assert Bloco.CRONOGRAMA in blocks[5]


def test_signature_block_closes_an_open_section() -> None:
    pages = [
        "ANEXO II\nCONTEUDO PROGRAMATICO\nLingua Portuguesa",
        "Governanca de TI\nEstrategia e alinhamento",
        "Goiania, 24 de agosto de 2026\nPresidente da Comissao do Concurso",
        "algum texto solto de rodape",
    ]
    blocks = classify_document(pages)
    assert Bloco.CONTEUDO_PROGRAMATICO in blocks[1]
    assert blocks[3] == [Bloco.IRRELEVANTE]


def test_blank_page_resets_the_sweep() -> None:
    pages = [
        "ANEXO II\nCONTEUDO PROGRAMATICO",
        "",
        "prosa que nao e conteudo",
    ]
    blocks = classify_document(pages)
    assert blocks[1] == [Bloco.IRRELEVANTE]
    assert Bloco.CONTEUDO_PROGRAMATICO not in blocks[2]
