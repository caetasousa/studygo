# studygo — atalhos para desenvolver, verificar e publicar.
#
# `make` sozinho lista tudo. A descrição de cada alvo é o comentário `##` na
# própria linha, então a ajuda nunca sai de sincronia com os alvos de verdade.
#
# Os três checks (backend, frontend, edital-processor) são os mesmos que o
# CLAUDE.md manda rodar; `make deploy` roda todos antes de subir, para que uma
# versão quebrada não chegue no ar por esquecimento.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Compose de desenvolvimento = docker-compose.yml + docker-compose.override.yml
# (o override entra sozinho e liga o hot reload). PROD_COMPOSE ignora o override
# de propósito, para rodar as imagens de produção aqui na máquina.
COMPOSE      := docker compose
PROD_COMPOSE := docker compose -f docker-compose.yml

ANSIBLE_DIR := ansible
# Onde o stack vive na VPS. Ainda /opt/annygo, do nome antigo do projeto — é o
# diretório com o volume do Postgres em produção (ver CLAUDE.md).
REMOTE_APP_DIR := /opt/annygo

.PHONY: help up down restart logs ps rebuild reset prod-local \
        check check-backend check-frontend check-processor fmt \
        status commit deploy deploy-fast provision deploy-status deploy-logs health

help: ## Lista os alvos disponíveis
	@echo "studygo — make <alvo>"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Exemplos:"
	@echo "  make logs svc=backend"
	@echo "  make commit m=\"fix(backend): fechar lacuna do plano\""

# ---------------------------------------------------------------- desenvolver

up: ## Sobe o stack local (hot reload)
	$(COMPOSE) up -d

down: ## Para o stack local
	$(COMPOSE) down

restart: ## Reinicia um serviço (svc=backend) ou todos
	$(COMPOSE) restart $(svc)

logs: ## Segue os logs (svc=backend para um só)
	$(COMPOSE) logs -f $(svc)

ps: ## Status dos containers locais
	$(COMPOSE) ps

rebuild: ## Rebuilda as imagens e sobe (use quando mudar dependência)
	$(COMPOSE) up -d --build

reset: ## Derruba o stack APAGANDO o banco local e sobe de novo
	$(COMPOSE) down -v
	$(COMPOSE) up -d

prod-local: ## Sobe as imagens de produção aqui (sem hot reload)
	$(PROD_COMPOSE) up -d --build

# ------------------------------------------------------------------ qualidade

check: check-backend check-frontend check-processor ## Roda todos os checks
	@echo "✓ backend, frontend e edital-processor ok"

check-backend: ## go build + vet + test
	cd backend && go build ./... && go vet ./... && go test ./...

check-frontend: ## svelte-check + vitest
	cd frontend && npm run check && npm test

check-processor: ## ruff + mypy --strict + pytest
	cd edital-processor && uv run ruff check . && uv run mypy --strict app && uv run pytest

fmt: ## Formata o código Go e o Python do processor
	cd backend && gofmt -w .
	cd edital-processor && uv run ruff format .

# ------------------------------------------------------------------------ git

status: ## git status resumido
	@git status --short

# Commita o que JÁ está no stage, depois de passar nos checks.
#
# Não roda `git add -A` de propósito: um arquivo solto na raiz (uma chave SSH
# gerada sem querer, um PDF de teste) entraria no commit sem ninguém ver. Você
# escolhe o que entra com `git add`, o make só garante que o que entra passa.
commit: ## Commita o que está no stage (m="tipo(escopo): mensagem")
ifndef m
	$(error use: make commit m="fix(backend): descrição no imperativo")
endif
	@git diff --cached --quiet && { echo "nada no stage — use 'git add' antes"; exit 1; } || true
	@echo "--- vai entrar no commit ---"
	@git diff --cached --name-status
	@echo "----------------------------"
	$(MAKE) check
	git commit -m "$(m)"
	@echo
	@echo "commitado. o push é seu: git push"

# ---------------------------------------------------------------------- deploy

deploy: check ## Checks + build das imagens + sobe na VPS
	cd $(ANSIBLE_DIR) && ansible-playbook deploy.yml

deploy-fast: ## Deploy SEM rodar os checks (use quando já rodou)
	cd $(ANSIBLE_DIR) && ansible-playbook deploy.yml

provision: ## Reaplica a infra da VPS (nginx, firewall, TLS)
	cd $(ANSIBLE_DIR) && ansible-playbook site.yml

deploy-status: ## Status dos containers na VPS
	cd $(ANSIBLE_DIR) && ansible vps -b -a "docker compose -f $(REMOTE_APP_DIR)/docker-compose.yml ps"

deploy-logs: ## Últimas linhas de log na VPS (svc=backend)
	cd $(ANSIBLE_DIR) && ansible vps -b -a "docker compose -f $(REMOTE_APP_DIR)/docker-compose.yml logs --tail 80 $(svc)"

health: ## Bate no /health do domínio de produção
	@domain=$$(grep '^app_domain:' $(ANSIBLE_DIR)/inventory/group_vars/vps/main.yml | awk '{print $$2}'); \
	echo "GET https://$$domain/health"; \
	curl -fsS "https://$$domain/health" && echo
