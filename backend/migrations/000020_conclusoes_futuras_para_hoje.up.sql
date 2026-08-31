-- Brings completions dated in the future onto the day they were made.
--
-- The old client recorded a completion on the day the topic was PLANNED for, so
-- a subject finished ahead of schedule produced a record dated in the future —
-- the plan claiming the study is yet to happen, while the student had already
-- done it. 000019 could not repair those: by the data alone they look like
-- legitimate future plans.
--
-- A completed record dated after today is not a plan, though — a plan is not
-- something you have already finished. Those are moved to the current date,
-- along with the activity they belong to.
--
-- Only completions move. Anything not marked done is left exactly as it is.
WITH mover AS (
    SELECT rb.plano_id, rb.data AS antiga, rb.disciplina, rb.atividade_id
    FROM registros_bloco rb
    WHERE rb.concluido AND rb.data > CURRENT_DATE
)
UPDATE atividades a
SET data = CURRENT_DATE,
    posicao = 1000 + a.posicao -- provisional: renumbered below
FROM mover m
WHERE a.id = m.atividade_id;

-- The record follows the activity, keeping (plano_id, data, disciplina) unique:
-- a same-day duplicate of the same discipline would collide with the legacy
-- index, so those are dropped in favour of the row already on the target day.
DELETE FROM registros_bloco rb
USING registros_bloco outro
WHERE rb.concluido
  AND rb.data > CURRENT_DATE
  AND outro.plano_id = rb.plano_id
  AND outro.data = CURRENT_DATE
  AND outro.disciplina = rb.disciplina
  AND outro.atividade_id IS NOT DISTINCT FROM rb.atividade_id;

UPDATE registros_bloco
SET data = CURRENT_DATE
WHERE concluido AND data > CURRENT_DATE;

-- Positions dense again on every day the move touched.
WITH ordenado AS (
    SELECT id, row_number() OVER (PARTITION BY plano_id, data ORDER BY posicao, criado_em) - 1 AS nova
    FROM atividades
)
UPDATE atividades a
SET posicao = o.nova
FROM ordenado o
WHERE a.id = o.id AND a.posicao <> o.nova;
