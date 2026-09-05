# 🔄 Fluxo de trabalho

**alterar → verificar → commitar → publicar**

Todo o caminho de uma mudança, do editor até o ar, passa por `make`. Rode `make`
sozinho na raiz para ver a lista completa de alvos.

```
editar código
     │
     ├── make up ............ stack local de pé, hot reload
     │
     ├── make check ......... backend + frontend + edital-processor
     │
     ├── git add <arquivos>
     ├── make commit m="..." . roda os checks de novo e commita
     ├── git push ........... você, sempre
     │
     └── git push/tag ....... pipeline testa, publica e implanta
              │
              └── make health  confirma no ar
```

---

---

## 1️⃣ Desenvolver

```bash
make up                  # sobe tudo com hot reload → http://localhost:5173
make logs svc=backend    # acompanhar a API
make ps                  # status
make down                # parar
```

Salvou um arquivo, o navegador recarrega sozinho (frontend) ou a API reinicia em
~5s (backend). `--build` só é necessário quando muda uma **dependência**:

```bash
make rebuild
```

Para começar do zero, apagando o banco local:

```bash
make reset
```

Para ver como fica **de verdade** (imagens de produção, sem hot reload, igual ao
que roda na VPS):

```bash
make prod-local
```

---

## 2️⃣ Verificar

```bash
make check
```

Roda os três, na ordem, e para no primeiro que falhar:

| | Alvo | O que roda | Onde |
|---|---|---|---|
| 🐹 | `make check-backend` | `go build` · `go vet` · `go test` | `backend/` |
| 🧡 | `make check-frontend` | `npm run check` · `npm test` | `frontend/` |
| 🐍 | `make check-processor` | `ruff` · `mypy --strict` · `pytest` | `edital-processor/` |
| 🐘 | `make check-db` | migrations + repositories + fluxos (Testcontainers) | `backend/` |

Os testes que precisam de um PostgreSQL de verdade — migrations, repositories e
os fluxos verticais — ficam fora do `make check`, para que ele não exija Docker:

```bash
make check-db
```

> [!WARNING]
> Eles rodam contra containers efêmeros, um database por teste, e nunca tocam no
> banco local. Sem Docker a suíte **falha** em vez de pular — verde sem ter
> testado nada é pior que vermelho.

Um teste que precise de banco leva a build tag `integration`; um que só leia
arquivos, não.

Mudou o **contrato HTTP**? O snapshot em
`backend/internal/adapter/httpapi/testdata` falha e mostra o que mudou. Regrave
com `ATUALIZAR_CONTRATO=1 go test ./internal/adapter/httpapi` e diga no commit
qual campo mudou — `frontend/src/lib/types.ts` muda junto.

`make fmt` formata o Go (`gofmt`) e o Python (`ruff format`) antes de commitar.

> Os testes marcados `integration` e `gemini` do `edital-processor` são pulados
> fora do container / sem chave real — é esperado ver `5 skipped`.

---

## 3️⃣ Commitar

```bash
git add backend/internal/domain/plano/replanejar.go   # você escolhe o que entra
make commit m="fix(backend): fechar lacuna deixada pelo assunto adiantado"
git push
```

`make commit`:

1. exige a mensagem em `m=` — sem ela, para;
2. recusa se **nada** estiver no stage;
3. mostra a lista de arquivos que vão entrar;
4. roda `make check`;
5. só então commita.

> [!IMPORTANT]
> **Ele não roda `git add -A` de propósito.** > Já aconteceu de um par de chaves SSH gerado sem querer aparecer solto na raiz
> do repo — com `add -A` isso entra no commit sem ninguém ver. Você escolhe o que
> entra; o make garante que o que entra passa nos checks.

A mensagem segue [Conventional Commits](https://www.conventionalcommits.org/):
`tipo(escopo): descrição no imperativo, minúscula, sem ponto final`.

| | Tipo | Quando |
|---|---|---|
| ✨ | `feat` | funcionalidade nova |
| 🐛 | `fix` | correção de bug |
| ♻️ | `refactor` | reorganização sem mudar comportamento |
| 🧪 | `test` | só testes |
| 📝 | `docs` | só documentação |
| 🔧 | `chore`, `ci`, `build`, `style`, `perf` | o resto |

Escopos usados aqui: `backend`, `frontend`, `ansible`, `docker`, `nginx`,
`claude`. Omita o escopo só quando a mudança atravessa o repo inteiro.

**O `git push` é sempre seu** — nenhum alvo do Makefile empurra para o remoto.

---

## 4️⃣ Publicar

```bash
git push                      # main → pipeline implanta em staging sozinha
git tag v1.2.3 && git push --tags   # libera o botão manual de produção
make health                   # GET https://<app_domain>/health → {"status":"ok"}
```

Quem publica é a pipeline, não a sua máquina: ela roda os mesmos checks, constrói
a imagem **uma vez**, testa a imagem de pé e promove esse mesmo digest para
staging e produção. Detalhes e rollback em [ci-cd.md](ci-cd.md).

Quando a mudança for de **infraestrutura** (nginx, firewall, certificado), e não
de código:

```bash
make provision
```

Depois do deploy, para olhar a VPS sem abrir SSH na mão:

```bash
make deploy-status                       # docker compose ps remoto (produção)
make deploy-status env=staging           # o mesmo, em staging
make deploy-logs svc=backend             # últimas 80 linhas
make deploy-logs svc=backend env=staging
```

### 📦 O que o deploy faz

As imagens são buildadas **na sua máquina**, salvas em tarball, copiadas e
carregadas na VPS — o código-fonte nunca vai para o servidor. As migrations
rodam sozinhas no boot do backend (com advisory lock, então o worker pode subir
junto). Repetir o deploy é seguro.

O passo a passo do **primeiro** deploy de um servidor novo (bootstrap, lockdown,
provisionamento) está em [deploy.md](deploy.md) — aquilo roda uma vez só.

---

## 📖 Resumo dos alvos

| Alvo | Para quê |
|---|---|
| `make` / `make help` | lista tudo |
| `make up` `down` `restart` `ps` `logs` | stack local |
| `make rebuild` `reset` `prod-local` | variações do stack local |
| `make check` (+ `-backend` `-frontend` `-processor`) | qualidade |
| `make fmt` | formatação |
| `make status` `commit` | git |
| `git push` · `git tag v*` · `make provision` | publicar e mexer na infra |
| `make deploy-status` `deploy-logs` `health` | olhar a produção |
