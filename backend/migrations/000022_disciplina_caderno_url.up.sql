-- A per-discipline link to an external error notebook (TEC, Qconcursos, a
-- personal doc). The schedule's review block links here so a study session can
-- jump straight to where the student keeps that subject's mistakes.
ALTER TABLE disciplinas ADD COLUMN caderno_url text NOT NULL DEFAULT '';
