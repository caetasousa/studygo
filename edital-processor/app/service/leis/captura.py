"""Baixar → limpar → classificar → montar → verificar: a prévia da lei.

A fonte vem pela porta das fontes, o PDF pela do leitor, a conferência pelo
Gemini; as regras de cada passo são do domínio (domain/leis).
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable

from app.domain.leis.captura import (
    Aviso,
    Captura,
    aviso_de_divergencia,
    fechar,
    refs_sob,
    registrar_problemas,
    verificar_o_recorte,
    visiveis_do_recorte,
)
from app.domain.leis.classificar import combinar, por_regras
from app.domain.leis.fontes import Fonte, FonteInvalida, Original
from app.domain.leis.limpeza import Paragrafo, paragrafos_de_html, paragrafos_de_pdf
from app.domain.leis.montar import Montagem, montar
from app.domain.leis.verificar import verificar
from app.port.fontes import Http
from app.port.llm import LLMProvider
from app.port.pdf import LeitorDePdf
from app.service.leis.conferencia import por_gemini
from app.service.leis.fontes import baixar

# (etapa, feitos, total): para a tela dizer em que pé a captura está.
Progresso = Callable[[str, int, int], None]


def paginas_do_pdf(bruto: bytes, pdf: LeitorDePdf) -> list[str]:
    paginas = pdf.paginas(bruto)
    vazias = [i + 1 for i, t in enumerate(paginas) if len(t.strip()) < 40]
    if vazias:
        # PDF escaneado: o OCR do processador existe, mas o texto dele não é
        # confiável o bastante para virar lei sem conferência humana.
        raise FonteInvalida(
            f"páginas sem camada de texto: {vazias[:10]} — PDF escaneado precisa de "
            "conferência manual"
        )
    return paginas


async def capturar(
    fonte: Fonte,
    http: Http,
    provider: LLMProvider | None,
    progresso: Progresso | None = None,
    recorte: list[str] | None = None,
    *,
    pdf: LeitorDePdf,
) -> Captura:
    """A prévia da lei. Falha da fonte vira bloqueio; o resto sobe como exceção.

    Com recorte, guarda só o que está sob as raízes dele, e o Gemini confere só
    esses parágrafos: a Constituição inteira leva minutos, os arts. 37 a 43,
    segundos (K43).
    """

    def etapa(nome: str, feitos: int = 0, total: int = 0) -> None:
        if progresso is not None:
            progresso(nome, feitos, total)

    captura = Captura(
        fonte=fonte.url_publica, gemini=provider is not None, recorte=list(recorte or [])
    )
    paragrafos: list[Paragrafo] = []
    montagem: Montagem | None = None
    visiveis: set[str] | None = None
    try:
        etapa("baixando")
        original: Original = await asyncio.to_thread(baixar, fonte, http)
        captura.original_sha256 = original.sha256
        etapa("organizando")
        if original.pdf:
            paginas = await asyncio.to_thread(paginas_do_pdf, original.bruto, pdf)
            paragrafos = paragrafos_de_pdf(paginas)
        else:
            assert original.html is not None
            paragrafos = await asyncio.to_thread(paragrafos_de_html, original.html)
        captura.paragrafos = len(paragrafos)
        classes = por_regras(paragrafos)
        textos = {p.id: p.texto for p in paragrafos}

        conferir = paragrafos
        if recorte:
            # A árvore pelas regras diz que parágrafos formam o recorte.
            pelas_regras = await asyncio.to_thread(montar, paragrafos, classes)
            existe = {d.ref for d in pelas_regras.dispositivos}
            faltam = [r for r in recorte if r not in existe]
            if faltam:
                raise ValueError(f"o recorte cita {', '.join(faltam)}, que a lei não tem (K42)")
            ids = {
                pid
                for ref in refs_sob(pelas_regras, recorte)
                for pid in pelas_regras.origem.get(ref, [])
            }
            conferir = [p for p in paragrafos if p.id in ids]

        if provider is not None:
            etapa("classificando", 0, 1)
            deles = await por_gemini(
                conferir, provider, ao_lote=lambda f, t: etapa("classificando", f, t)
            )
            por_id = {p.id: c for p, c in zip(conferir, deles, strict=True)}
            classes, divergencias = combinar(
                classes, [por_id.get(p.id) for p in paragrafos], paragrafos
            )
            captura.avisos.extend(aviso_de_divergencia(d, textos) for d in divergencias)
        else:
            captura.avisos.append(
                Aviso(
                    "sem-gemini",
                    "A classificação não foi conferida pelo Gemini (o processador está sem "
                    "chave): só as regras organizaram os dispositivos.",
                )
            )
        etapa("verificando")
        montagem = await asyncio.to_thread(montar, paragrafos, classes)
        if recorte:
            visiveis = visiveis_do_recorte(montagem, recorte)
        if recorte:
            problemas = await asyncio.to_thread(
                verificar_o_recorte, original.html, paragrafos, montagem, recorte
            )
        else:
            problemas = await asyncio.to_thread(verificar, original.html, paragrafos, montagem)
        registrar_problemas(captura, problemas, visiveis)
    except (FonteInvalida, ValueError) as exc:
        # ValueError é a recusa com motivo: um lote que o Gemini devolveu
        # torto duas vezes, uma árvore que não monta, um recorte sem par.
        captura.bloqueios.append(str(exc))

    fechar(captura, paragrafos, montagem, visiveis)
    return captura
