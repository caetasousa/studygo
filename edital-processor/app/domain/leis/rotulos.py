"""Como um parágrafo de lei se anuncia: "Art. 1º-A", "§ 2º", "IV –", "a)".

Uma implementação só, usada pela limpeza (onde quebrar parágrafo), pela regra
de classificação e pela montagem (rótulo e ref).
"""

from __future__ import annotations

import re
from dataclasses import dataclass

# Letra do sufixo colada ao traço ("1º-A", "9º -A"). Com espaço depois do
# traço é o estilo antigo de separar o rótulo do texto ("Art. 1º - A União"),
# e o "A" é a primeira palavra do artigo.
_SUFIXO = r"(?:[-–]([A-Z]{1,2})(?![A-Za-zÀ-ú]))?"
_ORDINAL = r"(º|°|o(?=[\s.\-–]))?"

# "Art. 5 7." existe na fonte (LGPD): o espaço no meio do número é erro de
# digitação dela, e o número é 57.
ARTIGO = re.compile(rf"^Art\.?\s*(\d+(?: \d+(?=\s*[.º°]))?)\s*{_ORDINAL}\s*\.?\s*{_SUFIXO}")
PARAGRAFO = re.compile(rf"^§\s*(\d+)\s*{_ORDINAL}\s*\.?\s*{_SUFIXO}")
PARAGRAFO_UNICO = re.compile(r"^Par[áa]grafo\s+[úu]nico", re.IGNORECASE)
_ROMANO = r"M{0,3}(?:CM|CD|D?C{0,3})(?:XC|XL|L?X{0,3})(?:IX|IV|V?I{0,3})"
# Com letra ("I-A"), a CF às vezes dispensa o traço antes do texto ("I-A o
# Conselho Nacional de Justiça").
INCISO = re.compile(
    r"^([IVXLCDM](?: ?[IVXLCDM])*)(?:[-–]([A-Z]{1,2})(?:\s*[-–—]\s*|\s+)|\s*[-–—]\s*)"
)
# A redação da EC 45 escreve incisos sem traço: "II processar e julgar". Só o
# numeral seguido de palavra minúscula, para não ler "I" de frase alguma.
INCISO_SEM_TRACO = re.compile(r"^([IVXLCDM]+) (?=[a-zà-ú])")
ALINEA = re.compile(r"^([a-z])(?:-([A-Z]))? ?\)\s*")
ITEM = re.compile(r"^(\d{1,2})(?:[.)]|\s*[-–])\s+")
AGRUPAMENTO = re.compile(
    r"^(PARTE|LIVRO|T[IÍ]TULO|CAP[IÍ]TULO|SE[CÇ][AÃ]O|SUBSE[CÇ][AÃ]O)\s+"
    r"([IVXLCDM]+(?:\s*-\s*[A-Z])?|[ÚU]NIC[OA]|\d+)(?![A-Za-zÀ-ú])\.?",
    re.IGNORECASE,
)
ADCT = re.compile(r"^ATO DAS DISPOSI[CÇ][OÕ]ES CONSTITUCIONAIS TRANSIT[OÓ]RIAS$", re.IGNORECASE)

_TIPO_AGRUPAMENTO = {
    "parte": "parte",
    "livro": "livro",
    "titulo": "titulo",
    "capitulo": "capitulo",
    "secao": "secao",
    "subsecao": "subsecao",
}
_CHAVE_AGRUPAMENTO = {
    "parte": "parte",
    "livro": "livro",
    "titulo": "tit",
    "capitulo": "cap",
    "secao": "sec",
    "subsecao": "subsec",
}


@dataclass(frozen=True)
class Rotulo:
    tipo: str
    rotulo: str
    # A parte da ref que o dispositivo acrescenta à do pai: "art1-a", "inc4".
    chave: str
    # O que vem depois do rótulo no mesmo parágrafo; num cabeçalho, o nome.
    resto: str


