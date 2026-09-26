# CLAUDE.md

## 📦 Projeto

`studygo` é uma aplicação de planos de estudo para concursos, composta por:

- `backend/`: API Go e worker.
- `frontend/`: SPA SvelteKit/Svelte 5.
- `edital-processor/`: serviço interno Python/FastAPI para processar editais.
- `ansible/`: provisionamento e deploy da VPS.

O backend utiliza um único hexágono. Não introduza bounded contexts, ORM,
frameworks ou novas camadas arquiteturais sem apresentar uma necessidade
concreta e discutir os trade-offs.

## 📚 Fontes de verdade

Evite duplicar informações destes arquivos:

| | Arquivo | Contém |
|---|---|---|
| 🛠️ | `Makefile` | comandos disponíveis |
| 📖 | `README.md` | produto e visão geral da stack |
| 🐣 | `docs/como-funciona.md` | o projeto sem jargão, para quem chega agora |
| 📐 | `docs/arquitetura.md` | decisões estruturais, modelo de dados e vocabulário |
| 🚀 | `docs/rodar-local.md` | ambiente local e variáveis |
| 🔄 | `docs/fluxo-de-trabalho.md` | desenvolvimento, checks e commits |
| 🚢 | `docs/deploy.md` | VPS, Ansible e produção |
| 🔁 | `docs/ci-cd.md` | pipeline, runners, digest e rollback |
| 📌 | manifests, lockfiles, Dockerfiles | versões das dependências |

Quando documentação e código divergirem, investigue a implementação e atualize
a documentação relacionada.

## 🧭 Antes de alterar código

- Leia a implementação e os testes relacionados antes de propor mudanças.
- Verifique `git status` e preserve alterações não relacionadas.
- Faça mudanças incrementais e limitadas ao pedido.
- Não crie interfaces, DTOs, mappers ou camadas sem uma responsabilidade real.
- Não faça refatorações oportunistas fora do escopo.

## 🧱 Fronteiras arquiteturais

No backend:

- `domain/`: entidades, invariantes e regras de negócio puras.
- `service/`: casos de uso e orquestração da aplicação.
- `port/`: interfaces necessárias pelos casos de uso.
- `adapter/httpapi/`: transporte HTTP, DTOs, tags JSON e mapeamento.
- `adapter/postgres/`: persistência e SQL, organizados por responsabilidade.
- `platform/`: infraestrutura técnica compartilhada.
- `cmd/`: composition roots.

Regras que valem sem exceção:

- O domínio não conhece HTTP, JSON, SQL nem framework algum.
- **Tag JSON só em adapter.** O contrato da API vive em `adapter/httpapi`; o
  cliente do serviço interno (`adapter/editalproc`) carrega as tags do
  contrato dele. `domain/` e `service/` não têm nenhuma:
  os casos de uso devolvem tipos sem tag (`service.PlanoMontado` e companhia)
  e o adapter os traduz em DTO.
- **Todo SQL vive em `adapter/postgres`.** Nenhuma consulta em service ou handler.
- Handlers autenticam, decodificam, chamam um caso de uso e serializam. Nada mais.
- Repositories persistem; não decidem política. Atribuir código de disciplina,
  por exemplo, é decisão do domínio.
- Mappers só onde os dois lados da fronteira são de fato diferentes.

O `edital-processor` produz uma prévia revisável e nunca escreve no PostgreSQL.
A persistência acontece no backend após confirmação do usuário.

## ⚠️ Invariantes importantes

- **O cronograma é materializado.** `atividades` é o cronograma de verdade: toda
  atividade de todo dia existe como linha desde a criação do plano, com id
  próprio. `plano.Gerar` é um planejador puro cujo resultado é GRAVADO — não
  existe atividade "derivada", id sintético nem reconciliação na leitura.
- **O progresso é registrado por atividade**, nunca por dia.
- **A conclusão de um dia é derivada** de suas atividades (`plano.DiaConcluido`)
  e jamais informada pelo cliente.
