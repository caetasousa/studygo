-- Devolve a estrutura. Os links gravados não voltam: quem os apagou foi o UP, e
-- o runner não executa os .down automaticamente.

ALTER TABLE disciplinas DROP COLUMN IF EXISTS notebook_url;
