"""Baixar → limpar → classificar → montar → verificar: a prévia da lei.

Nada aqui grava. O resultado vai para a tela, e quem publica é o backend,
depois que a pessoa revisou: um bloqueio impede a publicação; um aviso só
precisa ser marcado como revisado.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
from collections import Counter
from collections.abc import Callable
from dataclasses import dataclass, field

from app.leis.classificar import combinar, por_gemini, por_regras
from app.leis.fontes import Fonte, FonteInvalida, Http, Original, baixar
from app.leis.limpeza import Paragrafo, paragrafos_de_html, paragrafos_de_pdf
from app.leis.montar import Montagem, montar
from app.leis.verificar import SALTO, verificar
from app.providers.base import LLMProvider

# (etapa, feitos, total): para a tela dizer em que pé a captura está.
Progresso = Callable[[str, int, int], None]


@dataclass(frozen=True)
class Aviso:
    # Estável entre capturas da mesma fonte: é o que a pessoa marca como
    # revisado, e o que o backend confere antes de publicar.
    id: str
    texto: str
    trecho: str = ""


@dataclass
class Captura:
    fonte: str
    gemini: bool
    versao: str | None = None
    original_sha256: str | None = None
    paragrafos: int = 0
    dispositivos: list[dict[str, object]] | None = None
    bloqueios: list[str] = field(default_factory=list)
    avisos: list[Aviso] = field(default_factory=list)
    resumo: dict[str, object] = field(default_factory=dict)

    @property
    def publicavel(self) -> bool:
        return not self.bloqueios and self.dispositivos is not None


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


def _resumo(paragrafos: list[Paragrafo], montagem: Montagem | None) -> dict[str, object]:
    vigentes = [p for p in paragrafos if not p.anterior]
    resumo: dict[str, object] = {
        "vigentes": len(vigentes),
        "anteriores": len(paragrafos) - len(vigentes),
        "notas": sum(len(p.notas) for p in paragrafos),
        "riscados": [f"{p.id}: {r}" for p in paragrafos for r in p.riscado],
        "juncoes": [f"{p.id}: {j}" for p in paragrafos for j in p.juncoes],
    }
    if montagem is not None:
        resumo["tipos"] = dict(Counter(d.tipo for d in montagem.dispositivos))
        resumo["revogados"] = sum(1 for d in montagem.dispositivos if d.revogado)
        resumo["descartados"] = list(montagem.descartados)
    return resumo


def _aviso_de_divergencia(divergencia: str, textos: dict[str, str]) -> Aviso:
    """ "p0328: a regra diz nome, o Gemini diz solto" → aviso de p0328.

    O id deixa o palpite do Gemini de fora: ele muda de palpite a cada
    execução, e a revisão é do parágrafo e do tipo que a regra deu (K17b).
    """
    pid = divergencia.split(":", 1)[0]
    return Aviso(divergencia.split(", o Gemini", 1)[0], divergencia, textos.get(pid, "")[:240])


async def capturar(
    fonte: Fonte,
    http: Http,
    provider: LLMProvider | None,
    progresso: Progresso | None = None,
) -> Captura:
    """A prévia da lei. Falha da fonte vira bloqueio; o resto sobe como exceção."""

    def etapa(nome: str, feitos: int = 0, total: int = 0) -> None:
        if progresso is not None:
            progresso(nome, feitos, total)

    captura = Captura(fonte=fonte.url_publica, gemini=provider is not None)
    paragrafos: list[Paragrafo] = []
    montagem: Montagem | None = None
    try:
        etapa("baixando")
        original: Original = await asyncio.to_thread(baixar, fonte, http)
        captura.original_sha256 = original.sha256
        etapa("organizando")
        if original.pdf:
            paginas = await asyncio.to_thread(_paginas_do_pdf, original.bruto)
            paragrafos = paragrafos_de_pdf(paginas)
        else:
            assert original.html is not None
            paragrafos = await asyncio.to_thread(paragrafos_de_html, original.html)
        captura.paragrafos = len(paragrafos)
        classes = por_regras(paragrafos)
        textos = {p.id: p.texto for p in paragrafos}
        if provider is not None:
            etapa("classificando", 0, 1)
            deles = await por_gemini(
                paragrafos, provider, ao_lote=lambda f, t: etapa("classificando", f, t)
            )
            classes, divergencias = combinar(classes, deles, paragrafos)
            captura.avisos.extend(_aviso_de_divergencia(d, textos) for d in divergencias)
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
        for problema in await asyncio.to_thread(verificar, original.html, paragrafos, montagem):
            if problema.startswith(SALTO):
                salto = problema.removeprefix(SALTO)
                captura.avisos.append(
                    Aviso(
                        f"salto: {salto}",
                        f"A numeração dos artigos salta ({salto}). Confira na fonte se "
                        "o artigo que falta existe: se existir, a captura o perdeu.",
                    )
                )
            else:
                captura.bloqueios.append(problema)
    except (FonteInvalida, ValueError) as exc:
        # ValueError é a recusa com motivo: um lote que o Gemini devolveu
        # torto duas vezes, uma árvore que não monta.
        captura.bloqueios.append(str(exc))

    captura.resumo = _resumo(paragrafos, montagem)
    if montagem is not None and not captura.bloqueios:
        captura.dispositivos = [d.model_dump() for d in montagem.dispositivos]
        captura.versao = _versao(captura.dispositivos)
    return captura
