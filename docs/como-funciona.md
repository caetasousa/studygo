# Como o studygo funciona

Explicação sem jargão. Se você nunca mexeu no projeto, comece por aqui.

Os outros documentos são mais técnicos e servem de referência quando você já
souber o básico. Este aqui é o mapa.

---

## O que o studygo faz

Ajuda alguém que estuda para concurso público a montar e seguir um plano de
estudos.

A pessoa cadastra o concurso que vai prestar, com as matérias e assuntos que
caem na prova. O sistema distribui esse conteúdo pelos dias até a data da prova,
respeitando quanto tempo ela tem disponível. Depois, ela marca o que estudou, e
o sistema acompanha o progresso.

Também dá para enviar o PDF do edital e deixar o sistema extrair as matérias
sozinho, em vez de digitar tudo à mão.

E há um catálogo de **provas anteriores**, o mesmo para todo mundo. Algumas
contas escolhidas — os **curadores** — enviam o PDF de uma prova e o gabarito;
o sistema lê as questões e recorta as figuras do próprio PDF; o curador confere
tudo ao lado do original e só então publica. Na aba **Questões**, quem estuda
resolve uma prova inteira, como no dia, ou treina uma matéria com as questões de
todas as provas — só as que ainda não fez, ou só as que errou.

---

## As quatro peças

O studygo não é um programa só. São quatro programas que conversam entre si.

Pense num restaurante:

| Peça | No restaurante seria | O que faz |
|---|---|---|
| **frontend** | o salão | a tela que você vê e clica |
| **backend** | a cozinha | onde as decisões acontecem |
| **PostgreSQL** | a despensa | onde tudo fica guardado |
| **edital-processor** | um especialista contratado | lê o PDF do edital (ou da prova) e devolve o conteúdo |

Existe ainda o **worker**: um ajudante que roda sozinho de tempos em tempos para
enviar lembretes e para ler as provas enviadas, um pedaço por vez. Ninguém pede
nada a ele; ele acorda, faz o trabalho e volta a dormir.

```
Você → tela (frontend) → cozinha (backend) → despensa (PostgreSQL)
                              ↓
                     especialista (edital-processor)
```

O frontend nunca fala direto com a despensa. Tudo passa pela cozinha — é ela que
sabe as regras.

---

## Por que dividir assim

Cada peça faz uma coisa só. Isso importa porque:

- dá para mexer na tela sem risco de estragar os dados;
- o especialista de PDF pode ficar lento sem travar o resto;
- se um pedaço quebra, os outros continuam de pé.

O preço é ter mais peças para cuidar. É uma troca consciente.

---

## As três regras que não se quebram

Se você mexer no código, estas são as que mais causam estrago quando ignoradas.

### 1. O cronograma é escrito, não calculado na hora

Quando o plano é criado, **todos os dias e todas as tarefas são gravados** no
banco na mesma hora.

Poderia ser diferente: o sistema poderia calcular o cronograma toda vez que você
abrisse a tela. Não faz isso de propósito. Se calculasse na hora, mudar qualquer
regra mudaria o passado — dias que você já estudou apareceriam diferentes.

### 2. Cada coisa tem um número de identidade

Matérias, tarefas e registros são ligados por um número interno, nunca pelo nome.

Por isso você pode renomear "Português" para "Língua Portuguesa" sem perder nada
do que já estudou. Se a ligação fosse pelo nome, renomear quebraria o histórico.

### 3. O que você já estudou nunca se apaga

O banco recusa apagar uma tarefa que tenha registro de estudo. Não é uma
verificação que dá para esquecer de fazer — é o próprio banco que bloqueia.

---

## Como o código está organizado

O backend é dividido em camadas. A regra é simples: **as regras de negócio não
sabem nada sobre internet, telas ou banco de dados.**

