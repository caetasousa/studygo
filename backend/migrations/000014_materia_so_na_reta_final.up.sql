-- A matéria de peso baixo que o estudante prefere ver só perto da prova: fora
-- da fase de aprender, e estudada — não revisada — na reta final.
ALTER TABLE plano_disciplinas ADD COLUMN so_na_reta_final boolean NOT NULL DEFAULT false;
