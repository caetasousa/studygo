"""O que tem de ser verdade antes de a lei poder ser publicada.

Cada verificação devolve problemas em português. O salto de numeração
(`SALTO`) pode ser legítimo — a lei pulou mesmo um número — e vira aviso para a
pessoa conferir; o resto impede a publicação.
"""

from __future__ import annotations

import hashlib
import re

from app.leis.limpeza import Paragrafo, normalizar, texto_visivel
from app.leis.montar import Montagem

SALTO = "artigo fora de sequência: "
_REF_ARTIGO = re.compile(r"^(?:(adct)\.)?art(\d+)(?:-([a-z]{1,2}))?$")


def _sha(texto: str) -> str:
    return hashlib.sha256(texto.encode("utf-8")).hexdigest()


def texto_remontado(montagem: Montagem) -> str:
    pedacos: list[str] = []
    for d in montagem.dispositivos:
        pedacos.extend(t for t in (d.texto, d.nome) if t)
    return normalizar(" ".join(pedacos))


def texto_dos_paragrafos(paragrafos: list[Paragrafo], montagem: Montagem) -> str:
    descartados = set(montagem.ids_descartados)
    return normalizar(
        " ".join(
            p.texto for p in paragrafos if not p.anterior and p.texto and p.id not in descartados
        )
    )


def _na_ordem(html: str, paragrafos: list[Paragrafo]) -> list[str]:
    visivel = texto_visivel(html)
    cursor = 0
    problemas: list[str] = []
    for p in paragrafos:
        for pedaco in p.pedacos or (p.texto,):
            if not pedaco:
                continue
            achado = visivel.find(pedaco, cursor)
            if achado < 0:
                problemas.append(
                    f"{p.id}: texto fora do original, ou fora de ordem: {pedaco[:80]!r}"
                )
                break
            cursor = achado + len(pedaco)
    return problemas


def sequencia(montagem: Montagem) -> list[str]:
    """Os artigos vigentes crescem de um em um (5, 5-A, 6); um salto só passa
    calado se o artigo que falta existe como revogado.

    Os revogados ficam fora da ordem de propósito: o Planalto põe o artigo
    incluído por medida provisória e depois revogado onde ele entrou na
    história (55-K antes do 55-A), não onde a numeração o colocaria.
    """
    artigos: list[tuple[str, int, str, str, bool]] = []
    for d in montagem.dispositivos:
        m = _REF_ARTIGO.match(d.ref) if d.tipo == "artigo" else None
        if m:
            revogado_sem_texto = d.revogado and not d.texto
            artigos.append((m.group(1) or "", int(m.group(2)), m.group(3) or "", d.ref,
                            revogado_sem_texto))  # fmt: skip
    revogados = {(dom, n) for dom, n, _, _, sem_texto in artigos if sem_texto}

    problemas: list[str] = []
    ultimo: dict[str, tuple[int, str, str]] = {}
    for dominio, numero, sufixo, ref, sem_texto in artigos:
        if sem_texto:
            continue
        antes = ultimo.get(dominio)
        n0, s0 = (antes[0], antes[1]) if antes else (0, "")
        cresce = (numero, sufixo) > (n0, s0)
        faltam = [n for n in range(n0 + 1, numero) if (dominio, n) not in revogados]
        if not cresce or faltam:
            problemas.append(f"{SALTO}{antes[2] if antes else 'início'} → {ref}")
        ultimo[dominio] = (numero, sufixo, ref)
    return problemas


def verificar(
    html: str | None,
    paragrafos: list[Paragrafo],
    montagem: Montagem,
) -> list[str]:
    problemas = list(montagem.problemas)
    if _sha(texto_remontado(montagem)) != _sha(texto_dos_paragrafos(paragrafos, montagem)):
        problemas.append(
            "o texto remontado da árvore difere do texto dos parágrafos "
            "(sha256 diferente): algum dispositivo perdeu, ganhou ou trocou texto"
        )
    if html is not None:
        problemas.extend(_na_ordem(html, paragrafos))
    problemas.extend(sequencia(montagem))
    return problemas
