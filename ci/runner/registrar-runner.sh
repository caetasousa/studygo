#!/usr/bin/env bash
# Registra um runner contra o GitLab.com com a configuração adequada à sua tag.
#
#   sudo ./registrar-runner.sh local-build  glrt-XXXX
#   sudo ./registrar-runner.sh local-deploy glrt-YYYY
#
# O token vem por argumento e não é gravado em lugar nenhum além do
# config.toml do runner (modo 600, fora de qualquer repositório).
set -euo pipefail

TAG="${1:-}"
TOKEN="${2:-}"
URL_GITLAB="${GITLAB_URL:-https://gitlab.com}"
CONFIG=/etc/gitlab-runner/config.toml

if [[ $EUID -ne 0 ]]; then
    echo "Rode com sudo: sudo $0 <tag> <token>" >&2
    exit 1
fi

if [[ -z "$TAG" || -z "$TOKEN" ]]; then
    echo "uso: sudo $0 <local-build|local-deploy> <token>" >&2
    exit 1
fi

case "$TAG" in
    local-build|local-deploy) ;;
    *) echo "ERRO: tag deve ser local-build ou local-deploy" >&2; exit 1 ;;
esac

# Registrar duas vezes criaria runners duplicados disputando os mesmos jobs.
# A tag não aparece mais no config.toml (fica no servidor), então a checagem é
# pela descrição, que é o que este script controla.
if [[ -f "$CONFIG" ]] && grep -q "wsl-$TAG" "$CONFIG" 2>/dev/null; then
    echo "Já existe um runner 'wsl-$TAG' em $CONFIG."
    echo "Para recriar: sudo gitlab-runner unregister --name \"wsl-$TAG\" e rode de novo."
    exit 0
fi

echo "==> Registrando runner '$TAG' em $URL_GITLAB"

# Com token de autenticação (glrt-), tag, "protected" e "run untagged" vêm do
# GitLab — foram definidos quando você criou o runner no site. O comando aceita
# apenas nome e configuração de executor; passar --locked ou --tag-list aqui é
# recusado com FATAL.
#
# Só o runner de build recebe privilégio, e só porque o docker-in-docker exige:
# sem ele o dind não carrega os módulos de rede e nem chega a subir.
#
# O custo é real: um job nesse runner pode montar o disco do host e obter root
# no WSL. Contido por (a) o runner de deploy continuar sem privilégio e sem
# build, (b) pipelines de fork desabilitadas no projeto — sem isso, um fork
# executaria código arbitrário nesta máquina.
if [[ "$TAG" == "local-build" ]]; then
    PRIVILEGIADO=true
else
    PRIVILEGIADO=false
fi

gitlab-runner register \
    --non-interactive \
    --url "$URL_GITLAB" \
    --token "$TOKEN" \
    --executor docker \
    --docker-image "alpine:3.21" \
    --docker-privileged="$PRIVILEGIADO" \
    --docker-volumes "/certs/client" \
    --description "wsl-$TAG"

echo "==> Ajustando concorrência"
# A máquina tem 20 threads; 4 jobs simultâneos usam bem sem sufocar o desktop.
if grep -q '^concurrent = ' "$CONFIG"; then
    sed -i 's/^concurrent = .*/concurrent = 4/' "$CONFIG"
else
    sed -i '1i concurrent = 4' "$CONFIG"
fi

chmod 600 "$CONFIG"
systemctl restart gitlab-runner

echo
echo "==> Runner '$TAG' registrado."
gitlab-runner list 2>&1 | grep -v "^Runtime platform" || true

if [[ "$TAG" == "local-deploy" ]]; then
    cat <<'FIM'

ATENÇÃO — o runner local-deploy fala com seus servidores.
No GitLab, confirme que ele está marcado como "Protected": só assim
ele fica restrito a branches e tags protegidas, e um merge request
não consegue usá-lo para alcançar produção.
FIM
fi
