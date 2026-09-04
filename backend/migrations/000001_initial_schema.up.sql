-- Schema inicial do studygo.
--
-- Uma baseline só: o produto não tem histórico a preservar, então este arquivo
-- descreve o modelo final direto, sem ALTER, sem backfill, sem função e sem
-- trigger. Regra de negócio nenhuma mora aqui — o banco cuida de integridade
-- referencial, unicidade, nulabilidade e constraints estruturais; política,
-- cálculo e fluxo moram no domínio e na aplicação.
--
-- Duas decisões de modelo que valem a leitura:
--
--  1. O CRONOGRAMA É MATERIALIZADO. `atividades` é o cronograma de verdade, não
--     um remendo por cima de um plano gerado em memória: toda atividade de todo
--     dia existe como linha desde a criação do plano. Por isso `registros_
--     atividade.atividade_id` é NOT NULL e tem FK de verdade — não há registro
--     órfão nem chave alternativa por (data, disciplina).
--
--  2. REFERÊNCIA POR ID, NUNCA POR VALOR. Atividades e registros apontam para
--     `disciplinas(id)`. O `codigo` continua sendo o mnemônico que aparece na
--     tela, mas quem identifica a disciplina é a chave primária: renomear uma
--     disciplina não pode desligar o histórico dela.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- ---------------------------------------------------------------- usuários

