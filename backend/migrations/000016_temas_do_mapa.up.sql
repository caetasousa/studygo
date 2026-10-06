-- Os tópicos da matéria que o mapa cobre: o cronograma oferece o mapa só na
-- atividade desses assuntos, e a ementa os marca.
--
-- Pelo texto do tópico, como a atividade o grava: a ementa não tem id estável
-- (editar o concurso regrava os tópicos). Vazio é a matéria inteira — é o que
-- os vínculos que já existem continuam sendo.
ALTER TABLE disciplinas_mapas ADD COLUMN temas text[] NOT NULL DEFAULT '{}';
