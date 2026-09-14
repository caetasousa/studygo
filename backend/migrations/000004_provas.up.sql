-- Catálogo de provas anteriores.
--
-- Duas famílias de tabela, com leitores diferentes:
--
--  1. IMPORTAÇÃO é trabalho de curadoria: a fila que o worker consome, o
--     rascunho revisável e os arquivos que ele referencia. Nada disso aparece
--     para quem só consulta o catálogo.
--  2. PUBLICAÇÃO é o que o catálogo mostra. Cada publicação é uma revisão
--     imutável; publicar de novo cria a próxima, e a anterior continua gravada.
--
-- Questão e texto de apoio são guardados uma vez só, pelo conteúdo: as questões
-- de Conhecimentos Gerais caem iguais em todos os cargos do mesmo concurso, e
-- uma revisão nova repete quase tudo da anterior. A prova aponta para eles e
-- guarda só o que é dela — número, matéria, resposta, onde está no PDF.
--
-- Os PDFs e recortes ficam no volume `provas_data`, não aqui. O banco guarda só
-- QUEM referencia cada arquivo — é isso que decide quem pode baixá-lo e o que a
-- limpeza pode apagar.

CREATE TABLE provas (
    id           uuid        PRIMARY KEY,
    -- A revisão que o catálogo mostra; o histórico fica em provas_revisoes.
    revisao      integer     NOT NULL DEFAULT 0,
    visivel      boolean     NOT NULL DEFAULT true,
    publicado_em timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provas_importacoes (
    id               uuid        PRIMARY KEY,
    criador          uuid        NOT NULL REFERENCES usuarios(id),
    -- Hash dos PDFs enviados; vazio quando a importação não os segura.
    hash             text        NOT NULL DEFAULT '',
    documento        uuid        NOT NULL,
    gabarito_arquivo text        NOT NULL DEFAULT '',
    estado           text        NOT NULL
                                 CHECK (estado IN ('na_fila', 'processando', 'em_revisao',
                                                   'falhou', 'publicada', 'cancelada')),
    -- Controle de concorrência otimista: toda escrita incrementa, e quem
    -- grava com a versão errada perde.
    versao           integer     NOT NULL DEFAULT 1,
    etapa            integer     NOT NULL DEFAULT 0,
    falhas           integer     NOT NULL DEFAULT 0,
    chamadas         integer     NOT NULL DEFAULT 0,
    processado_ms    bigint      NOT NULL DEFAULT 0,
    -- Reserva do worker. Um resultado só é aceito se vier da tentativa que
    -- ainda detém a reserva.
    tentativa        text        NOT NULL DEFAULT '',
    reserva_ate      timestamptz,
    disponivel_em    timestamptz NOT NULL DEFAULT now(),
    erro             text        NOT NULL DEFAULT '',
    regioes          jsonb       NOT NULL DEFAULT '[]',
    rascunho         jsonb       NOT NULL DEFAULT '{}',
    prova_id         uuid        REFERENCES provas(id),
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now()
);

-- Duas importações ativas nunca têm os mesmos PDFs. Hash vazio é "sem hash":
-- quem decide quando uma importação solta o hash é o domínio.
CREATE UNIQUE INDEX provas_importacoes_hash
    ON provas_importacoes (hash)
    WHERE hash <> '';

CREATE INDEX provas_importacoes_fila
    ON provas_importacoes (disponivel_em, criado_em)
    WHERE estado IN ('na_fila', 'processando');

-- O que cada etapa produziu e quanto levou. O rascunho acumulado mora na
-- importação; aqui fica o resultado isolado, para auditar uma extração ruim sem
-- chamar o Gemini de novo — até a publicação, quando as questões passam a morar
-- na revisão.
CREATE TABLE provas_etapas (
    importacao_id uuid        NOT NULL REFERENCES provas_importacoes(id) ON DELETE CASCADE,
    etapa         integer     NOT NULL,
    resultado     jsonb       NOT NULL,
    duracao_ms    bigint      NOT NULL DEFAULT 0,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (importacao_id, etapa)
);

CREATE TABLE provas_arquivos (
    id        uuid        PRIMARY KEY,
    extensao  text        NOT NULL CHECK (extensao IN ('pdf', 'png')),
    criado_em timestamptz NOT NULL DEFAULT now()
);

-- Um arquivo pode servir a várias importações: a revisão de uma prova publicada
-- herda o PDF original e os recortes da importação que a gerou.
CREATE TABLE provas_importacao_arquivos (
    importacao_id uuid NOT NULL REFERENCES provas_importacoes(id) ON DELETE CASCADE,
    arquivo_id    uuid NOT NULL REFERENCES provas_arquivos(id),
    PRIMARY KEY (importacao_id, arquivo_id)
);

-- A identificação da prova e o gabarito daquela revisão; as questões e os
-- textos ficam nas tabelas abaixo.
CREATE TABLE provas_revisoes (
    prova_id      uuid        NOT NULL REFERENCES provas(id),
    revisao       integer     NOT NULL,
    importacao_id uuid        NOT NULL REFERENCES provas_importacoes(id),
    conteudo      jsonb       NOT NULL,
    publicado_por uuid        NOT NULL REFERENCES usuarios(id),
    publicado_em  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (prova_id, revisao)
);

-- O conteúdo de uma questão — texto, alternativas, figuras —, uma linha por
-- conteúdo diferente. A impressão identifica o conteúdo (quem a calcula é o
-- domínio): o mesmo conteúdo publicado de novo cai na mesma linha.
CREATE TABLE provas_questoes_conteudo (
    id        uuid        PRIMARY KEY,
    impressao text        NOT NULL UNIQUE,
    conteudo  jsonb       NOT NULL,
    criado_em timestamptz NOT NULL DEFAULT now()
);

-- Uma linha por questão publicada: é o que garante número único por revisão e
-- o que a consulta por disciplina filtra. `lugar` é o que a questão é nesta
-- prova — resposta do gabarito, onde está no PDF.
CREATE TABLE provas_questoes (
    prova_id    uuid    NOT NULL,
    revisao     integer NOT NULL,
    numero      integer NOT NULL CHECK (numero > 0),
    disciplina  text    NOT NULL DEFAULT '',
    conteudo_id uuid    NOT NULL REFERENCES provas_questoes_conteudo (id),
    lugar       jsonb   NOT NULL,
    PRIMARY KEY (prova_id, revisao, numero),
    FOREIGN KEY (prova_id, revisao) REFERENCES provas_revisoes (prova_id, revisao)
);

CREATE INDEX provas_questoes_conteudo_id ON provas_questoes (conteudo_id);

CREATE TABLE provas_apoios_conteudo (
    id        uuid        PRIMARY KEY,
    impressao text        NOT NULL UNIQUE,
    conteudo  jsonb       NOT NULL,
    criado_em timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provas_apoios (
    prova_id      uuid    NOT NULL,
    revisao       integer NOT NULL,
    identificador text    NOT NULL,
    ordem         integer NOT NULL,
    conteudo_id   uuid    NOT NULL REFERENCES provas_apoios_conteudo (id),
    lugar         jsonb   NOT NULL,
    PRIMARY KEY (prova_id, revisao, identificador),
    FOREIGN KEY (prova_id, revisao) REFERENCES provas_revisoes (prova_id, revisao)
);

CREATE INDEX provas_apoios_conteudo_id ON provas_apoios (conteudo_id);
