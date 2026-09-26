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
