#!/usr/bin/env bash
# Prepara a distro do WSL que faz de servidor para o Ansible entrar nela.
#
# É o bootstrap.yml deste tipo de servidor. Não é um playbook porque o
# Ansible entra por SSH — e é justamente o SSH que este script cria. Na VPS o
# provedor entregava um root por SSH; aqui o equivalente é `wsl.exe -u root`.
# Todo o resto (Docker rootless, nginx, túnel) é do site.yml.
#
# Idempotente: rodar de novo num servidor pronto não muda nada, e o script
# diz o que mudou. Uso, a partir da distro de desenvolvimento:
#
#   wsl.exe -d ubuntu-server -u root -- bash -s -- \
#     <usuario> "$(cat ~/.ssh/studygo_ci.pub)" [usuario-antigo ...] \
#     < ansible/bootstrap-wsl.sh
#
#   <usuario>         o usuário de deploy do projeto (um por projeto)
#   chave pública     a que a esteira usa (CI_SSH_PRIVATE_KEY)
#   usuario-antigo    usuários a remover, com o Docker rootless e os
#                     volumes deles — só depois de conferir que não há
#                     dado a guardar
set -euo pipefail

usuario=${1:?informe o usuário de deploy}
chave=${2:?informe a chave pública da esteira}
shift 2
remover=("$@")

mudou=0
feito() { echo "  mudou: $*"; mudou=$((mudou + 1)); }

[ "$(id -u)" = 0 ] || { echo "rode como root (wsl.exe -u root)"; exit 1; }
case "$chave" in ssh-*) ;; *) echo "a chave não parece uma chave pública SSH"; exit 1 ;; esac

echo "bootstrap do servidor no WSL — usuário '$usuario'"

# --- pacotes que o Ansible precisa para entrar -------------------------------
faltam=()
for p in openssh-server python3 sudo; do
	dpkg -s "$p" >/dev/null 2>&1 || faltam+=("$p")
done
if [ ${#faltam[@]} -gt 0 ]; then
	DEBIAN_FRONTEND=noninteractive apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${faltam[@]}" >/dev/null
	feito "instalados: ${faltam[*]}"
fi

# --- sshd na 2222, só por chave ----------------------------------------------
# A rede do WSL2 é a mesma para todas as distros: a 22 fica livre para quem
# quiser um sshd na de desenvolvimento, e o job de deploy chega por
# 172.17.0.1:2222.
grava() { # grava <arquivo> <conteúdo>: só escreve se mudou
	if [ "$(cat "$1" 2>/dev/null)" != "$2" ]; then
		mkdir -p "$(dirname "$1")"
		printf '%s\n' "$2" > "$1"
		feito "$1"
		return 0
	fi
	return 1
}

sshd_mudou=0
# Nome da primeira montagem (25/09/2026), por projeto — mas a regra é do
# servidor inteiro.
if [ -e /etc/ssh/sshd_config.d/10-studygo.conf ]; then
	rm -f /etc/ssh/sshd_config.d/10-studygo.conf
	feito "removido 10-studygo.conf (substituído por 10-servidor-wsl.conf)"
	sshd_mudou=1
fi
grava /etc/ssh/sshd_config.d/10-servidor-wsl.conf "# Servidor no WSL: ver ansible/bootstrap-wsl.sh
Port 2222
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no" && sshd_mudou=1

# No Ubuntu 24.04+ o sshd sobe por socket, e a porta vem do ssh.socket. As duas
# linhas: sem a de IPv4, o BindIPv6Only=ipv6-only do Ubuntu deixa o sshd só
# em [::].
grava /etc/systemd/system/ssh.socket.d/porta.conf "[Socket]
ListenStream=
ListenStream=0.0.0.0:2222
ListenStream=[::]:2222" && sshd_mudou=1

sshd -t
if [ "$sshd_mudou" = 1 ]; then
	systemctl daemon-reload
	systemctl restart ssh.socket
fi
systemctl is-active --quiet ssh.socket || { systemctl enable --now ssh.socket; feito "ssh.socket ligado"; }

# --- usuário de deploy do projeto --------------------------------------------
if ! id "$usuario" >/dev/null 2>&1; then
	useradd -m -s /bin/bash "$usuario"
	feito "usuário $usuario criado"
fi
id -nG "$usuario" | grep -qw sudo || { usermod -aG sudo "$usuario"; feito "$usuario no grupo sudo"; }

# O Ansible do site.yml instala pacotes e serviços: sudo sem senha, como o
# bootstrap.yml faz na VPS.
if grava "/etc/sudoers.d/$usuario" "$usuario ALL=(ALL) NOPASSWD:ALL"; then
	chmod 440 "/etc/sudoers.d/$usuario"
	visudo -cf "/etc/sudoers.d/$usuario" >/dev/null
fi

home=$(getent passwd "$usuario" | cut -d: -f6)
install -d -m 700 -o "$usuario" -g "$usuario" "$home/.ssh"
if grava "$home/.ssh/authorized_keys" "$chave"; then
	chown "$usuario:$usuario" "$home/.ssh/authorized_keys"
	chmod 600 "$home/.ssh/authorized_keys"
fi

# --- usuários antigos --------------------------------------------------------
for antigo in "${remover[@]}"; do
	[ "$antigo" = "$usuario" ] && { echo "não removo o próprio usuário de deploy"; exit 1; }
	id "$antigo" >/dev/null 2>&1 || continue
	loginctl disable-linger "$antigo" 2>/dev/null || true
	pkill -u "$antigo" 2>/dev/null || true
	sleep 2
	userdel -r "$antigo" 2>/dev/null || userdel -r -f "$antigo"
	rm -f "/etc/sudoers.d/$antigo"
	feito "usuário $antigo removido, com o Docker rootless e os volumes dele"
done

if [ "$mudou" = 0 ]; then
	echo "nada a mudar: o servidor já está como este script descreve"
else
	echo "$mudou mudança(s). Próximo passo: ansible-playbook site.yml -i inventory/<ambiente>/hosts.ini"
fi
