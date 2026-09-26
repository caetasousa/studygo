# Plano — Legislação interativa

Referência: o concurso no banco local — **TCE-GO, Técnico de Controle Externo –
TI, FCC, prova em 17/01/2027**. A legislação está em duas disciplinas,
Legislação Institucional (LEG, 8 questões) e Legislação Aplicada à TI (LEGTI,
4 questões, peso 2): 16 dos ~115 pontos, em 19 normas.

Página para visualizar: https://claude.ai/artifact/NAoePLNHEiHicrravEGhYy

## Situação — 26/09/2026

| Fase | Estado |
|---|---|
| 0 · Catálogo e cenários | ✅ as normas do edital em `conteudo/leis/README.md` (17), grupo L no `e2e/CENARIOS.md`, falhas da captura em `edital-processor/app/leis/README.md` |
| 1 · Captura | ✅ CF/88, Lei 16.168, LGPD e Marco Civil capturadas **com conferência do Gemini**, texto conferido com o original; faltam os links da Constituição de Goiás e do Regimento Interno |
| 2 · Leitura no app | ✅ migration 000008, captura pela tela (**Legislação → Adicionar lei**, qualquer conta logada, por ora), menu Legislação, leitor com sumário e link direto; E2E L1–L16 verdes |
| 3 · Questões A | ✅ 228 questões em 22 unidades (CF 48, Lei 16.168 133, LGPD 22, Marco Civil 25), em `conteudo/leis/<slug>/questoes.json`; entram pela página da lei |
| 4 · No ar | ⏳ falta capturar as quatro leis no servidor e importar as questões delas (roteiro em `docs/deploy.md`) |
| 5 · B e C | ⏳ com você: levantar os links e capturar pela tela; lei sem questões já é lida normalmente |

O que mudou em relação ao desenho abaixo: em 26/09/2026 a captura saiu da
linha de comando e virou tela — cola-se o link, o `edital-processor` devolve a
prévia, e quem publica é o backend. Com isso saíram o `normas.toml`, o pacote
(`studygo.lei/1` num arquivo) e os `make leis-*`. As refs citadas pela questão
ficam num array da própria questão, e não em tabela à parte; a divergência
entre regra e Gemini só pesa quando a estrutura está em jogo e o palpite dele é
possível — e vira aviso na prévia, marcado como revisado antes de publicar.

## Decisões

- A lei é **baixada e organizada pelo processador, a pedido da tela**; a
  pessoa revisa a prévia antes de publicar (até 25/09/2026 era local, por
  linha de comando).
- O Gemini **só classifica** os parágrafos numerados (artigo, inciso,
  capítulo…); nunca devolve texto. O texto final vem do original e só é gravado
  se conferir 100% com ele (hash).
- **Questões só da prioridade A por enquanto**, por capítulo/seção do recorte do
  edital, estilo **FCC (A–E)**, feitas pelo **Claude Code localmente**. Cada uma
  cita o trecho literal que a justifica.
- As normas B e C (e as A sem fonte confirmada) entram pelo **link que você
  cola na tela**; delas, por enquanto, só a leitura.
- Fora do escopo: TEC, Qconcursos, importar pelo link dentro do app, gerar
  questões em produção ou com o Gemini, questões por artigo/inciso, normas que o
  edital não cita.

## Normas

| Prior. | Norma | Fonte | Link |
|---|---|---|---|
| A | CF/88 — Administração Pública (arts. 37–43) e fiscalização (arts. 70–75) | Planalto | confirmado |
| A | Lei Orgânica do TCE-GO — Lei 16.168/2007 (148 arts.) | Casa Civil GO (API) | confirmado |
| A | LGPD — Lei 13.709/2018 (recorte técnico) | Planalto | confirmado |
| A | Marco Civil — Lei 12.965/2014 | Planalto | confirmado |
| A | Constituição de Goiás — dispositivos do TCE-GO | — | **você informa** |
| A | Regimento Interno do TCE-GO — Res. 22/2008 | — | **você informa** |
| B | Lei 20.756/2020 (estatuto GO) · Lei 15.122/2005 (PCCR TCE-GO) · LC 205/2025 · RA 17/2024 (Política de SI) | — | **você informa** |
| C | RA 14/2024 · RA 15/2024 · RA 14/2025 · RN 13/2016 (CETI) · Código de Ética · políticas institucionais · PDTI 2025–2026 (só leitura) | — | **você informa** |

"Segurança em contratações de TIC" e "certificação digital" não nomeiam norma:
ficam como conteúdo técnico da disciplina.

---

## Fase 0 — Catálogo e cenários · dia 1

Antes de qualquer código.

- `conteudo/leis/normas.toml`: as 19 normas (slug, nome, disciplina,
  prioridade, fonte, **link**, recorte). As 4 A confirmadas já preenchidas; as
  demais com `link:` vazio para você preencher.
- `e2e/CENARIOS.md`, grupo **L**: como a legislação pode quebrar para quem usa
  (importar pacote; versão repetida não duplica; versão nova preserva respostas;
  artigo traz as questões que o citam; resposta gravada; qualquer conta importa;
  link direto para o dispositivo; sugestão de vínculo pelo tópico).
