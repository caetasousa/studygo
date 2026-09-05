package port

import "time"

// Clock is the source of "now". Services depend on it so tests can pin the date;
// production wires SystemClock.
type Clock interface {
	Now() time.Time
}

// Fuso é onde o dia vira, para todo mundo.
//
// O produto é de concursos brasileiros e o cronograma é feito de DATAS, não de
// instantes: "o que estudo hoje" tem que responder o mesmo às 22h de terça e às
// 8h de terça. Com o relógio em UTC, o dia virava às 21:00 de Brasília — quem
// estudasse à noite via o cronograma do dia seguinte, e quem estudasse de
// madrugada tinha o dia anterior dado como perdido.
//
// Fixo de propósito, não configurável: um fuso por instalação seria mentira
// (dois usuários em fusos diferentes precisariam de dois valores), e a resposta
// certa para isso é o fuso viajar com o usuário — mudança de modelo, não de
// variável de ambiente.
var Fuso = fusoBrasilia()

// fusoBrasilia prefere o banco de fusos do sistema, que acompanha mudanças de
// regra, e cai num offset fixo se ele não existir na imagem.
//
// O fallback é exato hoje: o Brasil extinguiu o horário de verão em 2019, então
// UTC−3 vale o ano inteiro. Se algum dia voltar, é o tzdata que salva — por isso
// ele vem primeiro.
func fusoBrasilia() *time.Location {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return loc
	}

	return time.FixedZone("-03", -3*60*60)
}

// SystemClock returns the real wall-clock time in the app's timezone.
type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().In(Fuso)
}
