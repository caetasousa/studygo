-- Os pedidos de mapa: o PDF da aula na fila da conta. O backend o entrega ao
-- edital-processor, que faz o mapa com o Claude Code e o devolve pela porta
-- interna; o backend o importa como a tela importaria. Aqui fica só a fila.
--
-- O PDF é material pago e só fica enquanto serve: o pedido pronto o perde
-- (pdf NULL) e guarda só o nome, o mapa que saiu e o relatório.
CREATE TABLE mapas_pedidos (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id    uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    -- A matéria em que o mapa vai morar. Sem ela, o mapa entra sem vínculo.
    disciplina_id uuid        REFERENCES disciplinas (id) ON DELETE SET NULL,
    arquivo       text        NOT NULL CHECK (length(arquivo) BETWEEN 1 AND 200),
    pdf           bytea       CHECK (pdf IS NULL OR octet_length(pdf) BETWEEN 1 AND 41943040),
    situacao      text        NOT NULL DEFAULT 'na_fila'
                              CHECK (situacao IN ('na_fila', 'processando', 'pronto', 'falhou')),
    -- O slug do mapa que o pedido gerou, quando pronto.
    mapa_slug     text,
    relatorio     text        NOT NULL DEFAULT '' CHECK (length(relatorio) <= 20000),
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX mapas_pedidos_fila ON mapas_pedidos (usuario_id, situacao, criado_em);
