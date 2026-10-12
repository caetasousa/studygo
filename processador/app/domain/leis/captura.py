"""A prévia da lei: o que a captura entrega e as regras que a montam.

Nada aqui grava. O resultado vai para a tela, e quem publica é o backend,
depois que a pessoa revisou: um bloqueio impede a publicação; um aviso só
precisa ser marcado como revisado. O caminho baixar → limpar → classificar →
montar → verificar é do caso de uso (service/leis/captura.py).
"""

from __future__ import annotations

import hashlib
import json
from collections import Counter
from dataclasses import dataclass, field

from app.domain.leis.limpeza import Paragrafo
from app.domain.leis.montar import REPETIDO, SOB_O_CAPUT, Montagem
from app.domain.leis.verificar import SALTO, sequencia, verificar


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


def versao(dispositivos: list[dict[str, object]]) -> str:
    canonico = json.dumps(dispositivos, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonico.encode("utf-8")).hexdigest()


def resumo(paragrafos: list[Paragrafo], montagem: Montagem | None) -> dict[str, object]:
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


def aviso_de_divergencia(divergencia: str, textos: dict[str, str]) -> Aviso:
    """ "p0328: a regra diz nome, o Gemini diz solto" → aviso de p0328.

    O id deixa o palpite do Gemini de fora: ele muda de palpite a cada
    execução, e a revisão é do parágrafo e do tipo que a regra deu (K17b).
    """
    pid = divergencia.split(":", 1)[0]
    return Aviso(divergencia.split(", o Gemini", 1)[0], divergencia, textos.get(pid, "")[:240])


def refs_sob(montagem: Montagem, raizes: list[str]) -> set[str]:
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


def visiveis_do_recorte(montagem: Montagem, raizes: list[str]) -> set[str]:
    """O que o recorte guarda: o que está sob as raízes, as divisões acima delas
    (o leitor precisa saber onde está) e a epígrafe (o nome da lei) — K40."""
    pais = {d.ref: d.pai for d in montagem.dispositivos}
    guardar = refs_sob(montagem, raizes)
    for r in raizes:
        pai = pais.get(r)
        while pai:
            guardar.add(pai)
            pai = pais.get(pai)
    epigrafe = next((d.ref for d in montagem.dispositivos if d.tipo == "preambulo"), None)
    if epigrafe:
        guardar.add(epigrafe)
    return guardar


def verificar_o_recorte(
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
        subarvore = refs_sob(montagem, [raiz])
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

    sob = refs_sob(montagem, raizes)
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


def salto_no_recorte(salto: str, visiveis: set[str] | None) -> bool:
    """ "art3 → art4": o aviso só interessa se o artigo de chegada foi guardado (K44)."""
    return visiveis is None or salto.rsplit("→", 1)[-1].strip() in visiveis


def registrar_problemas(captura: Captura, problemas: list[str], visiveis: set[str] | None) -> None:
    """O que a verificação achou vira aviso (a pessoa revisa e publica) ou
    bloqueio (não publica). Com recorte, só conta o que caiu nele (K44)."""
    for problema in problemas:
        if problema.startswith(SALTO):
            salto = problema.removeprefix(SALTO)
            if not salto_no_recorte(salto, visiveis):
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


def fechar(
    captura: Captura,
    paragrafos: list[Paragrafo],
    montagem: Montagem | None,
    visiveis: set[str] | None,
) -> None:
    """O resumo e, sem bloqueio, os dispositivos guardados e a versão deles."""
    captura.resumo = resumo(paragrafos, montagem)
    if montagem is not None and not captura.bloqueios:
        captura.dispositivos = [
            d.model_dump() for d in montagem.dispositivos if visiveis is None or d.ref in visiveis
        ]
        captura.versao = versao(captura.dispositivos)
