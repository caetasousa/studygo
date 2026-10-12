"""Extrai o texto de uma aula em PDF, página por página, sem o que identifica o comprador.

Uso: texto.py aula.pdf saida.txt

O PDF do curso traz no rodapé o nome, o CPF e o e-mail de quem comprou, e às
vezes uma marca escondida no meio das frases (==15ab95==). Nada disso pode
chegar ao mapa: corta-se o cabeçalho e o rodapé de cada página e apaga-se o que
sobrar com cara de CPF, e-mail, marca ou nome do comprador (a variável
MAPA_COMPRADOR, nomes separados por "|"). No fim, diz quanto tirou de cada um.
"""

import os
import re
import sys
from pathlib import Path

import pymupdf

CPF = re.compile(r"\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b")
EMAIL = re.compile(r"[\w.+-]+@[\w-]+\.[\w.-]+")
MARCA = re.compile(r"==[0-9a-fA-F]{6}==")


def main(src, dst):
    nomes = [n.strip() for n in os.environ.get("MAPA_COMPRADOR", "").split("|") if n.strip()]
    contas = {"cpf": 0, "email": 0, "marca": 0, "nome": 0}
    doc = pymupdf.open(src)
    with Path(dst).open("w", encoding="utf-8") as f:
        for i, p in enumerate(doc):
            r = p.rect
            t = p.get_text("text", clip=pymupdf.Rect(0, 48, r.width, r.height - 56), sort=True)
            for chave, rx in (("cpf", CPF), ("email", EMAIL), ("marca", MARCA)):
                t, n = rx.subn("", t)
                contas[chave] += n
            for nome in nomes:
                t, n = re.subn(re.escape(nome), "", t, flags=re.I)
                contas["nome"] += n
            f.write(f"\n=== p{i + 1}\n{t}")
    print(f"{len(doc)} páginas; removidos: {contas}", file=sys.stderr)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
