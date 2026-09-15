-- O gabarito de cada revisão publicada, fora das questões.
--
-- A resposta morava dentro da questão publicada (provas_questoes.lugar), e o
-- gabarito inteiro dentro do JSON da revisão: a mesma resposta em dois
-- lugares, e o gabarito sem existência própria. A questão é o que o caderno
-- diz; a resposta é o que o gabarito diz — dois documentos da banca.
--
-- Um gabarito por revisão. Não há histórico de preliminar e definitivo: o
-- gabarito que muda entra numa revisão nova da prova.

CREATE TABLE provas_gabaritos (
    prova_id uuid    NOT NULL,
    revisao  integer NOT NULL,
    -- O que o documento diz de si: o código do cargo, o tipo do caderno e se
    -- a banca o publicou como preliminar ou definitivo.
    cargo    text    NOT NULL DEFAULT '',
    caderno  text    NOT NULL DEFAULT '',
    tipo     text    NOT NULL DEFAULT '',
    PRIMARY KEY (prova_id, revisao),
    FOREIGN KEY (prova_id, revisao) REFERENCES provas_revisoes (prova_id, revisao)
);

-- Uma linha por questão do gabarito; resposta vazia é questão anulada. Não
-- aponta para provas_questoes: o gabarito é da prova, e a questão que o
-- rascunho perdeu continua no gabarito.
CREATE TABLE provas_gabarito_respostas (
    prova_id uuid    NOT NULL,
    revisao  integer NOT NULL,
    numero   integer NOT NULL CHECK (numero > 0),
    resposta text    NOT NULL DEFAULT '' CHECK (resposta IN ('', 'A', 'B', 'C', 'D', 'E')),
    situacao text    NOT NULL DEFAULT '',
    PRIMARY KEY (prova_id, revisao, numero),
    FOREIGN KEY (prova_id, revisao) REFERENCES provas_gabaritos (prova_id, revisao)
);
