CREATE TABLE plano_questoes (
    plano_id      uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    questoes      integer NOT NULL,
    PRIMARY KEY (plano_id, disciplina_id)
);

INSERT INTO plano_questoes (plano_id, disciplina_id, questoes)
SELECT pd.plano_id, d.id, pd.questoes
  FROM plano_disciplinas pd
  JOIN planos p      ON p.id = pd.plano_id
  JOIN disciplinas d ON d.concurso_id = p.concurso_id AND d.codigo = pd.disciplina;

DROP TABLE plano_disciplinas;

ALTER TABLE planos
    DROP COLUMN simulados,
    DROP COLUMN discursiva,
    DROP COLUMN intervalos_revisao,
    DROP COLUMN pct_questoes,
    DROP COLUMN revisao_por_questoes,
    DROP COLUMN questoes_por_revisao,
    DROP COLUMN limiar_fraco;
