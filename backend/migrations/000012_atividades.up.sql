-- Individually addressable schedule activities.
--
-- Until now the only manual override was `reordenacoes`: one row per DATE,
-- holding that day's whole item list as a JSON blob. An item had no id, no
-- date and no position of its own — its order was its index in the array and
-- its date was the row that contained it — so the finest thing a user could
-- move was an entire day (plano.Trocar swapped two dates wholesale).
--
-- This table gives every activity its own identity, date and position, which is
-- what lets one subject move without dragging the rest of the day with it.
--
-- The plan itself stays generated, not stored: a row here is an OVERRIDE of the
-- engine's output for one activity. `origem_dia`/`origem_pos` record where the
-- engine had put it, so a regeneration can tell "the user moved this" from "the
-- engine changed its mind".
CREATE TABLE atividades (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id      uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,

    -- Where the activity sits now. `posicao` orders activities inside a day;
    -- gaps are fine and expected (moves rewrite a day's positions densely).
    data          date    NOT NULL,
    posicao       integer NOT NULL,

    -- What the activity is. `disciplina` is the discipline codigo; it is empty
    -- for whole-day activities (simulado, revisão, véspera) that belong to no
    -- single subject.
    disciplina    text    NOT NULL DEFAULT '',
    tema          text    NOT NULL DEFAULT '',
    passada       integer NOT NULL DEFAULT 1,
    tipo          text    NOT NULL DEFAULT 'conteudo',

    -- Planned duration in minutes. 0 means "use the plan's default block size",
    -- so an untouched activity follows the config instead of freezing a value.
    duracao_min   integer NOT NULL DEFAULT 0,

    -- Where the engine originally placed it, for regeneration merges.
    origem_dia    date,
    origem_pos    integer,

    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT atividades_posicao_nao_negativa CHECK (posicao >= 0),
    CONSTRAINT atividades_duracao_nao_negativa CHECK (duracao_min >= 0),
    CONSTRAINT atividades_passada_valida CHECK (passada BETWEEN 1 AND 9)
);

CREATE INDEX atividades_plano_data_idx ON atividades (plano_id, data);

-- Two activities in the same day may never claim the same slot. Deferrable so a
-- move can renumber a whole day inside one transaction without tripping over
-- itself mid-update.
ALTER TABLE atividades
    ADD CONSTRAINT atividades_plano_data_posicao_key
    UNIQUE (plano_id, data, posicao) DEFERRABLE INITIALLY DEFERRED;

-- Marks a plan as "the user has arranged activities by hand". Until the first
-- move the plan is purely generated and this stays false, so an untouched plan
-- keeps following the engine exactly as before.
ALTER TABLE planos ADD COLUMN atividades_manuais boolean NOT NULL DEFAULT false;
