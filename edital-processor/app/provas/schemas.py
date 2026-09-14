"""Contrato interno das provas com o backend.

Os aliases são os nomes dos campos do domínio em Go (Numero, Blocos,
Retangulo…): o backend decodifica a resposta direto em prova.Rascunho. Quem
renomear um campo lá precisa renomear aqui.

Há dois conjuntos de modelos. Os "Lido" são o que se pede ao Gemini — só o que
está na página, sem campo que convide o modelo a resolver a questão ou a
marcar revisão. Os demais são o rascunho que volta ao backend.
"""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field


def _alias(nome: str) -> str:
    return "".join(parte.capitalize() for parte in nome.split("_"))


class Modelo(BaseModel):
    model_config = ConfigDict(alias_generator=_alias, populate_by_name=True)


def _opcoes(*valores: str) -> Any:
    # Enum só no schema enviado ao Gemini: um valor fora da lista não derruba
    # a região inteira na validação, só vira o padrão na normalização.
    return Field(default=valores[0], json_schema_extra={"enum": list(valores)})


# --- o que o Gemini devolve ----------------------------------------------------


class BlocoLido(Modelo):
    tipo: str = _opcoes("texto", "codigo", "imagem")
    texto: str = ""
    formato: str = _opcoes("", "negrito", "italico", "sublinhado")
    descricao: str = ""
    # [ymin, xmin, ymax, xmax] normalizados de 0 a 1000 sobre a imagem enviada.
    retangulo: list[float] | None = None


class AlternativaLida(Modelo):
    letra: str
    blocos: list[BlocoLido] = Field(default_factory=list)


class QuestaoLida(Modelo):
    numero: int = Field(ge=1)
    disciplina: str = ""
    blocos: list[BlocoLido] = Field(default_factory=list)
    alternativas: list[AlternativaLida] = Field(default_factory=list)
    completa: bool = True
    # Onde a questão está na imagem, para a revisão mostrá-la ao lado do
    # rascunho: [ymin, xmin, ymax, xmax] normalizados de 0 a 1000.
    retangulo: list[float] | None = None


class ApoioLido(Modelo):
    id: str
    blocos: list[BlocoLido] = Field(default_factory=list)
    questoes: list[int] = Field(default_factory=list)
    # Onde o texto está na imagem, com aviso e fonte: [ymin, xmin, ymax, xmax].
    retangulo: list[float] | None = None


class RegiaoLida(Modelo):
    questoes: list[QuestaoLida] = Field(default_factory=list)
    apoios: list[ApoioLido] = Field(default_factory=list)


class QuestaoParaClassificar(Modelo):
    numero: int
    secao: str = ""
    texto: str = ""


class MateriaDaQuestao(Modelo):
    numero: int
    materia: str


class Classificacao(Modelo):
    materias: list[MateriaDaQuestao] = Field(default_factory=list)


class Metadados(Modelo):
    orgao: str = ""
    ano: int = 0
    # O código ("F06") confere o gabarito; o nome é o que o aluno reconhece.
    cargo: str = ""
    cargo_nome: str = ""
    caderno: str = ""
    total: int = 0


# --- o rascunho que volta ao backend -------------------------------------------


class Origem(Modelo):
    pagina: int = Field(ge=1)
    retangulo: list[float] = Field(min_length=4, max_length=4)
    regiao: str = ""


class Bloco(Modelo):
    tipo: str = "texto"
    texto: str = ""
    formato: str = ""
    arquivo: str = ""
    descricao: str = ""
    origem: Origem | None = None
    revisado: bool = False
    # Tamanho da figura na tela; quem escolhe é o curador, o processador nunca.
    largura: int = 0


class Alternativa(Modelo):
    letra: str
    blocos: list[Bloco] = Field(default_factory=list)


class Questao(Modelo):
    numero: int = Field(ge=1)
    disciplina: str = ""
    blocos: list[Bloco] = Field(default_factory=list)
    alternativas: list[Alternativa] = Field(default_factory=list)
    apoios: list[str] = Field(default_factory=list)
    origens: list[Origem] = Field(default_factory=list)
    resposta: str = ""
    situacao: str = ""
    revisada: bool = False
    completa: bool = True


class Apoio(Modelo):
    id: str
    blocos: list[Bloco] = Field(default_factory=list)
    questoes: list[int] = Field(default_factory=list)
    # A frase do caderno que diz quais questões usam o texto; sai do texto e
    # fica aqui para o curador conferir a faixa.
    aviso: str = ""
    origens: list[Origem] = Field(default_factory=list)
    revisado: bool = False


class Gabarito(Modelo):
    cargo: str = ""
    caderno: str = ""
    tipo: str = ""
    respostas: dict[str, str] = Field(default_factory=dict)
    situacoes: dict[str, str] = Field(default_factory=dict)


class Extracao(Modelo):
    """Uma chamada de extração: modelo, consumo e a versão do prompt que a fez."""

    modelo: str = ""
    tokens_entrada: int = 0
    tokens_saida: int = 0
    regiao: str = ""
    prompt: str = ""
    versao: str = ""


class Rascunho(Modelo):
    extracoes: list[Extracao] = Field(default_factory=list)
    questoes: list[Questao] = Field(default_factory=list)
    apoios: list[Apoio] = Field(default_factory=list)
    alertas: list[str] = Field(default_factory=list)
