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

Para as matérias que são texto de lei, há a **Legislação**: a lei aparece
artigo por artigo, e cada artigo traz questões sobre ele. A lei não é baixada
pelo app — ela é organizada no computador de quem cuida do conteúdo, conferida
palavra por palavra com o texto oficial, e só então publicada.

---

## As quatro peças

O studygo não é um programa só. São quatro programas que conversam entre si.

Pense num restaurante:

| Peça | No restaurante seria | O que faz |
|---|---|---|
| **frontend** | o salão | a tela que você vê e clica |
| **backend** | a cozinha | onde as decisões acontecem |
| **PostgreSQL** | a despensa | onde tudo fica guardado |
| **edital-processor** | um especialista contratado | lê o PDF do edital e devolve as matérias |

Existe ainda o **worker**: um ajudante que roda sozinho de tempos em tempos para
enviar lembretes. Ninguém pede nada a ele; ele acorda, faz o trabalho e volta a
dormir.

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
3. envia                        → make push
4. a esteira testa de novo, sozinha
5. passando tudo, vai para o servidor automaticamente
6. você confere se ficou bom    → make servidor-health
```

Os passos 4 e 5 são automáticos. Se algo quebrou, voltar atrás é um clique.

**Por que a máquina testa de novo, se você já testou?** Porque a sua máquina
tem coisas que o servidor não tem. Um código que funciona só aí não serve.

Detalhes em [ci-cd.md](ci-cd.md).

---

## Onde o sistema fica no ar

Num servidor próprio: uma segunda distro Linux (`ubuntu-server`) dentro do WSL
deste mesmo computador, separada da que se usa para programar. Ela não tem
porta aberta para a internet: um programa da Cloudflare, o **túnel**, abre uma
conexão de dentro para fora, e as visitas chegam por ela.

| | |
|---|---|
| Endereço | `https://<palavras>.trycloudflare.com` — muda quando o túnel reinicia; `make servidor-endereco` mostra o da vez |
| Quem usa | você e quem estiver testando |
| Dados | de teste |
| Como publica | automático, a cada envio para a `main` |

> ⚠️ O servidor é este computador: com ele desligado, o site sai do ar. E o
> navegador pode avisar "site perigoso" no endereço `trycloudflare.com` — é a
> fama do serviço grátis, não um problema do app. Um domínio próprio resolve
> ([cloudflare-tunnel.md](cloudflare-tunnel.md)).

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

---

## Se algo der errado

**O site está fora do ar?**
```bash
make servidor-health            # o servidor respondeu?
make servidor-status            # os programas estão rodando?
make servidor-logs svc=backend  # o que o backend registrou?
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
| entender o túnel da Cloudflare | [cloudflare-tunnel.md](cloudflare-tunnel.md) |
| entender as decisões técnicas | [arquitetura.md](arquitetura.md) |
| saber quais comandos existem | `make` |
