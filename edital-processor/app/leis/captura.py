"""Baixar → limpar → classificar → montar → verificar → gravar."""

from __future__ import annotations

import asyncio
import hashlib
import json
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path

from app.leis.catalogo import Norma
from app.leis.classificar import combinar, por_gemini, por_regras
from app.leis.fontes import FonteInvalida, Http, Original, baixar
from app.leis.limpeza import Paragrafo, paragrafos_de_html, paragrafos_de_pdf
from app.leis.montar import Montagem, montar
from app.leis.verificar import verificar
from app.providers.base import LLMProvider

FORMATO = "studygo.lei/1"


@dataclass
class Resultado:
    slug: str
    gravado: bool
    problemas: list[str] = field(default_factory=list)


def _paginas_do_pdf(bruto: bytes) -> list[str]:
    import pymupdf

    with pymupdf.open(stream=bruto, filetype="pdf") as doc:
        paginas = [str(pagina.get_text()) for pagina in doc]
    vazias = [i + 1 for i, t in enumerate(paginas) if len(t.strip()) < 40]
    if vazias:
        # PDF escaneado: o OCR do processador existe, mas o texto dele não é
        # confiável o bastante para virar lei sem conferência humana.
        raise FonteInvalida(
            f"páginas sem camada de texto: {vazias[:10]} — PDF escaneado precisa de "
            "conferência manual"
        )
    return paginas


def _versao(dispositivos: list[dict[str, object]]) -> str:
    canonico = json.dumps(dispositivos, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonico.encode("utf-8")).hexdigest()


def _relatorio(
    norma: Norma,
    original: Original | None,
    paragrafos: list[Paragrafo],
    montagem: Montagem | None,
    gemini: bool,
    problemas: list[str],
    gravado: bool,
) -> str:
    linhas = [f"# Captura — {norma.curto}", "", f"- Norma: {norma.nome}"]
    if original is not None:
        linhas += [
            f"- Fonte: {original.url_publica}",
            f"- sha256 do original: `{original.sha256}` ({len(original.bruto):,} bytes)",
        ]
    linhas.append(
        "- Classificação: regras + Gemini"
        if gemini
        else "- Classificação: só regras — **sem conferência do Gemini**"
    )
    vigentes = [p for p in paragrafos if not p.anterior]
    linhas += [
        f"- Parágrafos: {len(paragrafos)} ({len(vigentes)} vigentes, "
        f"{len(paragrafos) - len(vigentes)} de redação anterior)",
        f"- Notas de redação: {sum(len(p.notas) for p in paragrafos)}",
    ]
    if montagem is not None:
        tipos = Counter(d.tipo for d in montagem.dispositivos)
        linhas.append(
            "- Dispositivos: "
            + ", ".join(f"{n} {t}" for t, n in sorted(tipos.items(), key=lambda x: -x[1]))
        )
        revogados = sum(1 for d in montagem.dispositivos if d.revogado)
        linhas.append(f"- Revogados: {revogados}")
    linhas += ["", "## Resultado", ""]
    linhas.append(
        "Gravado em `lei.json`." if gravado else "**NÃO gravado**: corrija os problemas abaixo."
    )
    if problemas:
        linhas += ["", "## Problemas", ""]
        linhas += [f"- {p}" for p in problemas]
    if montagem is not None and montagem.descartados:
        linhas += ["", "## Descartado (cabeçalho do site)", ""]
        linhas += [f"- {t}" for t in montagem.descartados]
    parciais = [(p.id, r) for p in paragrafos for r in p.riscado]
    if parciais:
        linhas += ["", "## Riscado dentro de parágrafo vigente", ""]
        linhas += [f"- {pid}: {r}" for pid, r in parciais]
    juncoes = [(p.id, j) for p in paragrafos for j in p.juncoes]
    if juncoes:
        linhas += ["", "## Hifenização desfeita (PDF) — confira", ""]
        linhas += [f"- {pid}: {j}" for pid, j in juncoes]
    return "\n".join(linhas) + "\n"


def _aceita(divergencia: str, aceitas: list[str]) -> bool:
    """ "p0328: a regra diz nome" aceita esse parágrafo, seja qual for o palpite."""
    return any(divergencia == a or divergencia.startswith(f"{a},") for a in aceitas)


def _divergencias(divergencias: list[str], paragrafos: list[Paragrafo], aceitas: list[str]) -> str:
    if not divergencias:
        return ""
    textos = {p.id: p.texto for p in paragrafos}
    linhas = [
        "",
        "## Onde o Gemini discordou da regra",
        "",
        "A regra prevaleceu. Se ela estiver certa, declare em `aceitar` no normas.toml "
        'o parágrafo e o tipo da regra ("p0328: a regra diz nome"); se não, corrija a regra.',
        "",
    ]
    for d in divergencias:
        pid = d.split(":", 1)[0]
        marca = "aceita" if _aceita(d, aceitas) else "pendente"
        linhas.append(f"- [{marca}] `{d}` — {textos.get(pid, '')[:120]!r}")
    return "\n".join(linhas) + "\n"


def capturar(
    norma: Norma,
    raiz: Path,
    http: Http,
    provider: LLMProvider | None,
) -> Resultado:
    pasta = raiz / norma.slug
    pasta.mkdir(parents=True, exist_ok=True)
    relatorio = pasta / "captura.md"
    original: Original | None = None
    paragrafos: list[Paragrafo] = []
    montagem: Montagem | None = None
    problemas: list[str] = []
    revisar: list[str] = []

    try:
        original = baixar(norma, http)
        (pasta / f"original.{original.extensao}").write_bytes(original.bruto)
        if original.pdf:
            paragrafos = paragrafos_de_pdf(_paginas_do_pdf(original.bruto))
        else:
            assert original.html is not None
            paragrafos = paragrafos_de_html(original.html)
        classes = por_regras(paragrafos)
        if provider is not None:
            deles = asyncio.run(por_gemini(paragrafos, provider))
            classes, divergencias = combinar(classes, deles, paragrafos)
            # A divergência revisada (a regra estava certa) é declarada, uma a
            # uma, em `aceitar`; a mensagem traz o id, então não serve para outra.
            problemas.extend(d for d in divergencias if not _aceita(d, norma.aceitar))
            revisar = divergencias
        montagem = montar(paragrafos, classes)
        problemas.extend(
            verificar(
                original.html,
                paragrafos,
                montagem,
                norma.recorte,
                norma.aceitar,
            )
        )
    except (FonteInvalida, ValueError) as exc:
        problemas.append(str(exc))

    gravado = not problemas and montagem is not None and original is not None
    if gravado:
        assert montagem is not None and original is not None
        dispositivos = [d.model_dump() for d in montagem.dispositivos]
        lei = {
            "formato": FORMATO,
            "lei": {
                "slug": norma.slug,
                "nome": norma.nome,
                "curto": norma.curto,
                "reconhecer": norma.reconhecer,
                "fonte": original.url_publica,
            },
            "versao": _versao(dispositivos),
            "captura": {
                "original_sha256": original.sha256,
                "gemini": provider is not None,
                "paragrafos": len(paragrafos),
            },
            "dispositivos": dispositivos,
        }
        temporario = pasta / "lei.json.tmp"
        temporario.write_text(json.dumps(lei, ensure_ascii=False, indent=1) + "\n", "utf-8")
        temporario.replace(pasta / "lei.json")

    relatorio.write_text(
        _relatorio(norma, original, paragrafos, montagem, provider is not None, problemas, gravado)
        + _divergencias(revisar, paragrafos, norma.aceitar),
        "utf-8",
    )
    return Resultado(norma.slug, gravado, problemas)
