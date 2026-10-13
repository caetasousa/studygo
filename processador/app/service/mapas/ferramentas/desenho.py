"""As figuras didáticas do mapa sem aula, no padrão das que os mapas já têm.

Uso: desenho.py figura.json saida.png

Sem PDF de onde recortar, o que se aprende pelo desenho — um processo, os
níveis de um modelo, uma hierarquia, um quadro comparativo — é desenhado aqui,
sempre com o mesmo visual: fundo branco, título em negrito, caixas
arredondadas na paleta de sempre, setas cinza e uma nota de rodapé. Quem
estuda não pode notar diferença entre um mapa de aula e um sem aula.

O JSON diz o modelo e o conteúdo; a altura da figura acompanha o texto, que
quebra dentro das caixas. Cada item é [título, texto] (ou só o título):

  {"modelo": "fluxo", "titulo": "...", "etapas": [["Planejamento", "objetivos"], ...],
   "faixa": ["Monitoramento", "acompanha todas"], "nota": "..."}
  {"modelo": "camadas", "titulo": "...", "niveis": [[...], ...]}      (de cima para baixo)
  {"modelo": "piramide", "titulo": "...", "camadas": [[...], ...]}    (do topo à base)
  {"modelo": "escada", "titulo": "...", "degraus": [[...], ...]}      (do mais baixo ao topo)
  {"modelo": "hierarquia", "titulo": "...", "raiz": "...", "filhos": [[...], ...]}
  {"modelo": "grupos", "titulo": "...", "grupos": [["Grupo", ["item", ...]], ...]}
  {"modelo": "quadrantes", "titulo": "...", "quadrantes": [sup-esq, sup-dir, inf-esq, inf-dir],
   "eixo_x": ["à esquerda", "à direita"], "eixo_y": ["em cima", "embaixo"]}
  {"modelo": "cartoes", "titulo": "...", "cartoes": [[...], ...]}
  {"modelo": "ciclo", "titulo": "...", "etapas": [[...], ...], "centro": "..."}

Para o que nenhum modelo serve, desenhe com a classe Desenho, que é o mesmo
visual (no script: sys.path.insert(0, "<esta pasta>"); from desenho import Desenho).
Nunca baixe nem copie imagem. Abra o PNG e confira que lê bem: o mapa aceita
até 2 MB por imagem.
"""

import html
import json
import math
import sys
from pathlib import Path

import pymupdf

# (fundo, borda e título) de cada cor: as das figuras dos mapas.
COR = {
    "azul": ("#e8f0fe", "#3b6fd8"),
    "verde": ("#e6f4ea", "#2e7d4f"),
    "laranja": ("#fdf0e3", "#d9822b"),
    "roxo": ("#f1eafb", "#7b4fc4"),
    "amarelo": ("#fff8e1", "#b8860b"),
    "vermelho": ("#fdeaea", "#c0392b"),
    "cinza": ("#f1f3f4", "#5f6368"),
    "azulescuro": ("#1a2b4c", "#1a2b4c"),
}
SEQUENCIA = ["azul", "verde", "laranja", "roxo", "amarelo", "vermelho", "cinza"]
TINTA = "#1f2937"
APAGADO = "#5f6368"
LARGURA = 1200
MARGEM = 20
LIMITE = 2 * 1024 * 1024


def quebrar(texto, largura_px, tam):
    """As linhas do texto numa largura, pela largura média de um caractere."""
    maxc = max(4, int(largura_px / (tam * 0.53)))
    linhas = []
    for par in str(texto).split("\n"):
        atual = ""
        for p in par.split(" "):
            if len(atual) + len(p) + (1 if atual else 0) <= maxc:
                atual = (atual + " " + p).strip()
            else:
                if atual:
                    linhas.append(atual)
                atual = p
        linhas.append(atual)
    return linhas


def altura_do_texto(texto, largura_px, tam, entre=1.25):
    return len(quebrar(texto, largura_px, tam)) * tam * entre