- **Identidade é por id, nunca por valor.** Atividades e registros apontam para
  `disciplinas(id)`. O `codigo` é o mnemônico exibido, e é preservado quando o
  concurso é editado — regenerá-lo desligaria cronograma e histórico.
- Um registro é história: a FK `registros_atividade → atividades` é RESTRICT.
- A geração do plano é protegida pelo golden test em `backend/internal/domain/plano`.
- A sigla de disciplina tem **uma única implementação**, em
  `domain/concurso/sigla.go` — tanto derivar do nome quanto normalizar a tag que
  o usuário escolheu. O frontend exibe o `codigo` que a API manda e nunca o
  calcula.

## 🐘 Banco de dados e migrations

Migrations ficam em `backend/migrations/`, numeradas em sequência a partir da
baseline `000001_initial_schema`. O runner executa apenas os `.up.sql`; não há
rollback automático pelos `.down.sql`.

- Migrations criam ESTRUTURA. Nada de backfill, função, trigger ou regra de
  negócio — há um teste que falha o build se isso aparecer
  (`TestMigrations_NaoContemLogicaDeNegocio`).
- Trate migrations que podem ter sido aplicadas como imutáveis; corrija com uma
  migration nova.
- A próxima migration é a **000009** (a 000008 criou a legislação). As
  000004–000007 (catálogo de provas, hoje no provasGo) saíram do bundle mas
  estão registradas em staging e em bancos locais: reusar um desses números
  faz a migration nova ser pulada.
- Rollback de código não reverte schema. Migration destrutiva (`DROP TABLE`,
  `DROP COLUMN`, `RENAME`, `ALTER COLUMN ... TYPE`, `SET NOT NULL`, `TRUNCATE`)
  só entra numa publicação posterior à que parou de usar o que ela remove, e
  declara isso num `-- contract:` — sem o marcador, o build falha
  (`TestMigrations_DestrutivaDeclaraOContract`).
- O banco cuida de PK, FK, UNIQUE, NOT NULL, CHECK, índices e transações.
- O domínio e a aplicação cuidam de políticas, cálculos, fluxos e validações.
- DDL destrutivo ou transformação com risco de perda exige aprovação e uma
  estratégia de recuperação.
- Nunca execute reset do banco sem autorização explícita.

## 🔤 Nomenclatura

Conceitos de negócio em português, sem acento nos identificadores: `Usuario`,
`Concurso`, `Plano`, `Disciplina`, `Atividade`, `Registro`, `Anotacao`.

Termos técnicos universais permanecem em inglês: HTTP, JSON, JWT, handler,
middleware, repository, adapter, service, port, worker, request, response,
token, hash, slug, upsert, batch.

**Comentários e godoc em português.** Eles explicam por que o código é assim —
não repetem o que ele faz.

## 🧪 Testes

- **NUNCA** escreva testes unitários depois de escrever o código.
- Prefira altamente testes E2E como o único mecanismo de teste. Use-os para
  verificar se recursos complexos funcionam. No final dos testes E2E, produza
  um artefato verificável e repetível.
- Se for necessário testar um sistema em isolamento, **PRIMEIRO** escreva todas
  as maneiras pelas quais ele poderia falhar, **DEPOIS** escreva o código.

A suíte E2E vive em `e2e/`: o catálogo de falhas em `e2e/CENARIOS.md` (cada
teste cita o id que cobre) e o relatório de cada execução em
`e2e/relatorio/resumo.md`. Como rodar e o que ela sobe estão em
[`docs/fluxo-de-trabalho.md`](docs/fluxo-de-trabalho.md).

## ✅ Comandos e verificação

O `Makefile` é a interface principal. Rode `make` para listar os alvos.

| | Comando | O que faz |
|---|---|---|
| 🐳 | `make up` / `make down` | stack local |
| ⚡ | `make check` | checks dos três serviços — sem Docker |
| 🐘 | `make check-db` | integração com PostgreSQL efêmero — **exige Docker** |
| 🧭 | `make e2e` | o app inteiro pelo navegador, num stack isolado — **exige Docker** |
| 🎨 | `make fmt` | formatação Go e Python |

