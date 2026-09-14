"""Selective OCR (spec §7.9-7.10).

Renders the pages ``ocr_decision`` picked to images and runs Tesseract on each,
with a per-page timeout and a concurrency cap. ``image_to_data`` is used so word
bounding boxes survive for evidence; the plain text is rebuilt from those words.

Tesseract is an external binary. When it is missing the module raises
``OCRUnavailable`` — the caller degrades to "no OCR for this page", never a
crash.
"""

from __future__ import annotations

import concurrent.futures
import io
from dataclasses import dataclass, field

import pymupdf

from app.core.config import Settings
from app.core.errors import OCRUnavailable
from app.core.logging import get_logger
from app.services.normalize import normalize_text

_log = get_logger(__name__)


@dataclass(frozen=True)
class WordBox:
    text: str
    x0: float
    y0: float
    x1: float
    y1: float
    conf: float  # 0..1


@dataclass
class OCRPage:
    physical_page: int
    text: str
    words: list[WordBox] = field(default_factory=list)
    mean_conf: float | None = None


def _import_tesseract() -> object:
    try:
        import pytesseract
    except ImportError as exc:  # pragma: no cover - import guard
        raise OCRUnavailable("pytesseract is not installed") from exc
    try:
        pytesseract.get_tesseract_version()
    except Exception as exc:
        raise OCRUnavailable("the tesseract binary is not available") from exc
    return pytesseract


def _render_page(data: bytes, physical_page: int, dpi: int) -> bytes:
    doc = pymupdf.open(stream=data, filetype="pdf")
    try:
        page = doc.load_page(physical_page - 1)
        zoom = dpi / 72.0
        pix = page.get_pixmap(matrix=pymupdf.Matrix(zoom, zoom), alpha=False)
        return bytes(pix.tobytes("png"))
    finally:
        doc.close()


def _ocr_one(png: bytes, physical_page: int, settings: Settings, tess: object) -> OCRPage:
    from PIL import Image

    image = Image.open(io.BytesIO(png))
    scale = 72.0 / settings.ocr_dpi  # px -> pt, so boxes match the page geometry

    data = tess.image_to_data(  # type: ignore[attr-defined]
        image,
        lang=settings.ocr_language,
        output_type=tess.Output.DICT,  # type: ignore[attr-defined]
        timeout=settings.ocr_timeout_seconds,
    )

    words: list[WordBox] = []
    confs: list[float] = []
    lines: dict[tuple[int, int, int], list[str]] = {}
    for i, raw in enumerate(data["text"]):
        token = raw.strip()
        conf = float(data["conf"][i])
        if not token or conf < 0:
            continue
        x, y, w, h = (data["left"][i], data["top"][i], data["width"][i], data["height"][i])
        words.append(
            WordBox(
                text=token,
                x0=x * scale,
                y0=y * scale,
                x1=(x + w) * scale,
                y1=(y + h) * scale,
                conf=conf / 100.0,
            )
        )
        confs.append(conf / 100.0)
        key = (data["block_num"][i], data["par_num"][i], data["line_num"][i])
        lines.setdefault(key, []).append(token)

    text = normalize_text("\n".join(" ".join(w) for w in lines.values()))
    mean_conf = sum(confs) / len(confs) if confs else None
    return OCRPage(physical_page=physical_page, text=text, words=words, mean_conf=mean_conf)


def ocr_image(png: bytes, settings: Settings) -> str:
    """OCR de uma imagem já renderizada — um recorte de prova. Existe ao lado de
    run_ocr porque aquele renderiza a página inteira, e a página de uma prova
    pode ter dezenas de milhões de pixels. Levanta OCRUnavailable sem o
    Tesseract."""
    return ocr_image_com_linhas(png, settings)[0]


@dataclass(frozen=True)
class LinhaOCR:
    """Uma linha do OCR de um recorte. topo e pe vão de 0 (alto do recorte) a 1
    (pé): é a medida que serve para recortar de novo o mesmo retângulo."""

    texto: str
    topo: float
    pe: float


