"""Dos parágrafos classificados à árvore de dispositivos, com ref estável.

A ref é o endereço jurídico: `art71.inc2`, `art70.parunico`, `tit4.cap1`,
`adct.art1`. É ela que o link direto, as questões e a próxima versão da lei
usam para achar o dispositivo.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from pydantic import BaseModel

from app.leis import rotulos
from app.leis.classificar import AGRUPAMENTOS, Classe
from app.leis.limpeza import Paragrafo


class Dispositivo(BaseModel):
    ref: str
    pai: str | None
    tipo: str
    rotulo: str
    nome: str
    texto: str
    notas: list[str]
    anteriores: list[str]
    revogado: bool


class Montagem(BaseModel):
    dispositivos: list[Dispositivo]
    problemas: list[str]
    descartados: list[str]
    # Por id, e não pelo texto: o site repete no corpo a anotação que ele
    # descarta no cabeçalho (", DEC 2-12-2024."), e o do corpo fica.
    ids_descartados: list[str] = []
    # ref → ids dos parágrafos vigentes de onde o dispositivo saiu.
    origem: dict[str, list[str]]


_NIVEL = {
    "parte": 1,
    "livro": 2,
    "titulo": 3,
    "capitulo": 4,
    "secao": 5,
    "subsecao": 6,
    "artigo": 10,
    "paragrafo": 11,
    "inciso": 12,
    "alinea": 13,
    "item": 14,
}
# Sob quem cada dispositivo pode ficar.
_PAIS = {
    "paragrafo": ("artigo",),
    "inciso": ("artigo", "paragrafo"),
    "alinea": ("inciso", "paragrafo"),
    "item": ("alinea", "inciso"),
}
# A fonte que repete o rótulo de uma divisão no mesmo pai (duas "Seção I" num
# capítulo da Lei 20.756) não bloqueia: a segunda ganha ref própria e a
# pessoa confere — pode ser erro da fonte, pode ser sumário lido como corpo.
REPETIDO = "rótulo repetido na fonte: "
SOB_O_CAPUT = "alínea sob o caput: "
_NOME_DO_TIPO = {"paragrafo": "parágrafo", "inciso": "inciso", "alinea": "alínea", "item": "item"}


def _eh_revogado(texto: str, notas: list[str]) -> bool:
    if "(Revogad" in texto or "(REVOGAD" in texto:
        return True
    return any(n.lstrip("(- ").lower().startswith("revogad") for n in notas)


@dataclass
class _Estado:
    dispositivos: list[Dispositivo] = field(default_factory=list)
    problemas: list[str] = field(default_factory=list)
    descartados: list[str] = field(default_factory=list)
    ids_descartados: list[str] = field(default_factory=list)
    origem: dict[str, list[str]] = field(default_factory=dict)
    pilha: list[Dispositivo] = field(default_factory=list)
    refs: set[str] = field(default_factory=set)
    contadores: dict[str, int] = field(default_factory=dict)
    no_adct: bool = False
    # Onde a numeração recomeça (resolução + anexos, K45): índice de início de
    # cada bloco e o prefixo das refs dele.
    blocos: list[tuple[int, str]] = field(default_factory=list)
    prefixo: str = ""
    notas_pendentes: list[str] = field(default_factory=list)
    # Redação anterior à espera do dispositivo vigente que vem depois dela.
    anteriores_pendentes: dict[str, list[str]] = field(default_factory=dict)

    def contar(self, chave: str) -> int:
        self.contadores[chave] = self.contadores.get(chave, 0) + 1
        return self.contadores[chave]

    def ultimo(self) -> Dispositivo | None:
        return self.dispositivos[-1] if self.dispositivos else None


def _chave(classe: Classe, texto: str) -> tuple[str, str] | None:
    lido = rotulos.ler(texto)
    if lido is None or lido.tipo != classe.tipo:
        return None
    return (lido.tipo, lido.chave)


def _novo(e: _Estado, p: Paragrafo, tipo: str, texto: str, revogado: bool) -> Dispositivo | None:
    """Encaixa um dispositivo estrutural na árvore e devolve-o (None se inválido)."""
    lido = rotulos.ler(texto)
    if lido is None or lido.tipo != tipo:
        e.problemas.append(f"{p.id}: classificado como {tipo}, mas sem rótulo de {tipo}")
        return None
    nivel = _NIVEL[tipo]
    if tipo in AGRUPAMENTOS:
        while e.pilha and _NIVEL[e.pilha[-1].tipo] >= nivel:
            e.pilha.pop()
        if lido.chave == "adct":
            e.no_adct = True
            e.pilha.clear()
        pai = e.pilha[-1] if e.pilha else None
        ref = f"{pai.ref}.{lido.chave}" if pai else e.prefixo + lido.chave
    elif tipo == "artigo":
        while e.pilha and _NIVEL[e.pilha[-1].tipo] >= nivel:
            e.pilha.pop()
        pai = e.pilha[-1] if e.pilha else None
        ref = ("adct." if e.no_adct else e.prefixo) + lido.chave
    else:
        while e.pilha and _NIVEL[e.pilha[-1].tipo] >= nivel:
            e.pilha.pop()
        pai = e.pilha[-1] if e.pilha else None
        if (
            tipo == "alinea"
            and pai is not None
            and pai.tipo == "artigo"
            and pai.texto.rstrip().endswith(":")
        ):
            # O caput que anuncia a lista ("…os seguintes direitos:") e segue
            # direto para as alíneas foge da técnica, mas é o texto da lei.
            # Também é o que sobra de um inciso perdido: a pessoa confere.
            e.problemas.append(f"{SOB_O_CAPUT}{pai.ref}")
        elif pai is None or pai.tipo not in _PAIS[tipo]:
            onde = f"{pai.tipo} {pai.ref}" if pai else "nada"
            e.problemas.append(
                f"{p.id} ({lido.rotulo}): {_NOME_DO_TIPO[tipo]} sob {onde}, "
                f"precisa estar sob {' ou '.join(_PAIS[tipo])}"
            )
            return None
        ref = f"{pai.ref}.{lido.chave}"

    if ref in e.refs and tipo in AGRUPAMENTOS:
        n = 2
        while f"{ref}-{n}" in e.refs:
            n += 1
        e.problemas.append(f"{REPETIDO}{ref} → {ref}-{n}")
        ref = f"{ref}-{n}"
    elif ref in e.refs:
        e.problemas.append(f"{p.id}: ref repetida {ref} ({lido.rotulo})")
        return None
    nome = lido.resto if tipo in AGRUPAMENTOS else ""
    rotulo_texto = texto[: texto.rfind(lido.resto)].strip() if nome else texto
    d = Dispositivo(
        ref=ref,
        pai=pai.ref if pai else None,
        tipo=tipo,
        rotulo=lido.rotulo,
        nome=nome,
        texto=rotulo_texto,
        notas=[],
        anteriores=[],
        revogado=revogado,
    )
    e.refs.add(ref)
    e.dispositivos.append(d)
    e.pilha.append(d)
    return d


def _solto(e: _Estado, tipo: str, texto: str) -> Dispositivo:
    if tipo in ("preambulo", "fecho"):
        ref = f"{tipo}{e.contar(tipo)}"
        pai = None
        if tipo == "fecho":
            e.pilha.clear()
    else:
        pai = e.pilha[-1] if e.pilha else None
        base = pai.ref if pai else "solto"
        ref = f"{base}.txt{e.contar(base)}" if pai else f"solto{e.contar(base)}"
    d = Dispositivo(
        ref=ref,
        pai=pai.ref if pai else None,
        tipo=tipo,
        rotulo="",
        nome="",
        texto=texto,
        notas=[],
        anteriores=[],
        revogado=False,
    )
    e.refs.add(ref)
    e.dispositivos.append(d)
    return d


def _proximo_vigente(paragrafos: list[Paragrafo], classes: list[Classe], i: int) -> int | None:
    for j in range(i + 1, len(paragrafos)):
        if not paragrafos[j].anterior and paragrafos[j].texto and classes[j].tipo != "descartar":
            return j
    return None


# Até onde procurar, à frente, o vigente de uma redação antiga. O Planalto
# empilha as versões antigas logo antes da nova; mais longe que isso já é
# outro dispositivo.
_JANELA = 80


def _irmao_atras(e: _Estado, chave: tuple[str, str]) -> Dispositivo | None:
    nivel = _NIVEL[chave[0]]
    for d in reversed(e.dispositivos):
        if d.tipo not in _NIVEL:
            continue
        if _NIVEL[d.tipo] < nivel:
            return None
        if _NIVEL[d.tipo] == nivel and d.ref.rsplit(".", 1)[-1] == chave[1]:
            return d
    return None


def _irmao_a_frente(
    paragrafos: list[Paragrafo], classes: list[Classe], i: int, chave: tuple[str, str]
) -> int | None:
    nivel = _NIVEL[chave[0]]
    for j in range(i + 1, min(len(paragrafos), i + 1 + _JANELA)):
        p, c = paragrafos[j], classes[j]
        if p.anterior or not p.texto or c.tipo not in _NIVEL:
            continue
        if _NIVEL[c.tipo] < nivel:
            return None
        if _chave(c, p.texto) == chave:
            return j
    return None


_DE_ARTIGO = ("artigo", "paragrafo", "inciso", "alinea", "item")


def _blocos(paragrafos: list[Paragrafo], classes: list[Classe]) -> list[tuple[int, str]]:
    """Onde a numeração recomeça do art. 1º, e o prefixo de cada parte.

    Uma resolução que aprova um regimento tem os artigos dela e, em anexo, o
    regimento, que recomeça do art. 1º (K45): a resolução vira "resolucao." e
    o regimento, o texto que se estuda, fica com as refs limpas. Com mais de um
    anexo (K45b), cada um é "anexoN.". O ADCT, que também recomeça, é à parte.
    """
    inicios: list[int] = []
    for i, (p, c) in enumerate(zip(paragrafos, classes, strict=True)):
        if p.anterior or c.tipo not in ("artigo", *AGRUPAMENTOS):
            continue
        lido = rotulos.ler(p.texto)
        if lido is None:
            continue
        if lido.chave == "adct":
            return []
        if c.tipo == "artigo" and lido.chave == "art1":
            inicios.append(i)
    if len(inicios) < 2:
        return []
    # O bloco começa no cabeçalho que antecede o art. 1º ("ANEXO II",
    # "CAPÍTULO I"): recua enquanto o parágrafo anterior não for artigo.
    comecos = []
    for i in inicios[1:]:
        j = i
        while j > 0 and classes[j - 1].tipo not in (
            "artigo",
            "paragrafo",
            "inciso",
            "alinea",
            "item",
        ):
            j -= 1
        comecos.append(j)
    if len(comecos) == 1:
        return [(0, "resolucao."), (comecos[0], "")]
    return [(0, "resolucao."), *((c, f"anexo{n}.") for n, c in enumerate(comecos, start=1))]


def montar(paragrafos: list[Paragrafo], classes: list[Classe]) -> Montagem:
    e = _Estado()
    e.blocos = _blocos(paragrafos, classes)
    inicios = dict(e.blocos)

    for i, (p, classe) in enumerate(zip(paragrafos, classes, strict=True)):
        if i in inicios:
            # Outra parte do documento: nada do bloco anterior é pai aqui.
            e.prefixo = inicios[i]
            e.pilha.clear()
        if p.anterior:
            _encaixar_anterior(e, paragrafos, classes, i)
            continue
        if classe.tipo == "descartar":
            e.descartados.append(p.texto)
            e.ids_descartados.append(p.id)
            continue
        if not p.texto:
            # Só nota ("Vide…"): vai para o dispositivo de antes, ou o próximo.
            alvo = e.ultimo()
            if alvo is not None:
                alvo.notas.extend(p.notas)
            else:
                e.notas_pendentes.extend(p.notas)
            continue

        d: Dispositivo | None
        if classe.tipo == "nome":
            ultimo = e.ultimo()
            continua = ultimo is not None and ultimo.nome.endswith((",", " E", " e"))
            if ultimo is not None and ultimo.tipo in AGRUPAMENTOS and (not ultimo.nome or continua):
                # Goiás quebra nome longo em linhas: "DO PRESIDENTE," / "DO OUVIDOR E".
                ultimo.nome = f"{ultimo.nome} {p.texto}".strip()
                ultimo.notas.extend(p.notas)
                e.origem[ultimo.ref].append(p.id)
                continue
            d = _solto(e, "solto", p.texto)
        elif classe.tipo in _NIVEL:
            d = _novo(e, p, classe.tipo, p.texto, _eh_revogado(p.texto, list(p.notas)))
            if d is None:
                continue
        else:
            d = _solto(e, classe.tipo, p.texto)

        d.notas = [*e.notas_pendentes, *p.notas]
        e.notas_pendentes = []
        d.anteriores.extend(e.anteriores_pendentes.pop(p.id, []))
        d.anteriores.extend(p.riscado)
        d.revogado = d.revogado or _eh_revogado(d.texto, d.notas)
        e.origem[d.ref] = [p.id]

    return Montagem(
        dispositivos=e.dispositivos,
        problemas=e.problemas,
        descartados=e.descartados,
        ids_descartados=e.ids_descartados,
        origem=e.origem,
    )


def _encaixar_anterior(
    e: _Estado, paragrafos: list[Paragrafo], classes: list[Classe], i: int
) -> None:
    """A redação riscada vai para o dispositivo vigente de mesmo rótulo.

    Goiás põe a redação antiga logo depois da nova; o Planalto, antes — às
    vezes um bloco inteiro de antigas antes do bloco de novas. Então vale o
    irmão de mesmo rótulo já montado, senão o que vem à frente, sem sair do
    mesmo pai. Sem nenhum dos dois, o dispositivo foi revogado sem
    substituto e entra na árvore como revogado: sumir com ele esconderia um
    buraco na numeração.
    """
    p = paragrafos[i]
    texto = p.texto if not p.notas else f"{p.texto} {' '.join(p.notas)}"
    chave = _chave(classes[i], p.texto)

    if chave is None or chave[0] not in _NIVEL:
        # Sem rótulo (um nome de título riscado): fica com o vizinho.
        j = _proximo_vigente(paragrafos, classes, i)
        if j is not None and classes[j].tipo not in _NIVEL:
            e.anteriores_pendentes.setdefault(paragrafos[j].id, []).append(texto)
        elif (ultimo := e.ultimo()) is not None:
            ultimo.anteriores.append(texto)
        elif j is not None:
            e.anteriores_pendentes.setdefault(paragrafos[j].id, []).append(texto)
        return

    if (irmao := _irmao_atras(e, chave)) is not None:
        irmao.anteriores.append(texto)
        return
    if (j := _irmao_a_frente(paragrafos, classes, i, chave)) is not None:
        e.anteriores_pendentes.setdefault(paragrafos[j].id, []).append(texto)
        return

    d = _novo(e, p, chave[0], p.texto, True)
    if d is None:
        # Não coube na árvore (o pai sumiu, ou a ref já existe por
        # renumeração): a redação antiga não bloqueia a captura; fica com o
        # dispositivo de antes.
        e.problemas.pop()
        if (ultimo := e.ultimo()) is not None:
            ultimo.anteriores.append(texto)
        return
    d.texto = ""
    d.anteriores.append(texto)
    e.origem[d.ref] = []
