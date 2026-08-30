-- The block length becomes the input: the config screen sets minutos_bloco and
-- blocos_por_dia, and horas_dia is what falls out of them (plus the review
-- tail). 0 means "no explicit length yet" — horas_dia is used as stored, and the
-- screen shows the length implied by it until the first save solidifies one.
ALTER TABLE planos
    ADD COLUMN minutos_bloco integer NOT NULL DEFAULT 0
                            CHECK (minutos_bloco = 0 OR minutos_bloco BETWEEN 15 AND 240);
