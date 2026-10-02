# Mapas mentais

Cada `<slug>.md` desta pasta é o mapa mental de UMA aula ou assunto, escrito fora
do app como um outline de texto. Ele entra pela tela — **Mapas mentais → Importar
mapa** — e passa a ser da conta que importou. As questões da aula vão ao lado,
em `<slug>.questoes.json`, e entram pela página do mapa (**Manter este mapa →
Importar questões**).

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
  a banca inverte, `[cai]` o que cai em prova, `[ex]` exemplo. (`[questao]` ainda
  é aceita, para não recusar mapa antigo, mas as questões agora vão no arquivo de
  questões, abaixo.)
- **Negrito**: `**termo**`. Nada além disso é interpretado.
- Linhas em branco são ignoradas.

Limites: até 5.000 itens, 10 níveis, 500 caracteres por item. Um item longo vira
um item curto com filhos: o mapa é para ser lido de relance.

## Questões

As questões da aula não entram no mapa: ficam em `<slug>.questoes.json`, um
arquivo por mapa, e se resolvem na página dele, como as da lei.

```json
{
  "mapa": "itil-4",
  "questoes": [
    {
      "id": "fcc-01",
      "ramo": "Práticas de gerenciamento",
      "origem": "FCC · 2026 · Auditor Fiscal (SEFAZ SP)/TIC",
      "enunciado": "Para reduzir falhas de parametrização… a prática que deve receber prioridade é:",
      "alternativas": ["…", "…", "…", "…", "…"],
      "gabarito": "B",
      "comentario": "(a) Errado. …\n(b) Correto. …"
    },
    {
      "id": "teoria-04",
      "ramo": "Sistema de Valor de Serviço (SVS)",
      "origem": "CEBRASPE · 2025 · Analista Judiciário (TRF 6ª Região)",
      "enunciado": "Julgue o item a seguir.\nOtimizar e automatizar é um dos princípios orientadores do ITIL v4.",
      "gabarito": "Certo",
      "comentario": "Correto. O ITIL v4 define sete princípios orientadores…"
    }
  ]
}
```

- `mapa` é o slug do mapa: o arquivo de outro mapa é recusado.
- `id` é a chave da questão: importar de novo casa por ela, mantém as respostas
  e desativa a que sumiu do arquivo. Não reaproveite o id de uma questão para
  outra.
- `origem` **começa pela banca**, separada do resto por ` · ` ("FGV · 2024 ·
  …"): é daí que a página agrupa as questões por banca. "CEBRASPE (CESPE)" conta
  como CEBRASPE e "ADAPTADA - FGV" como FGV; a questão do próprio professor vai
  como "Inédita do professor · …".
- `ramo` é o título de um ramo principal do mapa (sem o negrito; caixa e acento
  não contam). É onde a questão aparece na página.
- Sem `alternativas`, a questão é de julgar e o `gabarito` é `Certo` ou `Errado`.
  Com elas, de 2 a 5 (A–E), e o `gabarito` é a letra.
- `comentario` é o que a tela mostra depois da resposta; quebras de linha (`\n`)
  aparecem.
- Na de múltipla escolha, o comentário que traz cada alternativa numa linha
  própria, começando pela letra (`a) Errada. …`, `(B) Correto. …`), aparece
  separado: cada trecho debaixo da sua alternativa — aberto o da certa ao
  acertar e o da marcada ao errar, os outros a um toque — e o que vem antes do
  `a)` como comentário geral. Só separa com todas as letras, em ordem e com
  texto; fora isso, o comentário aparece inteiro.

Limites: 1.000 questões por arquivo; enunciado e comentário até 5.000
caracteres, alternativa até 2.000. O arquivo com problema é recusado inteiro, e
a mensagem diz a questão e o quê.
