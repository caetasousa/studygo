# Captura de leis

Baixa a lei da fonte oficial, separa o texto vigente da redação anterior e das
notas, e organiza em dispositivos (título, capítulo, artigo, parágrafo, inciso,
alínea). Roda dentro do processador, chamada pelo backend quando alguém cola o
link em **Legislação → Adicionar lei**:

```
POST /internal/leis/pesquisas       {"tema": "Constituição…: Administração Pública"}  → fonte e estrutura
POST /internal/leis/capturas        {"link": "https://www.planalto.gov.br/…", "recorte": ["tit3.cap7"]}  → {"id": …}
GET  /internal/leis/capturas/{id}   → estado, etapa e, pronta, a lei e o que revisar
```

A pesquisa acha a fonte pelo tópico (Constituição Federal pelo nome; lei
federal pelo número e ano, no Planalto; lei de Goiás pelo número, na API da
Casa Civil) e devolve a estrutura, só pelas regras. Na dúvida, "não achei":
nunca um palpite.

A captura demora (a Constituição leva uns quatro minutos com o Gemini), por
isso é assíncrona: o backend consulta até ela ficar pronta. O processador **nunca
grava** a lei: devolve a prévia, e quem publica é o backend, depois que a
pessoa revisou.

O texto de cada dispositivo sai do original, nunca da IA: o Gemini só diz o
**tipo** de cada parágrafo numerado (`p0001`…). O que sai da captura:

- **bloqueios** — o texto não confere com o original, a árvore é inválida:
  a lei não pode ser publicada;
- **avisos** — o Gemini discordou da regra onde ela tem certeza, a numeração
  salta, a classificação não foi conferida pelo Gemini: a regra prevaleceu, e a
  pessoa marca na prévia que revisou cada um antes de publicar.

## Como a captura pode errar

Escrito antes do código. Cada teste em `tests/unit/test_leis_*.py` cita o id
que cobre; um item sem teste é lacuna declarada.

### Baixar

| id | Como erra | O que sai errado |
|---|---|---|
| K1 | aceita um link fora do domínio da fonte (`planalto` fora de planalto.gov.br, `casacivil-go` fora da API de Goiás) | a "lei" vem de qualquer lugar |
| K2 | aceita resposta que não é a lei: status ≠ 200, página de erro, HTML sem nenhum "Art.", JSON sem `conteudo` | grava uma lei vazia ou uma página de erro |
| K3 | decodifica o HTML do Planalto (Windows-1252) como UTF-8, ou o contrário | "funÃ§Ã£o" no lugar de "função" |

### Limpar

| id | Como erra | O que sai errado |
|---|---|---|
| K4 | entidades (`&nbsp;`, `&ordm;`) ficam no texto ou viram outro caractere | "Art.&nbsp;9" não é reconhecido como artigo |
| K5 | o parágrafo partido em várias linhas do HTML vira dois, ou as palavras colam | "contábil,financeira", ou um inciso cortado ao meio |
| K6 | texto riscado entra como vigente — `<strike>`, `<s>`, `<del>`, `text-decoration: line-through` no próprio parágrafo ou num `<span>`, classe `conteudo-revogado` | o estudante lê a redação revogada como lei |
| K6b | o riscado vem com uma marca de situação fora do risco — "(Rejeitada)" de medida provisória, "(Revogado)" — e a marca faz o parágrafo parecer vigente | a redação de uma MP rejeitada colide com a vigente, ou vira texto de lei |
| K7 | nota de redação entra no texto vigente, ou some: link "(Redação dada pela…)", "(Incluído…)", "(Vide…)", "Vigência", `<nota>` e `<vide>` de Goiás | "(Redação dada pela EC 19)" no meio do parágrafo, ou a nota perdida |
| K6c | só o conteúdo é riscado e o rótulo fica de fora ("I -" + ~~impostos sobre:~~) | um inciso vazio "I -" vigente, colidindo com o inciso de verdade |
| K8 | riscado parcial: um trecho riscado dentro de um parágrafo vigente | a palavra revogada fica no meio da frase vigente |
| K9 | cabeçalho e nome no mesmo parágrafo ("CAPÍTULO I&lt;br&gt;DISPOSIÇÕES PRELIMINARES") ou em dois parágrafos, em maiúsculas ou não ("Seção II" / "Do Conselho Nacional…") | capítulo sem nome, ou nome virando texto solto |
| K10 | o cabeçalho do site entra como texto da lei ("Presidência da República", "Texto compilado") | lixo antes do preâmbulo |
| K10b | o sumário de links do topo (Emendas, "Ato das Disposições Constitucionais Transitórias") é lido como lei | o link do ADCT abre o ADCT antes do art. 1º e toda a Constituição vira `adct.*` |

