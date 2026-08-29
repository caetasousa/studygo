-- The spaced-review queue. Until now "retome os temas de D-1, D-7 e D-30
-- listados ao lado" was a fixed string with nothing listed beside it, and the
-- worker's reminders were computed by counting back positions in the plan's day
-- list — so for someone studying Mon-Fri, "D-7" landed 9 or 10 days back.
--
-- A topic studied on a day enters at etapa 0 and climbs one stage each time it
-- is recalled well, so a weak topic comes back sooner and a mastered one leaves.
CREATE TABLE revisoes (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id    uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    disciplina  text    NOT NULL,
    tema        text    NOT NULL,
    origem_data date    NOT NULL,
    etapa       integer NOT NULL DEFAULT 0,
    vence_em    date    NOT NULL,
    feita_em    date,
    questoes    integer,
    acertos     integer,
    criada_em   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plano_id, disciplina, tema, etapa)
);

CREATE INDEX revisoes_fila_idx ON revisoes (plano_id, vence_em) WHERE feita_em IS NULL;
