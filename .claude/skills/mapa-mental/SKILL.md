---
name: mapa-mental
description: Monta o mapa mental de uma aula em PDF (ou de um assunto), como outline de texto em conteudo/mapas/<slug>.md, cobrindo todo o conteúdo do material e validado pela importação, e as questões da aula em conteudo/mapas/<slug>.questoes.json. Use quando o usuário mandar uma aula, PDF ou resumo e pedir o mapa mental dela.
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

## Vários PDFs de uma vez

O padrão é o mesmo com um PDF ou com dez — o usuário pediu isso expressamente:

- **Um mapa por aula**, cada um com o seu `slug` e o seu `.questoes.json`. Não
  funda aulas num mapa só, nem quebre uma aula em vários.
- Antes de escrever, leia o tópico do edital que as aulas cobrem e confira que
  a soma dos mapas o cobre inteiro. Dentro de cada aula, vale a cobertura total
  de cima: o recorte pelo edital decide **quais aulas** entram, nunca o que sai
  de dentro de uma aula.
- Assunto que aparece em duas aulas fica no mapa da aula que o trata como tema;
  na outra, um item curto basta. Questão repetida entre aulas entra numa só.
- Valide e importe cada mapa, e feche com um quadro: mapa, ramos, itens,
  questões e bancas.

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
  `[ex]` exemplo. Não use `[questao]`: questão não entra no mapa.

## Figuras

O que a aula ensina pelo desenho — um fluxo de BPMN, um diagrama de camadas —
entra como imagem, num item `![legenda](arquivo.png)` no lugar em que a teoria o
apresenta, e com o que ele mostra também escrito em itens (a legenda e o texto
são o que o filtro acha; a imagem, não). Formato e envio em
`conteudo/mapas/README.md`.

- Recorte a figura do PDF com PyMuPDF (`page.get_pixmap(clip=…, dpi=150)`),
  só o desenho: **nunca** o rodapé, onde vêm o nome e o CPF do comprador, nem o
  cabeçalho com a marca da plataforma. Abra cada PNG e confira antes de enviar.
- Salve em `conteudo/mapas/<slug>/`, fora do git, com nomes curtos e estáveis
  (`gateway-exclusivo.png`): reenviar o mesmo nome troca a imagem.
- Imagem decorativa (capa, foto do professor, ícone) não entra.

## O que todo mapa de aula leva, além da teoria

- **Números e nomes**: as contagens para decorar ("7 princípios, 4 dimensões"),
  os nomes que a banca cobra e os macetes da aula.

**Não** crie o ramo "Não confunda" nem a lista "Siglas" — o usuário pediu que
saíssem de toda importação (03/10/2026). O que a aula manda distinguir fica no
próprio ramo do assunto, como `[pegadinha]`; a sigla é expandida onde a teoria
a apresenta, no item dela.

## As questões da aula

Vão em `conteudo/mapas/<slug>.questoes.json` (formato em
`conteudo/mapas/README.md`), **não no mapa**: o usuário pediu que fossem como as
da lei, resolvidas na página, com o gabarito só depois da resposta.

- **Todas as questões do material**: as comentadas do fim e as que aparecem no
  meio da teoria ("Hora de praticar", exemplos de banca). Quando uma da teoria é
  a mesma de uma das comentadas, no mesmo formato, entra uma vez só.
- Texto **literal** do PDF: enunciado, alternativas e comentário do professor
  (com a análise de cada alternativa, uma por linha, começando pela letra —
  `a) Errada. …` —, que a tela mostra debaixo de cada alternativa; veja
  `conteudo/mapas/README.md`). O gabarito é o da aula; se
  o comentário e a linha "Gabarito" divergirem, pare e diga ao usuário.
- `origem` com banca, ano, cargo e órgão, sem dado do comprador, e **a banca
  primeiro**, separada por ` · ` (`FCC · TRT 15 · 2018`): a página agrupa as
  questões por ela. Escreva a banca sempre do mesmo jeito no arquivo (não ora
  `CEBRASPE`, ora `CEBRASPE (CESPE)`), senão o grupo se divide em dois.
- **Confira cada gabarito contra a tabela "Gabarito" da aula**: o script que
  extrai erra em quebra de linha ("Letra\nE") e em origem com parêntese aberto.
  Divergência zero antes de importar.
- Questão que depende de figura leva a figura transcrita entre colchetes. Erro
  da fonte (alternativas idênticas, gabarito contra a teoria) não se corrige:
  marque entre colchetes no próprio texto e conte ao usuário.
- `ramo` é o ramo principal do mapa que trata do assunto cobrado.
- Extrair do texto do PDF (PyMuPDF, no venv do edital-processor) com um script
  no scratchpad é mais fiel que redigitar: o texto das questões é longo.
  Tire o cabeçalho e o rodapé de cada página antes.

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
  oferecê-lo nelas. Confira com quais casou: `materia` casa por palavra e pode
  pegar outra matéria de nome parecido ("Engenharia de Software" também casa
  com "Engenharia de Software Assistida por IA"). Nesse caso, omita `materia` e
  vincule só por `reconhecer`, com o termo que o tópico do edital cita.
- **Os tópicos do mapa.** O cronograma mostra o mapa só nos tópicos que ele
  cobre (pedido de 05/10/2026: "marque os assuntos que já possuem mapa mental;
  os relacionados você pode agrupar"). A importação marca os que citam um termo
  de `reconhecer`; confira na página do mapa (**Tópicos**) e marque também os
  tópicos relacionados que a aula cobre de fato — inclusive em outra matéria
  (o mapa de PLN serve aos tópicos de prompts e RAG da ENGIA). Um tópico só
  entra se o mapa tiver ramo sobre ele: a marca é promessa de que o assunto
  está lá. Matéria inteira só quando o mapa cobre todos os tópicos dela.

## Fechar

1. Valide o texto importando-o na stack local (`make up` e **Mapas mentais →
   Importar mapa**): a mensagem lista todos os problemas com a linha de cada um
   — recuo que pula nível, marca desconhecida, item vazio. As questões, em
   **Manter este mapa → Importar questões**: a mensagem diz a questão e o
   problema (ramo que o mapa não tem, gabarito fora das alternativas…).
2. Confira a contagem ("N ramos, M itens") contra o que o material tem, e abra o
   mapa na largura de um celular (é onde ele mais é lido): o ramo mais fundo
   tem de ler bem, e o filtro tem de achar os termos que a aula destaca.
3. Diga ao usuário, no fim: onde o arquivo está, quantos ramos e itens tem, o
   que a aula traz de contraditório e, se ficou algo de fora de propósito, o
   quê. Quem estuda importa o mesmo arquivo no servidor, pela tela.
