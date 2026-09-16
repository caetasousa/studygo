"""O contrato das provas com o backend.

O backend decodifica a resposta destas rotas direto em `prova.Rascunho`,
`prova.Metadados` e `prova.Gabarito`, sem camada de tradução: os aliases daqui
são os nomes dos campos do domínio em Go. Isso é rápido e não tinha rede de
proteção — renomear um campo de um lado só aparecia em produção, como uma prova
importada vazia.

Este teste grava em `tests/contrato/provas.json` uma resposta de cada rota, com
todos os campos preenchidos, a partir dos modelos daqui. Do outro lado,
`backend/internal/adapter/provaproc/contrato_test.go` lê o mesmo arquivo e
confere o que o Go entendeu. Mudou o contrato de propósito? Regrave com
ATUALIZAR_CONTRATO=1 e rode os dois lados.
"""

from __future__ import annotations

import json
import os
from pathlib import Path

from app.provas.schemas import (
    Alternativa,
    Apoio,
    Bloco,
    Extracao,
    Gabarito,
    Metadados,
    Origem,
    Questao,
    Rascunho,
)

CONTRATO = Path(__file__).parents[1] / "contrato" / "provas.json"


def _exemplo() -> dict[str, object]:
    origem = Origem(pagina=7, retangulo=[41.6, 27.14, 1050.67, 434.3], regiao="6")
    figura = Bloco(
        tipo="imagem",
        texto="",
        formato="",
        arquivo="2b1c9f7a-0b1e-4f3a-9c77-5a1d2e3f4b55",
        descricao="diagrama do fluxo",
        origem=origem,
        revisado=True,
        largura=60,
    )
    enunciado = Bloco(tipo="texto", texto="Considere o esquema.", formato="negrito")
    questao = Questao(
        numero=24,
        disciplina="Engenharia de Software",
        blocos=[enunciado, figura],
        alternativas=[
            Alternativa(letra=letra, blocos=[Bloco(tipo="texto", texto=f"alternativa {letra}")])
            for letra in "ABCDE"
        ],
        apoios=["r6-t1"],
        origens=[origem],
        resposta="",
        situacao="",
        revisada=False,
        completa=True,
    )
    apoio = Apoio(
        id="r6-t1",
        blocos=[Bloco(tipo="texto", texto="Texto de Sêneca.")],
        questoes=[1, 2, 3],
        aviso="Considere o texto para responder às questões de 1 a 3.",
        origens=[origem],
        revisado=False,
    )
    rascunho = Rascunho(
        extracoes=[
            Extracao(
                modelo="gemini-2.5-flash",
                tokens_entrada=1234,
                tokens_saida=567,
                regiao="6",
                prompt="fcc-v9",
                versao="9",
            )
        ],
        questoes=[questao],
        apoios=[apoio],
        alertas=["Questão 24 foi lida duas vezes com conteúdo diferente (regiões 5 e 6)."],
    )

    return {
        "metadados": Metadados(
            orgao="TJCE",
            ano=2026,
            cargo="F06",
            cargo_nome="Analista Judiciário - Especialidade Sistemas",
            caderno="004",
            total=60,
        ).model_dump(by_alias=True),
        "gabarito": Gabarito(
            cargo="F06",
            caderno="4",
            tipo="preliminar",
            respostas={"1": "B", "2": ""},
            situacoes={"1": "Gabarito sem alteração", "2": "Anulada"},
        ).model_dump(by_alias=True),
        "extrair": rascunho.model_dump(by_alias=True),
    }


def test_contrato_com_o_backend() -> None:
    atual = _exemplo()

    if os.getenv("ATUALIZAR_CONTRATO") == "1":
        CONTRATO.parent.mkdir(parents=True, exist_ok=True)
        texto = json.dumps(atual, indent=2, ensure_ascii=False) + "\n"
        CONTRATO.write_text(texto, encoding="utf-8")

    gravado = json.loads(CONTRATO.read_text(encoding="utf-8"))
    assert gravado == atual, (
        "o contrato das provas mudou; se foi de propósito, regrave com "
        "ATUALIZAR_CONTRATO=1 e rode o teste do backend "
        "(internal/adapter/provaproc) para ver o que o Go deixou de entender"
    )
