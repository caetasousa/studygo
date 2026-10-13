"""Converte um diagrama em SVG, desenhado para o mapa, num PNG.

Uso: diagrama.py desenho.svg saida.png

O mapa sem aula não tem PDF de onde recortar figura: o que se aprende pelo
desenho é desenhado em SVG, e o PyMuPDF o rasteriza — sem baixar nem copiar
imagem de lugar nenhum. Fundo branco, 150 dpi; o mapa aceita até 2 MB por
imagem. Abra o PNG e confira que lê bem.
"""

import sys
from pathlib import Path

import pymupdf


def main(src, dst):
    with pymupdf.open(src) as doc:
        pix = doc[0].get_pixmap(dpi=150, alpha=False)
    pix.save(dst)
    tamanho = Path(dst).stat().st_size
    print(f"{dst}: {pix.width}x{pix.height}, {tamanho // 1024} KB")
    if tamanho > 2 * 1024 * 1024:
        print("aviso: passa de 2 MB — simplifique o desenho ou reduza o tamanho do SVG")


if __name__ == "__main__":
    main(*sys.argv[1:])
