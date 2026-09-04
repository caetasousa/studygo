# 📄 Fixture do edital

![FCC](https://img.shields.io/badge/banca-FCC-004A8F)
![Cargos](https://img.shields.io/badge/cargos-2-blue)
![Determinístico](https://img.shields.io/badge/Gemini-não%20chamada-success)

`edital_fcc_tcego.json` é a extração **real** de um edital da FCC (TCE-GO 2026,
25 páginas, 2 cargos), capturada uma vez contra o `edital-processor` com a
Gemini de verdade e congelada aqui.

---

## 🎯 Por que ela existe

Importar o PDF a cada execução seria lento, custaria cota da API e daria um
resultado diferente a cada vez — um modelo generativo não é determinístico. Com
a fixture, os testes do assistente rodam em milissegundos, de graça, sem rede, e
falham por regressão de código, nunca por variação do modelo.

O que ela contém é dado de produção de verdade, com os cantos que dados
inventados à mão não teriam:

| | O quê | Por que importa |
|---|---|---|
| 👥 | **2 cargos** (A01 administrativo, B02 TI) | como a maioria dos editais da FCC |
| 🔢 | **total só no grupo** (25 gerais + 45 específicas) | gera `blocker` e obriga o usuário a ratear |
| ⚖️ | **peso por grupo**, não por matéria | o wizard precisa propagar |
| 📅 | **22 marcos** do cronograma oficial | vira a tela de datas |
| 🔤 | nomes longos e acentuados | exercitam a geração de código de disciplina |

---

## 🚧 O que ela NÃO substitui

A suíte Python do `edital-processor` continua sendo quem testa PDF, OCR,
chunking e o contrato com a Gemini. Esta fixture é o degrau seguinte: o backend
Go consumindo o resultado já extraído.

---

## 🔄 Como recapturar

> [!IMPORTANT]
> Quando o contrato do processor mudar (campo novo, formato diferente), a fixture
> precisa ser refeita — **nunca editada à mão**, ou ela deixa de representar o
> que a produção devolve.

```bash
# com o stack de pé, GEMINI_API_KEY definida e um edital em mãos
go run ./internal/adapter/editalproc/cmd/recapturar -pdf caminho/do/edital.pdf
```

O comando faz os três passos do assistente para cada cargo e regrava este
arquivo. Confira o diff antes de commitar: uma mudança grande costuma significar
que o prompt ou o schema mudou, não que o edital mudou.

---

## 🔒 Privacidade

> [!NOTE]
> O PDF original **não** é versionado (`*.pdf` está no `.gitignore`). O que fica
> aqui é a extração estruturada — nomes de disciplina, temas e datas, todos
> públicos por serem conteúdo de edital.
