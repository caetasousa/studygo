-- O roteiro editável da revisão semanal saiu do produto.
--
-- A tela pedia um título e um número de questões para cada semana do rodízio, e
-- ninguém mantinha isso: em produção a tabela nunca recebeu uma linha. O dia de
-- revisão continua existindo — o que ele faz vem da rotação que o edital sugere
-- ou da padrão do motor, e não mais de um formulário por plano.
--
-- contract: nenhuma versão publicada lê plano_ciclo — a tabela saiu junto com o
-- código que a usava (50a17ef), antes da primeira tag, e nunca teve uma linha.

DROP TABLE IF EXISTS plano_ciclo;
