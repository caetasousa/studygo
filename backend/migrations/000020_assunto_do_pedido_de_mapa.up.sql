-- O pedido de mapa sem aula: o assunto que quem estuda escreve na tela (o
-- tópico do edital que nenhuma aula cobre). O processador faz o mapa pelas
-- fontes oficiais, sem PDF. Vazio é o pedido de PDF, como sempre foi.
ALTER TABLE mapas_pedidos
    ADD COLUMN assunto text NOT NULL DEFAULT '' CHECK (length(assunto) <= 2000);
