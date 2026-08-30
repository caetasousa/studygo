-- Disciplina codigos used to be positional (D01..DNN), which is what the
-- schedule shows on every activity chip — so a day read as "D02 · D07" instead
-- of "MATRA · CONEX". New concursos get mnemonics from concurso.Siglas; this
-- rewrites the ones already stored.
--
-- The rule mirrors the Go one: fold accents, drop the connectives and generic
-- openers ("noções de", "de", "e"), then take 3 letters of the first
-- significant word plus 2 of the second (a single word gives 4). Collisions
-- within a concurso get a numeric suffix.
--
-- Codigos are referenced by value in atividades and registros_bloco, so the
-- old->new mapping is captured BEFORE the rename and applied to both.

CREATE OR REPLACE FUNCTION sigla_de(nome text) RETURNS text AS $$
DECLARE
    limpo   text;
    partes  text[];
    palavra text;
    boas    text[] := '{}';
BEGIN
    limpo := lower(translate(
        nome,
        'áàâãäéèêëíìîïóòôõöúùûüçñÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇÑ',
        'aaaaaeeeeiiiiooooouuuucnAAAAAEEEEIIIIOOOOOUUUUCN'
    ));
    limpo := regexp_replace(limpo, '[^a-z0-9 ]', ' ', 'g');
    partes := regexp_split_to_array(trim(limpo), '\s+');

    IF array_length(partes, 1) IS NULL THEN
        RETURN NULL;
    END IF;

    FOREACH palavra IN ARRAY partes LOOP
        CONTINUE WHEN palavra = '' OR palavra IN (
            'de','da','do','das','dos','e','em','a','o','as','os','para','com',
            'no','na','nos','nas','nocoes','nocao','aplicada','aplicado',
            'aplicadas','aplicados','geral','gerais','basica','basicas',
            'introducao'
        );
        boas := boas || palavra;
    END LOOP;

    -- A name made entirely of ignored words still has to yield something.
    IF array_length(boas, 1) IS NULL THEN
        boas := partes;
    END IF;

    IF array_length(boas, 1) IS NULL OR boas[1] = '' THEN
        RETURN NULL;
    END IF;

    IF array_length(boas, 1) = 1 THEN
        RETURN upper(substr(boas[1], 1, 4));
    END IF;

    RETURN upper(substr(boas[1], 1, 3) || substr(boas[2], 1, 2));
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- old -> new, per concurso, with collisions suffixed.
CREATE TEMP TABLE mapa_siglas ON COMMIT DROP AS
WITH cand AS (
    SELECT
        d.id,
        d.concurso_id,
        d.codigo AS antigo,
        sigla_de(d.nome) AS s,
        ROW_NUMBER() OVER (
            PARTITION BY d.concurso_id, sigla_de(d.nome) ORDER BY d.ordem, d.id
        ) AS n
    FROM disciplinas d
    WHERE sigla_de(d.nome) IS NOT NULL AND sigla_de(d.nome) <> ''
)
SELECT
    id,
    concurso_id,
    antigo,
    CASE WHEN n = 1 THEN s ELSE s || n::text END AS novo
FROM cand;

-- Rename the referencing rows first, while `antigo` still matches what they
-- hold. Matching is scoped by concurso through the plan, since codigos are only
-- unique within one.
UPDATE atividades a
SET disciplina = m.novo
FROM mapa_siglas m
JOIN planos p ON p.concurso_id = m.concurso_id
WHERE a.plano_id = p.id
  AND a.disciplina = m.antigo
  AND m.antigo <> m.novo;

UPDATE registros_bloco rb
SET disciplina = m.novo
FROM mapa_siglas m
JOIN planos p ON p.concurso_id = m.concurso_id
WHERE rb.plano_id = p.id
  AND rb.disciplina = m.antigo
  AND m.antigo <> m.novo;

UPDATE disciplinas d
SET codigo = m.novo
FROM mapa_siglas m
WHERE d.id = m.id AND d.codigo <> m.novo;

DROP FUNCTION sigla_de(text);
