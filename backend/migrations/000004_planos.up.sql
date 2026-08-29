-- One study plan per user per concurso, plus everything the artifact kept in
-- localStorage: daily records, edital checkmarks, manual reorderings, notes.

CREATE TABLE planos (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    concurso_id      uuid        NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    inicio           date        NOT NULL,
    prova            date        NOT NULL,
    horas_dia        numeric(4, 2) NOT NULL DEFAULT 2,
    dias_estudo      integer[]   NOT NULL DEFAULT '{1,2,3,4,5}',
    dia_revisao      integer     NOT NULL DEFAULT 5,
    reta_final_dias  integer     NOT NULL DEFAULT 28,
    tema_ui          text        NOT NULL DEFAULT 'system',
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, concurso_id)
);

CREATE TABLE plano_questoes (
    plano_id      uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    questoes      integer NOT NULL,
    PRIMARY KEY (plano_id, disciplina_id)
);

CREATE TABLE registros_dia (
    id        uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id  uuid          NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data      date          NOT NULL,
    horas     numeric(5, 2),
    concluido boolean       NOT NULL DEFAULT false,
    questoes  integer,
    acertos   integer,
    nota      text          NOT NULL DEFAULT '',
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plano_id, data)
);

CREATE TABLE marco_checks (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    marco_id uuid    NOT NULL REFERENCES marcos (id) ON DELETE CASCADE,
    cumprido boolean NOT NULL DEFAULT false,
    PRIMARY KEY (plano_id, marco_id)
);

CREATE TABLE reordenacoes (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data     date    NOT NULL,
    tipo     text    NOT NULL,
    itens    jsonb   NOT NULL DEFAULT '[]',
    meta     integer NOT NULL DEFAULT 0,
    PRIMARY KEY (plano_id, data)
);

CREATE TABLE anotacoes (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id      uuid        NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data          date,
    disciplina_id uuid        REFERENCES disciplinas (id) ON DELETE SET NULL,
    texto         text        NOT NULL,
    resolvido     boolean     NOT NULL DEFAULT false,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX registros_dia_plano_id_idx ON registros_dia (plano_id);
CREATE INDEX anotacoes_plano_id_idx     ON anotacoes (plano_id);
