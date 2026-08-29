-- Per-discipline records inside a day. Until now a day carried a single
-- questoes/acertos pair, and CalcularStats split it evenly across the day's two
-- blocks — so per-discipline accuracy was an estimate. Keyed by the discipline
-- `codigo` (like reordenacoes.itens), not by disciplina_id, because editing a
-- concurso deletes and re-inserts every disciplina with fresh uuids.

CREATE TABLE registros_bloco (
    plano_id      uuid          NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data          date          NOT NULL,
    disciplina    text          NOT NULL,
    horas         numeric(5, 2),
    questoes      integer,
    acertos       integer,
    nota          text          NOT NULL DEFAULT '',
    atualizado_em timestamptz   NOT NULL DEFAULT now(),
    PRIMARY KEY (plano_id, data, disciplina)
);

CREATE INDEX registros_bloco_plano_id_idx ON registros_bloco (plano_id);
