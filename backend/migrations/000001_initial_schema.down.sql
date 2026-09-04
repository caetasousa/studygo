-- Rollback da baseline: derruba tudo na ordem inversa das dependências.
--
-- DROP aqui é rollback de uma migration de criação, não DDL destrutivo sobre um
-- banco em uso. O runner nunca executa este arquivo sozinho — ele existe para
-- desfazer a baseline à mão em desenvolvimento.

DROP TABLE IF EXISTS anotacoes;
DROP TABLE IF EXISTS marco_checks;
DROP TABLE IF EXISTS registros_dia;
DROP TABLE IF EXISTS registros_atividade;
DROP TABLE IF EXISTS atividades;
DROP TABLE IF EXISTS plano_ciclo;
DROP TABLE IF EXISTS plano_disciplinas;
DROP TABLE IF EXISTS planos;
DROP TABLE IF EXISTS conteudo_programatico;
DROP TABLE IF EXISTS marcos;
DROP TABLE IF EXISTS fontes;
DROP TABLE IF EXISTS temas;
DROP TABLE IF EXISTS disciplinas;
DROP TABLE IF EXISTS concursos;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS usuarios;
