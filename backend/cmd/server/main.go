package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studygo/internal/adapter/crypto"
	"studygo/internal/adapter/editalproc"
	"studygo/internal/adapter/httpapi"
	"studygo/internal/adapter/postgres"
	"studygo/internal/platform/config"
	"studygo/internal/platform/db"
	"studygo/internal/platform/httpserver"
	"studygo/internal/platform/middleware"
	"studygo/internal/port"
	"studygo/internal/service"
	"studygo/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("server exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}

		logger.Info("migrations applied")
	}

	// O mesmo processador lê editais, captura leis e faz os mapas mentais.
	var (
		editalProc port.EditalProcessor    = editalproc.Indisponivel{}
		capturador port.CapturadorDeLeis   = editalproc.Indisponivel{}
		mapas      port.ProcessadorDeMapas = editalproc.Indisponivel{}
	)
	if cfg.EditalProcessorURL != "" {
		cliente := editalproc.New(cfg.EditalProcessorURL, cfg.EditalProcessorToken)
		editalProc, capturador, mapas = cliente, cliente, cliente
		logger.Info("edital import enabled", slog.String("processor", cfg.EditalProcessorURL))
	}

	handler, interno := montarHandler(pool, cfg, editalProc, capturador, mapas, logger)

	// As rotas por onde o processador devolve o mapa: porta própria, que não é
	// publicada nem passa pelo nginx. O resultado traz as imagens do mapa; o
	// prazo de leitura é o da rede interna, sem pressa.
	srvInterno := httpserver.New(cfg.InternalAddr, httpserver.WithHandler(interno))

	go func() {
		if err := srvInterno.Run(ctx); err != nil && err != http.ErrServerClosed {
			logger.Error("internal server exited", slog.Any("error", err))
			cancel()
		}
	}()

	srv := httpserver.New(
		cfg.ServerAddr,
		httpserver.WithHandler(handler),
		// As rotas de importação esperam o Gemini (até uns 90s por chamada, duas
		// por passo). Todo o resto responde em milissegundos; este prazo largo só
		// existe para não cortar no meio uma resposta lenta que ia dar certo.
		httpserver.WithWriteTimeout(240*time.Second),
	)

	logger.Info("server starting",
		slog.String("addr", cfg.ServerAddr),
		slog.String("versao", cfg.Versao),
		slog.String("deploy", cfg.Deploy),
	)

	if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// montarHandler liga repositories, casos de uso e handlers no http.Handler que
// o servidor serve, com a cadeia de middlewares. Fica fora do run para que o
// teste de integração suba exatamente esta fiação — e não uma cópia dela.
//
// O processador de edital vem de fora: ele é o serviço externo, e quem decide
// qual usar (o cliente HTTP, ou nenhum) é a configuração.
func montarHandler(
	pool *pgxpool.Pool,
	cfg config.Config,
	editalProc port.EditalProcessor,
	capturador port.CapturadorDeLeis,
	processadorDeMapas port.ProcessadorDeMapas,
	logger *slog.Logger,
) (http.Handler, http.Handler) {
	clock := port.SystemClock{}
	hasher := crypto.NewArgon2Hasher(cfg.Argon2)
	tokens := crypto.NewJWTIssuer(cfg.JWTSecret, cfg.AccessTTL)

	usuarioRepo := postgres.NewUsuarioRepo(pool)
	concursoRepo := postgres.NewConcursoRepo(pool)
	planoRepo := postgres.NewPlanoRepo(pool)
	cronogramaRepo := postgres.NewCronogramaRepo(pool)
	cadernoRepo := postgres.NewCadernoRepo(pool)

	authService := service.NewAuthService(usuarioRepo, hasher, tokens, clock, cfg.RefreshTTL)
	leiService := service.NewLeiService(postgres.NewLeiRepo(pool), concursoRepo, capturador)
	mapaService := service.NewMapaService(postgres.NewMapaRepo(pool), concursoRepo)

	cifra, err := crypto.NewCifra(cfg.JWTSecret)
	if err != nil {
		panic(err) // só falha sem fonte de entropia ou com AES quebrado: não há o que servir
	}

	tokensDoClaude := service.NewTokenDoClaudeService(usuarioRepo, cifra)
	pedidosHandler := httpapi.NewPedidosDeMapaHandler(service.NewPedidosDeMapaService(
		postgres.NewMapaRepo(pool), concursoRepo, mapaService, processadorDeMapas, tokensDoClaude,
	), logger)

	// Os seis casos de uso do plano compartilham as mesmas dependências.
	deps := service.Dependencias{
		Planos:     planoRepo,
		Cronograma: cronogramaRepo,
		Concursos:  concursoRepo,
		Caderno:    cadernoRepo,
		Usuarios:   usuarioRepo,
		Relogio:    clock,
	}

	handlers := httpapi.Handlers{
		Health: httpapi.NewHealthHandler(
			service.NewHealthService(pool, db.NovoSchema(pool), cfg.Versao, cfg.Deploy), logger,
		),
		Auth: httpapi.NewAuthHandler(authService, cfg.RefreshTTL, logger),
		Concurso: httpapi.NewConcursoHandler(
			service.NewConcursoService(concursoRepo, editalProc), logger,
		),
		Plano: httpapi.NewPlanoHandler(
			service.NewPlanoService(deps),
			service.NewCronogramaService(deps),
			service.NewRegistroService(deps),
			service.NewEstatisticaService(deps),
			service.NewCadernoService(deps),
			service.NewDossieService(deps),
			service.NewPlanilhaService(deps),
			service.NewImportacaoTECService(deps),
			logger,
		),
		Lei:         httpapi.NewLeiHandler(leiService, logger),
		Mapa:        httpapi.NewMapaHandler(mapaService, logger),
		Processador: httpapi.NewProcessadorHandler(tokensDoClaude, service.NewConexaoDoClaudeService(processadorDeMapas), logger),
		Pedidos:     pedidosHandler,
	}

	router := httpapi.NewRouter(handlers, tokens, authService, httpapi.LimitesPadrao(logger), logger)

	// Teto global por IP, acima dos limites por rota. É generoso de propósito:
	// a SPA conversa bastante (toda ação do plano devolve o plano inteiro), então
	// o número aqui não é uma política de uso — é o que separa um usuário
	// intenso de um laço automatizado.
	global := middleware.NovoLimitador(240, 120, logger)

	publico := middleware.Chain(
		router,
		middleware.RequestID,
		middleware.Recover(logger),
		middleware.Logger(logger),
		middleware.CORS(cfg.CORSOrigin),
		global.Middleware,
	)

	interno := middleware.Chain(
		httpapi.NewRouterInterno(pedidosHandler, cfg.EditalProcessorToken, logger),
		middleware.RequestID,
		middleware.Recover(logger),
		middleware.Logger(logger),
	)

	return publico, interno
}
