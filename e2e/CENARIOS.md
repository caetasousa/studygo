# Como o studygo pode quebrar

Este catálogo veio ANTES dos testes: cada item é uma maneira concreta de uma
funcionalidade que já existe deixar de funcionar para quem usa. Cada teste em
`testes/` cita o id que cobre (`[A4]`), e o relatório final lista o resultado
por id — um item sem teste é uma lacuna declarada, não um esquecimento.

Os testes rodam contra o stack inteiro (frontend de produção, backend, worker,
Postgres e edital-processor) num projeto compose isolado, sempre a partir de
um banco vazio. Nada aqui toca no banco de quem desenvolve.

## A. Conta e sessão

| id | Como quebra | O que o usuário vê |
|---|---|---|
| A1 | o cadastro não cria a conta ou não leva ao primeiro concurso | fica preso na tela de cadastro, ou cai numa tela vazia |
| A2 | um e-mail já cadastrado é aceito de novo | duas contas com o mesmo e-mail, login ambíguo |
| A3 | senha errada entra, ou não diz nada | acesso indevido, ou o usuário sem saber por que não entrou |
| A4 | recarregar a página derruba a sessão (refresh em cookie) | todo F5 manda para o login |
| A5 | rota interna aberta sem sessão | tela quebrada em vez do login |
| A6 | sair não encerra a sessão no servidor | recarregar depois de sair volta logado |
| A7 | algum token volta a ser gravado no localStorage | credencial exposta a qualquer XSS |

## B. Concurso

| id | Como quebra | O que o usuário vê |
|---|---|---|
| B1 | o cadastro manual não gera o plano | concurso criado e Hoje/Cronograma vazios |
| B2 | o formulário aceita concurso sem disciplina ou sem data | plano inválido, erro 500 |
| B3 | a tag escolhida ("RLM") é ignorada, ou duas matérias ficam com a mesma | chip errado no cronograma, matérias indistinguíveis |
| B4 | renomear a disciplina desliga o histórico (identidade por valor) | o estudo registrado some depois de editar o concurso |
| B5 | os tópicos cadastrados não chegam ao conteúdo programático nem ao dia | ementa vazia, dia sem tema |
| B6 | as datas do edital não aparecem, ou "cumprido" não fica gravado | lembrete de inscrição perdido |
| B7 | dois concursos se misturam, ou trocar de plano não troca a tela | o registro de um aparece no outro |
| B8 | excluir o concurso não pede confirmação, ou cancelar exclui mesmo assim | perda de dados por um clique |
| B9 | a análise do edital sem nenhum cargo trava ou esconde o caminho manual | usuário sem como cadastrar |
| B10 | o assistente do edital perde pelo caminho o que foi lido — cargo, disciplinas, tópicos ou datas | o usuário revisa uma coisa e o plano nasce outra |

## C. Estudo do dia

| id | Como quebra | O que o usuário vê |
|---|---|---|
| C1 | registrar todas as matérias do dia não conclui o dia, ou os números do topo não mudam | progresso parado mesmo estudando |
| C2 | o dia conclui com uma matéria ainda aberta (conclusão informada, não derivada) | dia "feito" pela metade |
| C3 | o registro não fica gravado | recarregar apaga o estudo |
| C4 | adiar o dia não leva as matérias para o próximo dia livre | conteúdo perdido ou duplicado |
| C5 | mover uma matéria não troca a ordem, ou a troca não fica gravada | a ordem volta ao recarregar |
| C6 | uma matéria concluída pode ser movida | histórico reescrito |
| C7 | as estatísticas não batem com o que foi registrado | horas, questões ou acerto errados |
| C8 | o balanceamento não reflete as horas lançadas | painel de "onde estou devendo" mentindo |
| C9 | matéria abaixo de 70% não entra no caderno de erros, ou uma boa entra | erros sem revisão, ou revisão do que já se sabe |
| C10 | o link do caderno de erros colado no registro não vale para a matéria toda | link some no dia seguinte |
| C11 | a revisão do dia não abre com o que há para revisar, ou o resultado dela não fica gravado | o bloco de revisão vira enfeite |
| C12 | marcar na ementa um tópico já estudado não o traz para hoje, deixa a 1ª passada dele ainda agendada adiante, ou não move o progresso | o tópico que já foi estudado reaparece na sequência, e a estatística não conta o estudo |
| C13 | desmarcar o tópico não desfaz a conclusão | um clique errado vira estudo que não houve |
| C14 | marcar um tópico de uma atividade que junta vários (matéria com mais tópicos que vagas) marca todos, ou o tópico volta num replanejamento | só dá para marcar de dois em dois; o estudado reaparece |
| C15 | marcar o tópico num dia que não é de estudo (domingo) deixa a atividade concluída na data futura, sem reorganizar; ou o tópico estudado volta adiante com outra grafia | o tópico fica "feito lá no final" e o cronograma não anda; ou o conteúdo se repete |
| C16 | o tópico marcado como estudado antes da hora aparece como mais uma linha no dia em que foi marcado, ou desmarcar não o devolve | o dia vira um depósito de tópicos; ou o clique errado não tem volta |
| C17 | concluir ou marcar um tópico como estudado não move o balanceamento | o painel de "onde estou" não mostra o que já foi estudado |
| C18 | editar as questões de uma matéria no balanceamento devolve as outras ao número do edital | a retificação do edital digitada matéria por matéria se desfaz sozinha |
| C19 | a matéria com mais tópicos que horários aparece como incompleta no balanceamento, mesmo com todos os tópicos no cronograma (agrupados) | o painel diz que não dá para ver a matéria inteira quando dá |
| C20 | o topo da tela avisa de isenção, recurso, cobertura ou orçamento, e as datas que importam (inscrições, pagamento, prova) se perdem entre eles | a pessoa passa a ignorar os avisos e perde o boleto |
| C21 | depois de desmarcar um tópico estudado antes da hora, o dia fica com conteúdo depois de um antecipado, e a próxima conclusão grava o estudo mas falha ao reorganizar o cronograma (duas atividades na mesma posição do dia) | "erro interno" a cada matéria concluída ou tópico marcado; ao recarregar o estudo está lá, mas o cronograma não andou |