Durante a implementação, rode primeiro os testes relacionados. Antes de entregar
uma mudança transversal, rode `make check`, `make check-db` e `make e2e`.

`make check` não precisa de Docker. `make check-db` sobe containers efêmeros
(Testcontainers) e **nunca** toca no banco local — não há `TEST_DATABASE_URL`,
URL montada à mão nem porta fixa. Teste que precisa de banco leva a build tag
`integration`.

Os fakes de `internal/service` testam orquestração e NÃO reproduzem constraints
do PostgreSQL; para provocar falha de persistência num teste unitário, injete o
erro do contrato da porta. Constraint, transação e join se testam no banco real.

Mudou o contrato HTTP? O snapshot em
`backend/internal/adapter/httpapi/testdata` falha e aponta o que mudou. Regrave
com `ATUALIZAR_CONTRATO=1 go test ./internal/adapter/httpapi` e diga no commit
qual campo mudou e por quê — o frontend depende disso.

> [!CAUTION]
> `make reset` apaga o banco local. `make provision`, `bootstrap.yml` e
> `lockdown.yml` afetam dados ou infraestrutura: execute somente mediante
> pedido explícito.

## 🚢 Deploy

> [!IMPORTANT]
> **Desde 25/09/2026 só existe o ambiente da esteira (`staging`).** A VPS foi
> suspensa; o servidor é a distro `ubuntu-server` do WSL desta máquina,
> publicada por um túnel da Cloudflare (ver `docs/deploy.md`). "Mandar para
> produção" hoje é `make push` na `main`. Não rode `make release` nem crie tag
> sem pedido explícito — não há `deploy_production` para onde ela vá.

**Todo deploy passa pela pipeline do GitLab, sempre nesta ordem: staging
primeiro, produção depois. Nunca direto.**

- `staging` recebe o push na `main`, automaticamente — depois de o job `e2e`
  passar contra as imagens daquele build.
- `produção` só é liberada por tag, com aprovação manual na pipeline, e só
  depois de o `smoke_test` de staging passar. A tag é a data (`v2026.09.12`) e
  quem a cria é o `make release`, nunca um `git tag` à mão.
- Não existe caminho manual. Não crie um: nem alvo de Makefile, nem script,
  nem `ansible-playbook deploy.yml` na mão. Se aparecer um, remova.
- O Ansible não constrói nada — ele promove digests que a pipeline já testou.
  Compilar na máquina de quem publica desfaz a garantia de que o que subiu é
  o que passou nos testes.

Precisa de uma correção urgente em produção? Ela também passa por staging.
Para voltar atrás sem esperar, use o **Rollback environment** do GitLab
(Operate → Environments → production), que reexecuta o `deploy_production` de
uma versão anterior com os digests dela, sem reconstruir. O `rollback_production`
do template fica desligado no `.gitlab-ci.yml` — não o religue.

## 🔐 Segurança e produção

> [!CAUTION]
> O repositório é **público**. Nunca exponha credenciais, tokens, chaves
> privadas, arquivos `.env`, inventários privados, IPs de acesso ou artefatos
> pessoais. O domínio público da aplicação pode ser versionado.

No servidor atual (WSL), o usuário de deploy é `studygo` — um por projeto.
A VPS de produção (suspensa) utilizava nomes legados `annyGo`, incluindo usuário SSH, chave,
diretório, vhost e identidade do PostgreSQL. Não os renomeie como parte de uma
refatoração comum e nunca sobrescreva `~/.ssh/annygo_deploy`.

Não faça upgrades major de runtime, banco ou sistema operacional como trabalho
incidental. Quando um upgrade fizer parte da tarefa, verifique compatibilidade
e fixe uma versão explícita; nunca use `:latest`.

## 🌱 Git

- Nunca altere a identidade Git configurada.
- Antes de cada commit, mostre exatamente o que está staged e peça autorização.
- Nunca execute `git push`.
- Use Conventional Commits: `type(scope): descrição`.
