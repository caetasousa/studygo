#!/usr/bin/env bash
# Instala o GitLab Runner nesta máquina como serviço systemd.
#
# Só o RUNNER é instalado aqui. GitLab, Registry e projetos ficam no GitLab.com.
#
# Idempotente: rodar de novo não duplica repositório, pacote nem serviço.
#
#   sudo ./instalar-runner.sh
set -euo pipefail

VERSAO_MINIMA="17.0.0"
USUARIO_RUNNER="gitlab-runner"

if [[ $EUID -ne 0 ]]; then
    echo "Rode com sudo: sudo $0" >&2
    exit 1
fi

echo "==> 1/5 Verificando pré-requisitos"
if ! command -v docker >/dev/null; then
    echo "ERRO: Docker não encontrado. O executor docker precisa dele." >&2
    exit 1
fi
echo "    docker: $(docker --version)"

if ! systemctl is-system-running --quiet 2>/dev/null; then
    estado=$(systemctl is-system-running 2>/dev/null || true)
    if [[ "$estado" != "degraded" ]]; then
        echo "ERRO: systemd não está ativo (estado: $estado)." >&2
        echo "      No WSL, exige [boot] systemd=true em /etc/wsl.conf." >&2
        exit 1
    fi
fi
echo "    systemd: ativo"

echo "==> 2/5 Repositório oficial do GitLab Runner"
if [[ -f /etc/apt/sources.list.d/runner_gitlab-runner.list ]]; then
    echo "    já configurado, pulando"
else
    curl -fsSL "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" \
        -o /tmp/gitlab-runner-repo.sh
    bash /tmp/gitlab-runner-repo.sh
    rm -f /tmp/gitlab-runner-repo.sh
fi

echo "==> 3/5 Instalando o pacote"
if command -v gitlab-runner >/dev/null; then
    echo "    já instalado: $(gitlab-runner --version | head -1)"
else
    apt-get install -y gitlab-runner
fi

echo "==> 4/5 Serviço systemd"
# O pacote instala e habilita o serviço sozinho; aqui só confirmamos.
systemctl enable gitlab-runner >/dev/null 2>&1 || true
systemctl start gitlab-runner  >/dev/null 2>&1 || true
systemctl is-enabled gitlab-runner >/dev/null && echo "    habilitado no boot: sim"
systemctl is-active  gitlab-runner >/dev/null && echo "    em execução: sim"

echo "==> 5/5 Acesso ao Docker"
# O executor docker fala com o daemon local para criar os containers de job.
if id -nG "$USUARIO_RUNNER" | grep -qw docker; then
    echo "    $USUARIO_RUNNER já está no grupo docker"
else
    usermod -aG docker "$USUARIO_RUNNER"
    echo "    $USUARIO_RUNNER adicionado ao grupo docker"
    systemctl restart gitlab-runner
fi

cat <<'FIM'

==> Runner instalado.

Próximo passo: registrar contra o GitLab.com. No GitLab, vá em
  Grupo → Settings → CI/CD → Runners → New group runner

Crie DOIS runners, com estas tags:

  1) local-build    (para testes e build de imagens)
  2) local-deploy   (marque "Protected"; só ele fala com os servidores)

Depois rode, para cada um, com o token que o GitLab mostrar:

  sudo ./registrar-runner.sh local-build  glrt-XXXXXXXX
  sudo ./registrar-runner.sh local-deploy glrt-YYYYYYYY

O token aparece uma única vez na tela do GitLab. Ele NÃO deve ser
commitado, colado em issue nem salvo em arquivo do projeto.
FIM
