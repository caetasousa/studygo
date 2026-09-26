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

# Endereço do projeto no GitLab, tirado do remote (https ou ssh), para os alvos
# imprimirem o link da pipeline e da página de ambientes.
GITLAB_URL := $(shell git remote get-url $(REMOTE_CI) 2>/dev/null \
	| sed -e 's|^git@\([^:]*\):|https://\1/|' -e 's/\.git$$//')

ANSIBLE_DIR := ansible

# O servidor: a distro ubuntu-server do WSL desta máquina (docs/deploy.md). O
# SSH é o do usuário de deploy do projeto, na 2222; o Docker lá é o rootless
# dele, então todo comando docker precisa do DOCKER_HOST do usuário.
SERVIDOR     := ssh -i ~/.ssh/studygo_ci -p 2222 studygo@127.0.0.1
SERVIDOR_APP := /opt/studygo-staging
DOCKER_SRV   := DOCKER_HOST=unix:///run/user/$$(id -u)/docker.sock docker compose -f $(SERVIDOR_APP)/docker-compose.yml

.PHONY: help up down restart logs ps rebuild reset prod-local \
        check check-backend check-frontend check-processor check-db e2e fmt lint \
        servidor-endereco \
        status commit push deploy provision servidor-status servidor-logs servidor-health

help: ## Lista os alvos disponíveis
	@echo "studygo — make <alvo>"
	@echo
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Exemplos:"
	@echo "  make logs svc=backend"
	@echo "  make commit m=\"fix(backend): fechar lacuna do plano\""
	@echo "  make servidor-logs svc=backend"

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

# O app inteiro, pelo navegador: a suíte de e2e/testes contra um stack
# descartável com as imagens de produção (projeto compose studygo-e2e, banco
# vazio, portas próprias). Cada teste cobre um item de e2e/CENARIOS.md, e o
# relatório conferível fica em e2e/relatorio/resumo.md.
#
# Fora do `check` pelo mesmo motivo do check-db: exige Docker e leva minutos.
e2e: ## Testes E2E do app inteiro num stack isolado (exige Docker)
	./e2e/rodar.sh

# ------------------------------------------------------------------- servidor

# O endereço público do servidor no WSL (Quick Tunnel da Cloudflare). Muda a
# cada reinício do serviço cloudflared-studygo; o log guarda o da vez.
servidor-endereco: ## Mostra o endereço público atual (*.trycloudflare.com) do servidor no WSL
	@$(SERVIDOR) \
		"sudo journalctl -u cloudflared-studygo --no-pager -o cat | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -1" \
		|| echo "servidor fora do ar? confira: wsl.exe -l -v"

# Fora do `check` de propósito: o `check` é o que a pipeline roda, e ela usa um
# template externo fixado por tag. Acrescentar aqui uma ferramenta que o runner
# pode não ter quebraria a esteira sem aviso. Quando o lint estiver estável,
# promovê-lo é uma decisão consciente — e do lado do template.
lint: ## Lint do backend (golangci-lint; veja backend/.golangci.yml)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint não está instalado."; \
		echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	cd backend && golangci-lint run

cobertura: ## Cobertura dos testes, com os de integração (exige Docker)
	cd backend && go test -tags=integration -coverprofile=coverage.out ./... \
		&& go tool cover -func=coverage.out | tail -1

# Fora do `check` porque depende da rede e de banco de vulnerabilidade que muda
# sozinho: um `check` que falha sem ninguém ter mexido no código vira ruído.
seguranca: ## Varredura de vulnerabilidade nas dependências dos três serviços
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	cd frontend && npm audit --omit=dev
	cd edital-processor && uv run --with pip-audit pip-audit

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
# Na main, o push É a publicação: a pipeline testa e implanta no servidor.
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
	@echo "Não há deploy manual. Quem implanta é a pipeline do GitLab:"
	@echo
	@echo "  make push   → na main, testa e implanta no servidor"
	@echo
	@echo "Motivo: o artefato implantado precisa ser o mesmo que passou nos"
	@echo "testes. Compilar na máquina de quem publica desfaz essa garantia."
	@exit 1

# Infra, não aplicação: Docker rootless, nginx, túnel. A aplicação nunca sobe
# por aqui — quem publica é a pipeline. Um servidor novo começa pelo
# ansible/bootstrap-wsl.sh (docs/deploy.md).
provision: ## Reaplica a infra do servidor com o Ansible (tags=nginx,cloudflared para uma parte só)
	cd $(ANSIBLE_DIR) && ansible-playbook site.yml -i inventory/staging/hosts.ini \
		$(if $(tags),--tags $(tags),)

servidor-status: ## Containers da aplicação no servidor
	@$(SERVIDOR) '$(DOCKER_SRV) ps'

servidor-logs: ## Últimas linhas de log no servidor (svc=backend para um serviço só)
	@$(SERVIDOR) '$(DOCKER_SRV) logs --tail 80 $(svc)'

# Pelo mesmo caminho que o smoke_test da pipeline usa, e pelo endereço público.
servidor-health: ## Diz se o app responde no servidor e pelo endereço público, e que versão está no ar
	@echo "servidor: $$(curl -fsS -m 5 http://127.0.0.1:8480/health 2>/dev/null || echo 'não respondeu')"
	@url=$$($(MAKE) -s servidor-endereco); \
	echo "público:  $$url"; \
	echo "          $$(curl -fsS -m 15 $$url/health 2>/dev/null || echo 'não respondeu')"
