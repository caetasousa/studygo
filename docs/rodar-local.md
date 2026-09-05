# 🚀 Rodar localmente

![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Node](https://img.shields.io/badge/Node-24-339933?logo=nodedotjs&logoColor=white)


> **Em uma frase:** rode `make up` e abra http://localhost:5173.
>
> Isso liga as quatro peças do sistema no seu computador, dentro de caixas
> isoladas (containers), sem instalar nada permanentemente. Para desligar,
> `make down`. Novo por aqui? Leia [como-funciona.md](como-funciona.md) antes.

Do zero ao app rodando em três comandos.

---

## 📋 Pré-requisitos

- 🐳 **Docker + Docker Compose** — o jeito recomendado, sobe tudo
- 🐹 **Go 1.27+** e 🟢 **Node 24+** — opcional, só para hot-reload fora do Docker

---

## 1️⃣ Subir o stack com Docker

Na raiz do repositório:

```bash
cp .env.example .env
# edite .env e defina JWT_SECRET (qualquer string longa serve localmente)
docker compose up -d --build
```

Sobe quatro serviços:

| | Serviço | Porta | Papel |
|---|---|---|---|
| 🧡 | `frontend` | `5173` | SPA (SvelteKit) + nginx que faz proxy de `/api` |
| 🐹 | `backend` | `8080` | API Go; roda as migrations no boot |
| 🔔 | `worker` | — | lembretes de revisão (intervalo em `LEMBRETE_INTERVALO`) |
| 🐘 | `postgres` | `5432` | banco |
| 🐍 | `edital-processor` | — | PDF → prévia estruturada (interno, sem porta no host) |

Abra **http://localhost:5173**, crie uma conta e cadastre seu primeiro concurso.
O plano de estudos é gerado na hora.

```bash
docker compose ps                 # status
docker compose logs -f backend    # logs da API
docker compose logs -f worker     # ver os lembretes sendo calculados
docker compose down               # parar
docker compose down -v            # parar e apagar o banco (recomeçar do zero)
docker compose up -d --build      # subir de novo depois de mudar código
```

---

### 🔑 Variáveis do `.env`

| Variável | Obrigatória | Default | Para quê |
|---|---|---|---|
| `JWT_SECRET` | ✅ | — | assina os tokens de acesso |
| `POSTGRES_USER` / `_PASSWORD` / `_DB` / `_PORT` | | `studygo` / `studygo` / `studygo` / `5432` | credenciais do Postgres. Um `.env` anterior ao nome novo aponta para `annygo` — mantenha assim, ou o app sobe contra um banco vazio |
| `SERVER_PORT` / `FRONTEND_PORT` | | `8080` / `5173` | portas expostas no host |
| `CORS_ORIGIN` | | `http://localhost:5173` | origem liberada na API (só importa se o SPA e a API estiverem em origens diferentes) |
| `LEMBRETE_INTERVALO` | | `24h` | de quanto em quanto tempo o worker roda |
| `GEMINI_API_KEY` | | vazio | liga o "importar concurso a partir do PDF do edital". Lida pelo container `edital-processor`, não pelo backend. Sem ela, o cadastro é manual. Chave grátis em <https://aistudio.google.com/apikey> |
| `EDITAL_PROCESSOR_TOKEN` | | `dev-processor-token` | segredo que o backend apresenta ao `edital-processor` na rede do Compose. Troque em produção |

---

## 2️⃣ Desenvolvimento com hot-reload

Deixe o Postgres e o backend rodando pelo Compose e rode o frontend em modo dev:

```bash
docker compose up -d postgres backend worker

cd frontend
npm install
npm run dev            # http://localhost:5173, recarrega ao salvar
```

O Vite faz proxy de `/api` para `http://localhost:8080` (ajustável com `VITE_API_PROXY`).

Para mexer no **backend** fora do Docker (precisa do Postgres do Compose de pé):

```bash
cd backend
DATABASE_URL="postgres://studygo:studygo@localhost:5432/studygo?sslmode=disable" \
JWT_SECRET="dev" \
go run ./cmd/server
```

---

## 3️⃣ Checagens

```bash
make check      # backend + frontend + edital-processor — sem Docker, < 1 s
make check-db   # migrations, repositories e fluxos verticais — exige Docker
```

> [!TIP]
> `make check-db` sobe PostgreSQL **efêmero** (Testcontainers) e dá a cada teste
> um database próprio. Ele não lê o `.env`, não usa a porta 5432 e **não toca no
> seu banco local** — pode rodar com o Compose de pé ou desligado, tanto faz.

---

## 🩺 Problemas comuns

- ⚠️ **`address already in use :8080`** — outra instância (ou um `go run` antigo) está na porta. `docker compose down` e mate processos `server` órfãos.
- ⚠️ **backend reiniciando com erro de `planos` / relation does not exist** — o worker subiu antes das migrations. Ele se recupera sozinho no próximo tick; ou `docker compose restart worker`.
- ⚠️ **mudou a estrutura da migration** — `make reset` (ou `docker compose down -v`)
  para recriar o banco. Como há uma baseline só, editar o schema em
  desenvolvimento significa recriar, não acrescentar migration.
