-- Mapas mentais: a árvore de UM assunto, organizada a partir de uma aula
-- (conteudo/mapas/README.md) e importada pela tela.
--
-- Ao contrário da lei, o mapa é de quem o importou: deriva de material de
-- estudo pessoal, e por isso não há catálogo compartilhado.

CREATE TABLE mapas (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id   uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    -- O endereço estável do mapa. Importar de novo o mesmo slug troca o
    -- conteúdo e mantém o id — e, com ele, os vínculos com as matérias.
    slug         text        NOT NULL,
    titulo       text        NOT NULL,
    fonte        text        NOT NULL DEFAULT '',
    -- A matéria como a fonte a chama. Só orienta o vínculo na importação;
    -- quem vale é disciplinas_mapas.
    materia      text        NOT NULL DEFAULT '',
    importado_em timestamptz NOT NULL DEFAULT now(),
    UNIQUE (usuario_id, slug)
);

-- Os itens do mapa em pré-ordem: o pai vem sempre antes dos filhos, e a ordem
-- entre irmãos é a ordem da linha. Os ramos principais não têm pai.
CREATE TABLE mapas_itens (
    mapa_id uuid    NOT NULL REFERENCES mapas (id) ON DELETE CASCADE,
    ordem   integer NOT NULL CHECK (ordem >= 0),
    pai     integer,
    texto   text    NOT NULL CHECK (texto <> ''),
    marca   text    NOT NULL DEFAULT '' CHECK (marca IN ('', 'def', 'pegadinha', 'cai', 'ex', 'questao')),
    PRIMARY KEY (mapa_id, ordem),
    CHECK (pai IS NULL OR pai < ordem),
    FOREIGN KEY (mapa_id, pai) REFERENCES mapas_itens (mapa_id, ordem) ON DELETE CASCADE
);

-- O mapa que a matéria usa. Pela disciplina (id), não pelo código: editar o
-- concurso preserva o id, e o vínculo sobrevive a uma matéria renomeada.
CREATE TABLE disciplinas_mapas (
    disciplina_id uuid NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    mapa_id       uuid NOT NULL REFERENCES mapas (id) ON DELETE CASCADE,
    PRIMARY KEY (disciplina_id, mapa_id)
);

CREATE INDEX disciplinas_mapas_mapa_idx ON disciplinas_mapas (mapa_id);
