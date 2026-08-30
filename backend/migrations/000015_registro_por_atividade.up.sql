-- Records keyed by ACTIVITY, not by (day, discipline).
--
-- registros_bloco is keyed (plano_id, data, disciplina), so a day that schedules
-- the same discipline twice — a second pass, or two topics of one subject —
-- collapses both into a single row: recording one silently overwrote the other,
-- and completing one completed both. The scheduled activity is the real unit of
-- work, so the record follows it.
--
-- The old columns stay for one release: plano_service still reads the
-- (data, disciplina) rows for plans whose activities were never materialised,
-- and the golden stats path aggregates from them.
ALTER TABLE registros_bloco
    ADD COLUMN atividade_id uuid REFERENCES atividades (id) ON DELETE CASCADE;

-- One record per activity. Rows that predate this (atividade_id NULL) keep the
-- old key, so nothing already recorded is lost.
CREATE UNIQUE INDEX registros_bloco_atividade_key
    ON registros_bloco (atividade_id)
    WHERE atividade_id IS NOT NULL;

CREATE INDEX registros_bloco_atividade_idx ON registros_bloco (atividade_id);

-- Adopt existing records into the activity they clearly belong to: same plan,
-- same date, same discipline, and exactly ONE candidate activity. Where a day
-- has two activities of the same discipline the old row is ambiguous, so it is
-- left on the legacy key rather than guessed at.
UPDATE registros_bloco rb
SET atividade_id = uniq.id
FROM (
    SELECT a.plano_id, a.data, a.disciplina, min(a.id::text)::uuid AS id
    FROM atividades a
    WHERE a.disciplina <> ''
    GROUP BY a.plano_id, a.data, a.disciplina
    HAVING count(*) = 1
) uniq
WHERE rb.plano_id = uniq.plano_id
  AND rb.data = uniq.data
  AND rb.disciplina = uniq.disciplina
  AND rb.atividade_id IS NULL;

-- The old primary key (plano_id, data, disciplina) is exactly what made two
-- activities of the same discipline in one day collide. Identity now comes from
-- the activity, so the key must allow repeats of a discipline within a day.
ALTER TABLE registros_bloco DROP CONSTRAINT registros_bloco_pkey;

-- Legacy rows (atividade_id NULL) keep their old uniqueness so nothing recorded
-- before this can be duplicated; activity-keyed rows are already unique through
-- registros_bloco_atividade_key above.
CREATE UNIQUE INDEX registros_bloco_legado_key
    ON registros_bloco (plano_id, data, disciplina)
    WHERE atividade_id IS NULL;
