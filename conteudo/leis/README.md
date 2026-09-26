# Leis do edital

As normas que o edital do TCE-GO (Técnico de Controle Externo – TI, FCC) cobra
nas disciplinas LEG (Legislação Institucional) e LEGTI (Legislação Aplicada à
TI), e o que usar em **Legislação → Adicionar lei** para cada uma.

A lei entra pelo tópico do edital, em **Legislação → Pesquisar e importar**: o
app acha a fonte e guarda só o que o tópico pede. Os links abaixo servem para a
norma que a pesquisa não acha (a Constituição de Goiás, as resoluções do TCE). As
questões são escritas fora do app (`.claude/skills/questoes-de-lei`), ficam em
`<slug>/questoes.json` e entram pela página da lei, em **Manter esta lei →
Importar questões**. O `<slug>/lei.json` é o texto de referência para quem
escreve as questões.

## Prioridade A — com questões

| Lei (nome curto) | Link para colar | Sugerir quando o tópico citar | Recorte das questões |
|---|---|---|---|
| Constituição Federal | https://www.planalto.gov.br/ccivil_03/constituicao/constituicao.htm | Constituição da República Federativa do Brasil, Constituição Federal | arts. 37–43 e 70–75 |
| Lei Orgânica do TCE-GO | https://legisla.casacivil.go.gov.br/pesquisa_legislacao/86708 | 16.168 | a lei inteira |
| LGPD | https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm | 13.709 | arts. 5–6, 12–13 e 46–51 |
| Marco Civil da Internet | https://www.planalto.gov.br/ccivil_03/_ato2011-2014/2014/lei/l12965.htm | 12.965 | arts. 1–17 |
| Constituição de Goiás | *link a informar* | Constituição do Estado de Goiás | os dispositivos do Tribunal de Contas |
| Regimento Interno do TCE-GO | *link a informar* | Regimento Interno do Tribunal de Contas do Estado de Goiás, Resolução nº 22 | a norma inteira |

Avisos que já foram revisados (a regra estava certa) e voltam a cada captura:

- **Constituição Federal** — `p0010: a regra diz descartar`: é o link do
  sumário do Planalto para o ADCT, não o ADCT.
- **LGPD** — `p0649` e `p0653: a regra diz solto`: incisos da Lei 12.965
  citados entre aspas pelo art. 60, que a altera; não são incisos da LGPD.

## Prioridades B e C — só leitura, por enquanto

| Lei (nome curto) | Sugerir quando o tópico citar |
|---|---|
| Estatuto dos servidores de GO (Lei 20.756/2020) | 20.756 |
| PCCR do TCE-GO (Lei 15.122/2005) | 15.122 |
| LC estadual 205/2025 | Lei Complementar estadual nº 205 |
| RA 17/2024 (Segurança da Informação) | Resolução Administrativa nº 17/2024 |
| RA 14/2024 (Governança) | Resolução Administrativa nº 14/2024 |
| RA 15/2024 (Planejamento e Gestão) | Resolução Administrativa nº 15/2024 |
| RA 14/2025 (Diretoria de TI) | Resolução Administrativa nº 14/2025 |
| RN 13/2016 (CETI) | Resolução Normativa nº 13/2016 |
| Código de Ética do TCE-GO | Código de Ética dos Servidores do Tribunal de Contas |
| Políticas institucionais do TCE-GO | Políticas institucionais do Tribunal de Contas |
| PDTI 2025–2026 | Plano Diretor de Tecnologia da Informação |

Os links destas ainda não foram levantados. Qualquer página .gov.br, .leg.br
ou .jus.br serve, em HTML ou PDF com texto; PDF escaneado não passa.
