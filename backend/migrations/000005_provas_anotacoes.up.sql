-- Anotação do estudante numa questão de prova publicada: o porquê de uma
-- resposta estar certa ou errada, a pesquisa que ele fez, em markdown.
--
-- É de quem escreveu — ninguém mais lê. Acompanha a prova e o número da
-- questão, não a revisão: republicar a prova mantém a numeração, e a anotação
-- continua no lugar.

CREATE TABLE provas_anotacoes (
    usuario_id    uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    prova_id      uuid        NOT NULL REFERENCES provas (id) ON DELETE CASCADE,
    numero        integer     NOT NULL CHECK (numero > 0),
    texto         text        NOT NULL,
    atualizada_em timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usuario_id, prova_id, numero)
);