class Desenho:
    """Um SVG que cresce para baixo: a altura sai do que foi desenhado."""

    def __init__(self, largura=LARGURA):
        self.w = largura
        self.el = []
        self.fundo = 0.0

    def _ate(self, y):
        self.fundo = max(self.fundo, y)

    def texto(self, x, y, s, tam=14, cor=TINTA, peso="normal", ancora="middle"):
        self.el.append(
            f'<text x="{x:.1f}" y="{y:.1f}" font-family="Helvetica" font-size="{tam}" '
            f'font-weight="{peso}" text-anchor="{ancora}" fill="{cor}">{html.escape(str(s))}</text>'
        )
        self._ate(y + tam * 0.35)

    def bloco(self, x, y, larg, s, tam=14, cor=TINTA, peso="normal", ancora="middle", entre=1.25):
        """Texto quebrado na largura, a partir de y (base da 1ª linha). Devolve a altura."""
        linhas = quebrar(s, larg, tam)
        cx = x + larg / 2 if ancora == "middle" else x
        for i, linha in enumerate(linhas):
            self.texto(cx, y + i * tam * entre, linha, tam, cor, peso, ancora)
        return len(linhas) * tam * entre

    @staticmethod
    def altura_da_caixa(w, titulo=None, corpo=None, tam=16, tam_corpo=None):
        tc = tam_corpo or tam - 2
        alt = altura_do_texto(titulo, w - 24, tam) if titulo else 0
        if corpo:
            alt += altura_do_texto(corpo, w - 24, tc) + (6 if titulo else 0)
        return alt + 28

    def caixa(self, x, y, w, h, cor="azul", titulo=None, corpo=None, tam=16, rx=10, tam_corpo=None):
        f, s = COR[cor]
        tc = tam_corpo or tam - 2
        self.el.append(
            f'<rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" rx="{rx}" '
            f'fill="{f}" stroke="{s}" stroke-width="2"/>'
        )
        self._ate(y + h)
        alt = self.altura_da_caixa(w, titulo, corpo, tam, tam_corpo) - 28
        yy = y + (h - alt) / 2 + tam * 0.9
        cor_titulo = "#ffffff" if cor == "azulescuro" else s
        if titulo:
            yy += self.bloco(x + 12, yy, w - 24, titulo, tam, cor_titulo, "bold") + 6
        if corpo:
            self.bloco(x + 12, yy, w - 24, corpo, tc, "#ffffff" if cor == "azulescuro" else TINTA)

    def seta(self, x1, y1, x2, y2, cor=APAGADO, larg=2.5, tracejada=False):
        a = math.atan2(y2 - y1, x2 - x1)
        ponta = 11
        p1 = (x2 - ponta * math.cos(a - 0.4), y2 - ponta * math.sin(a - 0.4))
        p2 = (x2 - ponta * math.cos(a + 0.4), y2 - ponta * math.sin(a + 0.4))
        tr = ' stroke-dasharray="7 5"' if tracejada else ""
        self.el.append(
            f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2 - 6 * math.cos(a):.1f}" '
            f'y2="{y2 - 6 * math.sin(a):.1f}" stroke="{cor}" stroke-width="{larg}"{tr}/>'
        )
        self.el.append(
            f'<polygon points="{x2:.1f},{y2:.1f} {p1[0]:.1f},{p1[1]:.1f} {p2[0]:.1f},{p2[1]:.1f}" '
            f'fill="{cor}"/>'
        )
        self._ate(max(y1, y2))

    def linha(self, x1, y1, x2, y2, cor="#9aa0a6", larg=2):
        self.el.append(
            f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" '
            f'stroke="{cor}" stroke-width="{larg}"/>'
        )
        self._ate(max(y1, y2))

    def poligono(self, pontos, cor="azul"):
        f, s = COR[cor]
        pts = " ".join(f"{a:.1f},{b:.1f}" for a, b in pontos)
        self.el.append(f'<polygon points="{pts}" fill="{f}" stroke="{s}" stroke-width="2"/>')
        self._ate(max(b for _, b in pontos))

    def titulo(self, titulo):
        """O título da figura; devolve onde o desenho começa."""
        alt = self.bloco(MARGEM, 40, self.w - 2 * MARGEM, titulo, 22, peso="bold")
        return 40 + alt + 14

    def nota(self, nota, y):
        if nota:
            self.bloco(MARGEM, y + 34, self.w - 2 * MARGEM, nota, 14, cor=APAGADO)

    def salvar(self, destino):
        h = math.ceil(self.fundo + MARGEM + 6)
        svg = (
            f'<svg xmlns="http://www.w3.org/2000/svg" width="{self.w}" height="{h}">'
            f'<rect width="{self.w}" height="{h}" fill="#ffffff"/>' + "".join(self.el) + "</svg>"
        )
        with pymupdf.open(stream=svg.encode("utf-8"), filetype="svg") as doc:
            pix = doc[0].get_pixmap(dpi=144, alpha=False)
        pix.save(destino)
        tamanho = Path(destino).stat().st_size
        print(f"{destino}: {pix.width}x{pix.height}, {tamanho // 1024} KB")
        if tamanho > LIMITE:
            print("aviso: passa de 2 MB — o mapa recusa; encurte o conteúdo ou divida a figura")


