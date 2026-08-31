# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`studygo` is a study-plan app for Brazilian public-service exams (`concursos`), grown from the artifact at `claude.ai/code/artifact/ffbfa732…` (a single-file HTML planner for TCE-GO B02). It keeps that artifact's visual design and its plan-generation logic, now split into:

- **Backend** — Go (module `studygo`, package root `backend/`), hexagonal, Postgres via a hand-written SQL migration runner, argon2id + JWT auth. The plan engine (`internal/domain/plano`) is a faithful port of the artifact's `construir()`, locked in by a golden test against the artifact's own output.
- **Frontend** — SvelteKit SPA in `frontend/` (Svelte 5 runes, `adapter-static`, `ssr = false`), design tokens/CSS ported verbatim from the artifact. Talks to the API; no SSR.

The data model is multi-concurso generic (`concursos → disciplinas → temas / fontes → marcos → conteudo_programatico → rev_ciclo`). **What the student did is recorded per ACTIVITY, not per day**: `registros_bloco.atividade_id` points at one scheduled activity, so a day that schedules the same discipline twice keeps the two apart, and a day is finished only when every activity scheduled for it is (the day-level `concluido` is derived, never asserted by the client). Rows written before this carry a null `atividade_id` and keep the old `(data, disciplina)` key — they are adopted only where the discipline occurs once that day, since with two occurrences there is no honest way to say which activity owns them. **There is no seed** — every `concurso` is registered by a user (`concursos.owner_user_id`), either by hand or by importing the edital PDF. Edital import runs in a **separate internal service, `edital-processor/`** (Python/FastAPI): PDF validation, selective OCR (Tesseract), block classification, chunking, and the Gemini calls all live there; the Go backend reaches it through `port.EditalProcessor` (HTTP adapter `adapter/editalproc`, null adapter `editalproc.Indisponivel` when `EDITAL_PROCESSOR_URL` is unset). The processor returns a **reviewable preview** (`documentoId` handle, groups/disciplines/cronograma, evidence, alerts) — the Go backend persists nothing until the user confirms in the wizard. The engine tolerates a "thin" concurso — a discipline with no `temas` headlines the discipline name; an empty `rev_ciclo` falls back to `plano.RevCicloPadrao`. `scripts/seed/gen_golden.mjs` still extracts the golden-test fixtures from the artifact (`internal/domain/plano/testdata/*.json`) — that is the only remaining artifact-derived data, and it is test-only.

## Commands

Backend, from `backend/`:

```bash
go build ./...                 # build
go vet ./...
go test ./...                   # engine golden + property tests live in internal/domain/plano
go test -run TestGerar ./internal/domain/plano
```

Frontend, from `frontend/`:

```bash
npm run dev                     # vite dev server on :5173, proxies /api to :8080
npm run build                   # static build to frontend/build
npm run check                   # svelte-check (type check)
npm test                        # vitest — pure logic (estudo.ts, siglas, movimentação)
```

Full stack from the repo root (primary way to run everything):

```bash
docker compose up -d --build    # postgres + backend + worker + frontend
docker compose down
docker compose logs -f backend
```

**Hot reload is on by default.** `docker-compose.override.yml` is loaded
automatically by Compose, and points `frontend` and `backend` at their
Dockerfiles' `dev` stages: the frontend runs the vite dev server, the backend
runs `go run` under an `inotifywait` loop (`backend/dev-run.sh`), both with the
source bind-mounted. A save in the editor reaches the browser (or restarts the
API in ~5 s) with no rebuild — `--build` is only needed when a dependency
changes. `VITE_USE_POLLING=1` is set there because host filesystem events do
not cross a bind mount reliably on WSL2/macOS. To run the production images
locally instead, bypass the override with
`docker compose -f docker-compose.yml up -d --build`; `ansible/deploy.yml`
never sees it, and `docker build` with no `--target` still yields the
production stage (nginx / distroless).

Config is env vars (see `.env.example` at root). **`JWT_SECRET` is required**; `DATABASE_URL` is built by Compose from `POSTGRES_*`. The backend runs migrations on boot (advisory-locked, so the `worker` can run them too without racing). Edital import needs `GEMINI_API_KEY` (read by `edital-processor`, not the backend) and `EDITAL_PROCESSOR_TOKEN` (a shared secret on the compose network); with them unset, manual registration still works.

