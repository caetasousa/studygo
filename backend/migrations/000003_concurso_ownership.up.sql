-- Concursos are now registered by users (no shipped seed). owner_user_id stays
-- nullable so a shared/public catalogue (owner IS NULL) can be added later
-- without another migration.

ALTER TABLE concursos
    ADD COLUMN owner_user_id uuid REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX concursos_owner_idx ON concursos (owner_user_id);

-- Per-discipline study sources — laws, jurisprudence, materials, links. Filled
-- by the edital import and used to build the NotebookLM hand-off dossier.
CREATE TABLE fontes (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    ordem         integer NOT NULL,
    titulo        text    NOT NULL,
    url           text    NOT NULL DEFAULT '',
    tipo          text    NOT NULL DEFAULT 'lei',
    UNIQUE (disciplina_id, ordem)
);

CREATE INDEX fontes_disciplina_id_idx ON fontes (disciplina_id);
