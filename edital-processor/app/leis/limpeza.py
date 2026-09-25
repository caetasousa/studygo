"""Do HTML (ou do texto do PDF) aos parágrafos da lei — sem IA.

Separa, em cada parágrafo, o texto vigente, a redação anterior (riscada) e as
notas de redação. O texto que sai daqui é o texto que o estudante vai ler: só
entidades e espaços são normalizados; nenhuma palavra é trocada.
"""

from __future__ import annotations

import re
from collections import Counter
from dataclasses import dataclass
from html import unescape
from html.parser import HTMLParser

from app.leis import rotulos


@dataclass(frozen=True)
class Paragrafo:
    id: str
    texto: str
    anterior: bool = False
    notas: tuple[str, ...] = ()
    # Trechos riscados dentro de um parágrafo vigente (riscado parcial).
    riscado: tuple[str, ...] = ()
    # A classe do parágrafo na fonte, quando ela diz ("inciso" em Goiás).
    classe: str = ""
    # O texto vigente em pedaços contíguos do original: a verificação procura
    # cada um no texto visível da fonte.
    pedacos: tuple[str, ...] = ()
    # Palavras hifenizadas no fim da linha que o PDF obrigou a juntar.
    juncoes: tuple[str, ...] = ()
    # Todo o texto é link: no topo da página, é o sumário do site.
    link: bool = False


def normalizar(texto: str) -> str:
    return re.sub(r"\s+", " ", texto.replace("\xa0", " ")).strip()


# Uma nota de redação: "(Redação dada pela…)", "(Incluído pela…)", "Vigência".
# Só vale como nota quando é um link (Planalto) ou está em <nota>/<vide>
# (Goiás); no meio do texto, "(Revogado)" e "(VETADO)" são a própria lei.
_NOTA = re.compile(
    r"^[\s(\-–]*(Reda[çc][ãa]o dada|Inclu[íi]d[oa]|Acrescid|Acrescentad|Revogad[oa]|"
    r"Renumerad|Vide\b|Regulamento|Regulamenta|Promulga|Produ[çc][ãa]o de efeito|"
    r"Vig[êe]ncia|Com (a )?reda[çc][ãa]o|Alterad[oa]|Suprimid|Transformad|"
    r"Convers[ãa]o|Execu[çc][ãa]o suspensa|Vetad[oa] pel|Vide ADI|Norma anterior)",
    re.IGNORECASE,
)
# A mesma nota escrita como texto corrido no fim do parágrafo, sem link.
_NOTA_NO_FIM = re.compile(
    r"\((Reda[çc][ãa]o dada|Inclu[íi]d[oa] pel|Acrescid[oa] pel|Revogad[oa] pel|"
    r"Renumerad[oa] pel|Vide |Regulamento|Vig[êe]ncia|Produ[çc][ãa]o de efeito|"
    r"Vetad[oa] pel)[^()]*\)\s*$",
    re.IGNORECASE,
)
# A situação do dispositivo escrita ao lado do texto riscado.
_SITUACAO = re.compile(
    r"^(\((Rejeitad|Revogad|Vig[êe]ncia encerrada|Prejudicad|Sem efeito|Vetad|Caducad|"
    r"Perda de efic)[^()]*\)\s*)+$",
    re.IGNORECASE,
)
# O que sobra fora do risco quando a fonte riscou só o conteúdo: o rótulo.
_SO_ROTULO = re.compile(
    r"^(Art\.?\s*\d+\S*\.?|§\s*\d+\S*|Par[áa]grafo [úu]nico\.?|[IVXLCDM]+\s*[-–—]?|"
    r"[a-z] ?\)?|[A-Z])[\s.,;:]*$"
)
_SO_SEPARADOR = re.compile(r"^[\s\-–—]*$")
_SO_PONTUACAO = re.compile(r"^[\s\-–—.,;:]*$")

_BLOCOS = {
    "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "tr", "td",
    "th", "table", "tbody", "thead", "blockquote", "center", "body", "html", "dd", "dt",
}  # fmt: skip
_VAZIOS = {"br", "img", "meta", "link", "input", "hr", "wbr", "col", "base", "area"}
_IGNORADOS = {"script", "style", "head", "title", "noscript"}
_RISCO = {"strike", "s", "del"}


@dataclass
class _Elemento:
    tag: str
    riscado: bool
    nota: bool
    link: bool
    classe: str


