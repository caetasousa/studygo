-- The study method becomes the user's, not the app's: simulados and the essay
-- day can be turned off, the spaced-review intervals are editable, and each
-- discipline can be studied by questions only, by theory only, or both.

ALTER TABLE planos
    ADD COLUMN simulados            text         NOT NULL DEFAULT 'semanal'
                                    CHECK (simulados IN ('nunca', 'quinzenal', 'semanal')),
    ADD COLUMN discursiva           boolean      NOT NULL DEFAULT true,
    ADD COLUMN intervalos_revisao   integer[]    NOT NULL DEFAULT '{1,7,30}',
    ADD COLUMN pct_questoes         numeric(3, 2) NOT NULL DEFAULT 0.50,
    ADD COLUMN revisao_por_questoes boolean      NOT NULL DEFAULT true,
    ADD COLUMN questoes_por_revisao integer      NOT NULL DEFAULT 10,
    ADD COLUMN limiar_fraco         integer      NOT NULL DEFAULT 70;

-- plano_disciplinas replaces plano_questoes, keyed by the discipline `codigo`
-- instead of its uuid. Editing a concurso deletes and re-inserts every
-- disciplina with fresh uuids, which cascade-wiped the user's per-discipline
-- question estimates on every save.
CREATE TABLE plano_disciplinas (
    plano_id   uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    disciplina text    NOT NULL,
    questoes   integer NOT NULL,
    modo       text    NOT NULL DEFAULT 'completo'
               CHECK (modo IN ('completo', 'questoes', 'teoria')),
    PRIMARY KEY (plano_id, disciplina)
);

INSERT INTO plano_disciplinas (plano_id, disciplina, questoes)
SELECT pq.plano_id, d.codigo, pq.questoes
  FROM plano_questoes pq
  JOIN disciplinas d ON d.id = pq.disciplina_id;

DROP TABLE plano_questoes;
