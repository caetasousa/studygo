# Classificação das questões — 17/09/2026

Aplicada ao banco do Docker Compose **local**. Não aplicada a staging ou produção.

- 23 provas visíveis, com 1.416 ocorrências de questões publicadas.
- 826 ocorrências sem assunto receberam classificação pelo conteúdo do enunciado; as alternativas foram consultadas nos casos ambíguos.
- 374 ocorrências tiveram a matéria corrigida ou padronizada.
- 909 ocorrências alteradas ao todo (os dois conjuntos acima se sobrepõem).
- Resultado: 25 matérias e nenhuma questão publicada sem assunto.

O [registro CSV](classificacao-2026-09-17.csv) contém prova, revisão, número,
identificador do conteúdo e matéria/assunto antes e depois de cada alteração.
Foram reutilizados assuntos existentes e adicionados assuntos específicos quando
necessário. Exemplos de correções: Excel em Regulação, programação em Engenharia
de Software, direitos humanos em Legislação genérica e o erro “Racocínio”.

Matemática e Raciocínio Lógico passou a ter uma única grafia nas publicações,
com 78 ocorrências distribuídas entre 11 assuntos. Esses números incluem
ocorrências repetidas entre provas; o treino elimina conteúdos idênticos e
considera a disponibilidade de gabarito antes de montar seus totais.

A aplicação ocorreu em uma transação, comparando os valores anteriores e a
revisão atual antes de atualizar. Nenhum enunciado, alternativa, referência de
conteúdo, número ou outro campo de localização foi alterado. A comparação
integral das 1.416 ocorrências antes/depois confirmou essa preservação e a
correspondência das 909 alterações com o CSV. Conteúdos idênticos ficaram com
classificação consistente.

Cópia dos dados anteriores e SQL da aplicação nesta sessão:
`/tmp/studygo-classificacao/`. O CSV é o registro persistente no repositório;
os arquivos em `/tmp` são temporários. As importações publicadas não guardam
mais questões no rascunho; a classificação está em `provas_questoes`, nos campos
`disciplina` e `lugar.Disciplina`/`lugar.Assunto` da revisão vigente.
