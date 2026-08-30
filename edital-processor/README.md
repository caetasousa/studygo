# edital-processor

Internal service that turns a concurso **edital PDF** into a reviewable,
structured preview for annyGo. **Not public**: the Go backend is the only caller,
over the compose network, authenticated with a shared token.

The Go backend keeps everything else — the public `/api`, JWT auth, ownership,
persistence, the deterministic plan engine, spaced review, TEC import,
statistics. This service reads documents and proposes an extraction; Go persists
nothing until the user confirms.

## Status

### Phases 1–3 — complete and verified

| Area | State |
|---|---|
| PDF validation (§7.1–7.5) | ✅ MIME + real bytes, encryption, corruption, page/pixel/dimension limits |
| SHA-256 | ✅ |
| Native text extraction, per page (§7.6, §7.11–7.12) | ✅ PyMuPDF, 1-based physical pages, page labels |
| Table reconstruction (§7.7) | ✅ pdfplumber, best-effort |
| Per-page quality score (§7.8) | ✅ configurable weights, documented signals |
| Selective OCR (§7.9–7.10) | ✅ Tesseract via `image_to_data`, word boxes in page points, per-page timeout, concurrency cap; degrades to "no OCR" if the binary is absent |
| Block classification (§7.13) | ✅ per-page signals + a section sweep so Anexo II / Anexo III runs carry across pages |
| Chunk selection (§7.14–7.15) | ✅ stable ids (`p{page}#{n}`), page-anchored, only relevant blocks, delimiter wrapper marks chunks as data |
| Gemini adapter (§3, §16) | ✅ model fallback chain, retry/backoff, timeout, structured output (Gemini schema dialect), typed errors; ported from the Go adapter |
| LLM extraction steps | ✅ `analisar` (banca + cargos), `estrutura`, `conteudo`; maps raw JSON → §8 schema; **never invents question counts** |
| Evidence (§11) | ✅ JSON Pointer field, physical page, snippet, method, confidence set only when the snippet is found on the page |
| Confidence (§11) | ✅ computed from source quality + snippet validity + rule agreement + arithmetic + page conflict — never model-reported |
| Cross-validation (§12) | ✅ duplicate cargo codes, missing date/disciplines, questions-not-broken-down (blocker), sum mismatch, unmappable group (blocker), weight scope, invalid evidence page, ISO dates |
| Temporary artifacts (§5) | ✅ UUID, `ownerRef` binding, TTL, atomic write, no client paths, sweep |
| Typed errors (§6) | ✅ full set, stable codes, `transient` flag, request-id |
| Internal contract | ✅ `POST /internal/editais/{analisar,estrutura,conteudo}`; `analisar` takes a file or pasted text; `documentId` out; camelCase wire |
| Docker image | ✅ `python:3.13-slim-trixie`, Tesseract + `por`, non-root, healthcheck |
| Tests | ✅ 101 hermetic (ruff + ruff-format + mypy strict all green); 6 real-PDF/OCR integration (pass in the container); 3 real-Gemini (pass against the real edital) |

**Real edital verified end to end.** The TCE-GO Edital 01/2026 is a **26-page
fully scanned PDF with no text layer**. The pipeline OCR'd it at 92–94%
confidence, classified Anexo II (pages 19–22) as syllabus, and Gemini returned:
banca = Fundação Carlos Chagas; cargos A01 + B02; B02 = 25 questões peso 1 / 45
questões peso 2, Estudo de Caso, 270 min; "Engenharia de Software" as
B02-specific content with page evidence.

### Phases 4–6 — done

- **4 — Go HTTP client**: `port.EditalProcessor` (documentId-based, 3 methods);
  `adapter/editalproc` HTTP client + null adapter; `EDITAL_PROCESSOR_URL` /
  `EDITAL_PROCESSOR_TOKEN`; internal auth (`Authorization` + `X-Owner-Ref`) and
  `X-Request-Id` propagation; error-code mapping (transient / 5xx / 429 →
  `ErrProvedorIndisponivel` → 503, others → 4xx). Wizard + edit-screen
  "extract topics" keep their public paths; `arquivoUri`/full-text replaced by
  `documentoId`. The processor's `/analisar` gained a pasted-text path.
