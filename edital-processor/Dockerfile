# syntax=docker/dockerfile:1

# --- builder ----------------------------------------------------------------
# O uv instala as dependências e não serve para mais nada em runtime. Mantê-lo
# no estágio final custaria 55 MB de binário em toda imagem publicada.
FROM python:3.13-slim-trixie AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

COPY --from=ghcr.io/astral-sh/uv:0.12.7 /uv /uvx /bin/

WORKDIR /app
COPY pyproject.toml ./
RUN uv pip install --system --no-cache .

# --- runtime ----------------------------------------------------------------
FROM python:3.13-slim-trixie AS runtime

# Tesseract com português. libgl1 saiu: é dependência do opencv-python, que
# este serviço não usa — PyMuPDF, pdfplumber e Pillow renderizam por conta.
#
# O Claude Code (pacote do npm, versão fixada) faz os mapas mentais das aulas
# (app/mapas). O npm só serve à instalação e sai em seguida; fica o Node.
RUN apt-get update && apt-get install -y --no-install-recommends \
        tesseract-ocr \
        tesseract-ocr-por \
        nodejs npm ca-certificates \
    && npm install -g @anthropic-ai/claude-code@2.1.283 \
    && npm cache clean --force \
    && apt-get purge -y npm && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

# Só o que o uv instalou atravessa a fronteira: os pacotes e seus entrypoints.
COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

WORKDIR /app
COPY app ./app

RUN useradd --system --uid 10001 --create-home --home /home/appuser appuser \
    && mkdir -p /var/lib/edital-processor/work /var/lib/edital-processor/claude \
    && chown -R appuser:appuser /var/lib/edital-processor
USER appuser

# A conexão do Claude de cada conta (feita pela tela, uma pasta por conta) fica
# num volume próprio, para sobreviver ao deploy. Sem atualização automática:
# a versão é a da imagem, que passou pela esteira.
ENV EP_WORK_DIR=/var/lib/edital-processor/work \
    CLAUDE_CONFIG_DIR=/var/lib/edital-processor/claude \
    DISABLE_AUTOUPDATER=1 \
    DISABLE_TELEMETRY=1

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8000/healthz').status==200 else 1)"

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
