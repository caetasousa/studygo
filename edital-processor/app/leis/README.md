# Captura de leis

Ferramenta de linha de comando, só local: baixa a lei da fonte pública, separa
o texto vigente da redação anterior e das notas, organiza em dispositivos
(título, capítulo, artigo, parágrafo, inciso, alínea) e grava
`conteudo/leis/<slug>/lei.json` e `captura.md`. Não tem rota HTTP e não entra
na imagem de produção (`.dockerignore`).

```
uv run python -m app.leis capturar <slug> [--sem-gemini]
uv run python -m app.leis capturar --prioridade A
```

A norma vem de `conteudo/leis/normas.toml`. O texto de cada dispositivo sai do
original, nunca da IA: o Gemini só diz o **tipo** de cada parágrafo numerado
(`p0001`…), e onde a regra determinística tem certeza e ele discorda, a
captura para.

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
| K17b | a divergência já revisada (a regra estava certa) volta a bloquear a cada captura, ou uma divergência nova passa por ter sido "aceita" outra | ninguém consegue capturar, ou o aceite vira cheque em branco |
| K18 | sem chave do Gemini a captura grava como se ele tivesse conferido | captura "verificada" que ninguém verificou |

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
| K24 | o `recorte` do `normas.toml` cita um dispositivo que não existe | questões feitas sobre um recorte fantasma |

### Gravar

| id | Como erra | O que sai errado |
|---|---|---|
| K25 | grava `lei.json` quando alguma verificação falhou | lei quebrada pronta para o pacote |
| K26 | capturar de novo a mesma fonte muda a versão (a data entra no hash) | toda recaptura vira "versão nova" e mexe nas questões |

### PDF (link genérico)

| id | Como erra | O que sai errado |
|---|---|---|
| K27 | linhas do PDF não são juntadas em parágrafos, ou a palavra hifenizada no fim da linha fica partida | um inciso em cinco pedaços, "adminis- tração" |
| K28 | cabeçalho e rodapé de página repetidos entram no meio do texto | "Página 3 de 20" dentro do art. 12 |

Limite conhecido: juntar a hifenização de fim de linha desfaz também um hífen
legítimo que caia exatamente na quebra ("bem-/estar" vira "bemestar"). O
`captura.md` lista cada junção feita, para revisão.
