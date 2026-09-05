# 🔍 Auditoria — estratégia de testes do backend

![Testcontainers](https://img.shields.io/badge/Testcontainers-v0.44-1DB6C3?logo=docker&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)

> [!NOTE]
> Documento histórico: registra o diagnóstico que motivou a migração para
> Testcontainers e o que foi implementado. A referência viva da estratégia de
> testes é [arquitetura.md](arquitetura.md#-testes).

---

## 0️⃣ Estado do Git (preservado)

156 arquivos staged da refatoração arquitetural + 2 modificados + 2 untracked.
Nada de reset/checkout.

---

## 1️⃣ Premissas — todas confirmadas

| Premissa | Evidência |
|---|---|
| `postgres_test.go` usa `TEST_DATABASE_URL` | `postgres_test.go:30-33` |
| `migrate_test.go` usa o mesmo mecanismo | `migrate_test.go:29-32` |
| Ambos fazem `DROP SCHEMA public CASCADE` | `postgres_test.go:45`, `migrate_test.go:46` |
| `make check-db` sobe o Postgres do Compose e usa o banco de dev | `Makefile:95-101` |
| `-p 1` porque os pacotes recriam o mesmo schema | `Makefile:98-99` (comentário explícito) |
| `fakes_test.go` tem dublês das portas | `fakes_test.go`, 375 linhas |
| `domain/plano/testdata` = fixtures + golden do motor | `motor_test.go:55,125` |
| `httpapi/testdata` = snapshots de contrato JSON | `contrato_test.go:192` |
| `edital-processor/tests/fixtures` = suíte Python | fora do escopo Go |

**Correção de premissa:** o `Makefile` faz `-include .env` + `export` (linhas
22-23). O risco é maior do que "depende do `.env`": as credenciais do dev vazam
para **todo** alvo do Makefile, não só o de teste. Confirmei na prática nesta
sessão — o banco local `annygo` foi destruído por este harness.

---

## 2️⃣ Inventário dos testes

| Conjunto | Classificação | Depende de PG real? | Risco | Recomendação |
|---|---|---|---|---|
| `domain/plano/*_test.go` (39 testes, 6 arquivos) | unidade de domínio | não | nenhum | **manter** |
| `domain/concurso/sigla_test.go` (4) | unidade de domínio | não | nenhum | **manter** |
| `domain/plano/motor_test.go` (3, golden) | unidade de domínio | não | nenhum | **manter** |
| `service/plano_fluxo_test.go` (9) | unidade de aplicação | não (via fakes) | fake finge constraints | **simplificar** |
| `httpapi/contrato_test.go` (3) | contrato HTTP | não | nenhum | **manter** |
| `httpapi/concurso_handler_test.go` (8) | contrato HTTP | não | nenhum | **manter** |
| `httpapi/auth_middleware_test.go` (1) | contrato HTTP | não | nenhum | **manter** |
| `adapter/postgres/postgres_test.go` (8) | integração PG | **sim** | 🔴 apaga banco local | **migrar** |
| `platform/db/migrate_test.go` — 2 testes | integração migrations | **sim** | 🔴 apaga banco local | **migrar** |
| `platform/db` — `TestMigrations_NaoContemLogicaDeNegocio` | unidade (lê arquivos) | não | falso skip hoje | **separar da tag** |
| fluxo vertical (service + PG real) | — | — | — | **não existe: criar** |
| integração externa (Gemini) | Python | — | pré-existente | fora do escopo |

**Lacuna:** não há nenhum teste de fluxo vertical. Os testes de aplicação usam
fakes; os de repository não passam por service. O meio-campo está descoberto.

---

## 3️⃣ Auditoria dos fakes (`fakes_test.go`)

| Fake | Tipo | Veredito |
|---|---|---|
| `fakeConcursos` | fake stateful | **manter** — cenário legítimo |
| `fakePlanos` | fake stateful | **manter** |
| `fakeCaderno` | fake stateful | **manter** |
| `fakeUsuarios` | stub | **manter** |
| `relogioFixo` | stub | **manter** — relógio determinístico |
| `fakeCronograma.gravacoes` | **spy** | **manter** — prova que GET não escreve |
| `fakeCronograma.SubstituirAtividades` | **reimplementação do PG** | **simplificar** |

O problema está em `fakes_test.go:172-219`, código que eu escrevi:

- **linha 195**: `fmt.Errorf("atividades_plano_data_posicao_key: vaga duplicada")`
- **linha 211**: `fmt.Errorf("registros_atividade_atividade_id_fkey: ...")`

Isto fabrica nomes internos de constraint para fingir equivalência com o banco.
O comentário do cabeçalho (linhas 24-30) chega a afirmar paridade — afirmação que
**retiro**: não há suíte de contrato executada contra as duas implementações, e
sem isso a paridade é suposição.

**Ação:** as duas verificações de constraint saem do fake e passam a ser testadas
no PostgreSQL real. O spy `gravacoes` e o estado do cenário **permanecem** — são
orquestração, não semântica relacional.

---

## 4️⃣ Cobertura dos repositories

`UsuarioRepository` (7 métodos) — hoje **1** coberto:

| Método | Teste | Lacuna |
|---|---|---|
| `Criar` | ✅ `EmailEUnicoEIgnoraCaixa` | erro `ErrEmailEmUso` traduzido |
| `PorEmail` / `PorID` | ❌ | round-trip, "não encontrado" |
| `DefinirTema` | ❌ | persistência do tema |
| `GuardarRefreshToken` | ❌ | — |
| `RefreshTokenValido` | ❌ | **válido / expirado / revogado** |
| `RevogarRefreshToken` | ❌ | — |

`ConcursoRepository` (7) — **1** coberto (`Atualizar`, identidade preservada).
Faltam: `ListarPorDono` (ownership), `PorSlug`/`PorID` (não encontrado),
`Criar` (slug UNIQUE), `Remover` (cascade), `DefinirCadernoURL`.

`PlanoRepository` (4) — **1** coberto (`Salvar`/`PorUsuario`).
Faltam: `MarcarMarco`, **`ParaLembrete`** (consulta em lote, join com usuários).

`CronogramaRepository` (7) — **4** cobertos.
Faltam: `ApagarRegistros`, `RegistrosDia`, `SalvarRegistroDia`, atomicidade.

`CadernoRepository` (4) — cobertos em um teste só (CRUD encadeado).

**Teste que não prova o que anuncia:** `TestCronogramaRepo_RegistroRecusaAtividadeDeOutroPlano`
(`postgres_test.go:328`) cria **um** plano e passa um `uuid.New()` aleatório.
Isso prova "atividade inexistente", não isolamento entre planos. Corrigir criando
dois planos e uma atividade real no plano errado.

---

## 5️⃣ Lifecycle — comparação

| Opção | Isolamento | Tempo | Paralelismo | Veredito |
|---|---|---|---|---|
| 1 container por teste | máximo | ~1,5 s × 20 = 30 s+ | sim | caro demais |
| 1 container por pacote | por pacote | ~1,5 s × 2 = 3 s | entre pacotes | vazamento entre testes do mesmo pacote |
| **1 container/pacote + database por teste** | **por teste** | **~1,5 s + ~50 ms/teste** | **sim** | **recomendado** |
| container global entre pacotes | fraco | menor | — | proibido no brief |
| `reuse` persistente | nenhum | menor | — | proibido no brief |

**Recomendação:** um container por pacote (`TestMain`), e **cada teste cria seu
próprio database** via `CREATE DATABASE` a partir de um template já migrado.
`CREATE DATABASE ... TEMPLATE` copia um banco já migrado em milissegundos, então
cada teste começa limpo sem pagar container nem migration.

Isso permite **remover o `-p 1`** e rodar `t.Parallel()`.

---

## 6️⃣ Decisão sobre cada `testdata`

| Diretório | Consumidor | Protege | Decisão |
|---|---|---|---|
| `domain/plano/testdata/concurso_tcego.json` | `motor_test.go:55` | entrada do golden | **manter** |
| `domain/plano/testdata/golden_tcego_default.json` | `motor_test.go:125` | saída do motor (regressão no cronograma) | **manter** |
| `httpapi/testdata/{plano,estatisticas,caderno}.json` | `contrato_test.go:192` | forma do JSON que o frontend lê | **manter** |
| `edital-processor/tests/fixtures/` | pytest | PDF/OCR/Gemini | **não tocar** |

Nenhum deles é seed de PostgreSQL. Testcontainers substitui infraestrutura de
banco, não dados esperados. Mantenho o mecanismo `ATUALIZAR_CONTRATO=1`.

---

## 7️⃣ Plano de implementação

1. `go get testcontainers-go` + módulo `postgres`, versões fixas, `postgres:18-alpine`
   (alinhado a `docker-compose.yml:55` e à produção).
2. Helper em `internal/platform/pgtest` (dois pacotes precisam dele):
   container por pacote, template migrado, `CREATE DATABASE` por teste, cleanup
   registrado, mensagem clara sem Docker.
3. Build tag `integration`. `TestMigrations_NaoContemLogicaDeNegocio` fica **fora**
   da tag (só lê arquivos).
4. `make check-db` → `go test -tags=integration ./...`, sem Compose, sem URL,
   sem `pg_isready`, sem `-p 1`. Falha (não pula) sem Docker.
5. Corrigir o teste de isolamento entre planos; preencher as lacunas da seção 4.
6. Simplificar `fakes_test.go`: tirar as duas simulações de constraint e a
   afirmação de paridade; manter spy e estado.
7. Pequena suíte de fluxo vertical (service real + repository real).
8. Atualizar Makefile, CLAUDE.md, README, docs.

---

## ✅ O que foi implementado

Decisões adotadas: database por teste (via TEMPLATE), build tag `integration`, e
cobertura das lacunas de maior risco.

| Antes | Depois |
|---|---|
| `TEST_DATABASE_URL` + `t.Skip` | container efêmero; falha se não houver Docker |
| `DROP SCHEMA` no banco de dev | database próprio por teste, descartado no fim |
| `make check-db` sobe o Compose | `go test -tags=integration ./...` |
| `-p 1` obrigatório | paralelismo total |
| 8 testes de repository | 37 casos |
| fake fingindo constraints | constraints no banco real; fake injeta erro da porta |
| sem fluxo vertical | 6 fluxos com services e repositories reais |

**Tempos:** suíte rápida < 1 s (sem Docker); integração ~4 s.

**Gap conhecido:** não há CI neste repositório. Quando houver, o pipeline precisa
rodar `make check` e `make check-db` — o segundo exige um daemon Docker
disponível ao runner.