## Architecture

Single hexagonal (ports & adapters) layout under `backend/internal/` — not split by bounded context (deliberately one hexagon):

- `internal/domain/` — pure business logic, no infrastructure imports:
  - `user/` — account entity + registration validation
  - `concurso/` — exam catalogue types (Concurso, Disciplina, Fonte, Marco, ConteudoItem, RevItem) + `Concurso.Validar()` registration invariants, and `Sigla`/`Siglas` (`sigla.go`), which derive a discipline's `codigo` from its name (DIRAD, not D04) at registration time
  - `plano/` — **the engine**: `Gerar(cfg, concurso)` ports `construir()` (`motor.go`), plus `Blocos`, `CalcularStats`, `AplicarReordenacoes`, `Trocar`, `RevCicloPadrao`. `motor_test.go` + `testdata/*.json` is the golden test vs the artifact; `TestGerar_ConcursoMinimo` covers a thin user-registered concurso.
    `atividade.go` is the **movable unit**: `MoverAtividade` (insert) and `TrocarAtividades` (swap with the slot's occupant) rearrange one subject without dragging its day along, and `AplicarAtividades` reconciles the stored layout with a freshly generated plan — each stored activity claims the generated slot it came from (`OrigemDia`/`OrigemPos`), so raising `blocosPorDia` surfaces the extra block instead of being hidden by the old layout. An item the store has never seen carries a deterministic `gen:<ISO>:<pos>` id (`IDDerivado`), which lets the UI address it before anything is persisted without turning GET into a write; `AtividadesFaltantes` materialises exactly those on the next move.
- `internal/port/` — interfaces services depend on: `UserRepository`, `ConcursoRepository`, `PlanoRepository`, `PasswordHasher`, `TokenIssuer`, `Notifier`, `EditalProcessor`, `Clock`, `HealthChecker`, `Pinger`.
- `internal/service/` — use cases: `AuthService`, `ConcursoService` (concurso CRUD + `ImportarEdital`; wire types in `concurso_view.go`), `PlanoService` (orchestrates engine + persisted state → fat `PlanoResposta` view model in `plano_view.go`; also `Estatisticas`, `Caderno`, `Dossie`, `ExportarCSV` in `plano_extras.go`), `NotificacaoService` (spaced-review reminders), `HealthService`.
- `internal/adapter/` — `httpapi/` (handlers + `router.go` + auth middleware), `postgres/` (pgx repos, hand-written SQL), `crypto/` (argon2id, JWT), `editalproc/` (HTTP client for the Python edital-processor + `Indisponivel` null adapter), `notifier/` (slog stub; email adapter is a TODO behind the same port). The old `ai/` (in-process Gemini) and `platform/pdftext` (in-process PDF text) were removed when the pipeline moved to `edital-processor/`.
- `internal/platform/` — `config`, `db` (pgxpool + migration runner), `httpserver` (functional-options `net/http` wrapper), `middleware` (request-id, recover, logging, CORS).
- `cmd/server/main.go` — composition root; `cmd/worker/main.go` — reminder loop (ticker; `LEMBRETE_INTERVALO`). No `init()`.

**`edital-processor/`** (Python/FastAPI, its own `pyproject.toml` + `Dockerfile`, `python:3.13-slim-trixie` + Tesseract) — internal-only, no host port. `app/services/` is the pipeline (validate → hash → native text → quality score → selective OCR → classify → chunk → Gemini → consolidate → validate); `app/providers/gemini.py` is the only LLM adapter (model fallback chain ported from the old Go adapter); `app/schemas/` is the extraction contract (`null` for absence, groups ≠ disciplines, evidence with JSON Pointer + page + confidence). Tests: `ruff` + `mypy --strict` + `pytest` (hermetic), plus `-m integration` (real Tesseract, container only) and `-m gemini` (real key). It never writes to Postgres — the Go backend owns persistence.

Routing uses the stdlib 1.22+ `http.ServeMux` method+wildcard syntax (`mux.HandleFunc("PATCH /api/concursos/{slug}/plano/registros/{data}", ...)`) — no router dependency.

`migrations/` holds hand-written `NNNNNN_name.up.sql`/`.down.sql` applied by `internal/platform/db.Migrate` (embedded via `migrations/embed.go`, tracked in `schema_migrations`, one tx per file, `pg_advisory_lock` so multiple processes are safe). No ORM.

## Infrastructure (Ansible)

`ansible/` provisions the production VPS (Ubuntu, Hostinger). Structure:

- `bootstrap.yml` / `lockdown.yml` — one-shot, already run against the current VPS: create the `annyGo` sudo user with key-based SSH, then permanently disable root SSH login and password auth. Do not re-run unless standing up a new server from scratch.

**The project was renamed annyGo → studygo, and some names deliberately did not follow.** They are live state on a running server, not text: the Linux/SSH user `annyGo`, the deploy key `~/.ssh/annygo_deploy`, the app directory `/opt/annygo`, the nginx vhost `annygo.conf`, and the production Postgres user/database `annygo`. Renaming any of them from the playbooks alone would deploy beside the running system rather than to it — or, for the SSH user, lock every playbook out of a box where root SSH is permanently disabled. Each carries a comment saying so at the point it appears. Moving them is a maintenance-window job (stop the stack, move the directory, `ALTER DATABASE`/`ALTER ROLE`, re-point the vhost), never a find-and-replace.
- `site.yml` + `roles/common`, `roles/docker`, `roles/nginx`, `roles/certbot` — the ongoing box-provisioning playbook (apt updates, ufw firewall on 22/80/443, Docker Engine, nginx vhost + reverse proxy to the frontend container, Let's Encrypt cert). Run as `annyGo` with `become: true`. Fully idempotent (`changed=0` on a clean re-run) — the `nginx` role itself checks whether the cert already exists before rendering the vhost, so it doesn't fight with the `certbot` role over the file.
- `deploy.yml` — ships the app: builds the **backend, frontend and edital-processor** images **locally** (`delegate_to: localhost`), `docker save`s each to a tarball, copies them to the VPS, `docker load`s them, then runs the full stack (postgres + backend + worker + frontend + edital-processor) via a generated `docker-compose.yml` in `/opt/annygo`. Source is never copied to the VPS — only built images. Run whenever backend, frontend or the processor changes; `site.yml` is only box-level config.
- `templates/app.conf.j2` is the single vhost template, shared by the `nginx` and `certbot` roles — it conditionally renders the HTTP-only block or the HTTP→HTTPS + SSL block on `letsencrypt_cert_exists`. Edge nginx proxies **everything to the frontend container** (`frontend_port`); the frontend's own nginx serves the SPA and proxies `/api` to `backend:8080` on the compose network. `templates/docker-compose.prod.yml.j2` is `deploy.yml`'s rendered stack.
- `inventory/hosts.ini` and `inventory/group_vars/vps/secrets.yml` (`letsencrypt_email`, `jwt_secret`, `postgres_password`) are **gitignored** — copy the `.example` files and fill them in before running any playbook. Non-secret vars (`app_domain`, `backend_port`, `frontend_port`, `postgres_user`, `postgres_db`) live in the tracked `main.yml`.
- The deploy private key (`~/.ssh/annygo_deploy`) lives outside the repo, never inside it. If it's ever regenerated locally (overwriting the file instead of adding a new one), the VPS's `authorized_keys` still has the *old* public key and access is lost — since root SSH is permanently disabled, the only recovery path is the hosting provider's browser-based console (not SSH) to manually fix `/home/annyGo/.ssh/authorized_keys`. Don't regenerate this key casually.
- Known Ansible gotcha hit here: inventory `ansible_user` outranks both the play's `remote_user:` keyword and CLI `-u`. To force a different connection user for a one-off play (as `bootstrap.yml`/`lockdown.yml` do for `root`), set it via `vars: ansible_user: root` in the play, not `remote_user:`.

## Repository is public — keep it clean

This repo is public on GitHub. Beyond the standard secrets rule, avoid committing: real hostnames/IPs (`ansible/inventory/hosts.ini` is gitignored for this reason — use the `.example` file), `.env` files with real values (only `.env.example` files are tracked), Ansible `*.retry`/`.vault_pass` artifacts, and any other file whose only purpose is local/personal rather than something a stranger cloning the repo needs.

## Dependency versions

For anything running in Docker (Go toolchain, Postgres, base images), pin to the **latest stable major version** at the time it's introduced or bumped — check the project's release page or [endoflife.date](https://endoflife.date) rather than assuming, since this changes over time (e.g. Go 1.26→1.27, Postgres 17→18 were both bumped mid-project after checking). Skip pre-release/beta majors (e.g. don't jump to a Postgres major still in beta). Pin an explicit version tag, never `:latest`, so builds stay reproducible.

For the VPS's native (non-Docker) packages — currently only `nginx`, installed via `apt` in `roles/nginx` — stay on the Ubuntu-packaged version rather than chasing upstream mainline. Ubuntu backports security fixes onto its shipped version, so the version number looks old but is patched; this was a deliberate choice over containerizing nginx, to keep the reverse-proxy/certbot integration simple.

The Ubuntu release itself (currently 24.04 LTS) is an OS upgrade on a live VPS — don't bump it without the user's explicit go-ahead, even when a newer LTS exists.

Go module deps are kept minimal (`pgx`, `golang-jwt`, `golang.org/x/crypto`, `google/uuid`) — prefer stdlib / a little recode over a new dependency. The frontend pins SvelteKit 2 / Svelte 5 / Vite 6 in `frontend/package.json`; `node:24-alpine` and `nginx:1.27-alpine` in `frontend/Dockerfile`. `edital-processor/` pins its Python deps in `pyproject.toml` (exact `==`) and `python:3.13-slim-trixie` in its `Dockerfile`.

## Frontend conventions (project skill)

`.agents/skills/svelte5-best-practices` governs `frontend/**/*.svelte`. Key points already applied: runes only (`$state`/`$derived`/`$effect`/`$props`), `onclick` not `on:click`, callback props not `createEventDispatcher`, no module-level mutable state. Stores are rune classes in `*.svelte.ts` (`auth`, `planoStore`) with write-through `localStorage` caching so the UI stays instant; keys go through `src/lib/storageKey.ts`, which adopts what the old `annygo.*` keys still hold so the rename signs nobody out. Logic worth testing lives in plain `.ts` (`estudo.ts`, `arrastarToque.ts`) rather than inside components, because there is no component-test setup — vitest runs over the modules, no DOM. The API client (`src/lib/api.ts`) refreshes the JWT once on a 401 and retries. Wire types in `src/lib/types.ts` mirror the Go view models in `internal/service/plano_view.go` — keep them in sync by hand.

## Git workflow

- Commits use the repository's already-configured git identity (`caetasousa`) — never change `user.name`/`user.email`, local or global.
- **Claude MUST ASK before every commit — no exceptions.** Not after finishing a feature, not after the checks pass, not for a small fix. Describe what would go in the commit and wait for an explicit go-ahead. This replaces an earlier rule that allowed committing autonomously: it produced a run of commits over work that was still wrong, and a wrong commit is harder to undo than an unmade one.
- Claude MUST NEVER run `git push` (or any variant — `--force`, pushing tags, etc.), under any circumstance. Pushing to the remote is done exclusively by caetasousa.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): short description`, imperative mood, lowercase, no trailing period.
  - Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `style`, `perf`.
  - Scope is the area touched: `backend`, `ansible`, `docker`, `nginx`, `claude` (for this file), etc. Omit the scope only when a change genuinely spans everything (e.g. a repo-wide rename).
  - Body (optional, blank line after the subject): explain *why*, not what — the diff already shows what changed.
  - Examples: `feat(backend): add GET /health endpoint`, `chore(ansible): add certbot role for Let's Encrypt`, `fix(nginx): stop vhost from flapping between http/https on reruns`.

## Go conventions (project skills)

`.agents/skills/golang-*` define the style this codebase follows for any `.go` file; consult them for specifics rather than re-deriving conventions:

- `golang-code-style` — line breaking, `:=` vs `var`, no-nil slices/maps, early-return error handling, ≤4 function params
- `golang-error-handling` — wrap with `fmt.Errorf("...: %w", err)`, lowercase no-punctuation error strings, `errors.Is`/`errors.As`, single handling rule (log OR return, never both), `slog` for structured logging
- `golang-design-patterns` — functional options for constructors, no `init()`/mutable globals, timeouts on external calls, `defer Close()` immediately after opening
- `golang-testing` — table-driven tests with named subtests, test file named after the source file (not the function), `t.Parallel()` where independent

When architecture decisions come up (new bounded contexts, layering), ask before imposing a pattern — this repo already made that call once (single hexagon, no DDD contexts) specifically to stay simple at this stage.