def romano(texto: str) -> int | None:
    texto = texto.replace(" ", "").upper()
    if not texto or not re.fullmatch(_ROMANO, texto):
        return None
    valores = {"I": 1, "V": 5, "X": 10, "L": 50, "C": 100, "D": 500, "M": 1000}
    total = 0
    for i, c in enumerate(texto):
        v = valores[c]
        if i + 1 < len(texto) and valores[texto[i + 1]] > v:
            total -= v
        else:
            total += v
    return total


def _sem_acento(texto: str) -> str:
    trocas = str.maketrans("ÍÇÃÚíçãú", "ICAUicau")
    return texto.translate(trocas)


def _sufixo(letra: str | None) -> str:
    return f"-{letra.lower()}" if letra else ""


def ler(texto: str) -> Rotulo | None:
    """O rótulo com que o parágrafo começa, ou None se ele não começa por um."""
    if ADCT.match(texto):
        return Rotulo("parte", "ADCT", "adct", "")
    m = AGRUPAMENTO.match(texto)
    if m:
        tipo = _TIPO_AGRUPAMENTO[_sem_acento(m.group(1)).lower()]
        numero = m.group(2).replace(" ", "")
        letra = ""
        if "-" in numero:
            numero, letra = numero.split("-", 1)
            letra = f"-{letra.lower()}"
        valor = romano(numero)
        if numero.isdigit():
            n = numero
        elif valor is not None:
            n = str(valor)
        elif numero.upper().startswith(("ÚNIC", "UNIC")):
            n = "unico"
        else:
            n = numero.lower()
        rotulo = texto[: m.end()].rstrip(".").strip()
        chave = f"{_CHAVE_AGRUPAMENTO[tipo]}{n}{letra}"
        return Rotulo(tipo, rotulo, chave, texto[m.end() :].strip())
    m = ARTIGO.match(texto)
    if m:
        numero = m.group(1).replace(" ", "")
        ordinal = "º" if m.group(2) else ""
        suf = f"-{m.group(3)}" if m.group(3) else ""
        return Rotulo(
            "artigo",
            f"Art. {numero}{ordinal}{suf}",
            f"art{int(numero)}{_sufixo(m.group(3))}",
            texto[m.end() :].strip(),
        )
    m = PARAGRAFO_UNICO.match(texto)
    if m:
        return Rotulo("paragrafo", "Parágrafo único", "parunico", texto[m.end() :].strip())
    m = PARAGRAFO.match(texto)
    if m:
        ordinal = "º" if m.group(2) else ""
        suf = f"-{m.group(3)}" if m.group(3) else ""
        return Rotulo(
            "paragrafo",
            f"§ {m.group(1)}{ordinal}{suf}",
            f"par{int(m.group(1))}{_sufixo(m.group(3))}",
            texto[m.end() :].strip(),
        )
    m = INCISO.match(texto) or INCISO_SEM_TRACO.match(texto)
    if m:
        valor = romano(m.group(1))
        if valor is not None:
            numeral = m.group(1).replace(" ", "")
            letra_inciso: str | None = m.group(2) if m.re is INCISO else None
            suf = f"-{letra_inciso}" if letra_inciso else ""
            return Rotulo(
                "inciso",
                f"{numeral}{suf}",
                f"inc{valor}{_sufixo(letra_inciso)}",
                texto[m.end() :].strip(),
            )
    m = ALINEA.match(texto)
    if m:
        suf = f"-{m.group(2)}" if m.group(2) else ""
        return Rotulo(
            "alinea",
            f"{m.group(1)}{suf})",
            f"ali{m.group(1)}{_sufixo(m.group(2))}",
            texto[m.end() :].strip(),
        )
    m = ITEM.match(texto)
    if m:
        return Rotulo("item", f"{m.group(1)}.", f"item{int(m.group(1))}", texto[m.end() :].strip())
    return None


def em_maiusculas(texto: str) -> bool:
    letras = [c for c in texto if c.isalpha()]
    return len(letras) >= 3 and all(c.isupper() for c in letras)
