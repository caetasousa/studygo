# syntax=docker/dockerfile:1

# --- base -------------------------------------------------------------------
FROM python:3.13-slim-trixie AS base

# Tesseract with Portuguese, plus the runtime libs PyMuPDF/pdfplumber/Pillow need.
RUN apt-get update && apt-get install -y --no-install-recommends \
        tesseract-ocr \
        tesseract-ocr-por \
        libgl1 \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

# uv for reproducible installs from the pinned pyproject.
COPY --from=ghcr.io/astral-sh/uv:0.12.7 /uv /uvx /bin/

WORKDIR /app

# --- deps (cached) --------------------------------------------------------
COPY pyproject.toml ./
RUN uv pip install --system --no-cache .

# --- app -----------------------------------------------------------------
COPY app ./app

# non-root
RUN useradd --system --uid 10001 --home /home/appuser appuser \
    && mkdir -p /var/lib/edital-processor/work \
    && chown -R appuser:appuser /var/lib/edital-processor
USER appuser

ENV EP_WORK_DIR=/var/lib/edital-processor/work

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8000/healthz').status==200 else 1)"

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
