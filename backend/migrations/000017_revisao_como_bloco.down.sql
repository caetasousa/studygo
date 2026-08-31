-- Irreversible in substance: the queue's contents cannot be reconstructed from
-- what remains. This restores the columns so the schema shape matches, with the
-- defaults the code used to ship.
ALTER TABLE planos
    ADD COLUMN pct_revisao numeric(3, 2) NOT NULL DEFAULT 0.16,
    ADD COLUMN intervalos_revisao integer[] NOT NULL DEFAULT '{1,7,30}',
    ADD COLUMN revisao_por_questoes boolean NOT NULL DEFAULT true,
    ADD COLUMN questoes_por_revisao integer NOT NULL DEFAULT 10;

ALTER TABLE planos DROP COLUMN IF EXISTS minutos_revisao;
