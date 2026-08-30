DROP TABLE IF EXISTS plano_ciclo;

ALTER TABLE plano_disciplinas DROP COLUMN reforco;

ALTER TABLE planos
    DROP COLUMN blocos_por_dia,
    DROP COLUMN pct_revisao;
