"""As questões de uma região lidas do OCR, sem IA: o último recurso quando o
Gemini recusa a região — quase sempre por recitação. Antes, o que sobrava era
o OCR inteiro como um texto de apoio, e as questões da região faltavam.

O caderno da FCC desenha a questão sempre igual: o enunciado e as alternativas
"(A)" a "(E)", cada uma começando a linha. É por esse desenho que as questões
saem, e não pelo número: em muitos cadernos ele fica numa caixa à margem que o
OCR não lê. Quando o número aparece — "27. Um Tribunal…" ou "18." numa linha à
parte, na altura do enunciado —, ele vale; quando não, sai das vizinhas.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

from app.services.ocr import LinhaOCR

# O número da questão no começo da linha: "27. Um Tribunal", "27) Um", ou só
# "18." numa linha à parte — com o 1 que o OCR lê como "t", "l" ou "I" ("t0.").
_NUMERO = re.compile(r"^\s*((?=[tlIO]?\d)[\dtlIO]{1,3})\s*[.)](?:\s+|$)")
_DIGITO_DO_OCR = str.maketrans("tlIO", "1110")
# A letra da alternativa no começo da linha, com as trocas do OCR: "(AJ",
# "(B]", "(Cj", "(Cc)", "{D)", "(4)" no lugar do "(A)", e "A)" quando o
# parêntese de abrir sai numa linha à parte.
_MARCA = re.compile(
    r"^\s*(?:[(\[{]\s*([A-E4ÀÁ])[a-z]?\s*[)\]}jJI|l1]|([A-E])\))(?:\s*[)|\]])?"
    r"\s*[|:;.\-\u2014\u2013]*\s*"
)
_LETRA_DO_OCR = str.maketrans("4ÀÁ", "AAA")
# A alternativa seguinte na mesma linha, quando as alternativas são curtas e a
# prova as põe lado a lado: "(A) inflexível. (B) certo. (C) breve."
_MARCA_NO_MEIO = re.compile(r"\s[(\[{]\s*([B-E])\s*[)\]}jJ]\s")
# Aviso de texto de apoio: "Atenção: Para responder às questões de números 11
# a 20, baseie-se no texto a seguir."
_AVISO = re.compile(
    r"quest(?:ão|ões|oes)\s+(?:de\s+)?(?:n[úu]meros?\s+)?\d{1,3}\s*(?:a|e|até)\s*\d{1,3}",
    re.IGNORECASE,
)
_FONTE = re.compile(
    r"\((?:texto\s+)?(?:adaptado\s+de|dispon[íi]vel\s+em|fonte|extra[íi]do\s+de)\b", re.IGNORECASE
)
# O cabeçalho de cada página: "Caderno de Prova '24', Tipo 001", como o OCR o lê.
_CABECALHO = re.compile(r"ca\w{1,3}r?n?\w{0,2}\s+(?:de|se)\s+prova|\btipo\s+0\d\d\b", re.IGNORECASE)
# A linha está na mesma altura da anterior: o número à margem e o enunciado.
_MESMA_ALTURA = 0.012
# Espaço entre duas linhas maior que esta fração da altura da linha é outro
# parágrafo: é onde a alternativa (E) termina e a próxima questão começa.
_PARAGRAFO = 0.6
# Quanto o número pode pular de uma questão para a seguinte na mesma região.
_SALTO = 3


@dataclass
class QuestaoDoTexto:
    """Numero é 0 quando nem a questão nem as vizinhas o trouxeram."""

    numero: int
    enunciado: str
    alternativas: list[tuple[str, str]] = field(default_factory=list)
    topo: float = 0.0
    pe: float = 1.0

    @property
    def completa(self) -> bool:
        return [letra for letra, _ in self.alternativas] == list("ABCDE") and all(
            texto for _, texto in self.alternativas
        )


@dataclass
class _Linha:
    texto: str
    topo: float
    pe: float
    numero: int = 0
    letra: str = ""


def _separar_alternativas(linhas: list[LinhaOCR]) -> list[LinhaOCR]:
    """A linha com várias alternativas vira uma linha por alternativa. Só na
    linha que já começa por uma, e só com as letras seguintes, em ordem: "(A)"
    citado no meio de um enunciado continua texto."""
    out: list[LinhaOCR] = []
    for linha in linhas:
        comeco = _MARCA.match(linha.texto)
        if not comeco:
            out.append(linha)
            continue
        letra = (comeco[1] or comeco[2]).translate(_LETRA_DO_OCR)
        cortes = [0]
        for m in _MARCA_NO_MEIO.finditer(linha.texto, comeco.end()):
            if m[1] == chr(ord(letra) + 1):
                cortes.append(m.start() + 1)
                letra = m[1]
        for de, ate in zip(cortes, [*cortes[1:], len(linha.texto)], strict=True):
            out.append(LinhaOCR(linha.texto[de:ate], linha.topo, linha.pe))
    return out


def _preparar(linhas: list[LinhaOCR]) -> list[_Linha]:
    out: list[_Linha] = []
    for original in _separar_alternativas(linhas):
        texto = original.texto.strip()
        if not texto or _CABECALHO.search(texto):
            continue
        # Linha sem palavra nem número é lixo: código de barras, borda de caixa.
        if sum(c.isalnum() for c in texto) < 2 and not _MARCA.match(texto):
            continue
        linha = _Linha(texto, original.topo, original.pe)
        if m := _NUMERO.match(texto):
            linha.numero = int(m[1].translate(_DIGITO_DO_OCR))
            linha.texto = texto[m.end() :].strip()
        elif m := _MARCA.match(texto):
            letra = (m[1] or m[2]).translate(_LETRA_DO_OCR)
            linha.letra, linha.texto = letra, texto[m.end() :].strip()
        # O número à parte que o OCR pôs depois da primeira linha do enunciado.
        if (
            linha.numero
            and not linha.texto
            and out
            and not out[-1].numero
            and not out[-1].letra
            and abs(out[-1].topo - linha.topo) < _MESMA_ALTURA
        ):
            out[-1].numero = linha.numero
            continue
        # O número sozinho na linha dá nome à linha seguinte, na mesma altura.
        if out and out[-1].numero and not out[-1].texto and not linha.letra:
            out[-1].texto, out[-1].pe = linha.texto, linha.pe
            if linha.numero:
                out[-1].numero = linha.numero
            continue
        out.append(linha)
    return out


def questoes_das_linhas(linhas: list[LinhaOCR]) -> list[QuestaoDoTexto]:
    """As questões das linhas do OCR de uma região, em ordem.

    Uma questão é o enunciado — as linhas desde o fim da questão anterior — e
    as alternativas de (A) a (E), em ordem. A (E) continua até outro parágrafo
    ou um número. O que vem antes do primeiro (A) depois de uma alternativa
    solta é o fim de uma questão cortada pela região, e um texto de apoio
    (do aviso até a fonte) não entra no enunciado."""
    preparadas = _preparar(linhas)
    if not preparadas:
        return []
    # A altura de uma linha comum: a (E) com o pé de uma letra maior não pode
    # esticar o espaço que separa dois parágrafos.
    alturas = sorted(linha.pe - linha.topo for linha in preparadas)
    altura = max(alturas[len(alturas) // 2], 1e-6)

    def novo_paragrafo(anterior: _Linha, linha: _Linha) -> bool:
        return linha.topo - anterior.pe > _PARAGRAFO * altura

    prontas: list[QuestaoDoTexto] = []
    enunciado: list[_Linha] = []
    alternativas: list[tuple[str, list[_Linha]]] = []
    numero = 0
    anterior: _Linha | None = None
    no_apoio = False

    def fechar() -> None:
        nonlocal enunciado, alternativas, numero
        if alternativas and alternativas[0][0] == "A":
            partes = [linha for linha in enunciado] + [x for _, ls in alternativas for x in ls]
            prontas.append(
                QuestaoDoTexto(
                    numero,
                    " ".join(linha.texto for linha in enunciado).strip(),
                    [(letra, " ".join(x.texto for x in ls).strip()) for letra, ls in alternativas],
                    min(x.topo for x in partes),
                    max(x.pe for x in partes),
                )
            )
        enunciado, alternativas, numero = [], [], 0

    ultimo = 0
    for linha in preparadas:
        esperada = chr(ord(alternativas[-1][0]) + 1) if alternativas else "A"
        # Número que não segue a questão anterior é lixo do OCR, e número antes do
        # (A) de uma questão numerada é item de lista ("1. …"): os dois são texto.
        # Antes do (A) de linhas sem número — o fim de uma questão cortada, o nome
        # da matéria —, o número começa a questão.
        if linha.numero and (
            (ultimo and not 0 < linha.numero - ultimo <= _SALTO)
            or (numero and not alternativas and not no_apoio)
        ):
            linha.texto, linha.numero = f"{linha.numero}. {linha.texto}".strip(), 0
        if linha.numero:
            ultimo = linha.numero
            fechar()
            enunciado, numero, no_apoio = [linha], linha.numero, False
        elif _AVISO.search(linha.texto) and "texto" in linha.texto.lower():
            fechar()
            no_apoio = True
        elif no_apoio:
            # O texto de apoio acaba na fonte; a questão começa na linha seguinte.
            if _FONTE.search(linha.texto):
                no_apoio, enunciado = False, []
            elif linha.letra == "A":
                # Sem fonte: o enunciado é o último parágrafo antes do (A).
                no_apoio = False
                comeco = next(
                    (
                        k
                        for k in range(len(enunciado) - 1, 0, -1)
                        if novo_paragrafo(enunciado[k - 1], enunciado[k])
                    ),
                    0,
                )
                enunciado = enunciado[comeco:]
                alternativas = [("A", [linha])]
            else:
                enunciado.append(linha)
        elif linha.letra and linha.letra == esperada:
            alternativas.append((linha.letra, [linha]))
        elif linha.letra == "A" and alternativas and alternativas[-1][0] == "E":
            # Um (A) depois da (E) sem parágrafo que os separasse: a questão
            # seguinte começou no maior espaço entre as linhas da (E).
            linhas_da_e = alternativas[-1][1]
            corte = max(
                range(1, len(linhas_da_e) + 1),
                key=lambda k: (
                    linhas_da_e[k].topo - linhas_da_e[k - 1].pe if k < len(linhas_da_e) else -1
                ),
            )
            alternativas[-1] = ("E", linhas_da_e[:corte])
            seguinte = linhas_da_e[corte:]
            fechar()
            enunciado, alternativas = seguinte, [("A", [linha])]
        elif linha.letra and not alternativas:
            # Alternativa solta antes do (A): o fim da questão que a região cortou.
            enunciado = []
        elif alternativas:
            if alternativas[-1][0] == "E" and anterior and novo_paragrafo(anterior, linha):
                fechar()
                enunciado = [linha]
            else:
                alternativas[-1][1].append(linha)
        else:
            enunciado.append(linha)
        anterior = linha
    fechar()

    # Número fora de ordem é leitura errada: fica sem, e sai das vizinhas.
    for k, q in enumerate(prontas):
        antes = max((p.numero for p in prontas[:k] if p.numero), default=0)
        if q.numero and q.numero <= antes:
            q.numero = 0
    # Sem número, a questão ganha o da vizinha: a seguinte da anterior, ou a
    # anterior da seguinte.
    for k, q in enumerate(prontas):
        if not q.numero and k and prontas[k - 1].numero:
            q.numero = prontas[k - 1].numero + 1
    for k in range(len(prontas) - 2, -1, -1):
        if not prontas[k].numero and prontas[k + 1].numero > 1:
            prontas[k].numero = prontas[k + 1].numero - 1
    return prontas
