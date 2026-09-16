package httpapi

import "net/http"

// O refresh token viaja em cookie HttpOnly, e só nele.
//
// Ele vale 30 dias e é o que reabre a sessão; o access token vale 15 minutos.
// Guardados os dois no localStorage, um XSS levava a conta inteira em vez de um
// quarto de hora. HttpOnly tira o refresh do alcance do JavaScript — inclusive
// do nosso, que por isso nunca o vê.
//
// O Path restringe o envio a /api/auth: renovar e sair são as únicas rotas que
// o recebem, e nenhuma outra pode ser levada a agir por ele.
//
// SameSite=Strict, somado ao access token no header Authorization, é o que
// dispensa token de CSRF: requisição disparada por outro site não carrega o
// cookie, e as rotas que mudam dados nem olham para ele — elas exigem o header,
// que um formulário de terceiro não consegue montar.
const (
	cookieDoRefresh  = "studygo_refresh"
	caminhoDoRefresh = "/api/auth"
)

func (h *AuthHandler) gravarRefresh(w http.ResponseWriter, r *http.Request, token string) {
	//nolint:gosec // G124: o Secure é decidido por sobreTLS, e o linter não
	// consegue ver isso. Fixá-lo em true deixaria o desenvolvimento local, que é
	// http, sem sessão nenhuma.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieDoRefresh,
		Value:    token,
		Path:     caminhoDoRefresh,
		MaxAge:   int(h.refreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   sobreTLS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// apagarRefresh manda o navegador esquecer o cookie. Nome e Path têm de bater
// com os da gravação: divergindo qualquer um dos dois, o navegador entende que
// é outro cookie e o antigo continua lá.
func apagarRefresh(w http.ResponseWriter, r *http.Request) {
	//nolint:gosec // G124: mesmo motivo de gravarRefresh — e aqui o cookie vai
	// vazio e com Max-Age negativo, isto é, some.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieDoRefresh,
		Value:    "",
		Path:     caminhoDoRefresh,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sobreTLS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func refreshDaRequisicao(r *http.Request) string {
	cookie, err := r.Cookie(cookieDoRefresh)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// sobreTLS decide o atributo Secure pela conexão de fato: em produção o nginx
// termina o TLS e anuncia isso no X-Forwarded-Proto; em desenvolvimento o app
// abre em http://localhost, onde um cookie Secure simplesmente não seria
// guardado e a sessão morreria a cada renovação.
func sobreTLS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
