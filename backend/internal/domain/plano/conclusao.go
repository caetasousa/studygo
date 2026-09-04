package plano

import "github.com/google/uuid"

// As decisões sobre "isto pode ser mexido?", que o replanejamento consulta
// antes de tocar em qualquer dia.
//
// São regras do domínio do estudo, não orquestração: leem apenas o cronograma e
// os registros, sem repositório e sem relógio.

// AtividadeMovivel diz se uma atividade pode mudar de lugar.
//
// Só a conclusão trava: mover o que já foi estudado faria o cronograma mentir
// sobre o que aconteceu. Um dia não fica travado só por guardar trabalho
// concluído — as outras matérias dele continuam livres.
func AtividadeMovivel(registros Registros, id uuid.UUID) bool {
	return !registros.Concluida(id)
}

// RitmoMudou diz se uma mudança de configuração alterou como o dia é
// preenchido — quantos blocos ele tem, ou quanto dura cada um. São as duas
// opções que mudam o que o motor distribui, então mexer numa delas precisa
// alcançar o cronograma já gravado.
func RitmoMudou(antes, depois Config) bool {
	antes = antes.Normalizar()
	depois = depois.Normalizar()

	return antes.BlocosPorDia != depois.BlocosPorDia ||
		antes.MinutosBloco != depois.MinutosBloco
}
