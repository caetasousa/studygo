DROP TABLE IF EXISTS fontes;
DROP INDEX IF EXISTS concursos_owner_idx;
ALTER TABLE concursos DROP COLUMN IF EXISTS owner_user_id;