### Classificar

| id | Como erra | O que sai errado |
|---|---|---|
| K11 | rótulo de artigo lido errado: "Art. 1º-A", "Art. 5o", "Art. 10.", "Art. 9º -A", "Art. 5<sup>º</sup>" | ref `art1` para o 1º-A, colidindo com o 1º |
| K11b | a própria fonte tem erro de digitação no rótulo ("Art. 5 7.") | o art. 57 vira um segundo art. 5 |
| K12 | "Parágrafo único" e "§ 1º-A" lidos como texto solto | parágrafo pendurado no artigo errado |
| K13 | inciso com hífen, travessão ou meia-risca ("I -", "I –", "IV-A –"), ou com letra e sem traço ("I-A o Conselho") não reconhecido | inciso vira texto solto |
| K14 | alínea "a)" e item "1." confundidos, alínea com espaço ("a )") não reconhecida, ou alínea sem inciso aceita | hierarquia trocada |
| K14b | artigo que altera outra lei cita o texto dela entre aspas ("“Art. 7º …", "X – …” (NR)") e a citação vira dispositivo desta lei | um inciso X fantasma no art. 60 da LGPD, que é da Lei 12.965 |
| K15b | o fecho do corpo ("Brasília, 5 de outubro de 1988") vem antes do ADCT e tudo que vem depois vira fecho | alíneas do ADCT como assinatura, incisos sem artigo |
| K15 | o ADCT reinicia a numeração e a ref colide com o corpo da Constituição | `art1` do ADCT sobrescreve o art. 1º |
| K16 | o Gemini devolve id que não foi enviado, pula ou repete um id | parágrafo sem tipo, ou dois tipos |
| K16b | um id pulado uma vez pelo Gemini derruba a captura inteira da CF | captura que só passa na sorte |
| K17 | o Gemini discorda da regra onde a regra tem certeza ("Art. 71." é artigo) | dispositivo com tipo errado gravado em silêncio |
| K17c | todo palpite diferente do Gemini bloqueia, inclusive o impossível ("título" para um texto sem rótulo de título) e o que não muda a árvore (descartar × solto) — e o Gemini muda de palpite a cada execução | a captura da CF nunca termina; ou se aceita tudo às cegas |
| K17b | a divergência revisada (a regra estava certa) não tem como ser aceita, ou aceitar uma aceita outra | ninguém consegue publicar, ou o aceite vira cheque em branco |
| K18 | sem chave do Gemini a captura sai como se ele tivesse conferido | captura "verificada" que ninguém verificou |

### Montar e verificar

| id | Como erra | O que sai errado |
|---|---|---|
| K19 | inciso sem artigo, parágrafo sem artigo, alínea sem inciso | árvore inválida gravada |
| K20 | artigo fora de sequência (5, 7) ou repetido | um artigo perdido na limpeza passa sem aviso |
| K21 | duas refs iguais | o link direto abre o dispositivo errado |
| K22 | o texto remontado da árvore difere do texto do original | uma palavra da lei trocada, perdida ou duplicada |
| K23 | redação anterior sem dispositivo vigente correspondente some | o artigo revogado desaparece em vez de aparecer como revogado |
| K23b | um bloco de redações antigas vem antes do bloco vigente (IV e V antigos, depois IV e V novos) e só o primeiro casa | o V antigo vira dispositivo revogado e colide com o V vigente |
| K23c | artigo incluído por medida provisória e depois revogado aparece riscado fora da ordem (55-K antes do 55-A) | a verificação de sequência acusa salto onde não há |

