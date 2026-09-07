package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func quieto() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// relogioFalso avança só quando o teste manda, para que a reposição de fichas
// seja verificada sem dormir de verdade.
type relogioFalso struct {
	mu    sync.Mutex
	agora time.Time
}

func (r *relogioFalso) Now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.agora
}

func (r *relogioFalso) avancar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agora = r.agora.Add(d)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
}

// bater dispara n requisições do mesmo IP e devolve os status na ordem.
func bater(h http.Handler, n int, ip string) []int {
	out := make([]int, 0, n)

	for range n {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		r.Header.Set("X-Forwarded-For", ip)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		out = append(out, rec.Code)
	}

	return out
}

func TestLimitador_recusaDepoisDaRajada(t *testing.T) {
	t.Parallel()

	relogio := &relogioFalso{agora: time.Unix(0, 0)}
	lim := NovoLimitador(60, 3, quieto(), ComRelogio(relogio.Now))
	h := lim.Middleware(okHandler())

	status := bater(h, 5, "10.0.0.1")

	for i, code := range status[:3] {
		if code != http.StatusTeapot {
			t.Errorf("requisição %d = %d, quer %d dentro da rajada", i+1, code, http.StatusTeapot)
		}
	}

	for i, code := range status[3:] {
		if code != http.StatusTooManyRequests {
			t.Errorf("requisição %d = %d, quer 429 depois da rajada", i+4, code)
		}
	}
}

func TestLimitador_reponhaComOTempo(t *testing.T) {
	t.Parallel()

	relogio := &relogioFalso{agora: time.Unix(0, 0)}
	// 60 por minuto = uma ficha por segundo.
	lim := NovoLimitador(60, 2, quieto(), ComRelogio(relogio.Now))
	h := lim.Middleware(okHandler())

	bater(h, 2, "10.0.0.2") // esvazia o balde

	if got := bater(h, 1, "10.0.0.2")[0]; got != http.StatusTooManyRequests {
		t.Fatalf("com o balde vazio = %d, quer 429", got)
	}

	relogio.avancar(time.Second)

	if got := bater(h, 1, "10.0.0.2")[0]; got != http.StatusTeapot {
		t.Errorf("um segundo depois = %d, quer a ficha reposta", got)
	}
}

// O limite é por cliente: o excesso de um não pode fechar a porta do outro.
func TestLimitador_isolaClientes(t *testing.T) {
	t.Parallel()

	relogio := &relogioFalso{agora: time.Unix(0, 0)}
	lim := NovoLimitador(60, 1, quieto(), ComRelogio(relogio.Now))
	h := lim.Middleware(okHandler())

	bater(h, 2, "10.0.0.3") // este estourou

	if got := bater(h, 1, "10.0.0.4")[0]; got != http.StatusTeapot {
		t.Errorf("outro cliente = %d, quer passar", got)
	}
}

func TestLimitador_retryAfterDizQuandoVoltar(t *testing.T) {
	t.Parallel()

	relogio := &relogioFalso{agora: time.Unix(0, 0)}
	lim := NovoLimitador(6, 1, quieto(), ComRelogio(relogio.Now)) // uma ficha a cada 10s
	h := lim.Middleware(okHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.5")
	h.ServeHTTP(httptest.NewRecorder(), r)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, quer 429", rec.Code)
	}

	segundos, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q, quer um número", rec.Header().Get("Retry-After"))
	}

	if segundos < 1 || segundos > 10 {
		t.Errorf("Retry-After = %d, quer algo entre 1 e 10 segundos", segundos)
	}
}

// ComChave é o que faz a rota autenticada contar por conta em vez de por IP.
func TestLimitador_chavePersonalizada(t *testing.T) {
	t.Parallel()

	relogio := &relogioFalso{agora: time.Unix(0, 0)}
	lim := NovoLimitador(60, 1, quieto(),
		ComRelogio(relogio.Now),
		ComChave(func(r *http.Request) string { return r.Header.Get("X-Conta") }),
	)
	h := lim.Middleware(okHandler())

	chamar := func(conta string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/editais/analisar", nil)
		r.Header.Set("X-Forwarded-For", "10.0.0.9") // o MESMO IP nas duas contas
		r.Header.Set("X-Conta", conta)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)

		return rec.Code
	}

	chamar("ana")

	if got := chamar("ana"); got != http.StatusTooManyRequests {
		t.Errorf("segunda da mesma conta = %d, quer 429", got)
	}

	if got := chamar("bruno"); got != http.StatusTeapot {
		t.Errorf("outra conta no mesmo IP = %d, quer passar", got)
	}
}

func TestClienteIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{
			name:       "sem proxy usa a conexão",
			remoteAddr: "192.0.2.7:54321",
			want:       "192.0.2.7",
		},
		{
			// O edge sobrescreve o cabeçalho e o nginx do frontend acrescenta o
			// próprio salto, então a primeira entrada é o navegador.
			name:       "atrás dos dois nginx",
			xff:        "203.0.113.5, 127.0.0.1",
			remoteAddr: "172.18.0.4:33333",
			want:       "203.0.113.5",
		},
		{
			name:       "cabeçalho vazio cai na conexão",
			xff:        "   ",
			remoteAddr: "172.18.0.4:33333",
			want:       "172.18.0.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr

			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}

			if got := ClienteIP(r); got != tt.want {
				t.Errorf("ClienteIP = %q, quer %q", got, tt.want)
			}
		})
	}
}

// O limitador é atravessado por muitas goroutines ao mesmo tempo; o que este
// teste protege é a ausência de corrida (rodar com -race), não o número exato.
func TestLimitador_concorrente(t *testing.T) {
	t.Parallel()

	lim := NovoLimitador(6000, 50, quieto())
	h := lim.Middleware(okHandler())

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()
			bater(h, 4, "10.1.0."+strconv.Itoa(n))
		}(i)
	}

	wg.Wait()
}
