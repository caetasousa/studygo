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
# Onde o stack vive na VPS. Ainda /opt/annygo, do nome antigo do projeto — é o
# diretório com o volume do Postgres em produção (ver CLAUDE.md).
REMOTE_APP_DIR := /opt/annygo

.PHONY: help up down restart logs ps rebuild reset prod-local \
        check check-backend check-frontend check-processor check-db fmt lint \
        status commit push release deploy provision deploy-status deploy-logs health

help: ## Lista os alvos disponíveis
	@echo "studygo — make <alvo>"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Exemplos:"
	@echo "  make logs svc=backend"
	@echo "  make commit m=\"fix(backend): fechar lacuna do plano\""
	@echo "  make release        # mostra a tag de produção; go=1 cria e envia"

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
# um push — quando for a hora, `make release`.
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

# A tag que libera produção. O nome é a data — v2026.09.12, e v2026.09.12.1 na
# segunda publicação do dia —, porque o que se quer saber de uma versão no ar é
# de quando ela é. Número sem significado acabou em v1.0.0 e v1.0.1 apontando
# para o mesmo commit.
#
# Dois passos, para nada acontecer por engano: sem go=1 só mostra o que faria.
# A tag é sempre anotada, com os commits desde a anterior — é o changelog.
#
# Recusa quando a tag pediria produção de algo que staging não testou (HEAD
# diferente do que está no GitLab) ou quando não há nada novo desde a última
# tag. Republicar o mesmo commit não é versão nova: é Re-deploy na página de
# ambientes do GitLab.
release: ## Prepara a tag de produção (go=1 cria a tag e envia ao GitLab)
	@set -eu; \
	branch=$$(git rev-parse --abbrev-ref HEAD); \
	[ "$$branch" = main ] || { echo "a tag de produção sai da main — você está em '$$branch'"; exit 1; }; \
	[ -z "$$(git status --porcelain)" ] || { echo "árvore suja — commite ou descarte antes:"; git status --short; exit 1; }; \
	git fetch --quiet --tags $(REMOTE_CI) main; \
	if [ "$$(git rev-parse HEAD)" != "$$(git rev-parse $(REMOTE_CI)/main)" ]; then \
		echo "HEAD ($$(git rev-parse --short HEAD)) difere de $(REMOTE_CI)/main ($$(git rev-parse --short $(REMOTE_CI)/main))."; \
		echo "  a tag tem de apontar para o commit que staging testou."; \
		echo "  falta subir? make push e espere o smoke_test. falta baixar? git pull."; \
		exit 1; \
	fi; \
	ja=$$(git tag --points-at HEAD --list 'v*' | tr '\n' ' '); \
	if [ -n "$$ja" ]; then \
		echo "este commit já foi publicado como $$ja— não há versão nova para criar."; \
		echo "  reimplantar uma versão: Re-deploy/Rollback em $(GITLAB_URL)/-/environments"; \
		exit 1; \
	fi; \
	anterior=$$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || true); \
	base=v$$(date +%Y.%m.%d); tag=$$base; n=0; \
	while git rev-parse -q --verify "refs/tags/$$tag" >/dev/null; do n=$$((n+1)); tag=$$base.$$n; done; \
	intervalo=$${anterior:+$$anterior..}HEAD; \
	qtd=$$(git rev-list --count $$intervalo); \
	commits=$$(git log --format='- %s' $$intervalo); \
	echo "tag:      $$tag"; \
	echo "anterior: $${anterior:-(nenhuma)}"; \
	echo "entram $$qtd commit(s):"; \
	echo "$$commits"; \
	echo; \
	if [ "$(go)" != 1 ]; then \
		echo "nada foi criado. para criar a tag e enviar ao GitLab: make release go=1"; \
		exit 0; \
	fi; \
	printf '%s\n\n%s\n' "$$qtd commit(s) desde $${anterior:-o início}" "$$commits" | git tag -a "$$tag" -F -; \
	git push $(REMOTE_CI) "refs/tags/$$tag"; \
	echo; \
	echo "✓ $$tag enviada. pipeline: $(GITLAB_URL)/-/pipelines?ref=$$tag"; \
	echo "  quando o smoke_test passar, clique em deploy_production."

# ---------------------------------------------------------------------- deploy

deploy: ## O deploy é da pipeline — veja docs/ci-cd.md
	@echo "Não há deploy manual. Todo deploy passa pela pipeline do GitLab,"
	@echo "e sempre nesta ordem: staging primeiro, produção depois."
	@echo
	@echo "  staging:  make push → deploy automático"
	@echo "  produção: make release go=1"
	@echo "            → botão manual na pipeline, liberado só depois do smoke_test"
	@echo
	@echo "Motivo: o artefato implantado precisa ser o mesmo que passou nos"
	@echo "testes. Compilar na máquina de quem publica desfaz essa garantia, e"
	@echo "ir direto para produção pula quem autoriza a ida — o smoke_test."
	@echo
	@echo "Voltar atrás: $(GITLAB_URL)/-/environments"
	@echo "  → production → \"Rollback environment\" na versão desejada"
	@exit 1

# Infra, não aplicação: nginx, firewall, Docker, TLS. A aplicação nunca sobe
# por aqui — quem publica é a pipeline.
#
# `env` é obrigatório de propósito. Este playbook mexe numa VPS que roda
# produção, e um padrão silencioso convidava a acertá-la sem querer.
provision: ## Reaplica a infra da VPS (env=staging|production, tags=nginx,certbot)
ifndef env
	$(error use: make provision env=staging  — ou env=production, conscientemente)
endif
	cd $(ANSIBLE_DIR) && ansible-playbook site.yml -i inventory/$(env)/hosts.ini \
		$(if $(tags),--tags $(tags),)

deploy-status: ## Status dos containers (env=production|staging)
	cd $(ANSIBLE_DIR) && ansible app -i inventory/$(or $(env),production)/hosts.ini -b \
		-a "docker compose -f $(if $(filter staging,$(env)),/opt/studygo-staging,$(REMOTE_APP_DIR))/docker-compose.yml ps"

deploy-logs: ## Últimas linhas de log (svc=backend env=production|staging)
	cd $(ANSIBLE_DIR) && ansible app -i inventory/$(or $(env),production)/hosts.ini -b \
		-a "docker compose -f $(if $(filter staging,$(env)),/opt/studygo-staging,$(REMOTE_APP_DIR))/docker-compose.yml logs --tail 80 $(svc)"

health: ## Bate no /health e diz que versão está no ar (env=production|staging)
	@domain=$$(grep '^app_domain:' $(ANSIBLE_DIR)/inventory/$(or $(env),production)/group_vars/app/main.yml | awk '{print $$2}'); \
	echo "GET https://$$domain/health"; \
	resposta=$$(curl -fsS "https://$$domain/health") || exit 1; \
	echo "$$resposta"; \
	deploy=$$(echo "$$resposta" | sed -n 's/.*"deploy":"\([^"]*\)".*/\1/p'); \
	[ -z "$$deploy" ] || echo "implantado pela pipeline $(GITLAB_URL)/-/pipelines/$$deploy"
