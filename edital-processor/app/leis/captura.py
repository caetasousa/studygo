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
from app.leis.montar import REPETIDO, SOB_O_CAPUT, Montagem, montar
from app.leis.verificar import SALTO, sequencia, verificar
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
    # As raízes do que foi guardado ("tit3.cap7", "art37"); vazio é a lei inteira.
    recorte: list[str] = field(default_factory=list)

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


def _sob(montagem: Montagem, raizes: list[str]) -> set[str]:
    """As refs dentro das raízes (elas incluídas)."""
    pais = {d.ref: d.pai for d in montagem.dispositivos}
    alvo = set(raizes)
    dentro: set[str] = set()
    for d in montagem.dispositivos:
        ref: str | None = d.ref
        for _ in range(64):
            if ref is None:
                break
            if ref in alvo:
                dentro.add(d.ref)
                break
            ref = pais.get(ref)
    return dentro


def _visiveis(montagem: Montagem, raizes: list[str]) -> set[str]:
    """O que o recorte guarda: o que está sob as raízes, as divisões acima delas
    (o leitor precisa saber onde está) e a epígrafe (o nome da lei) — K40."""
    pais = {d.ref: d.pai for d in montagem.dispositivos}
    guardar = _sob(montagem, raizes)
    for r in raizes:
        pai = pais.get(r)
        while pai:
            guardar.add(pai)
            pai = pais.get(pai)
    epigrafe = next((d.ref for d in montagem.dispositivos if d.tipo == "preambulo"), None)
    if epigrafe:
        guardar.add(epigrafe)
    return guardar


def _verificar_o_recorte(
    html: str | None, paragrafos: list[Paragrafo], montagem: Montagem, raizes: list[str]
) -> list[str]:
    """A verificação só na região do recorte (K46): o que está fora dele — o
    ADCT, o texto que o site anexa ao fim — não é importado e não bloqueia.

    A região de cada raiz vai do primeiro ao último parágrafo de onde saiu algo
    dela; um parágrafo perdido no meio (que nem virou dispositivo) está dentro.
    """
    posicao = {p.id: i for i, p in enumerate(paragrafos)}
    dono = {pid: ref for ref, pids in montagem.origem.items() for pid in pids}
    faixas = []
    for raiz in raizes:
        subarvore = _sob(montagem, [raiz])
        indices = [
            posicao[pid]
            for ref in subarvore
            for pid in montagem.origem.get(ref, [])
            if pid in posicao
        ]
        if not indices:
            continue
        # A faixa vai até antes do próximo parágrafo que é de outra parte da
        # lei: o que se perdeu no fim dela (nem virou dispositivo) está dentro.
        fim = max(indices)
        while fim + 1 < len(paragrafos) and dono.get(paragrafos[fim + 1].id) in (None, *subarvore):
            fim += 1
        faixas.append((min(indices), fim))

    def dentro(pid: str) -> bool:
        i = posicao.get(pid)
        return i is not None and any(a <= i <= b for a, b in faixas)

    sob = _sob(montagem, raizes)
    parte = Montagem(
        dispositivos=[d for d in montagem.dispositivos if d.ref in sob],
        problemas=[],
        descartados=montagem.descartados,
        origem={},
    )
    # A numeração se confere na lei inteira: isolado, todo recorte "começa do
    # nada". O filtro do que interessa (K44) vem depois, em _salto_no_recorte.
    problemas = [
        p
        for p in verificar(html, [p for p in paragrafos if dentro(p.id)], parte)
        if not p.startswith(SALTO)
    ]
    problemas.extend(sequencia(montagem))
    for problema in montagem.problemas:
        pid = problema.split(":", 1)[0].split(" ", 1)[0]
        if not pid.startswith("p") or dentro(pid):
            problemas.append(problema)
    return problemas


def _salto_no_recorte(salto: str, visiveis: set[str] | None) -> bool:
    """ "art3 → art4": o aviso só interessa se o artigo de chegada foi guardado (K44)."""
    return visiveis is None or salto.rsplit("→", 1)[-1].strip() in visiveis


async def capturar(
    fonte: Fonte,
    http: Http,
    provider: LLMProvider | None,
    progresso: Progresso | None = None,
    recorte: list[str] | None = None,
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
            paginas = await asyncio.to_thread(_paginas_do_pdf, original.bruto)
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
                for ref in _sob(pelas_regras, recorte)
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
        if recorte:
            visiveis = _visiveis(montagem, recorte)
        if recorte:
            problemas = await asyncio.to_thread(
                _verificar_o_recorte, original.html, paragrafos, montagem, recorte
            )
        else:
            problemas = await asyncio.to_thread(verificar, original.html, paragrafos, montagem)
        for problema in problemas:
            if problema.startswith(SALTO):
                salto = problema.removeprefix(SALTO)
                if not _salto_no_recorte(salto, visiveis):
                    continue
                captura.avisos.append(
                    Aviso(
                        f"salto: {salto}",
                        f"A numeração dos artigos salta ({salto}). Confira na fonte se "
                        "o artigo que falta existe: se existir, a captura o perdeu.",
                    )
                )
            elif problema.startswith(REPETIDO):
                primeira, nova = problema.removeprefix(REPETIDO).split(" → ")
                if visiveis is not None and nova not in visiveis:
                    continue
                captura.avisos.append(
                    Aviso(
                        f"repetido: {primeira}",
                        f"A fonte repete o rótulo de {primeira}; a segunda divisão ficou "
                        f"como {nova}. Confira na fonte se são mesmo duas divisões.",
                    )
                )
            elif problema.startswith(SOB_O_CAPUT):
                artigo = problema.removeprefix(SOB_O_CAPUT)
                aviso = f"alinea-no-caput: {artigo}"
                if (visiveis is not None and artigo not in visiveis) or aviso in {
                    a.id for a in captura.avisos
                }:
                    continue
                captura.avisos.append(
                    Aviso(
                        aviso,
                        f"O caput de {artigo} anuncia alíneas sem inciso entre eles. Confira "
                        "na fonte se é assim mesmo ou se a captura perdeu um inciso.",
                    )
                )
            else:
                captura.bloqueios.append(problema)
    except (FonteInvalida, ValueError) as exc:
        # ValueError é a recusa com motivo: um lote que o Gemini devolveu
        # torto duas vezes, uma árvore que não monta, um recorte sem par.
        captura.bloqueios.append(str(exc))

    captura.resumo = _resumo(paragrafos, montagem)
    if montagem is not None and not captura.bloqueios:
        captura.dispositivos = [
            d.model_dump() for d in montagem.dispositivos if visiveis is None or d.ref in visiveis
        ]
        captura.versao = _versao(captura.dispositivos)
    return captura
