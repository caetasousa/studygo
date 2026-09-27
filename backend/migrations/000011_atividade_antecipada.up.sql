-- O tópico marcado como estudado antes da data dele fica gravado no dia em
-- que foi marcado — é o registro de que foi estudado, e conta nas
-- estatísticas —, mas não é conteúdo do dia: a tela não o lista, e compactar
-- o cronograma não o move.
ALTER TABLE atividades ADD COLUMN antecipada boolean NOT NULL DEFAULT false;
