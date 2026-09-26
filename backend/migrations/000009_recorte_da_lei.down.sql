-- Devolve a estrutura. Os recortes gravados não voltam: o runner não executa
-- os .down automaticamente.

ALTER TABLE disciplinas_leis DROP COLUMN IF EXISTS recorte;
