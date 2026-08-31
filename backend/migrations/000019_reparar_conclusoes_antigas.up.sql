-- Repairs completions recorded before finishing a topic could move it.
--
-- Until now the client recorded a completion on the day the topic was PLANNED
-- for, not the day it was studied, and without an activity id. So a subject
-- finished ahead of schedule stayed where the plan had put it, marked done on a
-- future date — the schedule claiming the study is yet to happen.
--
-- Two repairs, both conservative:
--
--  1. Link each id-less record to the activity of that discipline scheduled for
--     the same day. That is unambiguous whenever the discipline appears once
--     that day, which is the ordinary case; where it appears twice the row is
--     left alone rather than guessed at.
--
--  2. Move an activity whose record says it was completed on a day at or before
--     today onto that day. Records dated in the future are left as they are:
--     they describe a plan, not something that happened.
WITH unico AS (
    SELECT a.plano_id, a.data, a.disciplina, min(a.id::text)::uuid AS atividade_id
    FROM atividades a
    WHERE a.disciplina <> ''
    GROUP BY a.plano_id, a.data, a.disciplina
    HAVING count(*) = 1
)
UPDATE registros_bloco rb
SET atividade_id = u.atividade_id
FROM unico u
WHERE rb.atividade_id IS NULL
  AND rb.plano_id = u.plano_id
  AND rb.data = u.data
  AND rb.disciplina = u.disciplina;

-- Bring the finished activities onto the day they were recorded on. Positions
-- are appended after whatever the target day already holds, so nothing collides
-- with the deferrable unique index.
WITH alvo AS (
    SELECT rb.atividade_id,
           rb.data AS nova_data,
           a.data  AS antiga_data,
           a.plano_id,
           row_number() OVER (PARTITION BY a.plano_id, rb.data ORDER BY a.posicao) AS ordem
    FROM registros_bloco rb
    JOIN atividades a ON a.id = rb.atividade_id
    WHERE rb.concluido
      AND rb.data <> a.data
      AND rb.data <= CURRENT_DATE
),
base AS (
    SELECT plano_id, data, count(*) AS n
    FROM atividades
    GROUP BY plano_id, data
)
UPDATE atividades a
SET data = alvo.nova_data,
    posicao = COALESCE(base.n, 0) + alvo.ordem - 1
FROM alvo
LEFT JOIN base ON base.plano_id = alvo.plano_id AND base.data = alvo.nova_data
WHERE a.id = alvo.atividade_id;
