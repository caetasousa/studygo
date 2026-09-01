-- The daily review tail (caudaRevisao) has no way to log what happened in it:
-- how many questions, how many right. It is not an atividade — the engine
-- derives it fresh every time from the rolling review queue (FilaRevisao),
-- never a stored, individually addressable unit like a subject block — so the
-- one thing stable enough to key a record on is the date it closed.
CREATE TABLE registros_revisao (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id uuid NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data date NOT NULL,
    questoes integer,
    acertos integer,
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plano_id, data)
);

CREATE INDEX registros_revisao_plano_id_idx ON registros_revisao (plano_id);
