package service

import "testing"

func TestAlertaOrcamento(t *testing.T) {
	t.Parallel()

	linhas := func(pares ...[2]int) []LinhaBalanceamento {
		nomes := []string{"Português", "Informática", "Direito Penal"}
		out := make([]LinhaBalanceamento, 0, len(pares))

		for i, p := range pares {
			out = append(out, LinhaBalanceamento{
				Codigo:         "D0" + string(rune('1'+i)),
				Nome:           nomes[i%len(nomes)],
				Questoes:       p[0],
				QuestoesEdital: p[1],
				Delta:          p[0] - p[1],
			})
		}

		return out
	}

	casos := []struct {
		nome       string
		linhas     []LinhaBalanceamento
		querAlerta bool
		querNivel  string
		querTitulo string
		querTexto  string
	}{
		{
			nome:       "orçamento bate, sem alerta",
			linhas:     linhas([2]int{15, 15}, [2]int{25, 25}),
			querAlerta: false,
		},
		{
			nome:       "sem edital não dá para comparar",
			linhas:     linhas([2]int{15, 0}, [2]int{25, 0}),
			querAlerta: false,
		},
		{
			nome:       "excesso pequeno avisa e nomeia o culpado",
			linhas:     linhas([2]int{15, 15}, [2]int{35, 25}),
			querAlerta: true,
			querNivel:  "warn",
			querTitulo: "Você distribuiu 50 de 40 questões — tire 10",
			querTexto:  "provavelmente de Informática (+10)",
		},
		{
			nome:       "excesso grande vira danger",
			linhas:     linhas([2]int{15, 15}, [2]int{60, 25}),
			querAlerta: true,
			querNivel:  "danger",
			querTitulo: "Você distribuiu 75 de 40 questões — tire 35",
		},
		{
			nome:       "falta pede para distribuir",
			linhas:     linhas([2]int{10, 15}, [2]int{23, 25}),
			querAlerta: true,
			querNivel:  "warn",
			querTitulo: "Você distribuiu 33 de 40 questões — distribua mais 7",
			querTexto:  "provavelmente de Português (-5) e Informática (-2)",
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			got := alertaOrcamento(c.linhas)

			if !c.querAlerta {
				if len(got) != 0 {
					t.Fatalf("alertou %+v, não queria alerta", got)
				}

				return
			}

			if len(got) != 1 {
				t.Fatalf("gerou %d alertas, queria 1", len(got))
			}

			if got[0].Nivel != c.querNivel {
				t.Errorf("nível = %q, queria %q", got[0].Nivel, c.querNivel)
			}

			if got[0].Titulo != c.querTitulo {
				t.Errorf("título = %q, queria %q", got[0].Titulo, c.querTitulo)
			}

			if c.querTexto != "" && got[0].Texto != c.querTexto {
				t.Errorf("texto = %q, queria %q", got[0].Texto, c.querTexto)
			}
		})
	}
}
