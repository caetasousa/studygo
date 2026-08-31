-- A study record must outlive the plan that scheduled it.
--
-- registros_bloco.atividade_id was ON DELETE CASCADE, and ReplaceAtividades
-- rewrites the whole activity layout with a DELETE + INSERT on every move. So
-- rearranging the schedule silently erased what the student had recorded —
-- hours, questions, the completion itself.
--
-- The activity is the plan; the record is history. Losing the link is
-- acceptable when a plan is rebuilt, losing the record is not: ON DELETE SET
-- NULL keeps the row, which then falls back to its (data, disciplina) key just
-- like the records written before activities existed.
ALTER TABLE registros_bloco
    DROP CONSTRAINT registros_bloco_atividade_id_fkey;

ALTER TABLE registros_bloco
    ADD CONSTRAINT registros_bloco_atividade_id_fkey
    FOREIGN KEY (atividade_id) REFERENCES atividades (id) ON DELETE SET NULL;