```
Entra um pedido pela internet
        ↓
   [adapter/httpapi]  traduz de "internet" para "programa"
        ↓
   [service]          coordena os passos
        ↓
   [domain]           decide, segundo as regras do negócio
        ↓
   [adapter/postgres] guarda no banco
```

Por que separar assim: um dia você pode trocar o banco, ou oferecer um
aplicativo de celular. Se as regras estivessem misturadas com o banco e a tela,
qualquer troca dessas exigiria reescrever tudo.

O detalhe técnico está em [arquitetura.md](arquitetura.md).

---

## O caminho de uma mudança

Você mexeu no código. E agora?

```
1. você escreve o código
2. testa na sua máquina        → make check
3. envia                        → git push gitlab main
4. a esteira testa de novo, sozinha
5. vai para o ambiente de teste automaticamente
6. você confere se ficou bom
7. clica para publicar de verdade
```

Os passos 4 a 7 são automáticos, menos o último — publicar exige um clique seu.

**Por que a máquina testa de novo, se você já testou?** Porque a sua máquina
tem coisas que o servidor não tem. Um código que funciona só aí não serve.

Detalhes em [ci-cd.md](ci-cd.md).

---

## Os dois ambientes

| | Produção | Homologação |
|---|---|---|
| Endereço | cronograma.caetasousa.tech | staging.cronograma.caetasousa.tech |
| Quem usa | pessoas de verdade | só você, para conferir |
| Dados | reais | de teste |
| Como publica | você clica | automático |

São duas cópias do sistema, com bancos separados. A ideia é testar em
homologação antes de mexer no que as pessoas usam.

> ⚠️ Os dois vivem no mesmo servidor. Estão separados em tudo que guarda dados,
> mas dividem processador e memória. Se homologação consumir demais, produção
> sente. Por isso homologação tem limite de memória.

---

## Palavras que você vai encontrar

| Palavra | O que significa aqui |
|---|---|
| **concurso** | a prova que a pessoa vai fazer, com suas matérias |
| **plano** | o cronograma de estudos gerado para um concurso |
| **atividade** | uma tarefa de um dia ("estudar assunto X") |
| **registro** | a marca de que uma atividade foi feita |
| **disciplina** | uma matéria (Português, Direito...) |
| **deploy** | publicar uma versão nova no servidor |
| **pipeline / esteira** | a sequência automática de testes e publicação |
| **runner** | o programa que executa essa esteira |
| **digest** | a "impressão digital" de uma versão do sistema |
| **rollback** | voltar para a versão anterior |
| **migration** | uma mudança na estrutura do banco de dados |
| **prova** | uma prova anterior publicada no catálogo, com questões e gabarito |
| **importação** | uma prova sendo lida e revisada, antes de ir para o catálogo |
| **curador** | conta que pode importar e publicar provas |
| **recorte** | uma figura da prova, cortada do PDF original |

---

## Se algo der errado

**O site está fora do ar?**
```bash
make health              # produção respondeu?
make deploy-status       # os programas estão rodando?
make deploy-logs svc=backend    # o que o backend registrou?
```

**Publiquei e quebrou?**
Dá para voltar à versão anterior sem refazer nada, com um clique no GitLab. O
procedimento está em [ci-cd.md](ci-cd.md), na seção "Rollback".

**Quebrou na minha máquina?**
```bash
make down && make up     # desliga e liga tudo
make logs                # vê o que está acontecendo
```

---

## Onde procurar cada coisa

| Você quer... | Vá para |
|---|---|
| rodar o projeto na sua máquina | [rodar-local.md](rodar-local.md) |
| entender o dia a dia de trabalho | [fluxo-de-trabalho.md](fluxo-de-trabalho.md) |
| publicar ou desfazer uma publicação | [ci-cd.md](ci-cd.md) |
| mexer no servidor | [deploy.md](deploy.md) |
| entender as decisões técnicas | [arquitetura.md](arquitetura.md) |
| saber quais comandos existem | `make` |
