# 🐹 annyGo

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?logo=svelte&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Ansible](https://img.shields.io/badge/Ansible-deploy-EE0000?logo=ansible&logoColor=white)

**Plano de estudos para concursos públicos.** Você cadastra o concurso (ou manda
o PDF do edital e a IA preenche), e o app monta um cronograma dia-a-dia que
divide o tempo entre as disciplinas pelo **peso de cada uma na prova**, com
revisão espaçada, reta final, simulados e um caderno de erros.

Nasceu do artefato [`claude.ai/code/artifact/ffbfa732…`](https://claude.ai/code/artifact/ffbfa732-6b82-4525-a49d-15dcf7b83693)
— um planejador de arquivo único para o TCE-GO. O **design** e o **motor de
geração do plano** foram preservados na íntegra (o motor tem um _golden test_
contra a saída original); em volta deles cresceu um app multiusuário de verdade.

---

## O que ele faz

- **Cadastro de concurso** — manual (nome, data da prova, disciplinas com nº de
  questões) ou **a partir do edital**: envie o PDF e o Gemini extrai disciplinas,
  conteúdo programático, questões e o cronograma; você revisa antes de salvar.
- **Cronograma automático** — cada questão de conhecimentos específicos vale 2
  pontos, gerais vale 1; essa proporção define quantos dias cada disciplina
  recebe. Fases de **ciclo de conteúdo** (1ª passada no edital + revisão semanal)
  e **reta final** (revisão dirigida, discursiva, simulados).
- **Registro diário** — horas, questões, acertos e anotações por dia; edição
  inline e reordenação (setas / arrastar) das matérias.
- **Balanceamento** — quanto do seu tempo foi para cada disciplina _vs_ o ideal.
- **Estatísticas** — série de horas/acertos, streak de dias, evolução por
  disciplina.
- **Caderno de erros** — anotações livres + os dias com aproveitamento baixo, e
  um **dossiê pronto para o NotebookLM** por disciplina (ementa + leis + suas
  anotações).
- **Datas do edital** — cronograma oficial com checklist e alertas de prazo.
- **Lembretes de revisão espaçada** — um worker calcula os temas de D-1/D-7/D-30
  que vencem no dia (hoje só loga; e-mail fica atrás da mesma interface).
- **Multiusuário** — conta por e-mail/senha (argon2id + JWT), cada usuário com
  seus concursos e progresso isolados.

---

## 🧰 Stack

| | Tecnologia | Papel |
|---|---|---|
| 🐹 | **Go 1.27** | backend hexagonal (`domain / port / service / adapter`), `net/http` puro, sem framework. Motor do plano em `internal/domain/plano` |
| 🧡 | **SvelteKit 2 · Svelte 5** | frontend SPA (`adapter-static`, sem SSR), runes, tokens de design copiados do artefato |
| 🐘 | **PostgreSQL 18** | banco — modelo genérico `concurso → disciplinas → temas → marcos`, sem ORM |
| 📜 | **SQL à mão** | migrations `NNNNNN_nome.up/down.sql`, runner próprio (embed + `schema_migrations` + advisory lock) |
| 🔐 | **argon2id + JWT** | hash de senha (PHC) e auth com refresh rotativo |
| 🤖 | **Gemini API** | importação opcional do concurso a partir do edital (`GEMINI_API_KEY`) |
| 🔔 | **worker** | `cmd/worker` — lembretes diários de revisão espaçada |
| 🐳 | **Docker Compose** | `postgres + backend + worker + frontend` |
| 🌐 | **nginx + Let's Encrypt** | reverse proxy de borda + HTTPS na VPS |
| 📕 | **Ansible** | provisiona a VPS e faz o deploy (imagens buildadas localmente e enviadas prontas) |

---

## 📐 Arquitetura

```mermaid
flowchart LR
    U(("🧑 Usuário")) -- HTTPS --> N["🌐 nginx (borda)"]
    N --> F["🧡 Frontend<br/>SPA + nginx"]
    F -- "/api" --> B["🐹 Backend Go<br/>hexágono único"]
    W["🔔 worker"] --> P
    B --> P[("🐘 PostgreSQL")]
    B -. edital .-> G["🤖 Gemini API"]
```

Um hexágono só, sem bounded contexts — simples de propósito. Convenções de
código, camadas e regras do projeto: **[CLAUDE.md](CLAUDE.md)**.

---

## 🚀 Começar

```bash
git clone <url-do-repo> && cd annyGo
cp .env.example .env          # defina JWT_SECRET
docker compose up -d --build
open http://localhost:5173
```

- **[docs/rodar-local.md](docs/rodar-local.md)** — rodar e desenvolver localmente, variáveis do `.env`, hot-reload, checagens
- **[docs/deploy.md](docs/deploy.md)** — provisionar e atualizar a VPS com Ansible
- **[CLAUDE.md](CLAUDE.md)** — arquitetura e convenções para contribuir