def ocr_image_com_linhas(png: bytes, settings: Settings) -> tuple[str, list[LinhaOCR]]:
    """ocr_image, mais as linhas com a altura de cada uma — é por elas que se
    acha onde começa e termina um texto de apoio no recorte."""
    from PIL import Image

    tess = _import_tesseract()
    imagem = Image.open(io.BytesIO(png))
    data = tess.image_to_data(  # type: ignore[attr-defined]
        imagem,
        lang=settings.ocr_language,
        output_type=tess.Output.DICT,  # type: ignore[attr-defined]
        timeout=settings.ocr_timeout_seconds,
    )
    return texto_por_paragrafo(data), linhas_do_ocr(data, imagem.height)


def linhas_do_ocr(data: dict[str, list[object]], altura: int) -> list[LinhaOCR]:
    """As linhas do image_to_data, de cima para baixo, com a altura relativa."""
    linhas: dict[tuple[object, object, object], tuple[list[str], int, int]] = {}
    for i, bruto in enumerate(data["text"]):
        palavra = str(bruto).strip()
        if not palavra or float(str(data["conf"][i])) < 0:
            continue
        chave = (data["block_num"][i], data["par_num"][i], data["line_num"][i])
        topo = int(str(data["top"][i]))
        pe = topo + int(str(data["height"][i]))
        palavras, t, p = linhas.get(chave, ([], topo, pe))
        palavras.append(palavra)
        linhas[chave] = (palavras, min(t, topo), max(p, pe))
    altura = max(altura, 1)
    return sorted(
        (LinhaOCR(" ".join(w), t / altura, p / altura) for w, t, p in linhas.values()),
        key=lambda linha: linha.topo,
    )


def texto_por_paragrafo(data: dict[str, list[object]]) -> str:
    """Remonta o texto por parágrafo, e não por linha como no edital: o texto de
    uma prova vai ser lido, e a quebra de linha do caderno no meio da frase só
    atrapalha. A hifenização de fim de linha é desfeita como em normalize_text."""
    paragrafos: dict[tuple[object, object], dict[object, list[str]]] = {}
    for i, bruto in enumerate(data["text"]):
        palavra = str(bruto).strip()
        if not palavra or float(str(data["conf"][i])) < 0:
            continue
        paragrafo = paragrafos.setdefault((data["block_num"][i], data["par_num"][i]), {})
        paragrafo.setdefault(data["line_num"][i], []).append(palavra)

    texto = []
    for linhas in paragrafos.values():
        juntas = normalize_text("\n".join(" ".join(palavras) for palavras in linhas.values()))
        texto.append(juntas.replace("\n", " "))
    return "\n".join(texto)


def run_ocr(data: bytes, page_numbers: list[int], settings: Settings) -> list[OCRPage]:
    """OCR the given physical pages. Raises OCRUnavailable if Tesseract is
    missing; a page that times out is simply left out of the result — the
    caller already falls back to that page's native text (see
    pipeline._apply_ocr), so one slow page must not cost every other page its
    OCR just because they happened to share a batch."""
    if not page_numbers:
        return []

    tess = _import_tesseract()
    results: list[OCRPage] = []

    with concurrent.futures.ThreadPoolExecutor(
        max_workers=max(1, settings.ocr_max_concurrency)
    ) as pool:
        futures: dict[concurrent.futures.Future[OCRPage], int] = {}
        for page_number in page_numbers:
            png = _render_page(data, page_number, settings.ocr_dpi)
            futures[pool.submit(_ocr_one, png, page_number, settings, tess)] = page_number

        for future in concurrent.futures.as_completed(futures):
            page_number = futures[future]
            try:
                results.append(future.result())
            except RuntimeError as exc:  # pytesseract raises RuntimeError on timeout
                if "timeout" not in str(exc).lower():
                    raise
                _log.warning(
                    "OCR timed out on one page, keeping the rest of the batch",
                    extra={"stage": "ocr", "physical_page": page_number},
                )

    results.sort(key=lambda r: r.physical_page)
    return results
