# 🔄 Fluxo de trabalho


> **Em uma frase:** escreva o código, rode `make check`, faça o commit e
> `git push gitlab main`.
>
> O `make check` roda os mesmos testes que a esteira vai rodar — se passar aqui,
> tem boa chance de passar lá. Novo por aqui? Leia
> [como-funciona.md](como-funciona.md) antes.
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
     └── make push .......... você, sempre → pipeline testa e implanta no servidor
              │
              └── make servidor-health  confirma qual versão está no ar
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
que roda no servidor):

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

### O app inteiro, pelo navegador

```bash
make e2e
```

Sobe um stack descartável com as imagens de produção (projeto compose
`studygo-e2e`, banco vazio, portas 25173/28080/25432, `e2e/stack.env` no lugar
do `.env`), roda a suíte Playwright de `e2e/testes` e derruba tudo no fim — o
banco local não é tocado. Leva menos de um minuto.

Cada teste cobre um item de [`e2e/CENARIOS.md`](../e2e/CENARIOS.md), o catálogo
de como cada funcionalidade pode quebrar, escrito antes dos testes. O resultado
fica em `e2e/relatorio/`: `resumo.md` (cada cenário com o resultado, o print do
estado final, o commit e as imagens usadas) e `html/index.html` (o relatório do
Playwright, com o trace de cada falha).

- `MANTER=1 make e2e` deixa o stack de pé no fim, para investigar.
- `E2E_ARGS="-g C7" make e2e` roda só os testes que casam com o filtro.
- `E2E_EM_CONTAINER=1 make e2e` roda o Playwright dentro da imagem oficial,
  como a pipeline — sem depender do node da máquina.

O `edital-processor` desse stack é um dublê (`e2e/duble-processador`) que
devolve sempre a mesma leitura de edital: é o que deixa o assistente de
importação ser testado inteiro sem Gemini. O processador de verdade tem a
suíte dele (`make check-processor`).

A pipeline roda a mesma suíte no job `e2e` — ver
[ci-cd.md](ci-cd.md#o-caminho-de-uma-mudança).

Funcionalidade nova ganha primeiro a linha no catálogo — como ela pode quebrar
—, depois o teste, depois o código.

`make fmt` formata o Go (`gofmt`) e o Python (`ruff format`) antes de commitar.

> Os testes marcados `integration` e `gemini` do `edital-processor` são pulados
> fora do container / sem chave real — é esperado ver `5 skipped`.

---

## 3️⃣ Commitar

```bash
git add backend/internal/domain/plano/replanejar.go   # você escolhe o que entra
make commit m="fix(backend): fechar lacuna deixada pelo assunto adiantado"
make push
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

**O push é sempre um ato seu** — `make commit` nunca empurra nada. `make push` é
o invólucro que mostra o que vai subir e envia a `main` aos dois remotes (GitLab
primeiro, depois o espelho); tags ficam de fora.

---

## 4️⃣ Publicar

```bash
make push                     # main → pipeline testa e implanta no servidor
make servidor-health          # que versão está no ar (versao, deploy, schema)
make servidor-endereco        # o endereço público da vez
```

Quem publica é a pipeline, não a sua máquina: ela roda os mesmos checks, constrói
a imagem **uma vez**, testa a imagem de pé e promove esse mesmo digest para o
servidor. Há um ambiente só; não se cria tag de versão. Voltar atrás é o botão
**Rollback environment** do GitLab. Detalhes em [ci-cd.md](ci-cd.md).

Quando a mudança for de **infraestrutura** (nginx, túnel, Docker), e não de
código:

```bash
make provision                # tudo, inclusive o apt upgrade da role common
make provision tags=nginx     # só uma peça (nginx, cloudflared, docker, common)
```

Depois do deploy, para olhar o servidor sem abrir SSH na mão:

```bash
make servidor-status              # containers
make servidor-logs svc=backend    # últimas linhas de log
```

### 📦 O que o deploy faz

A pipeline constrói as imagens **uma vez**, publica no Registry e o Ansible
promove o mesmo digest no servidor — nada é montado na sua máquina, e o código-fonte
nunca vai para o servidor. Antes de subir, o deploy copia o banco; as migrations
rodam sozinhas no boot do backend (com advisory lock, então o worker pode subir
junto). Repetir o deploy é seguro. O caminho completo está em
[ci-cd.md](ci-cd.md).

O passo a passo de montar o servidor do zero (bootstrap, provisionamento,
túnel) está em [deploy.md](deploy.md) e [cloudflare-tunnel.md](cloudflare-tunnel.md)
— aquilo roda uma vez só.

---

## ⚖️ Legislação: capturar, escrever questões, publicar

A lei é organizada na sua máquina; o servidor só importa o pacote pronto.

```bash
# 1. baixar e organizar (normas de conteudo/leis/normas.toml; a chave do
#    Gemini vem do .env da raiz)
make leis-capturar slug=cf88        # ou prioridade=A; sem_gemini=1 só com as regras

# 2. conferir conteudo/leis/<slug>/captura.md — divergência regra × Gemini
#    revisada vai para `aceitar` no normas.toml, com o motivo

# 3. questões: peça ao Claude Code para seguir .claude/skills/questoes-de-lei;
#    elas ficam em conteudo/leis/<slug>/questoes.json
make leis-validar atualizar=1       # a mesma validação da importação

# 4. o que se importa em Legislação → Importar lei
make leis-pacote                    # conteudo/leis/pacotes/<slug>.json
```

Como a captura pode errar, e o que ela confere antes de gravar, está em
`edital-processor/app/leis/README.md`. O original baixado e os pacotes não vão
para o Git (o sha256 do original fica no `captura.md`).

## 📖 Resumo dos alvos

| Alvo | Para quê |
|---|---|
| `make` / `make help` | lista tudo |
| `make up` `down` `restart` `ps` `logs` | stack local |
| `make rebuild` `reset` `prod-local` | variações do stack local |
| `make check` (+ `-backend` `-frontend` `-processor`) | qualidade |
| `make check-db` · `make e2e` | banco real · o app inteiro pelo navegador (exigem Docker) |
| `make fmt` | formatação |
| `make leis-capturar` `leis-validar` `leis-pacote` | legislação: capturar, conferir questões, montar pacote |
| `make status` `commit` | git |
| `make push` | publicar (a pipeline testa e implanta) |
| `make provision` | mexer na infra do servidor |
| `make servidor-status` `servidor-logs` `servidor-health` `servidor-endereco` | olhar o servidor |