## D. Configurações e dados

| id | Como quebra | O que o usuário vê |
|---|---|---|
| D1 | o tema não é aplicado, ou não fica gravado na conta | volta ao escuro a cada carga |
| D2 | mudar os blocos por dia não refaz o cronograma, ou apaga os registros | plano velho, ou estudo perdido |
| D3 | a planilha exportada não traz o estudo de volta em outra conta | backup que não restaura |
| D4 | limpar registros não pede confirmação, ou não zera o progresso | perda por engano, ou botão sem efeito |
| D5 | restaurar a ordem automática não desfaz a troca manual | ordem manual presa para sempre |
| D6 | o dossiê do NotebookLM sai sem a ementa e as leis cadastradas | fonte inútil para colar |
| D7 | compactar não fecha o vão deixado no cronograma, ou desfaz a ordem manual | dias vazios no meio e conteúdo espremido no fim |
| D8 | reorganizar a partir de uma data mexe no que já foi estudado, ou não refaz o que vem depois | histórico reescrito, ou plano velho |
| D9 | adiar uma matéria para a reta final a deixa na fase de aprender, não refaz o cronograma, não fica gravado, ou apaga o que já foi estudado | a matéria de peso baixo continua tomando os dias de agora; ou o estudo dela some |
| D10 | na reta final a matéria adiada vem como "Revisão dirigida" de algo nunca estudado, deixa tópico de fora ou repete tópico; ou o balanceamento a acusa de incompleta | estuda como revisão o que nunca viu; tópico sem estudo; aviso falso de matéria que não é vista |
| D11 | voltar a matéria para o plano todo não a devolve à fase de aprender; ou adiar todas as matérias é aceito e esvazia a fase de aprender | um clique sem volta; semanas em branco |

## L. Legislação

A lei é capturada pela tela: cola-se o link da fonte oficial, o
`edital-processor` baixa e organiza, e a pessoa revisa a prévia antes de
publicar. As questões são escritas fora do app e entram pela tela, contra o
texto já publicado. Enquanto o app é de teste, qualquer conta logada captura e
importa (decisão de 25/09/2026). No stack de E2E o dublê do processador devolve
uma lei pequena e fixa (`e2e/fixtures/lei-exemplo.json`) para links do
Planalto terminados em `e2e/…`.