### Captura pela aplicação

| id | Como erra | O que sai errado |
|---|---|---|
| K29 | aceita um link fora das fontes oficiais (qualquer host, `http://`, IP, `localhost`) | o servidor baixa o que alguém mandar, inclusive da rede interna |
| K30 | a fonte oficial redireciona para fora dela e o redirecionamento é seguido | o mesmo que K29, por tabela |
| K31 | uma conta consulta a captura de outra pelo id | a prévia de outra pessoa vaza |
| K32 | uma exceção no meio da captura deixa o estado em "rodando" para sempre | a tela espera sem fim |
| K33 | capturas simultâneas sem limite (cada Constituição segura memória e o Gemini) | o processador cai para todo mundo |
| K34 | uma captura com bloqueio sai como publicável (o antigo K25: gravar quando a verificação falhou) | lei quebrada publicada |
| K35 | um aviso (divergência, salto de numeração) vira bloqueio sem saída, ou some da prévia | a Constituição nunca publica, ou publica sem ninguém ver |
| K36 | sem chave do Gemini a captura sai como se tivesse sido conferida | captura "verificada" que ninguém verificou (o K18, na tela) |
| K37 | captura pronta nunca expira | a memória do processador cresce com cada lei capturada |

### Pesquisa pelo tópico e captura do recorte

A importação começa pelo tópico do edital: a pesquisa acha a fonte oficial e
devolve a estrutura (divisões e artigos, só pelas regras, em segundos); a
captura depois confere com o Gemini só o recorte e guarda só ele.

| id | Como erra | O que sai errado |
|---|---|---|
| K38 | a fonte achada é de outra norma: número parecido, lei estadual tomada por federal (ou o contrário), complementar por ordinária, outro ano | importa a lei errada com cara de certa |
| K39 | tópico sem número nem nome conhecido (a Constituição de Goiás, uma resolução do TCE) vira um palpite em vez de "não achei" | o mesmo que K38; sem saída para colar o link |
| K40 | o recorte corta os pais: o artigo vem sem o capítulo e o título acima, ou sem a epígrafe | o leitor perde o contexto; o nome da lei se perde |
| K41 | o recorte deixa entrar o que está fora dele, ou corta um inciso de um artigo pedido | lei maior que o pedido, ou artigo pela metade |
| K42 | ref do recorte que a lei não tem é ignorada em silêncio | "arts. 74 e 999" importa só o 74 sem avisar |
| K43 | o Gemini confere a lei inteira mesmo com recorte (lento), ou não confere o recorte | a Constituição continua levando 4 minutos; ou o recorte sai sem conferência |
| K44 | aviso de fora do recorte (salto de numeração no ADCT) bloqueia ou pede revisão | a pessoa revisa o que não vai importar |

### Versão

| id | Como erra | O que sai errado |
|---|---|---|
| K26 | capturar de novo a mesma fonte muda a versão (a data entra no hash) | toda recaptura vira "versão nova" e mexe nas questões |

Limite conhecido do K26: onde a regra não tem certeza e o tipo não é
estrutural (o título "PREÂMBULO", um fecho), vale o palpite do Gemini, que muda
entre execuções. A versão da Constituição pode mudar de uma captura para outra
com os artigos idênticos; as unidades das questões, que só olham o texto dos
dispositivos citados, não mudam (conferido em 26/09/2026).

### PDF (link genérico)

| id | Como erra | O que sai errado |
|---|---|---|
| K27 | linhas do PDF não são juntadas em parágrafos, ou a palavra hifenizada no fim da linha fica partida | um inciso em cinco pedaços, "adminis- tração" |
| K28 | cabeçalho e rodapé de página repetidos entram no meio do texto | "Página 3 de 20" dentro do art. 12 |

Limite conhecido: juntar a hifenização de fim de linha desfaz também um hífen
legítimo que caia exatamente na quebra ("bem-/estar" vira "bemestar"). A
prévia lista cada junção feita, para revisão.
