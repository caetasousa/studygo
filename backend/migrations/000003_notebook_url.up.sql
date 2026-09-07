-- Link do NotebookLM por matéria.
--
-- O dossiê já existia e termina mandando o estudante criar um notebook no
-- NotebookLM com aquele conteúdo — e não havia onde guardar o endereço do
-- notebook criado. Toda vez era procurar de novo.
--
-- Coluna nova em vez de reaproveitar `fontes`: o notebook é UM por matéria, tem
-- posição fixa na tela e é editado do cronograma, não da lista ordenada de
-- fontes do cadastro. É irmão exato de caderno_url, e segue as mesmas regras.
--
-- NOT NULL DEFAULT '' pelo mesmo motivo de caderno_url: "sem link" é string
-- vazia, não NULL — o domínio não distingue os dois, e um nulo só criaria um
-- terceiro estado para o código tratar.

ALTER TABLE disciplinas ADD COLUMN notebook_url text NOT NULL DEFAULT '';
