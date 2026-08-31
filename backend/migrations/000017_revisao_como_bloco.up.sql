-- Review becomes a block of the day, and the spaced-review queue goes away.
--
-- The queue (revisoes) scheduled a topic to come back at D-1/D-7/D-30 whether
-- or not it had ever gone wrong. What actually needs revisiting is what you
-- missed — that is the error notebook, derived from the recorded batteries, so
-- the queue was a second, parallel answer to the same question.
--
-- The day's review slice stops being a percentage of the day and becomes a
-- block with a length of its own, beside the content blocks: moving one no
-- longer silently resizes the other.
ALTER TABLE planos
    ADD COLUMN minutos_revisao integer NOT NULL DEFAULT 20;

-- Carry the old proportion over, so an existing plan keeps roughly the day it
-- had instead of jumping to the default.
UPDATE planos
SET minutos_revisao = GREATEST(0, ROUND(horas_dia * 60 * pct_revisao)::integer)
WHERE pct_revisao > 0;

ALTER TABLE planos
    DROP COLUMN pct_revisao,
    DROP COLUMN intervalos_revisao,
    DROP COLUMN revisao_por_questoes,
    DROP COLUMN questoes_por_revisao;

DROP TABLE IF EXISTS revisoes;
