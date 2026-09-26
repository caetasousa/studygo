-- Devolve a estrutura. Os recortes gravados não voltam: o runner não executa
-- os .down automaticamente.

ALTER TABLE leis_versoes DROP COLUMN IF EXISTS recorte;
