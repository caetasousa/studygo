-- As imagens dos mapas mentais: o fluxo de um BPMN, um diagrama da aula. O
-- outline cita a imagem pelo nome num item (`![legenda](arquivo.png)`) e o
-- arquivo chega à parte, pela página do mapa. Fica no banco, e não em disco,
-- porque é da conta como o mapa: vai junto no backup e some com ele.
--
-- O nome é a chave dentro do mapa: enviar de novo o mesmo nome troca a imagem.
-- Reimportar o mapa troca os itens e mantém as imagens, que estão presas ao
-- mapa, não aos itens.
CREATE TABLE mapas_imagens (
    mapa_id    uuid        NOT NULL REFERENCES mapas (id) ON DELETE CASCADE,
    nome       text        NOT NULL CHECK (nome ~ '^[a-z0-9][a-z0-9-]*\.(png|jpg|jpeg|webp)$'),
    tipo       text        NOT NULL CHECK (tipo IN ('image/png', 'image/jpeg', 'image/webp')),
    dados      bytea       NOT NULL CHECK (octet_length(dados) BETWEEN 1 AND 2097152),
    enviada_em timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (mapa_id, nome)
);
