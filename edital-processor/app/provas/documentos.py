"""Regiões limitadas e recortes rastreáveis; nunca rasteriza páginas gigantes."""

from __future__ import annotations

import math
import uuid
from dataclasses import dataclass
from pathlib import Path

import pymupdf
from PIL import Image

from app.core.config import Settings
from app.core.errors import DocumentNotFound, InvalidPDF, RenderLimitExceeded
from app.provas.schemas import Origem
from app.services.validation import validate_upload


def arquivo(root: Path, identificador: str, extensao: str) -> Path:
    return root / f"{uuid.UUID(identificador)}.{extensao}"


def original(root: Path, identificador: str) -> Path:
    """O PDF que o backend gravou. Ausente é problema do volume, não do
    documento: repetir não resolve, e o erro precisa dizer isso."""
    caminho = arquivo(root, identificador, "pdf")
    if not caminho.is_file():
        raise DocumentNotFound("PDF ausente do volume de provas")
    return caminho


def regioes(root: Path, identificador: str, settings: Settings) -> list[Origem]:
    data = original(root, identificador).read_bytes()
    validate_upload(data, "application/pdf", settings)
    result: list[Origem] = []
    with pymupdf.open(stream=data, filetype="pdf") as doc:
        for i, page in enumerate(doc):
            rect = page.rect
            height = min(rect.height, rect.width * 1.42)
            y = 0.0
            while y < rect.height:
                bottom = min(y + height, rect.height)
                result.append(
                    Origem(
                        pagina=i + 1, retangulo=[0, y, rect.width, bottom], regiao=str(len(result))
                    )
                )
                if len(result) > settings.provas_max_regions:
                    raise RenderLimitExceeded("documento excede o limite de regiões")
                if bottom >= rect.height:
                    break
                y = bottom - height * 0.18
    return result


def render(root: Path, identificador: str, origem: Origem, settings: Settings) -> tuple[bytes, str]:
    with pymupdf.open(original(root, identificador)) as doc:
        if origem.pagina > len(doc):
            raise InvalidPDF("página inexistente")
        page = doc[origem.pagina - 1]
        clip = pymupdf.Rect(origem.retangulo)
        # O retângulo chega arredondado da tela: o pé da folha A4 (841,9199…)
        # vira 841,92. Passar da página por menos de meio ponto é a borda dela,
        # e o recorte fica nela; mais do que isso é retângulo errado.
        pagina = page.rect
        folga = pymupdf.Rect(pagina.x0 - 0.5, pagina.y0 - 0.5, pagina.x1 + 0.5, pagina.y1 + 0.5)
        if (
            not all(math.isfinite(v) for v in origem.retangulo)
            or clip.is_empty
            or not folga.contains(clip)
        ):
            raise InvalidPDF("retângulo fora da página")
        clip.intersect(pagina)
        scale = min(2.8, math.sqrt(settings.provas_region_pixels / (clip.width * clip.height)))
        if (
            math.ceil(clip.width * scale) * math.ceil(clip.height * scale)
            > settings.provas_region_pixels + 10000
        ):
            raise RenderLimitExceeded("recorte excede o limite de pixels")
        # get_pixmap usa a página rotacionada; a evidência também usa esse sistema.
        pix = page.get_pixmap(matrix=pymupdf.Matrix(scale, scale), clip=clip, alpha=False)
        text = page.get_text("text", clip=clip * page.derotation_matrix)
        return pix.tobytes("png"), str(text)


# Espaço de nomes dos ids de recorte. Fixo: mudá-lo faria todo recorte já
# pedido ser gravado de novo com outro id.
_RECORTES = uuid.UUID("5b0f7a52-2f7e-4d8e-9a57-3c1f64b2e9d1")


def id_do_recorte(identificador: str, origem: Origem) -> str:
    coordenadas = ",".join(f"{v:.2f}" for v in origem.retangulo)
    return str(uuid.uuid5(_RECORTES, f"{uuid.UUID(identificador)}:{origem.pagina}:{coordenadas}"))


