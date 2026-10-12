"""Busca questões no export do provasGo, com o código e o gabarito oficial.

Uso: provas.py <pasta-do-export> <regex> [--assunto TEXTO]

O regex procura no enunciado, nas alternativas e no assunto (sem caixa). Cada
questão sai com o id que o arquivo de questões do mapa usa
(provas-<pasta>-<número>), a banca, o ano, o órgão, o cargo e a resposta: a do
prova.json ou, se ela faltar, a lida no gabarito.pdf da prova.
"""

import json
import re
import sys
from pathlib import Path

import pymupdf


def texto_prova(bs):
    """Código curto entra na frase entre crases; bloco de código fica em linhas próprias."""
    s = ""
    for b in bs:
        if b["tipo"] == "texto":
            t = re.sub(r"\s*\n\s*", " ", b["texto"]).strip()
            if not t:
                continue
            if s and not s.endswith("\n"):
                # parágrafo novo quando o bloco anterior fechou a frase; senão, é a mesma frase
                s = s.rstrip() + ("\n" if re.search(r"[.:;?!]$", s.rstrip()) else " ")
            s += t
        elif b["tipo"] == "codigo" and "\n" not in b["texto"].strip():
            s += "`" + b["texto"].strip() + "`"
        elif b["tipo"] == "codigo":
            s = s.rstrip() + "\n" + re.sub(r"\n{2,}", "\n", b["texto"].strip("\n")) + "\n"
        else:
            s = (
                s.rstrip()
                + "\n"
                + f"[{b['tipo']}: {b.get('descricao') or b.get('arquivo')}]"
                + "\n"
            )
    linhas = []
    for linha in s.split("\n"):
        if linha.startswith("    ") or linha.startswith("\t"):
            linhas.append(linha.rstrip())
        else:
            linha = re.sub(r"[ \t]+", " ", linha).strip()
            linhas.append(re.sub(r"\s+([,.;:)])", r"\1", linha))
    return "\n".join(x for x in linhas if x.strip()).strip()


def gabarito_pdf(pdf):
    """O gabarito oficial: {número: (resposta, situação)}."""
    if not pdf.exists():
        return {}
    ls = [x.strip() for pg in pymupdf.open(pdf) for x in pg.get_text().split("\n") if x.strip()]
    d = {}
    for i in range(len(ls) - 2):
        if (
            re.fullmatch(r"\d{1,3}", ls[i])
            and re.fullmatch(r"[A-E*X]|Anulada|ANULADA", ls[i + 1])
            and re.search(r"Gabarito|Anulad|Alterad|atribu", ls[i + 2], re.I)
        ):
            d.setdefault(int(ls[i]), (ls[i + 1], ls[i + 2]))
    return d


def main(base, padrao, assunto=None):
    rx = re.compile(padrao, re.I)
    achadas = 0
    for f in sorted(Path(base).glob("*/prova.json")):
        pasta = f.parent.name
        p = json.loads(f.read_text(encoding="utf-8"))["prova"]
        gab = None
        for q in p["questoes"]:
            enun = texto_prova(q["blocos"])
            alts = [texto_prova(a["blocos"]) for a in q["alternativas"]]
            alvo = " ".join([enun, *alts, q.get("assunto") or ""])
            if not rx.search(alvo) or (
                assunto and assunto.lower() not in (q.get("assunto") or "").lower()
            ):
                continue
            resp = q.get("resposta")
            if not resp:
                gab = gabarito_pdf(f.parent / "gabarito.pdf") if gab is None else gab
                resp = "/".join(gab.get(q["numero"], ("?", "")))
            achadas += 1
            n = q["numero"]
            origem = f"{p['banca']} · {p['ano']} · {p['orgao']} · {p['cargoNome']} · questão {n}"
            print(f"##### provas-{pasta}-{n} | {origem} | {q.get('assunto')} | resposta={resp}")
            print(enun)
            for letra, a in zip("ABCDE", alts, strict=False):
                print(f"  {letra}) {a}")
            print()
    print(f"{achadas} questões", file=sys.stderr)


if __name__ == "__main__":
    args = sys.argv[1:]
    assunto = None
    if "--assunto" in args:
        i = args.index("--assunto")
        assunto = args[i + 1]
        del args[i : i + 2]
    main(args[0], args[1], assunto)