# --- os modelos -----------------------------------------------------------------


class EspecificacaoInvalida(Exception):
    pass


def _par(item):
    """[título, texto], {"titulo", "texto"} ou só o título."""
    if isinstance(item, dict):
        return str(item.get("titulo", "")), item.get("texto")
    if isinstance(item, list | tuple):
        return str(item[0]), (item[1] if len(item) > 1 else None)
    return str(item), None


def _lista(spec, campo):
    valor = spec.get(campo)
    if not isinstance(valor, list) or not valor:
        raise EspecificacaoInvalida(
            f"o modelo {spec['modelo']!r} precisa de {campo!r}: uma lista não vazia"
        )
    return valor


def fluxo(d, spec):
    etapas = [_par(e) for e in _lista(spec, "etapas")]
    y = d.titulo(spec["titulo"])
    n = len(etapas)
    por_linha = n if n <= 6 else math.ceil(n / 2)
    vao = 30
    w = (d.w - 2 * MARGEM - (por_linha - 1) * vao) / por_linha
    centros = []
    for inicio in range(0, n, por_linha):
        linha = etapas[inicio : inicio + por_linha]
        h = max(100, *(d.altura_da_caixa(w, t, c) for t, c in linha))
        for i, (t, c) in enumerate(linha):
            x = MARGEM + i * (w + vao)
            d.caixa(x, y, w, h, SEQUENCIA[(inicio + i) % len(SEQUENCIA)], t, c)
            if i < len(linha) - 1:
                d.seta(x + w + 3, y + h / 2, x + w + vao - 3, y + h / 2)
            centros.append((x + w / 2, y, y + h))
        if inicio + por_linha < n:
            # Da última caixa desta linha para a primeira da próxima.
            xa, _, fundo = centros[-1]
            d.seta(xa, fundo + 4, MARGEM + w / 2, fundo + 46, tracejada=True)
        y += h + 50
    y -= 50
    faixa = spec.get("faixa")
    if faixa:
        t, c = _par(faixa)
        topo = y + 60
        h = d.altura_da_caixa(d.w - 2 * MARGEM, t, c)
        d.caixa(MARGEM, topo, d.w - 2 * MARGEM, h, "roxo", t, c)
        for cx, _, fundo in centros:
            if fundo == y:
                d.seta(cx, topo - 3, cx, fundo + 4, cor=COR["roxo"][1], larg=2, tracejada=True)
        y = topo + h
    d.nota(spec.get("nota"), y)


def camadas(d, spec):
    niveis = [_par(e) for e in _lista(spec, "niveis")]
    y = d.titulo(spec["titulo"])
    n = len(niveis)
    for i, (t, c) in enumerate(niveis):
        recuo = MARGEM + 20 + (i if n > 1 else 0) * min(30, 200 / max(1, n - 1))
        w = d.w - 2 * recuo
        h = max(70, d.altura_da_caixa(w, t, c, 17))
        d.caixa(recuo, y, w, h, SEQUENCIA[i % len(SEQUENCIA)], t, c, 17)
        y += h + 14
    d.nota(spec.get("nota"), y - 14)


def piramide(d, spec):
    camadas_ = [_par(e) for e in _lista(spec, "camadas")]
    y0 = d.titulo(spec["titulo"])
    n = len(camadas_)
    alt = max(90, 300 / n)
    base = d.w - 2 * MARGEM - 200
    cx = d.w / 2

    def larg(y):
        return (y - y0) / (n * alt) * base

    for i, (t, c) in enumerate(camadas_):
        ya, yb = y0 + i * alt, y0 + (i + 1) * alt
        d.poligono(
            [
                (cx - larg(ya) / 2, ya),
                (cx + larg(ya) / 2, ya),
                (cx + larg(yb) / 2, yb),
                (cx - larg(yb) / 2, yb),
            ],
            SEQUENCIA[(n - 1 - i) % len(SEQUENCIA)],
        )
        meio = max(larg(ya), 200)
        d.bloco(cx - meio / 2, ya + alt * 0.55, meio, t, 16, peso="bold")
        if c:
            d.bloco(cx - 160, ya + alt * 0.55 + 22, 320, c, 13, cor="#3c4043")
    d.nota(spec.get("nota"), y0 + n * alt)


