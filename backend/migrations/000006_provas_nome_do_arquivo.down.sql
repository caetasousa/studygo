-- Devolve a estrutura. Os nomes gravados não voltam: o runner não executa os
-- .down automaticamente.

ALTER TABLE provas_importacoes
    DROP COLUMN IF EXISTS nome_gabarito,
    DROP COLUMN IF EXISTS nome_documento;
