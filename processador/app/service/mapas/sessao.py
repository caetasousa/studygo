"""A sessão do Claude para um pedido: o que ele recebe e o que devolve.

As instruções (a skill e o formato) vão como instrução de sistema; a mensagem
diz onde está o PDF e onde escrever; os argumentos deixam o Claude mexer só na
pasta do pedido e rodar só o Python das ferramentas. O que ele escreve na pasta
de saída é lido de volta, e o que falta vira erro para ele corrigir.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

from app.domain.mapas.pedido import ClaudeFalhou, Saida, Trabalho
from app.platform.config import Settings

AQUI = Path(__file__).resolve().parent
FERRAMENTAS = AQUI / "ferramentas"
INSTRUCOES = AQUI / "instrucoes"


def instrucoes_do_sistema() -> str:
    """A skill e o formato, como instrução de sistema da sessão do Claude."""
    return (
        (INSTRUCOES / "skill.md").read_text(encoding="utf-8")
        + "\n\n# O formato (conteudo/mapas/README.md)\n\n"
        + (INSTRUCOES / "formato.md").read_text(encoding="utf-8")
        + "\n\n# Nesta execução\n\nVocê roda sem ninguém olhando, dentro do processador do "
        "studygo. Não suba stack, não importe nada e não procure o repositório: quem importa é o "
        "backend, que devolve os erros se houver."
    )


def montar_prompt(t: Trabalho, settings: Settings, pdf: Path, trabalho: Path, saida: Path) -> str:
    """O pedido ao Claude: o da aula (prompt.md) ou, sem aula, o do assunto
    (prompt_assunto.md), que manda fazer o mapa pelas fontes oficiais."""
    provas = str(settings.provasgo_dir) if settings.provasgo_dir else ""
    modelo = "prompt_assunto.md" if t.sem_aula else "prompt.md"
    return (
        (INSTRUCOES / modelo)
        .read_text(encoding="utf-8")
        .format(
            pdf=pdf,
            arquivo=t.arquivo,
            assunto=t.assunto,
            materia=t.materia or "nenhuma (o mapa entra sem vínculo)",
            temas=json.dumps(list(t.temas), ensure_ascii=False) if t.temas else "nenhum",
            slugs=", ".join(sorted(t.slugs_existentes)) or "nenhum",
            provas=provas or "não configurado (sem questões do provasGo)",
            saida=saida,
            trabalho=trabalho,
            python=sys.executable,
            ferramentas=FERRAMENTAS,
        )
    )


def argumentos_do_claude(
    settings: Settings, sessao: str, primeira: bool, trabalho: Path, sistema: Path, web: bool
) -> list[str]:
    """`claude -p` sem perguntas: só arquivos e o Python das ferramentas, na pasta do pedido.

    Com `web` (o pedido sem aula), também a busca e a leitura de páginas: sem
    aula, a fonte são as normas e a documentação oficial, não a memória do
    modelo. O pedido de PDF não precisa da web, e não a ganha."""
    args = [
        settings.claude_bin,
        "-p",
        "--output-format",
        "json",
        "--permission-mode",
        "dontAsk",
        "--append-system-prompt-file",
        str(sistema),
        "--add-dir",
        str(trabalho),
        "--add-dir",
        str(FERRAMENTAS),
    ]
    if settings.provasgo_dir:
        args += ["--add-dir", str(settings.provasgo_dir)]
    args += ["--session-id", sessao] if primeira else ["--resume", sessao]
    args += [
        "--allowedTools",
        "Read",
        "Write",
        "Edit",
        "Glob",
        "Grep",
        f"Bash({sys.executable}:*)",
        "Bash(ls:*)",
        "Bash(mkdir:*)",
    ]
    if web:
        args += ["WebSearch", "WebFetch"]
    return args


def ler_saida(saida: Path) -> Saida:
    """Lê o que o Claude escreveu; o que falta vira erro para ele corrigir."""
    md = saida / "mapa.md"
    if not md.exists():
        raise ClaudeFalhou("o Claude terminou sem escrever o mapa (mapa.md)")

    questoes: dict[str, object] | None = None
    qj = saida / "questoes.json"
    if qj.exists():
        try:
            lido = json.loads(qj.read_text(encoding="utf-8"))
        except ValueError as exc:
            raise ClaudeFalhou(f"o questoes.json não é um JSON válido: {exc}") from exc
        questoes = lido if isinstance(lido, dict) else None

    temas: list[str] = []
    tj = saida / "topicos.json"
    if tj.exists():
        try:
            lido = json.loads(tj.read_text(encoding="utf-8"))
        except ValueError as exc:
            raise ClaudeFalhou(f"o topicos.json não é um JSON válido: {exc}") from exc
        if isinstance(lido, dict):
            temas = [str(x) for x in lido.get("temas", [])]

    rel = saida / "relatorio.md"
    pasta = saida / "imagens"
    imagens = sorted((p.name, p.read_bytes()) for p in pasta.glob("*") if p.is_file())
    return Saida(
        mapa=md.read_text(encoding="utf-8"),
        questoes=questoes,
        temas=temas,
        relatorio=rel.read_text(encoding="utf-8")[:19000] if rel.exists() else "",
        imagens=imagens,
    )
