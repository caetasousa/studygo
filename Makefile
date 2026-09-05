# studygo — atalhos para desenvolver, verificar e publicar.
#
# `make` sozinho lista tudo. A descrição de cada alvo é o comentário `##` na
# própria linha, então a ajuda nunca sai de sincronia com os alvos de verdade.
#
# Os três checks (backend, frontend, edital-processor) são os mesmos que o
# CLAUDE.md manda rodar, e os mesmos que a pipeline executa. O deploy saiu daqui:
# quem publica é o GitLab CI, para que o artefato implantado seja sempre o que
# passou nos testes (ver docs/ci-cd.md).

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Compose de desenvolvimento = docker-compose.yml + docker-compose.override.yml
# (o override entra sozinho e liga o hot reload). PROD_COMPOSE ignora o override
# de propósito, para rodar as imagens de produção aqui na máquina.
COMPOSE      := docker compose
PROD_COMPOSE := docker compose -f docker-compose.yml

# Este clone tem dois remotes e o nome não é o esperado: `gitlab` é o GitLab
# (onde a pipeline roda) e `origin` é o GitHub (espelho). Ficam em variável para
# o alvo `push` não depender de decorar qual é qual.
REMOTE_CI     := gitlab
REMOTE_MIRROR := origin

ANSIBLE_DIR := ansible
# Onde o stack vive na VPS. Ainda /opt/annygo, do nome antigo do projeto — é o
# diretório com o volume do Postgres em produção (ver CLAUDE.md).
REMOTE_APP_DIR := /opt/annygo

.PHONY: help up down restart logs ps rebuild reset prod-local \
        check check-backend check-frontend check-processor check-db fmt \
        status commit push deploy provision deploy-status deploy-logs health

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

# Os testes que exigem PostgreSQL de verdade: migrations, repositories e alguns
# fluxos verticais.
#
# Cada pacote sobe seu próprio container efêmero (Testcontainers) e cada teste
# ganha um database exclusivo dentro dele. Nada aqui toca no banco local: não há
# URL montada à mão, credencial do .env nem porta fixa — e por isso também não há
# `-p 1`, já que os testes não disputam schema nenhum.
check-db: ## Testes de integração com PostgreSQL efêmero (exige Docker)
	cd backend && go test -tags=integration ./...

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
	@echo "commitado. para publicar: make push"

# Publica nos dois remotes. O do GitLab vai primeiro porque é ele que dispara a
# pipeline; o GitHub é espelho e não roda nada.
#
# Tags NÃO sobem aqui, e isso é de propósito: uma tag habilita o job de produção
# na pipeline. Publicar produção é decisão consciente, não efeito colateral de
# um push — quando for a hora, `git push $(REMOTE_CI) v1.2.3`.
push: ## Envia o branch atual para o GitLab (dispara a pipeline) e para o GitHub
	@branch=$$(git rev-parse --abbrev-ref HEAD); \
	for r in $(REMOTE_CI) $(REMOTE_MIRROR); do \
		git remote get-url $$r >/dev/null 2>&1 || { echo "remote '$$r' não existe neste clone"; exit 1; }; \
	done; \
	echo "--- $$branch → $(REMOTE_CI) ($$(git remote get-url $(REMOTE_CI))) ---"; \
	git log --oneline $(REMOTE_CI)/$$branch..HEAD 2>/dev/null || echo "  (branch novo no remote)"; \
	echo "-------------------------------------------------------------"; \
	git push $(REMOTE_CI) $$branch || exit 1; \
	echo; \
	echo "--- espelhando em $(REMOTE_MIRROR) ($$(git remote get-url $(REMOTE_MIRROR))) ---"; \
	git push $(REMOTE_MIRROR) $$branch || { echo; echo "AVISO: o GitLab recebeu, o espelho não. Rode 'git push $(REMOTE_MIRROR) $$branch' depois."; exit 1; }; \
	echo; \
	echo "✓ publicado nos dois. a pipeline do GitLab já está rodando."

# ---------------------------------------------------------------------- deploy

deploy: ## O deploy é da pipeline — veja docs/ci-cd.md
	@echo "O deploy agora roda no GitLab CI, não daqui."
	@echo
	@echo "  staging:  push na main → deploy automático"
	@echo "  produção: git tag v1.2.3 && git push --tags → botão manual na pipeline"
	@echo
	@echo "Motivo: o artefato implantado precisa ser o mesmo que passou nos"
	@echo "testes. Compilar na máquina de quem faz o deploy desfaz essa garantia."
	@exit 1

provision: ## Reaplica a infra da VPS (nginx, firewall, TLS)
	cd $(ANSIBLE_DIR) && ansible-playbook site.yml -i inventory/production/hosts.ini

deploy-status: ## Status dos containers (env=production|staging)
	cd $(ANSIBLE_DIR) && ansible app -i inventory/$(or $(env),production)/hosts.ini -b \
		-a "docker compose -f $(if $(filter staging,$(env)),/opt/studygo-staging,$(REMOTE_APP_DIR))/docker-compose.yml ps"

deploy-logs: ## Últimas linhas de log (svc=backend env=production|staging)
	cd $(ANSIBLE_DIR) && ansible app -i inventory/$(or $(env),production)/hosts.ini -b \
		-a "docker compose -f $(if $(filter staging,$(env)),/opt/studygo-staging,$(REMOTE_APP_DIR))/docker-compose.yml logs --tail 80 $(svc)"

health: ## Bate no /health (env=production|staging)
	@domain=$$(grep '^app_domain:' $(ANSIBLE_DIR)/inventory/$(or $(env),production)/group_vars/app/main.yml | awk '{print $$2}'); \
	echo "GET https://$$domain/health"; \
	curl -fsS "https://$$domain/health" && echo
