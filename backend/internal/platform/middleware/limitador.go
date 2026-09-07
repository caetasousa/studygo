package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limitador é um balde de fichas por cliente: cada requisição gasta uma ficha, o
// balde repõe a uma taxa constante, e quem chega com o balde vazio leva 429.
//
// Ele existe porque sem teto nenhum o /api/auth/login aceita força bruta à
// vontade e as rotas de edital viram uma torneira aberta de CPU e de custo de
// IA. É a SEGUNDA linha: a primeira é o limit_req do nginx de borda, que conta
// pelo IP real da conexão e não tem como ser enganado por cabeçalho. Esta aqui
// vale para o que já entrou, e é a única que existe quando se roda sem o edge
// (local, staging sem TLS, teste).
//
// O estado é em memória e por processo. Com uma instância só, que é o desenho
// de hoje, isso é exato; com várias, cada uma passa a ter seu próprio balde e o
// teto efetivo multiplica pelo número de réplicas — quando isso for verdade, o
// lugar da contagem passa a ser o Redis, não este arquivo.
type Limitador struct {
	taxa       float64 // fichas por segundo
	capacidade float64 // rajada máxima
	chave      func(*http.Request) string
	agora      func() time.Time
	logger     *slog.Logger

	mu     sync.Mutex
	baldes map[string]*balde
}

type balde struct {
	fichas float64
	visto  time.Time
}

// maxBaldes limita a memória do limitador. Passando disso, a inserção seguinte
// varre os baldes cheios — quem está com o balde cheio não deve nada, e
// esquecê-lo não afrouxa limite nenhum.
const maxBaldes = 10_000

// OpcaoLimitador ajusta o que o construtor não pede.
type OpcaoLimitador func(*Limitador)

// ComRelogio troca a fonte de tempo. É o que permite testar reposição de fichas
// sem dormir de verdade.
func ComRelogio(agora func() time.Time) OpcaoLimitador {
	return func(l *Limitador) { l.agora = agora }
}

// ComChave troca como o cliente é identificado. O padrão é o IP; as rotas
// autenticadas passam o id do usuário, que é o sujeito certo do limite quando
// ele existe.
func ComChave(fn func(*http.Request) string) OpcaoLimitador {
	return func(l *Limitador) { l.chave = fn }
}

// NovoLimitador constrói um limitador de `porMinuto` requisições por minuto,
// tolerando uma rajada de `rajada`.
func NovoLimitador(porMinuto, rajada int, logger *slog.Logger, opts ...OpcaoLimitador) *Limitador {
	l := &Limitador{
		taxa:       float64(porMinuto) / 60,
		capacidade: float64(rajada),
		chave:      ClienteIP,
		agora:      time.Now,
		logger:     logger,
		baldes:     map[string]*balde{},
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Middleware recusa com 429 quem passou do teto.
func (l *Limitador) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		espera, ok := l.permitir(l.chave(r))
		if !ok {
			l.logger.WarnContext(
				r.Context(),
				"limite de requisições excedido",
				slog.String("path", r.URL.Path),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)

			segundos := int(espera.Seconds())
			if segundos < 1 {
				segundos = 1
			}

			w.Header().Set("Retry-After", strconv.Itoa(segundos))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"erro":"muitas requisições — espere um pouco e tente de novo"}`))

			return
		}

		next.ServeHTTP(w, r)
	})
}

// permitir gasta uma ficha da chave. Devolve quanto falta para a próxima ficha
// quando recusa, para que o Retry-After diga a verdade em vez de um número
// inventado.
func (l *Limitador) permitir(chave string) (time.Duration, bool) {
	agora := l.agora()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, existe := l.baldes[chave]
	if !existe {
		if len(l.baldes) >= maxBaldes {
			l.varrer(agora)
		}

		b = &balde{fichas: l.capacidade, visto: agora}
		l.baldes[chave] = b
	}

	// Repõe o que o tempo desde a última visita rendeu, até encher.
	b.fichas += agora.Sub(b.visto).Seconds() * l.taxa
	if b.fichas > l.capacidade {
		b.fichas = l.capacidade
	}

	b.visto = agora

	if b.fichas < 1 {
		return time.Duration((1 - b.fichas) / l.taxa * float64(time.Second)), false
	}

	b.fichas--

	return 0, true
}

// varrer esquece quem já repôs o balde inteiro. Chamada com o mutex tomado.
func (l *Limitador) varrer(agora time.Time) {
	cheio := time.Duration(l.capacidade/l.taxa) * time.Second

	for chave, b := range l.baldes {
		if agora.Sub(b.visto) >= cheio {
			delete(l.baldes, chave)
		}
	}
}

// ClienteIP identifica o cliente atrás dos proxies.
//
// O backend nunca vê a conexão do navegador: o nginx de borda fala com o nginx
// do frontend, que fala com ele. RemoteAddr é sempre o mesmo container, então
// limitar por ele limitaria todo mundo junto. Quem carrega o IP de verdade é o
// X-Forwarded-For, e a primeira entrada é a boa PORQUE o edge sobrescreve o
// cabeçalho em vez de acrescentar a ele (ver ansible/templates/app.conf.j2) —
// sem essa sobrescrita, qualquer um mandaria o próprio X-Forwarded-For e
// trocaria de identidade a cada requisição.
func ClienteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		primeiro, _, _ := strings.Cut(xff, ",")
		if ip := strings.TrimSpace(primeiro); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
