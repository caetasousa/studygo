---
name: questoes-de-lei
description: Escreve as questões de lei seca (estilo FCC, A–E) de uma norma já capturada em conteudo/leis/<slug>/lei.json, unidade por unidade do recorte do edital, e as deixa validadas para o pacote. Use quando pedirem questões de uma lei do catálogo, ou para revisar as de uma unidade que mudou.
---

# Questões de lei seca

As questões são escritas aqui, na máquina de quem estuda, e vão para produção
dentro do pacote da lei. O app não gera questão nenhuma. Quem confere cada uma
é o mesmo código da importação (`make leis-validar`); o que passa lá, passa lá
em cima.

## Antes de escrever

1. A lei tem de estar capturada: `conteudo/leis/<slug>/lei.json`. Sem ele,
   `make leis-capturar slug=<slug>`.
2. Leia o `recorte` da norma em `conteudo/leis/normas.toml`. Questão só sai do
   recorte. `questoes = false` → não escreva nada.
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
- `hash`: deixe vazio; `make leis-validar atualizar=1 slug=<slug>` preenche.

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

```
make leis-validar atualizar=1 slug=<slug>   # preenche hash, confere tudo
make leis-pacote slug=<slug>                # conteudo/leis/pacotes/<slug>.json
```

Se a validação acusar "unidade desatualizada", a lei mudou depois das
questões: releia a unidade no `lei.json` novo, corrija as questões afetadas,
apague o `hash` da unidade e valide de novo com `atualizar=1`. Nunca apague o
hash sem reler.
