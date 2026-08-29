-- The error notebook was free text with an optional date and discipline, and
-- `resolvido` was stored without ever scheduling anything. It now carries the
-- topic and where the error came from, and joins the review queue.
ALTER TABLE anotacoes
    ADD COLUMN tema            text NOT NULL DEFAULT '',
    ADD COLUMN origem          text NOT NULL DEFAULT 'manual'
                               CHECK (origem IN ('manual', 'revisao', 'tec', 'simulado')),
    ADD COLUMN url             text NOT NULL DEFAULT '',
    ADD COLUMN proxima_revisao date;

CREATE INDEX anotacoes_revisao_idx    ON anotacoes (plano_id, proxima_revisao);
CREATE INDEX anotacoes_disciplina_idx ON anotacoes (plano_id, disciplina_id);
