package usuario

import (
	"errors"
	"testing"
)

// As invariantes do cadastro. É o portão de entrada da aplicação e não tinha
// teste: a ordem em que as validações falham é contrato de tela — o formulário
// mostra UM erro por vez, e mostrar o errado manda o estudante corrigir o campo
// que não é o problema.

func TestValidarCadastro_aceitaOCasoNormal(t *testing.T) {
	t.Parallel()

	if err := ValidarCadastro("ana@exemplo.com", "Ana", "senha-de-verdade"); err != nil {
		t.Errorf("cadastro válido recusado: %v", err)
	}
}

func TestValidarCadastro_email(t *testing.T) {
	t.Parallel()

	validos := []string{
		"ana@exemplo.com",
		"ANA@EXEMPLO.COM",          // a normalização cuida da caixa
		"  ana@exemplo.com  ",      // e dos espaços
		"ana.maria+tag@sub.dom.br", // ponto e mais fazem parte de e-mail
	}

	for _, email := range validos {
		t.Run("aceita "+email, func(t *testing.T) {
			t.Parallel()

			if err := ValidarCadastro(email, "Ana", "senha-boa-123"); err != nil {
				t.Errorf("email %q recusado: %v", email, err)
			}
		})
	}

	invalidos := []string{
		"",
		"ana",
		"ana@",
		"@exemplo.com",
		"ana@exemplo",     // sem ponto no domínio
		"ana exemplo.com", // sem arroba
		"ana@ex emplo.com",
		"ana@@exemplo.com",
	}

	for _, email := range invalidos {
		t.Run("recusa "+email, func(t *testing.T) {
			t.Parallel()

			if err := ValidarCadastro(email, "Ana", "senha-boa-123"); !errors.Is(err, ErrEmailInvalido) {
				t.Errorf("email %q: erro = %v, quer ErrEmailInvalido", email, err)
			}
		})
	}
}

func TestValidarCadastro_nomeObrigatorio(t *testing.T) {
	t.Parallel()

	for _, nome := range []string{"", "   ", "\t\n"} {
		if err := ValidarCadastro("ana@exemplo.com", nome, "senha-boa-123"); !errors.Is(err, ErrNomeObrigatorio) {
			t.Errorf("nome %q: erro = %v, quer ErrNomeObrigatorio", nome, err)
		}
	}
}

// O piso é de oito CARACTERES contados como bytes; o teste fixa a fronteira
// exata, que é onde esse tipo de regra costuma errar por um.
func TestValidarCadastro_senha(t *testing.T) {
	t.Parallel()

	if err := ValidarCadastro("ana@exemplo.com", "Ana", "1234567"); !errors.Is(err, ErrSenhaFraca) {
		t.Errorf("sete caracteres: erro = %v, quer ErrSenhaFraca", err)
	}

	if err := ValidarCadastro("ana@exemplo.com", "Ana", "12345678"); err != nil {
		t.Errorf("oito caracteres devia passar: %v", err)
	}
}

// A ordem importa para a tela: e-mail primeiro, depois nome, depois senha.
func TestValidarCadastro_ordemDasFalhas(t *testing.T) {
	t.Parallel()

	// Tudo errado ao mesmo tempo: o e-mail é o que aparece.
	if err := ValidarCadastro("nao-e-email", "", "123"); !errors.Is(err, ErrEmailInvalido) {
		t.Errorf("erro = %v, quer ErrEmailInvalido primeiro", err)
	}

	// E-mail certo, nome e senha errados: o nome é o que aparece.
	if err := ValidarCadastro("ana@exemplo.com", "", "123"); !errors.Is(err, ErrNomeObrigatorio) {
		t.Errorf("erro = %v, quer ErrNomeObrigatorio antes da senha", err)
	}
}

func TestNormalizarEmail(t *testing.T) {
	t.Parallel()

	casos := map[string]string{
		"  Ana@Exemplo.COM ": "ana@exemplo.com",
		"ana@exemplo.com":    "ana@exemplo.com",
		"":                   "",
		"\tANA@X.BR\n":       "ana@x.br",
	}

	for entrada, esperado := range casos {
		if got := NormalizarEmail(entrada); got != esperado {
			t.Errorf("NormalizarEmail(%q) = %q, quer %q", entrada, got, esperado)
		}
	}
}

// TemaValido nunca falha: um valor desconhecido cai no padrão. É o que impede
// um cliente antigo (ou um valor gravado por uma versão futura) de deixar a
// conta sem tema nenhum.
func TestTemaValido(t *testing.T) {
	t.Parallel()

	casos := map[string]Tema{
		"light":     TemaClaro,
		"dark":      TemaEscuro,
		"system":    TemaSistema,
		"  light  ": TemaClaro,
		"":          TemaPadrao,
		"roxo":      TemaPadrao,
		"LIGHT":     TemaPadrao, // a comparação é sensível à caixa, de propósito
	}

	for entrada, esperado := range casos {
		if got := TemaValido(entrada); got != esperado {
			t.Errorf("TemaValido(%q) = %q, quer %q", entrada, got, esperado)
		}
	}
}
