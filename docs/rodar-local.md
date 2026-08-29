# Rodar localmente

## Pré-requisitos

- **Docker + Docker Compose** (jeito recomendado — sobe tudo)
- Opcional, só para desenvolver com hot-reload: **Go 1.27+** e **Node 24+**

## 1. Subir o stack com Docker

Na raiz do repositório:

```bash
cp .env.example .env
# edite .env e defina JWT_SECRET (qualquer string longa serve localmente)
docker compose up -d --build
```

Sobe quatro serviços:

| Serviço | Porta | Papel |
|---|---|---|
| `frontend` | `5173` | SPA (SvelteKit) + nginx que faz proxy de `/api` para o backend |
| `backend` | `8080` | API Go; roda as migrations no boot |
| `worker` | — | recalcula os lembretes de revisão espaçada (intervalo em `LEMBRETE_INTERVALO`) |
| `postgres` | `5432` | banco |

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

### Variáveis do `.env`

| Variável | Obrigatória | Default | Para quê |
|---|---|---|---|
| `JWT_SECRET` | ✅ | — | assina os tokens de acesso |
| `POSTGRES_USER` / `_PASSWORD` / `_DB` / `_PORT` | | `annygo` / `annygo` / `annygo` / `5432` | credenciais do Postgres |
| `SERVER_PORT` / `FRONTEND_PORT` | | `8080` / `5173` | portas expostas no host |
| `CORS_ORIGIN` | | `http://localhost:5173` | origem liberada na API (só importa se o SPA e a API estiverem em origens diferentes) |
| `LEMBRETE_INTERVALO` | | `24h` | de quanto em quanto tempo o worker roda |
| `GEMINI_API_KEY` | | vazio | liga o "importar concurso a partir do PDF do edital". Sem ela, o cadastro é manual. Chave grátis em <https://aistudio.google.com/apikey> |
| `GEMINI_MODEL` | | `gemini-3.6-flash` | modelo usado na importação |

## 2. Desenvolvimento com hot-reload

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
DATABASE_URL="postgres://annygo:annygo@localhost:5432/annygo?sslmode=disable" \
JWT_SECRET="dev" \
go run ./cmd/server
```

## 3. Checagens

```bash
cd backend  && go test ./... && go vet ./...   # inclui o golden test do motor do plano
cd frontend && npm run check                    # type-check (svelte-check)
```

## Problemas comuns

- **`address already in use :8080`** — outra instância (ou um `go run` antigo) está na porta. `docker compose down` e mate processos `server` órfãos.
- **backend reiniciando com erro de `planos` / relation does not exist** — o worker subiu antes das migrations. Ele se recupera sozinho no próximo tick; ou `docker compose restart worker`.
- **mudou a estrutura de uma migration** — `docker compose down -v` para recriar o banco.