def escada(d, spec):
    degraus = [_par(e) for e in _lista(spec, "degraus")]
    y = d.titulo(spec["titulo"])
    n = len(degraus)
    passo = min(70, 500 / max(1, n - 1)) if n > 1 else 0
    alturas = []
    for i, (t, c) in enumerate(degraus):
        w = d.w - 2 * MARGEM - i * passo
        alturas.append(max(54, d.altura_da_caixa(w, t, c, 16, 13)))
    topo = y
    for i in range(n - 1, -1, -1):
        t, c = degraus[i]
        x = MARGEM + i * passo
        w = d.w - MARGEM - x
        h = alturas[i]
        d.caixa(x, topo, w, h, SEQUENCIA[i % len(SEQUENCIA)])
        yy = topo + 24
        yy += d.bloco(x + 14, yy, w - 28, t, 16, peso="bold", ancora="start")
        if c:
            d.bloco(x + 14, yy + 2, w - 28, c, 13, ancora="start")
        topo += h + 8
    d.nota(spec.get("nota"), topo - 8)


def hierarquia(d, spec):
    filhos = [_par(e) for e in _lista(spec, "filhos")]
    raiz = spec.get("raiz")
    if not raiz:
        raise EspecificacaoInvalida("o modelo 'hierarquia' precisa de 'raiz'")
    y = d.titulo(spec["titulo"])
    wr = min(420, d.w - 2 * MARGEM)
    hr = max(50, d.altura_da_caixa(wr, str(raiz), None, 17))
    d.caixa((d.w - wr) / 2, y, wr, hr, "azulescuro", str(raiz), tam=17)
    n = len(filhos)
    vao = 10
    w = (d.w - 2 * MARGEM - (n - 1) * vao) / n
    tam = 15 if w > 180 else 13
    h = max(*(d.altura_da_caixa(w, t, c, tam, tam - 2) for t, c in filhos), 80)
    yf = y + hr + 40
    for i, (t, c) in enumerate(filhos):
        x = MARGEM + i * (w + vao)
        d.linha(d.w / 2, y + hr, x + w / 2, yf)
        d.caixa(x, yf, w, h, SEQUENCIA[i % len(SEQUENCIA)], t, c, tam, tam_corpo=tam - 2)
    d.nota(spec.get("nota"), yf + h)


def grupos(d, spec):
    grupos_ = _lista(spec, "grupos")
    y = d.titulo(spec["titulo"])
    n = len(grupos_)
    vao = 13
    w = (d.w - 2 * MARGEM - (n - 1) * vao) / n
    cabecas = []
    for g in grupos_:
        if not isinstance(g, list | tuple) or len(g) != 2 or not isinstance(g[1], list):
            raise EspecificacaoInvalida("cada grupo é [título, [itens]]")
        cabecas.append(str(g[0]))
    hc = max(60, *(d.altura_da_caixa(w, t, None, 16) for t in cabecas))
    fundo = y + hc
    for i, (t, itens) in enumerate(grupos_):
        x = MARGEM + i * (w + vao)
        d.caixa(x, y, w, hc, SEQUENCIA[i % len(SEQUENCIA)], str(t), tam=16)
        yi = y + hc + 10
        for item in itens:
            hi = max(30, altura_do_texto(item, w - 44, 13) + 14)
            d.caixa(x + 10, yi, w - 20, hi, "cinza", rx=6)
            d.bloco(x + 22, yi + 7 + 13 * 0.9, w - 44, item, 13)
            yi += hi + 6
        fundo = max(fundo, yi - 6)
    d.nota(spec.get("nota"), fundo)


