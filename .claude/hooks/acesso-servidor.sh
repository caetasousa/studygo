#!/usr/bin/env bash
# Trava do acesso ao servidor (a distro ubuntu-server do WSL).
#
# Regra do projeto (CLAUDE.md, "Acesso ao servidor"): o servidor só se alcança
# por SSH, e sempre com quem usa o Claude sabendo. Este hook roda antes de todo
# comando Bash do Claude Code:
#   - nega os caminhos que não são SSH (wsl.exe na distro do servidor, os
#     arquivos dela por /mnt/wsl ou \\wsl$) — por eles nada fica registrado;
#   - avisa na tela, com o comando, tudo o que chega ao servidor por SSH ou
#     mexe nele (ssh, scp, sftp, rsync, make servidor-*, make provision,
#     Ansible, as portas internas do nginx e do backend). Avisa sem pedir
#     aprovação: pedir a cada comando era interrupção demais.
set -uo pipefail

cmd="$(jq -r '.tool_input.command // empty' 2>/dev/null)"
[ -z "$cmd" ] && exit 0

negar() {
	jq -n --arg r "$1" \
		'{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}}'
	exit 0
}

# Sem permissionDecision: o comando segue o modo de permissão da sessão, e o
# aviso aparece para o usuário e para o Claude, que relata depois o que fez.
avisar() {
	jq -n --arg m "$1" \
		'{systemMessage: $m, hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: ("Este comando acessa o servidor. Depois dele, relate ao usuário o que foi feito lá e o que ficou para trás. " + $m)}}'
	exit 0
}

# Caminhos que não são SSH: proibidos.
if printf '%s' "$cmd" | grep -Eqi 'wsl(\.exe)?[^|;&]*(-d|--distribution)[[:space:]]+ubuntu-server|/mnt/wsl/|\\\\wsl(\$|\.localhost)\\+ubuntu-server|//wsl(\$|\.localhost)/ubuntu-server'; then
	negar "Acesso ao servidor fora do SSH é proibido neste projeto (CLAUDE.md, \"Acesso ao servidor\"). Use ssh -p 2222 studygo@127.0.0.1, avisando antes o que vai fazer."
fi

# Tudo o que chega ao servidor: aviso na tela. O syntax-check do Ansible só lê
# o playbook, sem conectar: sai da conta — mas outro ansible-playbook no mesmo
# comando ainda avisa.
checar="$(printf '%s' "$cmd" | sed -E 's/ansible-playbook[[:space:]]+--syntax-check//g')"
if printf '%s' "$checar" | grep -Eq '(^|[^[:alnum:]_-])(ssh|scp|sftp|rsync)([[:space:]]|$)|make[[:space:]]+([^|;&]*[[:space:]])?(servidor-[a-z-]+|provision)|(^|[;&|(`]|\$\()[[:space:]]*([A-Z_]+=[^[:space:]]*[[:space:]]+)*ansible(-playbook)?([[:space:]]|$)|(curl|wget)[^|;&]*:(8480|8081)'; then
	avisar "Acesso ao servidor: $(printf '%s' "$cmd" | head -c 300)"
fi

exit 0