def recortar(root: Path, identificador: str, origem: Origem, settings: Settings) -> str:
    """Grava o PNG de um retângulo do original e devolve o id dele.

    O id vem do documento e do retângulo. A tela de revisão pede a prévia da
    região toda vez que o curador volta a ela; com id aleatório, cada visita
    deixava um PNG de megabytes no volume durável, para sempre.
    """
    asset = id_do_recorte(identificador, origem)
    target = arquivo(root, asset, "png")
    if target.exists():
        return asset

    png, _ = render(root, identificador, origem, settings)
    # Temporário com nome único: dois pedidos do mesmo recorte ao mesmo tempo
    # não renomeiam o arquivo pela metade um do outro.
    temp = target.with_suffix(f".{uuid.uuid4().hex}.tmp")
    temp.write_bytes(png)
    temp.replace(target)
    return asset


# --- ajuste do recorte da figura ---------------------------------------------
#
# O retângulo que o Gemini dá para uma figura erra para os dois lados: pega o
# cabeçalho e as linhas de texto de cima e de baixo, ou corta o título do
# gráfico. O ajuste olha os pixels — vale para PDF de texto e para escaneado:
# o trecho vira faixas horizontais de tinta; o núcleo é a maior faixa dentro
# da caixa (o gráfico, o diagrama), e a figura cresce com as faixas próximas
# (título, rótulos, as outras linhas de quadradinhos) até esbarrar em prosa —
# faixa que sai da largura da figura e atravessa a janela, como uma linha de
# texto da coluna ou o fio do cabeçalho.

_ESCALA_DO_AJUSTE = 3.0  # pixels por ponto
_TINTA = 160  # abaixo disto, na escala de cinza, é tinta


@dataclass
class _Faixa:
    y0: int
    y1: int
    x0: int
    x1: int

    @property
    def altura(self) -> int:
        return self.y1 - self.y0

    @property
    def largura(self) -> int:
        return self.x1 - self.x0


def ajustar_figura(
    root: Path, identificador: str, caixa: Origem, limite: list[float], settings: Settings
) -> Origem:
    """A caixa da figura ajustada à figura: sem o texto em volta, com o que o
    modelo cortou dela. `limite` é o retângulo da região lida. Na dúvida, a
    caixa volta como veio — o curador ainda pode recortar à mão."""
    x0, y0, x1, y1 = caixa.retangulo
    largura, altura = x1 - x0, y1 - y0
    if largura <= 0 or altura <= 0:
        return caixa
    # Folga para achar o que o modelo cortou: o título costuma ficar acima.
    janela = [
        max(limite[0], x0 - 0.2 * largura),
        max(limite[1], y0 - 0.5 * altura),
        min(limite[2], x1 + 0.2 * largura),
        min(limite[3], y1 + 0.5 * altura),
    ]
    with pymupdf.open(original(root, identificador)) as doc:
        if caixa.pagina > len(doc):
            return caixa
        pix = doc[caixa.pagina - 1].get_pixmap(
            matrix=pymupdf.Matrix(_ESCALA_DO_AJUSTE, _ESCALA_DO_AJUSTE),
            clip=pymupdf.Rect(janela),
            colorspace=pymupdf.csGRAY,
            alpha=False,
        )
    tinta = Image.frombytes("L", (pix.width, pix.height), pix.samples).point(
        lambda v: 255 if v < _TINTA else 0
    )

    s = _ESCALA_DO_AJUSTE
    ajuste = _ajustar(
        tinta,
        [(x0 - janela[0]) * s, (y0 - janela[1]) * s, (x1 - janela[0]) * s, (y1 - janela[1]) * s],
    )
    if ajuste is None:
        return caixa
    fx0, fy0, fx1, fy1 = ajuste
    return caixa.model_copy(
        update={
            "retangulo": [
                janela[0] + fx0 / s,
                janela[1] + fy0 / s,
                janela[0] + fx1 / s,
                janela[1] + fy1 / s,
            ]
        }
    )


