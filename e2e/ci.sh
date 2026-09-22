#!/usr/bin/env bash
# O job `e2e` da pipeline (ver .gitlab-ci.yml): a suíte do `make e2e` contra
# as imagens que o build ACABOU de fazer, e não contra um build novo — o que
# se testa é exatamente o que o publish vai empurrar.
#
# Espera LOCAL_REGISTRY (o registry do host, preparado no before_script) e
# IMAGE_TAG (o SHA do commit, que nomeia as imagens do build).

set -euo pipefail
: "${LOCAL_REGISTRY:?defina LOCAL_REGISTRY}" "${IMAGE_TAG:?defina IMAGE_TAG}"
cd "$(dirname "$0")/.."

# As imagens do build com os nomes que o projeto compose studygo-e2e usaria:
# achando-as, o rodar.sh (com E2E_IMAGENS_PRONTAS=1) não reconstrói nada.
for c in backend frontend; do
	docker pull -q "$LOCAL_REGISTRY/$c:$IMAGE_TAG"
done
docker tag "$LOCAL_REGISTRY/backend:$IMAGE_TAG" studygo-e2e-backend
docker tag "$LOCAL_REGISTRY/backend:$IMAGE_TAG" studygo-e2e-worker
docker tag "$LOCAL_REGISTRY/frontend:$IMAGE_TAG" studygo-e2e-frontend

# A imagem do playwright tem 3,5 GB e o docker do job nasce vazio. Ela fica
# guardada no registry do host: só a primeira pipeline de cada versão a baixa
# da internet.
versao="$(sed -n 's/.*"@playwright\/test": "\([0-9.]*\)".*/\1/p' e2e/package.json)"
oficial="mcr.microsoft.com/playwright:v${versao}-noble"
guardada="$LOCAL_REGISTRY/cache/playwright:v${versao}-noble"
if docker pull -q "$guardada" 2>/dev/null; then
	docker tag "$guardada" "$oficial"
else
	docker pull -q "$oficial"
	docker tag "$oficial" "$guardada"
	docker push -q "$guardada" || echo "aviso: não guardei o playwright no registry do host; a próxima pipeline baixa de novo"
fi

# O daemon é o serviço "docker" do job: é lá que as portas do stack respondem.
E2E_IMAGENS_PRONTAS=1 E2E_EM_CONTAINER=1 E2E_HOST=docker exec ./e2e/rodar.sh
