-- O nome com que o curador enviou cada PDF.
--
-- O volume guarda os arquivos pelo id, e a curadoria só mostrava o que a capa
-- disse — órgão, ano, cargo. Com a capa mal lida, duas importações da mesma
-- prova pareciam provas diferentes, e o TRT-15 entrou duas vezes em staging sem
-- ninguém ver de que arquivo cada uma veio.
--
-- NOT NULL DEFAULT '' como as outras colunas de texto da importação: as
-- anteriores a esta ficam sem nome, e "sem nome" é string vazia, não NULL.

ALTER TABLE provas_importacoes
    ADD COLUMN nome_documento text NOT NULL DEFAULT '',
    ADD COLUMN nome_gabarito  text NOT NULL DEFAULT '';