def _faixas(tinta: Image.Image) -> list[_Faixa]:
    """Faixas horizontais de tinta, separadas por linhas brancas. Um vão de
    até um ponto não separa: acento e sublinhado são da mesma linha."""
    largura, altura = tinta.size
    faixas: list[_Faixa] = []
    atual: _Faixa | None = None
    vao = 0
    for y in range(altura):
        caixa = tinta.crop((0, y, largura, y + 1)).getbbox()
        if caixa is None:
            vao += 1
            continue
        if atual is not None and vao <= _ESCALA_DO_AJUSTE:
            atual.y1 = y + 1
            atual.x0, atual.x1 = min(atual.x0, caixa[0]), max(atual.x1, caixa[2])
        else:
            atual = _Faixa(y, y + 1, caixa[0], caixa[2])
            faixas.append(atual)
        vao = 0
    return faixas


def _ajustar(tinta: Image.Image, caixa: list[float]) -> list[float] | None:
    faixas = _faixas(tinta)
    largura_da_janela = tinta.size[0]
    s = _ESCALA_DO_AJUSTE

    def atravessa(f: _Faixa) -> bool:
        # Linha de texto da coluna, ou fio de cabeçalho: a janela a corta, ou
        # ela ocupa quase toda a janela.
        return f.x0 <= 2 or f.x1 >= largura_da_janela - 2 or f.largura >= 0.8 * largura_da_janela

    def sobre_a_caixa(f: _Faixa) -> float:
        return max(0.0, min(f.y1, caixa[3]) - max(f.y0, caixa[1])) * f.largura

    candidatas = [f for f in faixas if not atravessa(f) and f.altura >= s and sobre_a_caixa(f) > 0]
    if not candidatas:
        return None
    nucleo = max(candidatas, key=sobre_a_caixa)
    i = faixas.index(nucleo)

    # A altura de uma linha de texto, pela mediana das faixas desse tamanho:
    # vão maior que três linhas separa a figura do que vem depois.
    linhas = sorted(f.altura for f in faixas if 3 * s <= f.altura <= 14 * s)
    vao_maximo = 3 * (linhas[len(linhas) // 2] if linhas else 6 * s)

    grupo = [nucleo.x0, nucleo.y0, nucleo.x1, nucleo.y1]

    def cabe(f: _Faixa) -> bool:
        # Linha de texto passa da figura pelos dois lados; parte da figura
        # (o "+" antes dos quadradinhos) passa, no máximo, por um.
        tolerancia = 0.05 * (grupo[2] - grupo[0]) + 2 * s
        esquerda = f.x0 < grupo[0] - tolerancia
        direita = f.x1 > grupo[2] + tolerancia
        return not (esquerda and direita) and not ((esquerda or direita) and atravessa(f))

    acima = abaixo = None
    for f in reversed(faixas[:i]):
        if grupo[1] - f.y1 > vao_maximo or not cabe(f):
            acima = f
            break
        grupo = [min(grupo[0], f.x0), f.y0, max(grupo[2], f.x1), grupo[3]]
    for f in faixas[i + 1 :]:
        if f.y0 - grupo[3] > vao_maximo or not cabe(f):
            abaixo = f
            break
        grupo = [min(grupo[0], f.x0), grupo[1], max(grupo[2], f.x1), f.y1]

    # Figura que encolheu demais é leitura errada da página: fica a do modelo.
    area = (grupo[2] - grupo[0]) * (grupo[3] - grupo[1])
    if area < 0.15 * (caixa[2] - caixa[0]) * (caixa[3] - caixa[1]) or grupo[3] - grupo[1] < 10 * s:
        return None

    # Margem de respiro, sem entrar na linha de texto vizinha.
    margem = 4 * s
    topo = grupo[1] - margem if acima is None else max(grupo[1] - margem, (acima.y1 + grupo[1]) / 2)
    fundo = (
        grupo[3] + margem if abaixo is None else min(grupo[3] + margem, (grupo[3] + abaixo.y0) / 2)
    )
    return [
        max(0.0, grupo[0] - margem),
        max(0.0, topo),
        min(float(largura_da_janela), grupo[2] + margem),
        min(float(tinta.size[1]), fundo),
    ]
