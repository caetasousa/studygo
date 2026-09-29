# Mapas mentais

Cada arquivo desta pasta é o mapa mental de UMA aula ou assunto, escrito fora do
app como um outline de texto. Ele entra pela tela — **Mapas mentais → Importar
mapa** — e passa a ser da conta que importou.

**Só este README é versionado.** Os mapas derivam de aulas pagas, de uso
pessoal, e o repositório é público: o `.gitignore` deixa `conteudo/mapas/*` de
fora, como faz com os PDFs. Os testes E2E usam um mapa sintético
(`e2e/fixtures/mapa-exemplo.md`).

O roteiro para montar um mapa a partir de um PDF está em
`.claude/skills/mapa-mental`.

## Formato

```
# ITIL 4
slug: itil-4
fonte: Curso de Governança de TI · Aula 02
materia: Governança de TI
reconhecer: ITIL

- Ramo principal
  - [def] Uma **definição**, com o termo em negrito
    - Detalhe da definição
  - [pegadinha] O que a banca costuma inverter
```

- **A primeira linha é o título** (`# Título`). Ele é a raiz do mapa.
- **Metadados**, opcionais, logo abaixo do título, no formato `chave: valor`:
  - `slug`: o endereço estável do mapa (`/mapas/itil-4`); sem ele, sai do título.
    Importar de novo o mesmo slug **substitui** o conteúdo e mantém os vínculos.
  - `fonte`: de onde o material veio.
  - `materia`: o nome da matéria como a fonte a chama.
  - `reconhecer`: termos separados por vírgula que, achados num tópico do
    concurso, indicam a matéria.
  - Ao importar com um concurso aberto, o mapa é vinculado às matérias dele cujo
    nome bate com `materia` ou cujo tópico cita algum termo de `reconhecer`.
- **Itens**: `- texto`, com recuo de **2 espaços por nível**. Um item pode
  descer no máximo um nível de cada vez. Sem tabulação.
- **Marcas**, opcionais, no começo do item: `[def]` definição, `[pegadinha]` o que
  a banca inverte, `[cai]` o que cai em prova, `[ex]` exemplo, `[questao]` questão
  de prova e o que ela ensina.
- **Negrito**: `**termo**`. Nada além disso é interpretado.
- Linhas em branco são ignoradas.

Limites: até 5.000 itens, 10 níveis, 500 caracteres por item. Um item longo vira
um item curto com filhos: o mapa é para ser lido de relance.
