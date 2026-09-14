-- Devolve a estrutura. Provas publicadas e rascunhos não voltam, e os arquivos
-- do volume `provas_data` ficam órfãos: o runner não executa os .down
-- automaticamente, e quem rodar este à mão precisa limpar o volume também.

DROP TABLE IF EXISTS provas_apoios;
DROP TABLE IF EXISTS provas_apoios_conteudo;
DROP TABLE IF EXISTS provas_questoes;
DROP TABLE IF EXISTS provas_questoes_conteudo;
DROP TABLE IF EXISTS provas_revisoes;
DROP TABLE IF EXISTS provas_importacao_arquivos;
DROP TABLE IF EXISTS provas_arquivos;
DROP TABLE IF EXISTS provas_etapas;
DROP TABLE IF EXISTS provas_importacoes;
DROP TABLE IF EXISTS provas;