| id | Como quebra | O que o usuário vê |
|---|---|---|
| L1 | a lei capturada e publicada não vira uma lei legível, ou chega com o texto diferente do que o processador leu | lei vazia, ou uma palavra da lei trocada |
| L2 | publicar a mesma captura (ou a mesma versão) de novo duplica a lei, os dispositivos ou as questões | a lei aparece duas vezes, questões repetidas |
| L3 | atualizar o texto da lei apaga as questões ou as respostas; ou importar as questões de novo zera as respostas das que não mudaram | o progresso volta a zero a cada atualização |
| L4 | uma conta comum não vê a captura e a importação, ou é recusada | quem testa não consegue publicar uma lei |
| L5 | clicar no artigo não traz as questões que o citam, ou traz as de outro artigo | a questão não bate com o que se está lendo |
| L6 | a resposta não é gravada, ou o gabarito e o trecho não aparecem | responde e não aprende nada |
| L7 | o selo do artigo e o progresso da unidade não refletem as respostas | não se sabe o que falta nem onde errou |
| L8 | o link direto para um dispositivo não abre a lei naquele ponto | o cronograma e o caderno não conseguem apontar para o artigo |
| L9 | o tópico que cita a lei ("nº 16.168") não sugere a lei para a matéria, ou a sugestão confirmada não fica gravada | a lei não aparece no menu Legislação da matéria |
| L10 | a redação anterior ou as notas de redação se misturam ao texto vigente | o estudante decora um texto revogado |
| L11 | uma captura com problema que bloqueia (texto que não confere, artigo perdido) pode ser publicada | lei quebrada no catálogo de todos |
| L12 | um aviso da captura (o Gemini discordou da regra, salto na numeração) é publicado sem a pessoa marcar que revisou, ou não aparece na prévia | a lei entra com um tipo de dispositivo errado que ninguém viu |
| L13 | um link fora das fontes oficiais é aceito, ou o erro não diz o que fazer | o servidor baixa qualquer coisa; ou a pessoa não sabe por que falhou |
| L14 | a publicação de uma lei nova com o nome curto de outra sobrescreve a existente | a Constituição some debaixo de uma lei homônima |
| L15 | importar questões aceita uma questão cujo trecho não está na lei, ou uma unidade escrita para outra redação | questão que a lei não sustenta |
| L16 | a captura que ainda está rodando trava a tela, ou some se a pessoa esperar | ninguém sabe se terminou |
| L17 | a prévia da captura não mostra o que o edital do concurso pede daquela lei, ou mostra o recorte de outro tópico | estuda a Constituição inteira sem saber o que cai |
| L18 | vincular a matéria (pela prévia ou pela sugestão do tópico) não grava o recorte do edital, ou grava o de outra matéria | o recorte some ao recarregar |
| L19 | aberta no concurso, a lei mostra tudo em vez do recorte, ou esconde o recorte sem caminho para a lei inteira | lê o que não cai; ou não acha o resto da lei |
| L20 | ajustar o recorte à mão não fica gravado | a correção se perde, e o recorte automático volta |
| L21 | o link direto para um dispositivo fora do recorte não abre o dispositivo | o link do caderno não leva a lugar nenhum |
| L22 | pesquisar o tópico do edital não mostra a fonte e o que cada assunto pede, ou mostra divisões que o assunto não cita | importa sem saber o que está importando |
| L23 | importar a partir do tópico guarda a lei inteira, ou deixa de fora parte do que o tópico pede | a lei vem maior que o edital, ou falta artigo |
| L24 | outro tópico que pede mais artigos da mesma lei cria uma lei repetida, ou apaga o que já tinha | duas "Constituição Federal" no catálogo; ou artigos somem |
| L25 | excluir a lei não avisa quanto vai junto, apaga sem confirmar, ou deixa questão e resposta órfãs | importação errada para sempre; ou estudo perdido sem aviso |
| L26 | tópico de norma sem fonte conhecida trava a importação em vez de pedir o link | a Constituição de Goiás nunca entra |
| L27 | o tópico cuja lei já está no catálogo oferece importar de novo, e a sugestão aparece longe do tópico que a cita | a pessoa importa a mesma lei duas vezes, ou não sabe que lei responde a que tópico |
| L28 | "vincular todas" vincula lei que nenhum tópico da matéria cita, ou deixa uma sugerida de fora | leis erradas na matéria; ou um clique por lei, dezoito vezes |
| L29 | a lei importada só em parte aparece como "a lei inteira" na sugestão ou no vínculo | a pessoa acha que tem a lei toda para estudar |
| L30 | no leitor, parágrafo, inciso e alínea saem no mesmo recuo do artigo | a lei vira um bloco embolado, sem hierarquia |
| L31 | a página de leitura das leis mistura a importação, ou lista matéria que não tem lei nenhuma | a pessoa procura a lei que vai ler no meio de tópicos, botões de importar e matérias vazias |
| L32 | o cartão da lei não diz o que cai dela, quantos artigos tem, nem quantas questões a pessoa já respondeu | não dá para escolher o que estudar agora, nem ver o avanço |
| L33 | a lei do catálogo sem vínculo some da leitura, aparece sem caminho para resolver, ou o aviso fica por causa de tópico que nenhum vínculo resolve (o PDTI, uma instrução normativa de outra matéria) | a norma que cai na prova fica de fora; ou o aviso nunca some |