- Lista de como a captura erra, por fonte: riscado, entidades, linha partida,
  "Art. 1º-A", parágrafo único, revogado, cabeçalho fora do padrão, PDF.

**Pronto quando:** você revisou `normas.toml` e os cenários.

## Fase 1 — Captura local · semana 1

Ferramenta de linha de comando no `edital-processor` (reaproveita o Gemini e a
leitura de PDF que já existem; sem rota HTTP).

- Adaptadores: **Planalto** (HTML), **Casa Civil GO** (API JSON, lei
  compilada) e **link genérico** (HTML ou PDF) para os links que você informar.
- Passos: baixar e guardar o original com hash → limpar sem IA (encoding,
  entidades, linhas partidas; riscado vira "redação anterior"; notas de
  redação viram anotação) → numerar parágrafos → Gemini classifica ids (em lotes
  por Título) → verificar → gravar.
- Verificações que bloqueiam a gravação: cada parágrafo usado uma vez e em
  ordem; hash do texto remontado igual ao do texto limpo; hierarquia válida;
  artigos em sequência; regra independente ("Art. N" no começo ⇒ artigo).
- Saída: `conteudo/leis/<slug>/lei.json` (árvore com `ref` estável, ex.
  `art71.inc2`) e `captura.md` (contagens, hashes, divergências).
- `make leis-capturar slug=…` e `make leis-capturar prioridade=A`.

**Pronto quando:** as normas A capturadas com integridade 100% (as duas sem
link, assim que você informar).

## Fase 2 — Leitura interativa no app · semana 2

Ainda sem questões.

- Migrations a partir da **000008**: `leis`, `leis_versoes`,
  `leis_dispositivos`, `disciplinas_leis`.
- Importar pacote (um JSON por lei, formato `studygo.lei/1`) — qualquer conta
  logada, enquanto o app é de teste; mesma versão não duplica; versão nova
  vira a ativa.
- Menu "Meu concurso → **Legislação**"; vínculo com a matéria **sugerido pelo
  tópico** ("nº 16.168" no tópico ⇒ Lei 16.168), você confirma.
- `/leis/<slug>`: sumário navegável, texto em hierarquia, link direto por
  dispositivo (`/leis/cf88#art71.inc2`), notas discretas, redação anterior
  recolhida.

**Pronto quando:** `make check`, `make check-db` e `make e2e` verdes (cenários
L de leitura) e você aprovou a leitura no ambiente local.

## Fase 3 — Questões da prioridade A · semanas 3–4

Feitas pelo Claude Code, na sua máquina.

- Roteiro versionado como skill do repositório
  (`.claude/skills/questoes-de-lei/`): estilo FCC (A–E); literalidade, troca de
  palavra, competência, prazo, exceção; ~1 questão por 120 palavras, de 5 a 20
  por unidade.
- Unidades do recorte: CF arts. 37–43 (dividida por artigo-bloco, ~5.200
  palavras), CF arts. 70–75, Lei 16.168 por título, LGPD no recorte técnico,
  Marco Civil, Constituição GO (dispositivos do TCE) e Regimento Interno.
- Cada questão: enunciado, alternativas, gabarito, comentário, dispositivos
  citados e **trecho literal**; `make leis-validar` confere tudo (o trecho tem
  de existir nos dispositivos citados). Você revisa e marca as ruins.
- No app: clicar no artigo mostra as questões que o citam; responder mostra
  gabarito, comentário e trecho grifado; progresso por unidade e selo por
  artigo; filtro "só o que errei". Tabelas `leis_questoes`,
  `leis_questao_dispositivos`, `leis_respostas`.

**Pronto quando:** todas as unidades A com questões validadas e os cenários L
de resposta verdes.

## Fase 4 — No ar · fim da semana 4

- No servidor: **Legislação → Adicionar lei** com os links de
  `conteudo/leis/README.md`; revisar os avisos (os já conhecidos estão lá) e
  publicar; depois, na página de cada lei, importar o `questoes.json` dela.

**Pronto quando:** as normas A com questões no ar no servidor.

## Fase 5 — Normas B e C · conforme você informar os links

- Você cola o link em **Legislação → Adicionar lei**, revisa e publica **só a
  leitura**; o vínculo com a matéria sai do tópico, como nas A.
- Se o TCE-GO só tiver PDF escaneado: a captura recusa (bloqueio); texto
  conferido à mão fica para as normas C, no pior caso.

## Depois (não agora)

Questões das normas B e C; questão errada entra no caderno de erros e na
revisão do dia; o cronograma abre a lei no tópico do dia; aviso quando uma
norma mudar na fonte.

## Riscos

- **O Gemini classifica errado** → o texto não muda; as verificações barram a
  gravação e o `captura.md` mostra o que revisar.
- **Uma fonte muda de formato** → o adaptador falha dizendo o motivo; o original
  fica guardado.
- **Norma só em PDF escaneado** → OCR do processador; no pior caso, conferência
  manual.
- **Questão ruim** → trecho literal obrigatório, validador, sua revisão, e
  desativar sem apagar respostas.
- **Norma alterada depois** → hash por unidade marca as questões
  desatualizadas; recapturar pela tela quando a norma mudar.
