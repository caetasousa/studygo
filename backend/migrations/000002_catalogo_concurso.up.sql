-- Generic multi-concurso catalogue. TCE-GO B02 is the first row (seed 000003),
-- but the model carries no assumption specific to it.

CREATE TABLE concursos (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             text        NOT NULL UNIQUE,
    nome             text        NOT NULL,
    banca            text        NOT NULL DEFAULT '',
    cargo            text        NOT NULL DEFAULT '',
    emoji            text        NOT NULL DEFAULT '',
    prova_padrao     date        NOT NULL,
    reta_padrao_dias integer     NOT NULL DEFAULT 28,
    resumo           text        NOT NULL DEFAULT '',
    criado_em        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disciplinas (
    id              uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id     uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    codigo          text    NOT NULL,
    nome            text    NOT NULL,
    bloco           text    NOT NULL CHECK (bloco IN ('esp', 'ger')),
    peso            integer NOT NULL,
    questoes_padrao integer NOT NULL DEFAULT 0,
    ordem           integer NOT NULL,
    UNIQUE (concurso_id, codigo)
);

CREATE TABLE temas (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    ordem         integer NOT NULL,
    texto         text    NOT NULL,
    UNIQUE (disciplina_id, ordem)
);

CREATE TABLE marcos (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    ordem       integer NOT NULL,
    rotulo      integer NOT NULL,
    data_inicio date    NOT NULL,
    data_fim    date,
    titulo      text    NOT NULL,
    exige_acao  boolean NOT NULL DEFAULT false,
    e_prova     boolean NOT NULL DEFAULT false,
    UNIQUE (concurso_id, ordem)
);

CREATE TABLE conteudo_programatico (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    ordem       integer NOT NULL,
    tipo        text    NOT NULL CHECK (tipo IN ('ficha', 'rot', 'h', 'p')),
    texto       text    NOT NULL,
    UNIQUE (concurso_id, ordem)
);

CREATE TABLE rev_ciclo (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    ordem       integer NOT NULL,
    titulo      text    NOT NULL,
    questoes    integer NOT NULL DEFAULT 0,
    UNIQUE (concurso_id, ordem)
);

CREATE INDEX disciplinas_concurso_id_idx           ON disciplinas (concurso_id);
CREATE INDEX temas_disciplina_id_idx               ON temas (disciplina_id);
CREATE INDEX marcos_concurso_id_idx                ON marcos (concurso_id);
CREATE INDEX conteudo_programatico_concurso_id_idx ON conteudo_programatico (concurso_id);
CREATE INDEX rev_ciclo_concurso_id_idx             ON rev_ciclo (concurso_id);
