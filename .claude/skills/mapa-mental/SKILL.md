---
name: mapa-mental
description: Monta o mapa mental de uma aula em PDF (ou de um assunto), como outline de texto em conteudo/mapas/<slug>.md, cobrindo todo o conteúdo do material e validado pela importação. Use quando o usuário mandar uma aula, PDF ou resumo e pedir o mapa mental dela.
---

# Mapa mental de uma aula

O mapa é escrito aqui, fora do app, e entra nele pela tela (**Mapas mentais →
Importar mapa**, ou `POST /api/mapas` com `{"texto": "<o arquivo>", "concurso":
"<slug>"}`). O app não gera mapa nenhum. Quem confere o texto é a importação:
o que passa na stack local, passa no servidor.

O formato — título, metadados, itens `- texto` com 2 espaços por nível, marcas e
negrito — está em `conteudo/mapas/README.md`. Leia-o antes de escrever.

## O que o usuário espera

**Que o mapa cubra TODO o conteúdo do material.** Foi o pedido, com ênfase. Um
mapa bonito que esqueceu uma seção falhou. Por isso o trabalho é de cobertura
antes de ser de estilo:

1. **Leia o material inteiro, página por página**, e liste as seções dele: a
   teoria, os quadros "Saiba mais", os mapas mentais e o resumo que a própria
   aula traz, as tabelas, as questões comentadas e os gabaritos. Cada seção
   vira um ramo ou um pedaço de ramo.
2. **A fonte é o texto do PDF**, nunca a memória. Não acrescente conteúdo que a
   aula não traz, nem "corrija" a aula. Se ela se contradiz (o resumo diz uma
   coisa e a teoria outra), registre as duas e diga onde cada uma está ("No
   resumo da aula: …"). Se ela errou, isso é para o usuário saber, não para o
   mapa esconder.
3. **Confira no fim, ida e volta**: releia o PDF contra o mapa e procure o que
   ficou de fora — números, siglas, exemplos, exceções, o "cai muito". O que
   estava num quadro colorido costuma ser justamente o que cai em prova.

## Estrutura

- **Ramos principais** seguem as seções do material, na ordem dele (introdução,
  histórico, definições, cada modelo, cada grupo de práticas…). Uns 8 a 10; mais
  que isso, agrupe.
- **Um item é uma ideia curta** (até uns 200 caracteres; o limite duro é 500).
  Frase longa vira um item com filhos. O mapa é para ler de relance.
- Fundo até 6 níveis, no geral. Um ramo com dezenas de filhos diretos pede um
  nível de agrupamento no meio.
- Termos-chave em `**negrito**`, sempre o termo, não a frase inteira.
- **Marcas** (uma por item, no começo): `[def]` definição que a banca cobra pelo
  enunciado; `[pegadinha]` o que a banca inverte, misturando dois conceitos;
  `[cai]` o que a aula avisa que cai ("gravem isso", "aparece toda hora");
  `[ex]` exemplo; `[questao]` questão de prova e o que ela ensina.
- Questão comentada vai **junto do conceito que ela cobra**, com o gabarito e o
  motivo, e o índice de todas as questões da aula entra também num ramo à parte.

## O que todo mapa de aula leva, além da teoria

- **Não confunda**: um item por par de conceitos que a aula manda distinguir,
  com uma linha para cada lado. É onde a banca pesca.
- **Números e siglas**: as contagens para decorar ("7 princípios, 4 dimensões"),
  as siglas expandidas e os macetes da aula.
- **Questões comentadas**: uma por item, com banca, ano, órgão, gabarito e o que
  a alternativa certa ensina.

## O que nunca vai no arquivo

O repositório é público e o mapa deriva de material pago, de uso pessoal.

- Nada que identifique o comprador do material: nome, CPF ou e-mail que vêm
  como marca d'água no rodapé das páginas.
- Nem o aviso de pirataria, nem propaganda da plataforma.
- **O mapa não vai para o git.** `conteudo/mapas/*` está no `.gitignore`; só o
  README é versionado. Não force a inclusão nem copie o texto para outro lugar
  do repositório (testes usam `e2e/fixtures/mapa-exemplo.md`, sintético).

## Metadados

- `slug`: curto e estável (`itil-4`); importar de novo o mesmo slug troca o
  conteúdo e mantém os vínculos.
- `fonte`: a aula, o professor, o curso. Sem dado pessoal.
- `materia`: o nome da matéria como o edital a chama; `reconhecer`: termos que
  um tópico do edital cita ("ITIL"). Com o concurso aberto na tela, a
  importação vincula o mapa às matérias que casam, e o cronograma passa a
  oferecê-lo nelas.

## Fechar

1. Valide o texto importando-o na stack local (`make up` e **Mapas mentais →
   Importar mapa**): a mensagem lista todos os problemas com a linha de cada um
   — recuo que pula nível, marca desconhecida, item vazio.
2. Confira a contagem ("N ramos, M itens") contra o que o material tem, e abra o
   mapa na largura de um celular (é onde ele mais é lido): o ramo mais fundo
   tem de ler bem, e o filtro tem de achar os termos que a aula destaca.
3. Diga ao usuário, no fim: onde o arquivo está, quantos ramos e itens tem, o
   que a aula traz de contraditório e, se ficou algo de fora de propósito, o
   quê. Quem estuda importa o mesmo arquivo no servidor, pela tela.
