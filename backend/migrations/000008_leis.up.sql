-- Lei seca interativa: o catálogo de leis, as versões do texto, as questões e
-- as respostas do estudante.
--
-- O catálogo é GLOBAL (não tem dono): a lei é a mesma para todo mundo, e quem
-- a publica é a curadoria (LEIS_CURADORES), por um pacote montado fora do app.
-- A numeração continua em 000008: 000004–000007 foram usadas pelo catálogo de
-- provas, que saiu para o provasGo, e ainda constam em bancos existentes.

CREATE TABLE leis (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text        NOT NULL UNIQUE,
    nome       text        NOT NULL,
    curto      text        NOT NULL,
    fonte      text        NOT NULL DEFAULT '',
    -- Trechos que, achados num tópico do concurso, sugerem a lei para a matéria.
    reconhecer text[]      NOT NULL DEFAULT '{}',
    criada_em  timestamptz NOT NULL DEFAULT now()
);

-- Cada captura é uma versão. A anterior fica: é o texto que o estudante leu
-- quando respondeu, e reimportar a mesma versão não a duplica.
CREATE TABLE leis_versoes (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    lei_id       uuid        NOT NULL REFERENCES leis (id) ON DELETE CASCADE,
    versao       text        NOT NULL,
    ativa        boolean     NOT NULL DEFAULT false,
    importada_em timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lei_id, versao)
);

-- Uma versão ativa por lei, no máximo.
CREATE UNIQUE INDEX leis_versoes_uma_ativa ON leis_versoes (lei_id) WHERE ativa;

-- O texto, dispositivo a dispositivo. A ref ("art71.inc2") é o endereço
-- jurídico e identifica o dispositivo entre versões; o pai é a ref do nó de
-- cima, na mesma versão.
CREATE TABLE leis_dispositivos (
    versao_id  uuid    NOT NULL REFERENCES leis_versoes (id) ON DELETE CASCADE,
    ordem      integer NOT NULL CHECK (ordem >= 0),
    ref        text    NOT NULL,
    pai        text,
    tipo       text    NOT NULL,
    rotulo     text    NOT NULL DEFAULT '',
    nome       text    NOT NULL DEFAULT '',
    texto      text    NOT NULL DEFAULT '',
    notas      text[]  NOT NULL DEFAULT '{}',
    anteriores text[]  NOT NULL DEFAULT '{}',
    revogado   boolean NOT NULL DEFAULT false,
    PRIMARY KEY (versao_id, ordem),
    UNIQUE (versao_id, ref)
);

-- As unidades do recorte do edital sobre as quais há questões.
CREATE TABLE leis_unidades (
    versao_id    uuid    NOT NULL REFERENCES leis_versoes (id) ON DELETE CASCADE,
    ordem        integer NOT NULL CHECK (ordem >= 0),
    ref          text    NOT NULL,
    titulo       text    NOT NULL,
    dispositivos text[]  NOT NULL,
    hash         text    NOT NULL,
    PRIMARY KEY (versao_id, ordem),
    UNIQUE (versao_id, ref)
);

-- As questões são da LEI, não da versão: a chave vem do pacote e é o que
-- mantém o id (e as respostas) quando uma versão nova chega. Questão que sai
-- do pacote é desativada, nunca apagada. Os dispositivos citados são refs, e
-- por isso um array: eles valem contra a versão ativa, qualquer que seja.
CREATE TABLE leis_questoes (
    id           uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    lei_id       uuid    NOT NULL REFERENCES leis (id) ON DELETE CASCADE,
    chave        text    NOT NULL,
    unidade      text    NOT NULL,
    enunciado    text    NOT NULL,
    alternativas text[]  NOT NULL CHECK (cardinality(alternativas) = 5),
    gabarito     text    NOT NULL CHECK (gabarito IN ('A', 'B', 'C', 'D', 'E')),
    comentario   text    NOT NULL,
    trecho       text    NOT NULL,
    dispositivos text[]  NOT NULL,
    assinatura   text    NOT NULL,
    ativa        boolean NOT NULL DEFAULT true,
    UNIQUE (lei_id, chave)
);

-- Cada resposta é uma linha: responder de novo não apaga a anterior, e o
-- progresso lê a mais recente. A questão não pode sumir debaixo das respostas.
CREATE TABLE leis_respostas (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id    uuid        NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    questao_id    uuid        NOT NULL REFERENCES leis_questoes (id) ON DELETE RESTRICT,
    alternativa   text        NOT NULL CHECK (alternativa IN ('A', 'B', 'C', 'D', 'E')),
    acertou       boolean     NOT NULL,
    respondida_em timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX leis_respostas_ultima_idx ON leis_respostas (usuario_id, questao_id, respondida_em DESC);

-- A lei que a matéria cobra. Pela disciplina (id), não pelo código: editar o
-- concurso preserva o id, e o vínculo sobrevive a uma matéria renomeada.
CREATE TABLE disciplinas_leis (
    disciplina_id uuid NOT NULL REFERENCES disciplinas (id) ON DELETE CASCADE,
    lei_id        uuid NOT NULL REFERENCES leis (id) ON DELETE CASCADE,
    PRIMARY KEY (disciplina_id, lei_id)
);

CREATE INDEX disciplinas_leis_lei_idx ON disciplinas_leis (lei_id);
