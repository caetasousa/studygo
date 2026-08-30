-- Two blocks a day was the artifact's shape, not a rule: someone who can sit
-- down for five subjects should get five. And a subject you are struggling with
-- needs more weight than the exam alone gives it.
ALTER TABLE planos
    ADD COLUMN blocos_por_dia integer      NOT NULL DEFAULT 2
                              CHECK (blocos_por_dia BETWEEN 1 AND 6),
    ADD COLUMN pct_revisao    numeric(3, 2) NOT NULL DEFAULT 0.16
                              CHECK (pct_revisao >= 0 AND pct_revisao <= 0.4);

-- reforco multiplies the discipline's share: 2 means twice the blocks over the
-- plan and twice the minutes when it comes up.
ALTER TABLE plano_disciplinas
    ADD COLUMN reforco numeric(4, 2) NOT NULL DEFAULT 1
                       CHECK (reforco >= 0.25 AND reforco <= 3);

-- The weekly review rotation, per plan. Until now rev_ciclo was read but never
-- written by anything, so RevCicloPadrao was what every plan silently used —
-- and turning simulados off had to rewrite the cycle's text as a side effect.
CREATE TABLE plano_ciclo (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    ordem    integer NOT NULL,
    titulo   text    NOT NULL,
    questoes integer NOT NULL DEFAULT 0,
    PRIMARY KEY (plano_id, ordem)
);
