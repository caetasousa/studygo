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

## Fora da suíte, de propósito

- **A importação de desempenho do TEC** (o CSV do TEC no caderno de erros)
  não é coberta: decisão de 22/09/2026. O parser dele tem teste em
  `backend/internal/domain/tec`.
- **O Gemini de verdade.** No stack de E2E o `edital-processor` é trocado por
  um dublê (`e2e/duble-processador/`) que devolve sempre a mesma leitura: o
  assistente é testado inteiro, sem custo, sem rede e sem resposta diferente a
  cada execução. Quem testa o processador de verdade — PDF, OCR e o contrato
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
