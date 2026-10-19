package mapa

// MapaPronto é um mapa que vem com o app, igual para toda conta: o texto e as
// questões. Ninguém o tem até adicioná-lo; adicionado, é um mapa da conta como
// outro qualquer.
type MapaPronto struct {
	Texto    string
	Questoes ArquivoDeQuestoes
}
