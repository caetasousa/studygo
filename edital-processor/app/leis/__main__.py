"""uv run python -m app.leis capturar <slug> | --prioridade A [--sem-gemini]"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from app.core.config import Settings
from app.leis.captura import capturar
from app.leis.catalogo import ler_normas
from app.leis.classificar import por_regras
from app.leis.fontes import decodificar_html, http_padrao
from app.leis.limpeza import paragrafos_de_html
from app.providers.base import LLMProvider
from app.providers.gemini import GeminiProvider

_CONTEUDO = Path(__file__).resolve().parents[3] / "conteudo" / "leis"


def main(argv: list[str] | None = None) -> int:
    args = argparse.ArgumentParser(prog="python -m app.leis")
    sub = args.add_subparsers(dest="comando", required=True)
    cap = sub.add_parser("capturar", help="baixa, organiza e verifica normas de normas.toml")
    cap.add_argument("slug", nargs="?")
    cap.add_argument("--prioridade", choices=["A", "B", "C"])
    cap.add_argument("--sem-gemini", action="store_true", help="classifica só pelas regras")
    cap.add_argument("--conteudo", type=Path, default=_CONTEUDO)
    ver = sub.add_parser("paragrafos", help="mostra como a regra leu os parágrafos do original")
    ver.add_argument("slug")
    ver.add_argument("de", nargs="?", default="p0000")
    ver.add_argument("ate", nargs="?", default="p9999")
    ver.add_argument("--conteudo", type=Path, default=_CONTEUDO)
    opcoes = args.parse_args(argv)
    if opcoes.comando == "paragrafos":
        return _paragrafos(opcoes.conteudo / opcoes.slug, opcoes.de, opcoes.ate)

    normas = ler_normas(opcoes.conteudo / "normas.toml")
    if opcoes.slug:
        escolhidas = [n for n in normas if n.slug == opcoes.slug]
        if not escolhidas:
            print(f"norma {opcoes.slug!r} não está em normas.toml", file=sys.stderr)
            return 2
    elif opcoes.prioridade:
        escolhidas = [n for n in normas if n.prioridade == opcoes.prioridade]
    else:
        print("diga a norma (slug) ou --prioridade", file=sys.stderr)
        return 2

    provider: LLMProvider | None = None
    if not opcoes.sem_gemini:
        gemini = GeminiProvider(Settings())
        if not gemini.available():
            print(
                "sem EP_GEMINI_API_KEY: rode com --sem-gemini para capturar só com as regras",
                file=sys.stderr,
            )
            return 2
        provider = gemini

    falhas = 0
    for norma in escolhidas:
        if not norma.link:
            print(f"· {norma.slug}: sem link em normas.toml — pulada")
            continue
        resultado = capturar(norma, opcoes.conteudo, http_padrao, provider)
        if resultado.gravado:
            print(f"✓ {norma.slug}: gravada")
        else:
            falhas += 1
            print(f"✗ {norma.slug}: {len(resultado.problemas)} problema(s)")
            for p in resultado.problemas[:10]:
                print(f"    {p}")
        print(f"  relatório: {opcoes.conteudo / norma.slug / 'captura.md'}")
    return 1 if falhas else 0


def _paragrafos(pasta: Path, de: str, ate: str) -> int:
    """Para revisar o captura.md: o parágrafo, se é anterior, e o tipo pela regra."""
    originais = sorted(pasta.glob("original.*"))
    if not originais:
        print(f"sem original em {pasta}: capture primeiro", file=sys.stderr)
        return 2
    bruto = originais[0].read_bytes()
    html = (
        json.loads(bruto)["conteudo"] if originais[0].suffix == ".json" else decodificar_html(bruto)
    )
    paragrafos = paragrafos_de_html(html)
    for p, c in zip(paragrafos, por_regras(paragrafos), strict=True):
        if de <= p.id <= ate:
            marca = "ANT" if p.anterior else "   "
            print(p.id, marca, f"{c.tipo:<10}", repr(p.texto[:110]), list(p.notas[:2]) or "")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