## M. Mapas mentais

O mapa mental é escrito fora do app, a partir de uma aula, como um outline de
texto (`# título`, metadados `chave: valor` e itens `- texto` com recuo de dois
espaços), e entra pela tela: **Mapas mentais → Importar mapa**. Ele é da conta
que importou: é material de estudo pessoal, e não há catálogo compartilhado
como o das leis. O vínculo com a matéria é pelo id da disciplina, e é por ele
que o cronograma oferece o mapa. O mapa abre como uma página de tópicos
recolhíveis, no jeito do Notion (escolha de 28/09/2026): cada ramo é uma seção
colorida, e a página tem de ler bem no celular e no tablet, onde boa parte do
estudo acontece.

As questões das aulas não ficam no mapa: vêm num arquivo à parte
(`<slug>.questoes.json`, fora do git como o mapa), importado na página do mapa,
e são resolvidas ali, como as da lei — cada uma presa a um ramo, múltipla
escolha ou Certo/Errado, com o gabarito revelado só depois da resposta.

| id | Como quebra | O que o usuário vê |
|---|---|---|
| M1 | importar o texto não gera um mapa fiel: um item se perde, vai para o pai errado ou troca de ordem | o mapa abre com ramos faltando ou embaralhados |
| M2 | importar o mesmo mapa de novo duplica o mapa, ou apaga o vínculo com a matéria | dois mapas iguais na lista; o acesso pelo cronograma some |
| M3 | um texto com problema (sem título, recuo que pula um nível, marca desconhecida, item vazio, tabulação, chave desconhecida) é gravado pela metade, ou a recusa não diz a linha | mapa torto no app; ou a pessoa não sabe o que corrigir |
| M4 | o mapa de uma conta aparece, abre, é vinculado ou excluído por outra conta | o material de estudo de um vaza para o outro |
| M5 | as marcas (`[def]`, `[pegadinha]`, `[cai]`, `[ex]`, `[questao]`) aparecem como texto cru ou perdem o destaque, ou o **negrito** aparece com asteriscos | "[pegadinha]" escrito no item; nada destaca o que cai na prova |
| M6 | a lista de mapas não agrupa pela matéria vinculada, ou esconde o mapa que ainda não tem matéria | o mapa importado "some", ou aparece na matéria errada |
| M7 | vincular o mapa a uma matéria (ou desvincular) não fica gravado, ou vale para a disciplina de outro concurso | o vínculo some ao recarregar; o mapa aparece no concurso errado |
| M8 | importar com o concurso aberto não vincula à matéria que o texto indica (`materia:` ou `reconhecer:`), ou vincula a uma que nada tem a ver | importa e o cronograma não oferece o mapa; ou oferece o de outra matéria |
| M9 | o cronograma (ou o Hoje) não oferece o mapa da matéria que tem mapa, ou oferece o de uma matéria que não tem | não acha o mapa por onde estuda; ou um ícone sem destino |
| M10 | a matéria com mais de um mapa só deixa abrir um deles pelo cronograma | o segundo mapa fica fora de alcance por onde se estuda |
| M11 | excluir o mapa não pede confirmação, cancelar exclui mesmo assim, ou o cronograma continua apontando para o mapa apagado | perda por um clique; um link quebrado |
| M12 | algum item do mapa fica sem caminho até a tela: um tópico que não abre, "Abrir tudo" que deixa algo fechado, ou "Recolher tudo" que some com os ramos | parte da matéria "não existe" no mapa |
| M13 | um texto gigante (itens demais, fundo demais, linha enorme) é aceito sem limite | uma requisição que derruba o servidor, ou um mapa ilegível |
| M14 | o filtro não acha o termo digitado sem acento ou em outra caixa, esconde o caminho até o item achado, ou mostra o item achado sem o que há dentro dele | a busca diz que não há o que existe; ou acha o título e esconde o que ele diz |
| M15 | no celular ou no tablet a página rola para o lado, o tópico mais fundo fica espremido numa coluna estreita, o alvo do toque é pequeno demais, tocar no filtro dá zoom na página, ou o mapa abre todo desdobrado e o último ramo fica a várias telas de distância | o mapa não se lê no aparelho em que se estuda |
| M16 | importar as questões de um mapa grava uma questão a menos, fora de ordem, no ramo errado, ou importar de novo duplica, apaga as respostas ou deixa à vista a questão que saiu do arquivo | faltam questões; aparecem duas vezes; o que já respondi some; resolvo questão que o professor retirou |
| M17 | um arquivo de questões com problema (JSON ilegível, de outro mapa, ramo que o mapa não tem, gabarito fora das alternativas, Certo/Errado com alternativas, alternativa repetida, enunciado ou comentário vazio, chave repetida) é gravado pela metade, ou a mensagem não diz qual questão e o quê; ou alternativas que só diferem na caixa ou na pontuação (o que uma questão de tokenização cobra) são recusadas como repetidas | questões quebradas no mapa; não sei o que corrigir no arquivo |
| M18 | o gabarito ou o comentário chegam à tela antes da resposta; a correção erra (letra ou Certo/Errado); a resposta não fica gravada; responder de novo apaga a anterior ou não conta a nova | a questão não serve para treinar; o placar mente; perco o histórico |
| M19 | as questões do mapa de uma conta aparecem, são importadas ou respondidas por outra conta | o material e o desempenho de um vazam para o outro |
| M20 | a página do mapa não agrupa as questões pelo ramo, o placar (respondidas, certas) não anda ao responder, "Só o que errei" mostra o que acertei, ou no celular o diálogo das questões não cabe na tela e a alternativa não tem alvo de dedo | não acho as questões do assunto que acabei de revisar; não consigo resolver no celular |
| M21 | o mapa sem questões mostra uma seção vazia, ou excluir o mapa deixa questões e respostas para trás (ou não avisa que vão junto) | tela poluída; lixo que ninguém alcança |
| M22 | as questões do mapa não se agrupam por banca, a banca sai errada da origem ("CEBRASPE (CESPE)" e "CEBRASPE" viram duas; "ADAPTADA - FGV" não conta como FGV), o placar da banca não anda, ou o diálogo da banca traz questão de outra | não consigo treinar só a banca da minha prova |

