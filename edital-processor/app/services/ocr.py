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
from app.core.errors import OCRTimeout, OCRUnavailable
from app.services.normalize import normalize_text


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


def run_ocr(data: bytes, page_numbers: list[int], settings: Settings) -> list[OCRPage]:
    """OCR the given physical pages. Raises OCRUnavailable if Tesseract is
    missing; per-page timeouts raise OCRTimeout for that page only."""
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
                if "timeout" in str(exc).lower():
                    raise OCRTimeout(f"OCR timed out on page {page_number}") from exc
                raise

    results.sort(key=lambda r: r.physical_page)
    return results
