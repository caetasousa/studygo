DROP INDEX IF EXISTS anotacoes_disciplina_idx;
DROP INDEX IF EXISTS anotacoes_revisao_idx;

ALTER TABLE anotacoes
    DROP COLUMN tema,
    DROP COLUMN origem,
    DROP COLUMN url,
    DROP COLUMN proxima_revisao;
