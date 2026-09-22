#!/usr/bin/env bash
# Roda a suíte E2E contra um stack descartável e deixa o relatório em
# e2e/relatorio/. É o que `make e2e` chama, e o que o job `e2e` da pipeline roda.
#
# O stack é o docker-compose.yml da raiz SEM o override de desenvolvimento —
# as mesmas imagens que vão para produção, com o frontend servido pelo nginx —,
# mais e2e/docker-compose.e2e.yml, que troca o edital-processor pelo dublê. Ele
# roda num projeto compose próprio (studygo-e2e), com portas, volumes e .env
# próprios (e2e/stack.env), nasce com o banco vazio e morre no fim: nada aqui
# toca no banco de quem desenvolve.
#
#   MANTER=1               deixa o stack de pé no fim, para investigar uma falha
#   E2E_ARGS="-g C7"       repassa argumentos ao playwright (ex.: um id só)
#   E2E_EM_CONTAINER=1     roda o playwright na imagem oficial, dentro da rede
#                          do stack — é como a pipeline roda; localmente,
#                          reproduz a pipeline sem depender do node da máquina
#   E2E_IMAGENS_PRONTAS=1  não reconstrói backend, worker e frontend: usa as
#                          imagens studygo-e2e-* que já existirem (a pipeline
#                          as reetiqueta a partir do build)
#   E2E_HOST=docker        onde as portas publicadas respondem (na pipeline,
#                          o daemon é o serviço "docker", não a própria máquina)

set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
cd "$raiz"

projeto=studygo-e2e
# Caminhos absolutos: o script entra em e2e/ para rodar o playwright, e o
# `down` do fim roda de lá.
compose=(docker compose --project-directory "$raiz" -p "$projeto" --env-file "$raiz/e2e/stack.env"
	-f "$raiz/docker-compose.yml" -f "$raiz/e2e/docker-compose.e2e.yml")
host=${E2E_HOST:-localhost}
backend="http://$host:28080"
frontend="http://$host:25173"
# A imagem do playwright acompanha a versão fixada em e2e/package.json: é ela
# que traz o navegador que essa versão espera.
versao_pw="$(sed -n 's/.*"@playwright\/test": "\([0-9.]*\)".*/\1/p' e2e/package.json)"
imagem_pw="mcr.microsoft.com/playwright:v${versao_pw}-noble"
runner="$projeto-playwright"

derrubar() {
	docker rm -f "$runner" >/dev/null 2>&1 || true
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
if [ "${E2E_IMAGENS_PRONTAS:-0}" = 1 ]; then
	# Só o dublê é construído aqui; o resto são as imagens do build.
	"${compose[@]}" build edital-processor
	"${compose[@]}" up -d --quiet-pull
else
	"${compose[@]}" up -d --build --quiet-pull
fi

# O backend só responde depois de migrar; o nginx do frontend, depois de subir.
echo "→ esperando o backend e o frontend em $host"
for _ in $(seq 1 90); do
	if curl -fsS "$backend/health" >/dev/null 2>&1 && curl -fsS "$frontend/" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
curl -fsS "$backend/health" >/dev/null || { echo "o backend não subiu"; "${compose[@]}" logs --tail 40 backend; exit 1; }

# O que o resumo precisa saber e só daqui se vê: o commit e as imagens exatas.
# Dentro do container do playwright não há git nem docker.
commit="${CI_COMMIT_SHORT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo '?')}"
if [ -z "${CI:-}" ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
	commit="$commit (com alterações não commitadas)"
fi
imagens="$(docker ps --filter "label=com.docker.compose.project=$projeto" \
	--format '{{.Label "com.docker.compose.service"}} {{.Image}}' | while read -r servico img; do
	echo "$servico $(docker image inspect --format '{{.Id}}' "$img" | cut -c8-19)"
done | sort)"
export E2E_COMMIT="$commit" E2E_IMAGENS="$imagens"

rm -rf e2e/relatorio
set +e
if [ "${E2E_EM_CONTAINER:-0}" = 1 ]; then
	# Copia a suíte para dentro em vez de montar a pasta: no Docker-in-Docker da
	# pipeline, um -v apontaria para o disco do daemon, onde a pasta não existe.
	docker create --name "$runner" --network "${projeto}_default" --ipc=host --workdir /e2e \
		-e E2E_BASE_URL=http://frontend:5173 -e E2E_COMMIT -e E2E_IMAGENS -e CI \
		"$imagem_pw" bash -c "npm ci --no-audit --no-fund --silent && npx playwright test ${E2E_ARGS:-}; r=\$?; node resumo.mjs; exit \$r" >/dev/null
	tar -C e2e --exclude node_modules --exclude relatorio -cf - . | docker cp - "$runner:/e2e"
	docker start -a "$runner"
	resultado=$?
	docker cp "$runner:/e2e/relatorio" e2e/relatorio
else
	cd e2e
	[ -d node_modules ] || npm ci --no-audit --no-fund
	# shellcheck disable=SC2086 # E2E_ARGS é repassado palavra por palavra de propósito
	E2E_BASE_URL="$frontend" npx playwright test ${E2E_ARGS:-}
	resultado=$?
	node resumo.mjs
	cd "$raiz"
fi
set -e

echo
echo "relatório: e2e/relatorio/resumo.md  ·  e2e/relatorio/html/index.html"
exit $resultado