@dataclass
class _Pedaco:
    texto: str
    riscado: bool = False
    nota: bool = False
    link: bool = False
    quebra: bool = False


@dataclass
class _Bruto:
    pedacos: list[_Pedaco]
    classe: str


class _Leitor(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.pilha: list[_Elemento] = []
        self.atual: list[_Pedaco] = []
        self.saida: list[_Bruto] = []
        self.ignorando = 0

    def _classe_do_bloco(self) -> str:
        for e in reversed(self.pilha):
            if e.tag in _BLOCOS:
                return e.classe
        return ""

    def _fechar_paragrafo(self) -> None:
        if self.atual:
            self.saida.append(_Bruto(self.atual, self._classe_do_bloco()))
        self.atual = []

    def _desempilhar(self, tag: str) -> None:
        if any(e.tag == tag for e in self.pilha):
            while self.pilha:
                if self.pilha.pop().tag == tag:
                    break

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag in _IGNORADOS:
            self.ignorando += 1
            return
        if tag == "br":
            self.atual.append(_Pedaco("", quebra=True))
            return
        if tag in _VAZIOS:
            return
        a = {k: (v or "") for k, v in attrs}
        classe = a.get("class", "")
        if tag in _BLOCOS:
            self._fechar_paragrafo()
            if tag == "p":
                # <p> não se aninha: um <p> aberto fecha o anterior.
                self._desempilhar("p")
        self.pilha.append(
            _Elemento(
                tag=tag,
                riscado=(
                    tag in _RISCO
                    or "line-through" in a.get("style", "").replace(" ", "").lower()
                    or "conteudo-revogado" in classe.split()
                ),
                nota=tag in ("nota", "vide"),
                link=tag == "a" and bool(a.get("href")),
                classe=classe.split()[0] if classe.split() else "",
            )
        )

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag == "br":
            self.atual.append(_Pedaco("", quebra=True))

    def handle_endtag(self, tag: str) -> None:
        if tag in _IGNORADOS:
            self.ignorando = max(0, self.ignorando - 1)
            return
        if tag in _BLOCOS:
            self._fechar_paragrafo()
        self._desempilhar(tag)

    def handle_data(self, data: str) -> None:
        if self.ignorando:
            return
        self.atual.append(
            _Pedaco(
                data,
                riscado=any(e.riscado for e in self.pilha),
                nota=any(e.nota for e in self.pilha),
                link=any(e.link for e in self.pilha),
            )
        )

    def fim(self) -> list[_Bruto]:
        self.close()
        self._fechar_paragrafo()
        return self.saida


def _dividir_cabecalho(pedacos: list[_Pedaco]) -> list[list[_Pedaco]]:
    """ "CAPÍTULO I<br>DISPOSIÇÕES PRELIMINARES" são dois parágrafos."""
    linhas: list[list[_Pedaco]] = [[]]
    for p in pedacos:
        if p.quebra:
            linhas.append([])
        else:
            linhas[-1].append(p)
    primeira = normalizar("".join(p.texto for p in linhas[0] if not p.nota))
    lido = rotulos.ler(primeira)
    if len(linhas) > 1 and lido and lido.tipo in ("parte", "livro", "titulo", "capitulo",
                                                    "secao", "subsecao"):  # fmt: skip
        return [linha for linha in linhas if linha]
    return [[p if not p.quebra else _Pedaco(" ") for p in pedacos]]


def _marcar_notas(pedacos: list[_Pedaco]) -> None:
    # Um link inteiro que é nota de redação vira nota.
    i = 0
    while i < len(pedacos):
        if pedacos[i].link and not pedacos[i].nota:
            j = i
            while j < len(pedacos) and pedacos[j].link:
                j += 1
            if _NOTA.match(normalizar("".join(p.texto for p in pedacos[i:j]))):
                for p in pedacos[i:j]:
                    p.nota = True
            i = j
        else:
            i += 1
    # O traço solto que separa o texto da nota ("…;<br> - <nota>") vai com ela.
    for i, p in enumerate(pedacos):
        if not p.nota and _SO_SEPARADOR.match(p.texto):
            seguinte = next((q for q in pedacos[i + 1 :] if q.texto.strip()), None)
            if seguinte is not None and seguinte.nota:
                p.nota = True


def _corridas(pedacos: list[_Pedaco], chave: str) -> list[tuple[bool, str]]:
    corridas: list[tuple[bool, str]] = []
    for p in pedacos:
        valor = bool(getattr(p, chave))
        if corridas and corridas[-1][0] == valor:
            corridas[-1] = (valor, corridas[-1][1] + p.texto)
        else:
            corridas.append((valor, p.texto))
    return corridas


def _marcar_situacao(pedacos: list[_Pedaco]) -> None:
    """ "<strike>VII - …</strike> (Rejeitada)": a marca é nota, o parágrafo é anterior."""
    livres = [p for p in pedacos if not p.nota and not p.riscado and normalizar(p.texto)]
    riscados = [p for p in pedacos if not p.nota and p.riscado and normalizar(p.texto)]
    if not riscados or not livres:
        return
    fora = normalizar(" ".join(p.texto for p in livres))
    if _SITUACAO.match(fora):
        for p in livres:
            p.nota = True
    elif _SO_ROTULO.match(fora):
        # "I - <strike>impostos sobre:</strike>": o rótulo é da redação riscada.
        for p in livres:
            p.riscado = True


def _montar(pedacos: list[_Pedaco], classe: str, numero: int) -> Paragrafo | None:
    _marcar_notas(pedacos)
    _marcar_situacao(pedacos)
    # Notas vizinhas ("(Redação dada…) (Vide…)") saem numa corrida só.
    notas = [
        normalizar(n)
        for nota, t in _corridas(pedacos, "nota")
        if nota
        for n in re.split(r"(?<=\))\s+(?=\()", normalizar(t))
        if normalizar(n)
    ]
    corpo = [p for p in pedacos if not p.nota]
    substantivos = [p for p in corpo if not _SO_PONTUACAO.match(p.texto)]
    anterior = bool(substantivos) and all(p.riscado for p in substantivos)
    classe_limpa = "" if classe == "conteudo-revogado" else classe

    if anterior:
        # Os pedaços são contíguos no original: a nota no meio os separa.
        pedacos_texto = [normalizar(t) for nota, t in _corridas(pedacos, "nota") if not nota]
        pedacos_texto = [t for t in pedacos_texto if t]
        riscado: list[str] = []
    else:
        pedacos_texto = []
        atual = ""
        for p in pedacos:
            if p.nota or p.riscado:
                if normalizar(atual):
                    pedacos_texto.append(normalizar(atual))
                atual = ""
            else:
                atual += p.texto
        if normalizar(atual):
            pedacos_texto.append(normalizar(atual))
        riscado = [normalizar(r) for r in _juntar_riscados(pedacos)]

    texto = normalizar(" ".join(pedacos_texto))
    if not anterior and _SITUACAO.match(texto):
        # "(Rejeitada)" num parágrafo só: é a situação do dispositivo de antes.
        notas.insert(0, texto)
        texto, pedacos_texto = "", []
    # Nota escrita como texto no fim do parágrafo, sem link.
    while (m := _NOTA_NO_FIM.search(texto)) and m.start() > 0:
        notas.insert(0, normalizar(m.group(0)))
        texto = normalizar(texto[: m.start()])
        ultimo = normalizar(pedacos_texto[-1][: len(pedacos_texto[-1]) - len(m.group(0))])
        pedacos_texto[-1] = ultimo if ultimo else pedacos_texto[-1]
        pedacos_texto = [p for p in pedacos_texto if p]
    if not texto and not notas:
        return None
    return Paragrafo(
        id=f"p{numero:04d}",
        texto=texto,
        link=bool(substantivos) and all(p.link for p in substantivos),
        anterior=anterior,
        notas=tuple(notas),
        riscado=tuple(riscado),
        classe=classe_limpa,
        pedacos=tuple(p for p in pedacos_texto if p),
    )


def _juntar_riscados(pedacos: list[_Pedaco]) -> list[str]:
    trechos: list[str] = []
    atual = ""
    for p in pedacos:
        if p.nota:
            continue
        if p.riscado:
            atual += p.texto
        elif normalizar(atual) and p.texto.strip():
            trechos.append(atual)
            atual = ""
    if normalizar(atual):
        trechos.append(atual)
    return [t for t in trechos if normalizar(t)]


def paragrafos_de_html(html: str) -> list[Paragrafo]:
    leitor = _Leitor()
    leitor.feed(html)
    saida: list[Paragrafo] = []
    for bruto in leitor.fim():
        for linha in _dividir_cabecalho(bruto.pedacos):
            p = _montar(linha, bruto.classe, len(saida) + 1)
            if p is not None:
                saida.append(p)
    return saida


_TAG_BLOCO = re.compile(r"</?(?:" + "|".join(sorted(_BLOCOS)) + r"|br)\b[^>]*>", re.IGNORECASE)


def texto_visivel(html: str) -> str:
    """O texto que o navegador mostra, por um caminho independente do parser.

    Serve de conferência: o texto de cada parágrafo tem de aparecer aqui, na
    mesma ordem.
    """
    sem_codigo = re.sub(
        r"<(script|style|head)\b.*?</\1\s*>", " ", html, flags=re.IGNORECASE | re.DOTALL
    )
    sem_blocos = _TAG_BLOCO.sub(" ", sem_codigo)
    sem_tags = re.sub(r"<[^>]*>", "", sem_blocos)
    return normalizar(unescape(sem_tags))


# ------------------------------------------------------------------- PDF


def _normalizar_digitos(linha: str) -> str:
    return re.sub(r"\d+", "#", linha)


def _sem_cabecalho_e_rodape(paginas: list[list[str]]) -> list[list[str]]:
    """Tira a linha de topo/rodapé que se repete de página em página.

    Idêntica em todas (o título do documento): fica só a primeira. Variando só
    nos números (a paginação): sai toda.
    """
    if len(paginas) < 2:
        return paginas
    bordas: Counter[str] = Counter()
    exatas: dict[str, set[str]] = {}
    for linhas in paginas:
        for linha in {linhas[0], linhas[-1]} if linhas else set():
            if rotulos.ler(linha) is not None:
                continue
            chave = _normalizar_digitos(linha)
            bordas[chave] += 1
            exatas.setdefault(chave, set()).add(linha)
    repetidas = {k for k, n in bordas.items() if n >= 2 and n * 2 >= len(paginas)}
    vistas: set[str] = set()
    saida: list[list[str]] = []
    for linhas in paginas:
        nova: list[str] = []
        for i, linha in enumerate(linhas):
            chave = _normalizar_digitos(linha)
            if (i == 0 or i == len(linhas) - 1) and chave in repetidas:
                if len(exatas[chave]) > 1 or chave in vistas:
                    continue
                vistas.add(chave)
            nova.append(linha)
        saida.append(nova)
    return saida


def _comeca_paragrafo(linha: str) -> bool:
    return rotulos.ler(linha) is not None


def paragrafos_de_pdf(paginas: list[str]) -> list[Paragrafo]:
    linhas_por_pagina = [
        [normalizar(linha) for linha in pagina.splitlines() if normalizar(linha)]
        for pagina in paginas
    ]
    linhas = [linha for pagina in _sem_cabecalho_e_rodape(linhas_por_pagina) for linha in pagina]

    blocos: list[tuple[str, list[str]]] = []
    for linha in linhas:
        if not blocos:
            blocos.append((linha, []))
            continue
        atual, juncoes = blocos[-1]
        lido_atual = rotulos.ler(atual)
        cabecalho_sozinho = (
            lido_atual is not None and lido_atual.tipo in ("parte", "livro", "titulo",
                                                            "capitulo", "secao", "subsecao")
            and not lido_atual.resto
        )  # fmt: skip
        novo = (
            _comeca_paragrafo(linha)
            or cabecalho_sozinho
            or (rotulos.em_maiusculas(linha) and not rotulos.em_maiusculas(atual))
            or (rotulos.em_maiusculas(atual) and not rotulos.em_maiusculas(linha))
        )
        if novo:
            blocos.append((linha, []))
        elif re.search(r"[A-Za-zÀ-ú]-$", atual) and re.match(r"[a-zà-ú]", linha):
            fim = atual.rsplit(" ", 1)[-1]
            continuacao = re.match(r"[A-Za-zÀ-ú]+", linha)
            juncoes.append(f"{fim}|{continuacao.group(0) if continuacao else ''}")
            blocos[-1] = (atual[:-1] + linha, juncoes)
        else:
            blocos[-1] = (f"{atual} {linha}", juncoes)

    return [
        Paragrafo(
            id=f"p{i:04d}",
            texto=texto,
            pedacos=(texto,),
            juncoes=tuple(juncoes),
        )
        for i, (texto, juncoes) in enumerate(blocos, start=1)
    ]
