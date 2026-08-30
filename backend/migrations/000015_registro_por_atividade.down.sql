DROP INDEX IF EXISTS registros_bloco_legado_key;
DELETE FROM registros_bloco a USING registros_bloco b
 WHERE a.ctid > b.ctid
   AND a.plano_id = b.plano_id AND a.data = b.data AND a.disciplina = b.disciplina;
ALTER TABLE registros_bloco
    ADD CONSTRAINT registros_bloco_pkey PRIMARY KEY (plano_id, data, disciplina);
DROP INDEX IF EXISTS registros_bloco_atividade_idx;
DROP INDEX IF EXISTS registros_bloco_atividade_key;
ALTER TABLE registros_bloco DROP COLUMN IF EXISTS atividade_id;
