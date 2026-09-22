#!/usr/bin/env bash
# Roda a suíte E2E contra um stack descartável e deixa o relatório em
# e2e/relatorio/. É o que `make e2e` chama.
#
# O stack é o docker-compose.yml da raiz SEM o override de desenvolvimento —
# as mesmas imagens que vão para produção, com o frontend servido pelo nginx —,
# num projeto compose próprio (studygo-e2e), com portas, volumes e .env
# próprios (e2e/stack.env). Ele nasce com o banco vazio e morre no fim: nada
# aqui toca no banco de quem desenvolve.
#
#   MANTER=1 make e2e   deixa o stack de pé no fim, para investigar uma falha
#   E2E_ARGS="-g C7"    repassa argumentos ao playwright (ex.: rodar um id só)

set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
cd "$raiz"

projeto=studygo-e2e
# Caminhos absolutos: o script entra em e2e/ para rodar o playwright, e o
# `down` do fim roda de lá.
compose=(docker compose --project-directory "$raiz" -f "$raiz/docker-compose.yml" -p "$projeto" --env-file "$raiz/e2e/stack.env")
backend=http://localhost:28080
frontend=http://localhost:25173

derrubar() {
	"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}

no_fim() {
	if [ "${MANTER:-0}" = 1 ]; then
		echo "stack mantido: $frontend (derrube com: ${compose[*]} down -v)"
	else
		derrubar
	fi
}
trap no_fim EXIT

echo "→ stack de E2E do zero ($projeto)"
derrubar
"${compose[@]}" up -d --build --quiet-pull

# O backend só responde depois de migrar; o nginx do frontend, depois de subir.
echo "→ esperando o backend e o frontend"
for _ in $(seq 1 90); do
	if curl -fsS "$backend/health" >/dev/null 2>&1 && curl -fsS "$frontend/" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
curl -fsS "$backend/health" >/dev/null || { echo "o backend não subiu"; "${compose[@]}" logs --tail 40 backend; exit 1; }

cd e2e
[ -d node_modules ] || npm ci --no-audit --no-fund
rm -rf relatorio

set +e
# shellcheck disable=SC2086 # E2E_ARGS é repassado palavra por palavra de propósito
E2E_BASE_URL="$frontend" npx playwright test ${E2E_ARGS:-}
resultado=$?
set -e

node resumo.mjs "$projeto" || true
echo
echo "relatório: e2e/relatorio/resumo.md  ·  e2e/relatorio/html/index.html"
exit $resultado
