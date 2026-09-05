-- Recria a estrutura. O conteúdo não volta: quem apagou foi o UP, e o runner
-- não executa os .down automaticamente.

CREATE TABLE plano_ciclo (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    ordem    integer NOT NULL CHECK (ordem >= 0),
    titulo   text    NOT NULL,
    questoes integer NOT NULL DEFAULT 0 CHECK (questoes >= 0),

    PRIMARY KEY (plano_id, ordem)
);
