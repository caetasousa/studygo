# Golden-fixture generator

`gen_golden.mjs` extracts the plan-engine reference output from the **source
artifact** (`claude.ai/code/artifact/ffbfa732…`, the single-file HTML planner
this project grew from). It is the only artifact-derived data left in the repo,
and it is **test-only** — there is no database seed; every concurso is registered
by a user.

Re-run it only when that artifact changes; otherwise the committed JSON is the
source of truth.

```bash
# engine golden test fixtures (validate the Go port of construir())
node scripts/seed/gen_golden.mjs path/to/artifact.html \
  > backend/internal/domain/plano/testdata/golden_tcego_default.json
node scripts/seed/gen_golden.mjs path/to/artifact.html concurso \
  > backend/internal/domain/plano/testdata/concurso_tcego.json

# confirm the Go engine still matches
cd backend && go test ./internal/domain/plano
```

`gen_golden.mjs` pins the default config (início 2026-09-01, prova 2027-01-17,
2 h/dia, seg–sex, revisão sexta, reta 28) so the fixture is deterministic. If the
artifact's line layout shifts, adjust the `lines.slice(...)` ranges at the top of
the script.
