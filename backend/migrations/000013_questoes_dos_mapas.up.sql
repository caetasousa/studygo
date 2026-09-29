-- As questões das aulas saem do mapa mental e viram questões de verdade, como
-- as da lei: importadas de um arquivo à parte, presas ao mapa, resolvidas na
-- página dele.
--
-- A chave vem do arquivo e é o que mantém o id (e as respostas) quando o
-- arquivo é importado de novo; a questão que sai do arquivo é desativada, não
-- apagada. O ramo é o título de um ramo do mapa, guardado como texto: o mapa
-- se reimporta trocando os itens, e a questão não pode cair junto.
--
-- Sem alternativas, a questão é de julgar (gabarito CERTO ou ERRADO); com
-- elas, de múltipla escolha, de 2 a 5 (A–E).
CREATE TABLE mapas_questoes (
    id           uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    mapa_id      uuid    NOT NULL REFERENCES mapas (id) ON DELETE CASCADE,
    chave        text    NOT NULL CHECK (chave <> ''),
    ordem        integer NOT NULL CHECK (ordem >= 0),
    ramo         text    NOT NULL,
    origem       text    NOT NULL,
    enunciado    text    NOT NULL CHECK (enunciado <> ''),
    alternativas text[]  NOT NULL CHECK (cardinality(alternativas) IN (0, 2, 3, 4, 5)),
    gabarito     text    NOT NULL CHECK (gabarito IN ('A', 'B', 'C', 'D', 'E', 'CERTO', 'ERRADO')),
    comentario   text    NOT NULL CHECK (comentario <> ''),
    assinatura   text    NOT NULL,
    ativa        boolean NOT NULL DEFAULT true,
    UNIQUE (mapa_id, chave)
);

-- Cada resposta é uma linha: responder de novo não apaga a anterior, e a tela
-- lê a mais recente. O mapa é de uma conta só, então a resposta é do dono do
-- mapa; excluir o mapa (com confirmação) leva as questões e as respostas.
CREATE TABLE mapas_respostas (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    questao_id    uuid        NOT NULL REFERENCES mapas_questoes (id) ON DELETE CASCADE,
    resposta      text        NOT NULL CHECK (resposta IN ('A', 'B', 'C', 'D', 'E', 'CERTO', 'ERRADO')),
    acertou       boolean     NOT NULL,
    respondida_em timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX mapas_respostas_ultima_idx ON mapas_respostas (questao_id, respondida_em DESC);