CREATE TABLE usuarios (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext      NOT NULL UNIQUE,
    nome          text        NOT NULL DEFAULT '',
    senha_hash    text        NOT NULL,
    -- Preferência visual do usuário, não do plano: quem tem dois concursos não
    -- quer dois temas.
    tema_ui       text        NOT NULL DEFAULT 'dark'
                              CHECK (tema_ui IN ('light', 'dark', 'system')),
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expira_em  timestamptz NOT NULL,
    revogado   boolean     NOT NULL DEFAULT false,
    criado_em  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_usuario_idx ON refresh_tokens (usuario_id);

-- ---------------------------------------------------------------- catálogo

CREATE TABLE concursos (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    dono_id          uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    slug             text        NOT NULL UNIQUE,
    nome             text        NOT NULL,
    banca            text        NOT NULL DEFAULT '',
    cargo            text        NOT NULL DEFAULT '',
    emoji            text        NOT NULL DEFAULT '',
    prova_padrao     date        NOT NULL,
    reta_padrao_dias integer     NOT NULL DEFAULT 28 CHECK (reta_padrao_dias >= 7),
    resumo           text        NOT NULL DEFAULT '',
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX concursos_dono_idx ON concursos (dono_id);

CREATE TABLE disciplinas (
    id              uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id     uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    -- Mnemônico exibido no cronograma ("DIRAD"). Único dentro do concurso, mas
    -- não é a identidade: editar o concurso preserva o id de cada disciplina.
    codigo          text    NOT NULL,
    nome            text    NOT NULL,
    bloco           text    NOT NULL CHECK (bloco IN ('esp', 'ger')),
    peso            integer NOT NULL CHECK (peso > 0),
    questoes_padrao integer NOT NULL DEFAULT 0 CHECK (questoes_padrao >= 0),
    ordem           integer NOT NULL CHECK (ordem >= 0),
    caderno_url     text    NOT NULL DEFAULT '',
    UNIQUE (concurso_id, codigo)
);

CREATE INDEX disciplinas_concurso_idx ON disciplinas (concurso_id);

CREATE TABLE temas (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    ordem         integer NOT NULL CHECK (ordem >= 0),
    texto         text    NOT NULL,
    UNIQUE (disciplina_id, ordem)
);

CREATE TABLE fontes (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    disciplina_id uuid    NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    ordem         integer NOT NULL CHECK (ordem >= 0),
    titulo        text    NOT NULL,
    url           text    NOT NULL DEFAULT '',
    tipo          text    NOT NULL DEFAULT 'lei'
                          CHECK (tipo IN ('lei', 'jurisprudencia', 'material', 'link', 'questoes')),
    UNIQUE (disciplina_id, ordem)
);

CREATE TABLE marcos (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    ordem       integer NOT NULL CHECK (ordem >= 0),
    rotulo      integer NOT NULL,
    data_inicio date    NOT NULL,
    data_fim    date,
    titulo      text    NOT NULL,
    exige_acao  boolean NOT NULL DEFAULT false,
    e_prova     boolean NOT NULL DEFAULT false,
    UNIQUE (concurso_id, ordem),
    CONSTRAINT marcos_periodo_valido CHECK (data_fim IS NULL OR data_fim >= data_inicio)
);

CREATE INDEX marcos_concurso_idx ON marcos (concurso_id);

CREATE TABLE conteudo_programatico (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    concurso_id uuid    NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,
    ordem       integer NOT NULL CHECK (ordem >= 0),
    tipo        text    NOT NULL CHECK (tipo IN ('ficha', 'rot', 'h', 'p')),
    texto       text    NOT NULL,
    UNIQUE (concurso_id, ordem)
);

-- ------------------------------------------------------------------ planos

CREATE TABLE planos (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id      uuid          NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    concurso_id     uuid          NOT NULL REFERENCES concursos (id) ON DELETE CASCADE,

    -- Datas e ritmo.
    inicio          date          NOT NULL,
    prova           date          NOT NULL,
    horas_dia       numeric(4, 2) NOT NULL DEFAULT 2 CHECK (horas_dia >= 0),
    dias_estudo     integer[]     NOT NULL DEFAULT '{1,2,3,4,5}',
    dia_revisao     integer       NOT NULL DEFAULT 5 CHECK (dia_revisao BETWEEN 0 AND 6),
    reta_final_dias integer       NOT NULL DEFAULT 28 CHECK (reta_final_dias >= 0),

    -- Método de estudo.
    blocos_por_dia  integer       NOT NULL DEFAULT 2 CHECK (blocos_por_dia BETWEEN 1 AND 6),
    minutos_bloco   integer       NOT NULL DEFAULT 0
                                  CHECK (minutos_bloco = 0 OR minutos_bloco BETWEEN 15 AND 240),
    minutos_revisao integer       NOT NULL DEFAULT 20 CHECK (minutos_revisao BETWEEN 0 AND 240),
    revisao_semanal boolean       NOT NULL DEFAULT false,
    simulados       text          NOT NULL DEFAULT 'semanal'
                                  CHECK (simulados IN ('nunca', 'quinzenal', 'semanal')),
    discursiva      boolean       NOT NULL DEFAULT true,
    pct_questoes    numeric(3, 2) NOT NULL DEFAULT 0.50
                                  CHECK (pct_questoes BETWEEN 0.1 AND 0.9),
    limiar_fraco    integer       NOT NULL DEFAULT 70 CHECK (limiar_fraco BETWEEN 1 AND 100),

    criado_em       timestamptz   NOT NULL DEFAULT now(),
    atualizado_em   timestamptz   NOT NULL DEFAULT now(),

    UNIQUE (usuario_id, concurso_id),
    CONSTRAINT planos_periodo_valido CHECK (prova >= inicio)
);

CREATE INDEX planos_usuario_idx ON planos (usuario_id);

-- Ajustes do usuário por disciplina, dentro de um plano.
CREATE TABLE plano_disciplinas (
    plano_id      uuid          NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    disciplina_id uuid          NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    questoes      integer       NOT NULL DEFAULT 0 CHECK (questoes >= 0),
    modo          text          NOT NULL DEFAULT 'completo'
                                CHECK (modo IN ('completo', 'questoes', 'teoria')),
    reforco       numeric(4, 2) NOT NULL DEFAULT 1 CHECK (reforco BETWEEN 0.25 AND 3),
    PRIMARY KEY (plano_id, disciplina_id)
);

-- Rotação da revisão semanal, quando o plano a usa.
CREATE TABLE plano_ciclo (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    ordem    integer NOT NULL CHECK (ordem >= 0),
    titulo   text    NOT NULL,
    questoes integer NOT NULL DEFAULT 0 CHECK (questoes >= 0),
    PRIMARY KEY (plano_id, ordem)
);

-- -------------------------------------------------------------- cronograma

-- O cronograma materializado: uma linha por atividade agendada, incluindo os
-- dias fixos (simulado, discursiva, véspera, revisão semanal), que são
-- atividades de dia inteiro e por isso não apontam para disciplina nenhuma.
CREATE TABLE atividades (
    id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id      uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,

    data          date    NOT NULL,
    posicao       integer NOT NULL CHECK (posicao >= 0),

    -- NULL nas atividades de dia inteiro; obrigatório nas de conteúdo.
    disciplina_id uuid    REFERENCES disciplinas (id) ON DELETE CASCADE,
    tema          text    NOT NULL DEFAULT '',
    passada       integer NOT NULL DEFAULT 1 CHECK (passada BETWEEN 1 AND 9),
    tipo          text    NOT NULL DEFAULT 'conteudo'
                          CHECK (tipo IN ('conteudo', 'revisao', 'questoes', 'simulado', 'discursiva', 'vespera')),

    -- 0 = usar a duração padrão do bloco no plano.
    duracao_min   integer NOT NULL DEFAULT 0 CHECK (duracao_min >= 0),

    -- Marca a atividade que o usuário moveu à mão, para que um replanejamento
    -- do futuro não a arraste de volta.
    movida        boolean NOT NULL DEFAULT false,

    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),

    -- Uma atividade de conteúdo sem disciplina não teria o que estudar.
    CONSTRAINT atividades_conteudo_tem_disciplina
        CHECK ((tipo IN ('conteudo', 'questoes')) = (disciplina_id IS NOT NULL))
);

-- Duas atividades do mesmo dia nunca ocupam a mesma posição. DEFERRABLE porque
-- mover uma matéria renumera o dia inteiro dentro de uma transação, passando
-- por estados intermediários que colidiriam.
ALTER TABLE atividades
    ADD CONSTRAINT atividades_plano_data_posicao_key
    UNIQUE (plano_id, data, posicao) DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX atividades_plano_data_idx ON atividades (plano_id, data);
CREATE INDEX atividades_disciplina_idx ON atividades (disciplina_id);

-- --------------------------------------------------------------- registros

-- O que o estudante fez, por ATIVIDADE. Um dia que agenda a mesma disciplina
-- duas vezes tem dois registros independentes, e a conclusão do dia é derivada
-- daqui — nunca informada pelo cliente.
--
-- RESTRICT, não CASCADE: apagar uma atividade já estudada apagaria história. O
-- domínio impede remover atividade concluída; a constraint garante que um bug
-- não faça isso pelas costas.
CREATE TABLE registros_atividade (
    id            uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    atividade_id  uuid          NOT NULL UNIQUE REFERENCES atividades (id) ON DELETE RESTRICT,
    horas         numeric(5, 2) CHECK (horas IS NULL OR horas >= 0),
    questoes      integer       CHECK (questoes IS NULL OR questoes >= 0),
    acertos       integer       CHECK (acertos IS NULL OR acertos >= 0),
    nota          text          NOT NULL DEFAULT '',
    concluido     boolean       NOT NULL DEFAULT false,
    atualizado_em timestamptz   NOT NULL DEFAULT now(),

    CONSTRAINT registros_acertos_cabem_nas_questoes
        CHECK (acertos IS NULL OR questoes IS NULL OR acertos <= questoes)
);

-- O que pertence ao DIA e não a uma atividade: a anotação livre e o resultado
-- da cauda de revisão (que o motor deriva da fila, e por isso não é uma
-- atividade endereçável). Horas, questões, acertos e conclusão do dia NÃO
-- moram aqui: são somados a partir de registros_atividade.
CREATE TABLE registros_dia (
    plano_id         uuid        NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data             date        NOT NULL,
    nota             text        NOT NULL DEFAULT '',
    revisao_questoes integer     CHECK (revisao_questoes IS NULL OR revisao_questoes >= 0),
    revisao_acertos  integer     CHECK (revisao_acertos IS NULL OR revisao_acertos >= 0),
    atualizado_em    timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (plano_id, data),
    CONSTRAINT registros_dia_acertos_cabem_nas_questoes
        CHECK (revisao_acertos IS NULL OR revisao_questoes IS NULL
               OR revisao_acertos <= revisao_questoes)
);

CREATE TABLE marco_checks (
    plano_id uuid    NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    marco_id uuid    NOT NULL REFERENCES marcos (id) ON DELETE CASCADE,
    cumprido boolean NOT NULL DEFAULT false,
    PRIMARY KEY (plano_id, marco_id)
);

-- Caderno de erros: anotação livre do estudante, e o que o app anotou sozinho
-- a partir de um resultado ruim.
CREATE TABLE anotacoes (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    plano_id        uuid        NOT NULL REFERENCES planos (id) ON DELETE CASCADE,
    data            date,
    disciplina_id   uuid        REFERENCES disciplinas (id) ON DELETE SET NULL,
    tema            text        NOT NULL DEFAULT '',
    texto           text        NOT NULL,
    origem          text        NOT NULL DEFAULT 'manual'
                                CHECK (origem IN ('manual', 'revisao', 'tec', 'simulado')),
    url             text        NOT NULL DEFAULT '',
    proxima_revisao date,
    resolvido       boolean     NOT NULL DEFAULT false,
    criado_em       timestamptz NOT NULL DEFAULT now(),
    atualizado_em   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX anotacoes_plano_idx      ON anotacoes (plano_id);
CREATE INDEX anotacoes_disciplina_idx ON anotacoes (plano_id, disciplina_id);