def quadrantes(d, spec):
    qs = [_par(e) for e in _lista(spec, "quadrantes")]
    if len(qs) != 4:
        raise EspecificacaoInvalida("o modelo 'quadrantes' precisa de 4 quadrantes")
    y = d.titulo(spec["titulo"])
    ex = spec.get("eixo_x") or ["", ""]
    ey = spec.get("eixo_y") or ["", ""]
    lateral = 120 if any(ex) else MARGEM
    w = (d.w - 2 * lateral - 20) / 2
    h = max(200, *(d.altura_da_caixa(w, t, c, 22, 16) for t, c in qs))
    if ey[0]:
        d.texto(d.w / 2, y + 10, ey[0], 15, APAGADO, "bold")
        y += 30
    for i, (t, c) in enumerate(qs):
        x = lateral + (i % 2) * (w + 20)
        yy = y + (i // 2) * (h + 20)
        d.caixa(x, yy, w, h, ["verde", "laranja", "azul", "roxo"][i], t, c, 22, tam_corpo=16)
    meio = y + h + 10
    if ex[0]:
        d.bloco(MARGEM, meio, lateral - 30, ex[0], 14, APAGADO, "bold")
    if ex[1]:
        d.bloco(d.w - lateral + 10, meio, lateral - 30, ex[1], 14, APAGADO, "bold")
    fundo = y + 2 * h + 20
    if ey[1]:
        d.texto(d.w / 2, fundo + 28, ey[1], 15, APAGADO, "bold")
        fundo += 34
    d.nota(spec.get("nota"), fundo)


def cartoes(d, spec):
    itens = [_par(e) for e in _lista(spec, "cartoes")]
    y = d.titulo(spec["titulo"])
    n = len(itens)
    por_linha = n if n <= 5 else math.ceil(n / 2)
    vao = 16
    w = (d.w - 2 * MARGEM - (por_linha - 1) * vao) / por_linha
    for inicio in range(0, n, por_linha):
        linha = itens[inicio : inicio + por_linha]
        h = max(140, *(d.altura_da_caixa(w, t, c, 19, 15) for t, c in linha))
        for i, (t, c) in enumerate(linha):
            d.caixa(
                MARGEM + i * (w + vao),
                y,
                w,
                h,
                SEQUENCIA[(inicio + i) % len(SEQUENCIA)],
                t,
                c,
                19,
                tam_corpo=15,
            )
        y += h + vao
    d.nota(spec.get("nota"), y - vao)


def _borda(cx, cy, w, h, dx, dy):
    """O ponto da borda da caixa centrada em (cx, cy) na direção (dx, dy)."""
    t = min(w / 2 / abs(dx) if dx else math.inf, h / 2 / abs(dy) if dy else math.inf)
    return cx + dx * t, cy + dy * t


def ciclo(d, spec):
    etapas = [_par(e) for e in _lista(spec, "etapas")]
    y = d.titulo(spec["titulo"])
    n = len(etapas)
    w = 250
    h = max(80, *(d.altura_da_caixa(w, t, c) for t, c in etapas))
    rx, ry = (d.w - 2 * MARGEM - w) / 2, max(150, 50 * n)
    cx, cy = d.w / 2, y + ry + h / 2
    centros = []
    for i in range(n):
        a = -math.pi / 2 + 2 * math.pi * i / n
        centros.append((cx + rx * math.cos(a), cy + ry * math.sin(a)))
    for i, (t, c) in enumerate(etapas):
        x, yy = centros[i]
        d.caixa(x - w / 2, yy - h / 2, w, h, SEQUENCIA[i % len(SEQUENCIA)], t, c)
    for i in range(n):
        (x1, y1), (x2, y2) = centros[i], centros[(i + 1) % n]
        dx, dy = x2 - x1, y2 - y1
        norma = math.hypot(dx, dy)
        dx, dy = dx / norma, dy / norma
        a = _borda(x1, y1, w + 12, h + 12, dx, dy)
        b = _borda(x2, y2, w + 12, h + 12, -dx, -dy)
        d.seta(a[0], a[1], b[0], b[1])
    centro = spec.get("centro")
    if centro:
        t, c = _par(centro)
        wc = 260
        hc = d.altura_da_caixa(wc, t, c, 15)
        d.caixa(cx - wc / 2, cy - hc / 2, wc, hc, "cinza", t, c, 15)
    d.nota(spec.get("nota"), max(yc for _, yc in centros) + h / 2)


MODELOS = {
    "fluxo": fluxo,
    "camadas": camadas,
    "piramide": piramide,
    "escada": escada,
    "hierarquia": hierarquia,
    "grupos": grupos,
    "quadrantes": quadrantes,
    "cartoes": cartoes,
    "ciclo": ciclo,
}


def desenhar(spec, destino):
    if not isinstance(spec, dict):
        raise EspecificacaoInvalida("a especificação é um objeto JSON")
    modelo = MODELOS.get(spec.get("modelo"))
    if modelo is None:
        raise EspecificacaoInvalida(
            f"modelo {spec.get('modelo')!r} não existe; use um destes: {', '.join(MODELOS)}"
        )
    if not spec.get("titulo"):
        raise EspecificacaoInvalida("toda figura tem 'titulo'")
    d = Desenho(int(spec.get("largura", LARGURA)))
    modelo(d, spec)
    d.salvar(destino)


def main(entrada, destino):
    try:
        spec = json.loads(Path(entrada).read_text(encoding="utf-8"))
        desenhar(spec, destino)
    except (EspecificacaoInvalida, ValueError) as exc:
        print(f"erro: {exc}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(__doc__, file=sys.stderr)
        sys.exit(2)
    main(sys.argv[1], sys.argv[2])
