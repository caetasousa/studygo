"""Os mapas mentais em andamento: o PDF de uma aula vira mapa com o Claude Code.

O backend entrega o PDF (POST /internal/mapas/processamentos) e o serviço o põe
numa fila em memória, atendida um de cada vez: uma aula ocupa o Claude por
dezenas de minutos, e duas ao mesmo tempo não cabem na memória do servidor. Ao
terminar, o resultado vai ao backend pela porta interna dele, que o importa na
conta — este serviço nunca toca no banco. Se a importação recusa (outline
inválido, slug de outro mapa…), o motivo volta ao Claude, na mesma sessão, para
corrigir.

Perder a fila num reinício é aceitável: ao subir, o serviço pede ao backend o
que estava processando (redespacho), e o backend entrega de novo.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import sys
import tempfile
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from pathlib import Path

from app.core.config import Settings
from app.core.logging import get_logger
from app.mapas.conexao import Conexoes

_log = get_logger("mapas")

AQUI = Path(__file__).resolve().parent
FERRAMENTAS = AQUI / "ferramentas"
INSTRUCOES = AQUI / "instrucoes"


@dataclass(frozen=True)
class Trabalho:
    """Um PDF que o backend entregou para virar mapa."""

    pedido: str
    dono: str
    arquivo: str
    pdf: bytes
    materia: str = ""
    temas: tuple[str, ...] = ()
    slugs_existentes: tuple[str, ...] = ()
    token_claude: str = ""


@dataclass(frozen=True)
class Resposta:
    """O que o backend respondeu à entrega: o status e a mensagem."""

    status: int
    mensagem: str


@dataclass(frozen=True)
class Saida:
    """O que o Claude escreveu na pasta de saída, pronto para o backend."""

    mapa: str
    questoes: dict[str, object] | None
    temas: list[str]
    relatorio: str
    imagens: list[tuple[str, bytes]] = field(default_factory=list)


class ClaudeFalhou(Exception):
    """O Claude terminou com erro: o motivo vai para o pedido."""


# A ponte com o backend: entregar o resultado, avisar a falha. Injetada para o
# teste trocar a rede por um dublê (o backend é a fronteira de fora daqui).
Entregar = Callable[[str, Saida], Awaitable[Resposta]]
Avisar = Callable[[str, str], Awaitable[None]]
# Roda o Claude: (argumentos, mensagem, pasta, ambiente) -> (código, saída).
RodarClaude = Callable[[list[str], str, Path, dict[str, str]], Awaitable[tuple[int, str]]]


class Mapas:
    def __init__(
        self,
        settings: Settings,
        entregar: Entregar,
        avisar_falha: Avisar,
        rodar_claude: RodarClaude,
        conexoes: Conexoes | None = None,
    ) -> None:
        self._settings = settings
        self._entregar = entregar
        self._avisar_falha = avisar_falha
        self._rodar_claude = rodar_claude
        self._conexoes = conexoes
        self._fila: asyncio.Queue[Trabalho] = asyncio.Queue()
        self._na_fila: set[str] = set()
        self._consumidor: asyncio.Task[None] | None = None

    def iniciar(self, trabalho: Trabalho) -> None:
        """Põe o PDF na fila. O mesmo pedido entregue de novo (redespacho) não duplica."""
        if trabalho.pedido in self._na_fila:
            return
        self._na_fila.add(trabalho.pedido)
        self._fila.put_nowait(trabalho)
        if self._consumidor is None or self._consumidor.done():
            self._consumidor = asyncio.get_running_loop().create_task(self._consumir())

    async def esperar_fila(self) -> None:
        """Espera a fila esvaziar (para os testes e o desligamento)."""
        await self._fila.join()

    async def parar(self) -> None:
        if self._consumidor is not None:
            self._consumidor.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._consumidor

    async def _consumir(self) -> None:
        while True:
            trabalho = await self._fila.get()
            try:
                await self._processar(trabalho)
            except Exception:
                # Nada aqui pode matar a fila: o próximo PDF tem de ser atendido.
                _log.exception("processamento de mapa falhou", extra={"stage": "mapas"})
            finally:
                self._na_fila.discard(trabalho.pedido)
                self._fila.task_done()

    async def _processar(self, t: Trabalho) -> None:
        base = Path(self._settings.work_dir) / "mapas"
        base.mkdir(parents=True, exist_ok=True)
        trabalho = Path(tempfile.mkdtemp(prefix="mapa-", dir=base))
        try:
            await self._processar_em(t, trabalho)
        finally:
            # O PDF é material pago: não fica no disco depois do pedido.
            shutil.rmtree(trabalho, ignore_errors=True)

    async def _processar_em(self, t: Trabalho, trabalho: Path) -> None:
        # Sem token e sem conexão, o Claude falharia depois de subir: melhor
        # dizer logo o que falta.
        if (
            not t.token_claude
            and self._conexoes is not None
            and not (await self._conexoes.situacao(t.dono)).conectado
        ):
            await self._avisar_falha(
                t.pedido,
                "O Claude desta conta não está conectado. Conecte-o em Configurações → "
                'Processador de mapas e use "Pôr na fila de novo".',
            )
            return

        saida = trabalho / "saida"
        (saida / "imagens").mkdir(parents=True)
        pdf = trabalho / "aula.pdf"
        pdf.write_bytes(t.pdf)
        sistema = trabalho / "instrucoes.md"
        sistema.write_text(instrucoes_do_sistema(), encoding="utf-8")

        sessao = str(uuid.uuid4())
        mensagem = montar_prompt(t, self._settings, pdf, trabalho, saida)
        ambiente = self._ambiente(t)

        for tentativa in range(1, self._settings.mapas_tentativas + 1):
            try:
                await self._claude(sessao, mensagem, tentativa == 1, trabalho, sistema, ambiente)
                resultado = ler_saida(saida)
            except ClaudeFalhou as exc:
                await self._avisar_falha(t.pedido, str(exc))
                return

            resposta = await self._entregar(t.pedido, resultado)
            if resposta.status == 200:
                _log.info("mapa entregue", extra={"stage": "mapas", "pedido": t.pedido})
                return
            if resposta.status in (404, 409):
                # Excluído ou já resolvido do lado de lá: não há a quem entregar.
                _log.info("pedido não espera mais o mapa", extra={"stage": "mapas"})
                return
            if resposta.status != 422:
                await self._avisar_falha(
                    t.pedido,
                    f"o backend não aceitou o mapa ({resposta.status}): {resposta.mensagem}",
                )
                return

            mensagem = (
                f"O backend recusou a importação: {resposta.mensagem}\n"
                f"Corrija os arquivos em {saida} e termine."
            )

        await self._avisar_falha(
            t.pedido,
            f"depois de {self._settings.mapas_tentativas} tentativas, a importação ainda recusa: "
            f"{resposta.mensagem}",
        )

    def _ambiente(self, t: Trabalho) -> dict[str, str]:
        # O Claude da conta: a pasta da conexão dela, feita pela tela. O token
        # guardado em Configurações, se houver, tem precedência.
        ambiente = self._conexoes.ambiente(t.dono) if self._conexoes else dict(os.environ)
        if t.token_claude:
            ambiente["CLAUDE_CODE_OAUTH_TOKEN"] = t.token_claude
        if self._settings.mapa_comprador:
            ambiente["MAPA_COMPRADOR"] = self._settings.mapa_comprador
        return ambiente

    async def _claude(
        self,
        sessao: str,
        mensagem: str,
        primeira: bool,
        trabalho: Path,
        sistema: Path,
        ambiente: dict[str, str],
    ) -> None:
        args = argumentos_do_claude(self._settings, sessao, primeira, trabalho, sistema)
        codigo, saida = await self._rodar_claude(args, mensagem, trabalho, ambiente)
        if codigo == 0:
            return
        if '"api_error_status":401' in saida.replace(" ", ""):
            if "CLAUDE_CODE_OAUTH_TOKEN" in ambiente:
                raise ClaudeFalhou(
                    "o Claude recusou o token guardado em Configurações (401). Remova-o e "
                    "conecte o Claude pela mesma tela."
                )
            raise ClaudeFalhou(
                "o Claude recusou a conexão desta conta (401). Desconecte e conecte de novo em "
                "Configurações → Processador de mapas."
            )
        raise ClaudeFalhou(f"o Claude terminou com erro {codigo}: {saida[-1500:]}")


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
    provas = str(settings.provasgo_dir) if settings.provasgo_dir else ""
    return (
        (INSTRUCOES / "prompt.md")
        .read_text(encoding="utf-8")
        .format(
            pdf=pdf,
            arquivo=t.arquivo,
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
    settings: Settings, sessao: str, primeira: bool, trabalho: Path, sistema: Path
) -> list[str]:
    """`claude -p` sem perguntas: só arquivos e o Python das ferramentas, na pasta do pedido."""
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


async def rodar_claude_de_verdade(
    args: list[str], mensagem: str, pasta: Path, ambiente: dict[str, str], prazo: float
) -> tuple[int, str]:
    proc = await asyncio.create_subprocess_exec(
        *args,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        cwd=pasta,
        env=ambiente,
    )
    try:
        saida, erro = await asyncio.wait_for(proc.communicate(mensagem.encode()), prazo)
    except TimeoutError:
        proc.kill()
        await proc.wait()
        return 124, "o Claude passou do prazo e foi interrompido"
    texto = saida.decode(errors="replace")
    return proc.returncode or 0, texto if proc.returncode == 0 else (
        texto + erro.decode(errors="replace")
    )
