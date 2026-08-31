ALTER TABLE registros_bloco
    DROP CONSTRAINT registros_bloco_atividade_id_fkey;

ALTER TABLE registros_bloco
    ADD CONSTRAINT registros_bloco_atividade_id_fkey
    FOREIGN KEY (atividade_id) REFERENCES atividades (id) ON DELETE CASCADE;