- **5 — Go domain + frontend**: `distribuirBloco` (the invented equal split)
  removed — `questoes: null` + a `questions_not_broken_down` **blocker** alert;
  `peso` added to `DisciplinaInput` and threaded into `Disciplina.Peso` (0 = the
  block default; a positive value overrides — golden test unchanged, no
  migration). Frontend: `documentoId` through the wizard, cargo-code selection,
  per-discipline questions field (null until filled), "ratear como estimativa"
  button, sum-vs-group check, `advance` gated until every discipline has a count,
  alert list shown per step, `peso` field in `ConcursoForm`.
- **6 — Cutover**: `internal/adapter/ai` and `internal/platform/pdftext` deleted;
  `dslipak/pdf` dropped from `go.mod`; `edital-processor` added to
  `docker-compose.yml` (internal network, no host port, healthcheck, named
  volume) and to `ansible/deploy.yml` + `templates/docker-compose.prod.yml.j2`
  (build/save/copy/load the third image); `GEMINI_API_KEY` moved to the Python
  container; `.env.example`, `CLAUDE.md` and Ansible secrets updated.

**Verified end to end through the compose stack** (all 5 services up,
`edital-processor` healthy): wizard step 1 (pasted text) → banca "Fundação
Carlos Chagas", cargo B02, 10 vagas, `documentoId`; step 2 → groups with totals
25/45 and weights 1/2, every discipline `questoes: null`, discursiva
estudo_de_caso, 270 min, alerts `questions_not_broken_down` (blocker) +
`weight_scope_group_only` (info) + `missing_exam_date`; step 3 returns per
discipline. The full 26-page scanned PDF path works too (OCR ~40s), subject to
Gemini's current 503 "high demand" — the processor walks its model fallback
chain, and the backend maps a hard failure to a 503 with a retry hint.

## Run the tests

```bash
uv venv --python 3.13 .venv
uv pip install --python .venv/bin/python -e .
uv pip install --python .venv/bin/python pytest pytest-asyncio httpx ruff mypy

.venv/bin/ruff check app tests
.venv/bin/ruff format --check app tests
.venv/bin/mypy app
.venv/bin/python -m pytest tests -m "not gemini and not realpdf"   # 101 hermetic
```

OCR and real-Gemini tests need the container (Tesseract) and a key:

```bash
docker build -t edital-processor:dev .
docker run --rm -v "$PWD/tests:/app/tests:ro" -v "$PWD/pyproject.toml:/app/pyproject.toml:ro" \
  -e GEMINI_API_KEY=... --user root edital-processor:dev sh -c \
  'pip install -q pytest pytest-asyncio httpx && cd /app && python -m pytest tests -m "integration or gemini"'
```

`tests/fixtures/edital_real.pdf` (gitignored) enables the real-PDF tests; they
assert facts about the document, never values baked into the code.

## Configuration

Environment, prefix `EP_`. See `.env.example`. Key ones:

- `EP_SERVICE_TOKEN` — shared secret from the Go backend. Required when deployed.
- `EP_GEMINI_API_KEY` — moves here from the Go container in Phase 6.
- `EP_WORK_DIR`, `EP_ARTIFACT_TTL_SECONDS` — temporary document store.
- `EP_MIN_TEXT_SCORE`, `EP_MIN_TEXT_CHARS` — the OCR trigger.
- `EP_OCR_DPI`, `EP_OCR_TIMEOUT_SECONDS`, `EP_OCR_MAX_CONCURRENCY` — OCR limits.

## Future: async jobs

The wizard is synchronous today (one HTTP call per step, ~40 s for a 26-page
scan). A later phase can move `analisar` to a job: `POST` returns `202` + a job
id, the client polls, OCR runs on a worker. That needs a queue (a Postgres table
is enough — no Redis/Celery) and a worker process; it is out of scope for the
migration.
