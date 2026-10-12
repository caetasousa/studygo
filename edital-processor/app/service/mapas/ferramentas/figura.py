"""Recorta uma figura do PDF da aula em PNG, sem o cabeçalho nem o rodapé.

Uso: figura.py aula.pdf pagina x0 y0 x1 y1 saida.png

A página conta de 1, e as coordenadas são as do PyMuPDF (pontos, origem no
canto de cima). O recorte nunca entra nas faixas de cima e de baixo, onde ficam
a marca da plataforma e os dados do comprador. Abra o PNG e confira antes de
usar: o mapa aceita até 2 MB por imagem.
"""

import sys
from pathlib import Path

import pymupdf


def main(src, pagina, x0, y0, x1, y1, dst):
    p = pymupdf.open(src)[int(pagina) - 1]
    alto = p.rect.height
    clip = pymupdf.Rect(float(x0), max(50.0, float(y0)), float(x1), min(alto - 62, float(y1)))
    pix = p.get_pixmap(clip=clip, dpi=150)
    pix.save(dst)
    print(dst, pix.width, pix.height, Path(dst).stat().st_size)


if __name__ == "__main__":
    main(*sys.argv[1:8])
