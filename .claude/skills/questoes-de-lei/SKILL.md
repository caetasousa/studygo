---
name: questoes-de-lei
description: Escreve as questões de lei seca (estilo FCC, A–E) de uma norma já publicada no app, unidade por unidade do recorte do edital, em conteudo/leis/<slug>/questoes.json, validadas pela importação. Use quando pedirem questões de uma lei do catálogo, ou para revisar as de uma unidade que mudou.
---

# Questões de lei seca

As questões são escritas aqui, fora do app, e entram nele pela página da lei
(**Manter esta lei → Importar questões**, ou `POST /api/leis/<slug>/questoes`
com o conteúdo do `questoes.json`). O app não gera questão nenhuma. Quem
confere cada uma é a importação; o que passa na stack local, passa no
servidor.

## Antes de escrever

1. A lei tem de estar publicada no app (**Legislação → Adicionar lei**), e o
   texto dela em `conteudo/leis/<slug>/lei.json`. Sem ele, ou se a lei mudou,
   salve o que o app publicou: `GET /api/leis/<slug>` na stack local (`make
   up`), logado — o campo `dispositivos` tem o mesmo formato.
2. Leia o recorte da norma em `conteudo/leis/README.md`. Questão só sai do
   recorte; norma de prioridade B ou C → não escreva nada.
3. Leia o texto do recorte **em `lei.json`**, nunca de memória nem de outra
   fonte: a redação vigente é a da captura. Redação anterior (`anteriores`) e
   notas não são matéria de questão.

## Unidades

Uma unidade é um bloco do recorte que se estuda de uma vez: um capítulo, uma
seção, um grupo de artigos. Até ~2.500 palavras; maior que isso, divida (o
art. 37 da CF, sozinho, vira duas).

- `ref`: `<slug>-u<n>` (`cf88-u1`), estável — é a chave da unidade no app.
- `titulo`: como o estudante reconhece o bloco ("Arts. 70 a 75 — Fiscalização
  contábil, financeira e orçamentária").
- `dispositivos`: as refs das raízes da unidade (os artigos, ou a seção). Tudo
  que está abaixo delas é da unidade.
- `hash`: vazio numa unidade nova — a importação dá a ela o hash do texto
  publicado. Depois da primeira importação, copie para o arquivo o hash que o
  app gravou (`GET /api/leis/<slug>` → `unidades`): é ele que barra a
  importação quando a lei mudar e as questões não forem revistas.

Quantidade: uma questão a cada ~120 palavras da unidade, no mínimo 5 e no
máximo 20.

## Cada questão

Estilo FCC, a banca do edital de referência (TCE-GO):

- Enunciado curto que situa a norma ("Nos termos da Constituição Federal, …",
  "De acordo com a Lei Orgânica do TCE-GO, …"), seguido de cinco alternativas.
- Exatamente uma certa. As erradas saem das armadilhas que a FCC usa em lei
  seca: **troca de uma palavra** ("julgar" × "apreciar"), **troca de
  competência** (Congresso × TCU), **prazo ou número trocado** (60 × 90 dias),
  **exceção virada regra** (tirar o "salvo"), **sujeito trocado**, **"sempre" e
  "nunca"** onde a lei admite exceção. Alternativa errada por absurdo não
  ensina nada.
- Nada de "todas as anteriores" / "nenhuma das anteriores".
- Espalhe o gabarito entre A e E dentro da unidade.
- `dispositivos`: as refs MAIS específicas que justificam o gabarito
  (`art71.inc2`, não `art71`), todas dentro da unidade.
- `trecho`: cópia **literal** de um pedaço do texto de um dos dispositivos
  citados (ou de um descendente dele) — é o que o app grifa. Copie do
  `lei.json`, sem corrigir nada, nem acento, nem espaço.
- `comentario`: por que o gabarito é o gabarito e onde está a armadilha,
  em uma ou duas frases.
- `id`: `<slug>-<unidade>-<nn>` (`cf88-u1-07`), estável. Mudar o texto de uma
  questão mantém o id (e as respostas); apagar a questão do arquivo a
  desativa no app na próxima importação.

## Arquivo

`conteudo/leis/<slug>/questoes.json`:

```json
{
 "unidades": [
  {"ref": "cf88-u4", "titulo": "Arts. 70 a 75 — …", "dispositivos": ["art70", "art71", "art72", "art73", "art74", "art75"], "hash": ""}
 ],
 "questoes": [
  {"id": "cf88-u4-01", "unidade": "cf88-u4", "enunciado": "…", "alternativas": ["…", "…", "…", "…", "…"],
   "gabarito": "B", "comentario": "…", "dispositivos": ["art71.inc2"], "trecho": "julgar as contas dos administradores"}
 ]
}
```

## Fechar

1. Importe o `questoes.json` na stack local (a lei publicada nela também): a
   importação lista todos os problemas de uma vez — trecho que não está na
   lei, dispositivo fora da unidade, alternativa repetida.
2. Grave no arquivo o hash das unidades novas (ver **Unidades**).
3. Quem estuda importa o mesmo arquivo no servidor, pela página da lei.

Se a importação acusar "unidade desatualizada", a lei mudou depois das
questões: releia a unidade no texto novo, corrija as questões afetadas, apague
o `hash` da unidade e importe de novo. Nunca apague o hash sem reler.
