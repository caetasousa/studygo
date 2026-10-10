# Mapas mentais a partir do PDF da aula

O PDF que quem estuda envia em **Mapas mentais → Criar mapa a partir do PDF da
aula** vira mapa aqui, com o Claude Code e a skill `mapa-mental`. Este serviço
nunca toca no banco: o resultado volta ao backend, que o importa na conta pelas
mesmas regras da tela.

```
tela ──PDF──▶ backend ──POST /internal/mapas/processamentos (202)──▶ edital-processor
                 ▲                                                    │ claude -p
                 └──── POST :8081/internal/pedidos-de-mapa/{id}/… ────┘ (resultado ou falha)
```

- **Por evento.** O backend entrega o PDF assim que ele chega; nada consulta
  fila. Aqui ele entra numa fila em memória, atendida um PDF por vez (uma aula
  ocupa o Claude por dezenas de minutos, e duas não cabem na memória do
  servidor). Ao subir, o serviço pede ao backend o que estava "processando"
  (`/redespacho`): um reinício não perde pedido.
- **A porta interna do backend** (`INTERNAL_ADDR`, `:8081`) não é publicada nem
  passa pelo nginx; o token é o mesmo `EP_SERVICE_TOKEN`, no sentido contrário.
- **Recusa vira correção.** Se a importação recusa o mapa (422: linha inválida,
  slug de outro mapa…), o motivo volta ao Claude na mesma sessão (`--resume`),
  até `EP_MAPAS_TENTATIVAS` vezes; depois, o pedido falha com o motivo.
- **O PDF não fica.** Cada pedido trabalha numa pasta própria em
  `EP_WORK_DIR/mapas`, apagada ao terminar, dê certo ou não.

## A conexão do Claude

O Claude Code vem na imagem, com a versão fixada no `Dockerfile` (sem
atualização automática). Cada conta conecta a sua assinatura **pela tela**, em
**Configurações → Processador de mapas → Conectar o Claude** — sem terminal:

1. o backend pede o link (`POST /internal/claude/conexao`); `conexao.py` roda o
   `claude auth login` num pseudo-terminal, com `CLAUDE_CONFIG_DIR` na pasta da
   conta (`EP_CLAUDE_CONTAS_DIR/<id da conta>`, no volume `edital_claude`), e
   devolve o link que ele imprime;
2. quem estuda autoriza na página do Claude e cola o código na tela, que chega
   ao processo como se fosse digitado (`POST /internal/claude/conexao/codigo`).
   Código recusado: o Claude volta a pedir, e o mesmo link serve.

Todo mapa da conta roda com a pasta dela: a assinatura de uma conta nunca serve
à outra. O token de `claude setup-token`, se guardado em Configurações, vale no
lugar da conexão. Sem nenhum dos dois, o pedido falha na hora dizendo para
conectar; um 401 do Claude também vira falha com o que fazer.

## Arquivos

| | Caminho | O que é |
|---|---|---|
| 🧠 | `servico.py` | a fila, o `claude -p` e a leitura do que ele escreveu |
| 🔑 | `conexao.py` | a conexão do Claude de cada conta: o login num pseudo-terminal, a situação, a saída |
| 🔌 | `backend.py` | a entrega ao backend, com insistência (ele pode estar no meio de um deploy) |
| 📝 | `instrucoes/prompt.md` | o pedido ao Claude: onde está o PDF, onde escrever, as ferramentas |
| 📋 | `instrucoes/skill.md`, `instrucoes/formato.md` | cópias da skill e do formato — um teste falha se divergirem dos originais (`.claude/skills/mapa-mental/SKILL.md`, `conteudo/mapas/README.md`) |
| 🧰 | `ferramentas/` | `texto.py` (o texto sem os dados do comprador), `figura.py` (recorte), `provas.py` (questões do provasGo) |

## Configuração (`EP_`)

- `BACKEND_INTERNAL_URL` — onde devolver o mapa (`http://backend:8081`). Vazio
  desliga a entrega (o processamento falha sem ter a quem entregar).
- `MAPAS_TENTATIVAS` (3), `MAPAS_TIMEOUT_SECONDS` (3 h), `MAPAS_MAX_PDF_BYTES` (40 MiB).
- `CLAUDE_CONTAS_DIR` — onde fica a conexão de cada conta (no volume).
- `PROVASGO_DIR` — o export do provasGo, se montado (na stack local, de
  `~/.local/share/studygo/provasgo`).
- `MAPA_COMPRADOR` — o nome que o material pago traz no rodapé, para a
  ferramenta de texto tirar também. Vem do `.env` (`MAPA_COMPRADOR`), nunca do
  repositório.