## Fora da suíte, de propósito

- **A importação de desempenho do TEC** (o CSV do TEC no caderno de erros)
  não é coberta: decisão de 22/09/2026. O parser dele tem teste em
  `backend/internal/domain/tec`.
- **O Gemini de verdade.** No stack de E2E o `edital-processor` é trocado por
  um dublê (`e2e/duble-processador/`) que devolve sempre a mesma leitura — do
  edital e da lei: o assistente e a captura são testados inteiros, sem custo,
  sem rede e sem resposta diferente a cada execução. Quem testa o processador de verdade — PDF, OCR e o contrato
  com a IA — é a suíte dele (`make check-processor`), e o contrato entre os
  dois lados tem teste no Go (`adapter/editalproc`).

## Perguntas em aberto

Coisas que a exploração achou e que não são falha clara — dependem de uma
decisão de produto antes de virarem teste.

- **A observação escrita no registro da matéria não aparece em lugar nenhum
  além do próprio diálogo.** O caderno de erros lista "Notas lançadas nos
  dias", e o dossiê do NotebookLM promete "suas notas dos dias", mas os dois
  leem a nota do DIA, que só existe nos dias sem matéria (simulado, revisão
  geral). Num dia normal, o que o estudante escreve em "Observação" fica
  gravado (C3) e não chega a nenhum dos dois.
- **O mesmo vale para a nota da revisão do dia.** O diálogo diz "Vira uma
  anotação no caderno de erros desta disciplina", e o backend de fato grava
  uma anotação — mas a tela do caderno não mostra anotações desde e6d9a91
  (01/09, de propósito: "o NotebookLM cobre o resto"), e a anotação nasce sem
  disciplina, então também não entra no dossiê da matéria. Ela fica gravada
  (C11) e só volta ao reabrir a revisão.
- **Mudar os blocos por dia refaz o cronograma de amanhã em diante, mas o dia
  de hoje já estudado fica só com a matéria concluída — e ela passa a aparecer
  como um bloco do tamanho do dia inteiro.** Com uma matéria de 60 min
  concluída e 3 blocos por dia, hoje vira "1 bloco de 3h" com a matéria de
  180 min, e a outra matéria que estava agendada sai do dia. O estudo
  registrado continua certo (1,0 h); o que muda é o que a tela diz que o dia
  tinha. Visto em 21/09/2026 durante o D2.
- **O mesmo tópico aparece várias vezes seguidas no mesmo dia.** Com uma
  disciplina de um tópico só e 4 a 6 blocos por dia, 39 de 40 dias mostram,
  por exemplo, "Crimes contra a pessoa" repetido em sequência (medido em
  22/09/2026, pela API do stack de E2E). Era o que `plano.MesclarItensIguais`
  (0863109) corrigia, juntando os blocos iguais; a chamada saiu na
  materialização do cronograma (4b4fe6b) e a função ficou sem uso. Voltar a
  juntar muda a saída do motor — protegida pelo golden test — e o modelo: hoje
  cada bloco é uma atividade com registro próprio.
