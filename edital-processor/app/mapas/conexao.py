"""A conexão do Claude de cada conta, feita pela tela.

O `claude auth login` foi feito para um terminal: imprime o link de
autorização, espera o código colado e grava o login na pasta de configuração.
Aqui ele roda num pseudo-terminal, um por conta: a tela pede o link, quem
estuda autoriza no navegador e cola o código na tela, e o código chega ao
processo como se tivesse sido digitado.

Cada conta tem a sua pasta (`claude_contas_dir/<dono>`), que vira o
`CLAUDE_CONFIG_DIR` do login e de todo mapa dela: a assinatura de uma conta
nunca serve à outra. A pasta fica num volume, e o login sobrevive ao deploy.
"""

from __future__ import annotations

import asyncio
import contextlib
import fcntl
import json
import os
import pty
import re
import shutil
import signal
import struct
import subprocess
import termios
import threading
import time
from dataclasses import dataclass, field
from pathlib import Path

from fastapi import status

from app.core.config import Settings
from app.core.errors import ProcessorError
from app.core.logging import get_logger

_log = get_logger("mapas")

# O dono é o id da conta no backend: nada que vire caminho.
_DONO = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$")
# O link vem também dentro de um hyperlink de terminal (OSC 8); o primeiro basta.
_LINK = re.compile(rb"https://[^\s\x07\x1b]+/oauth/authorize[^\s\x07\x1b]*")
_SEQUENCIA = re.compile(r"\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[A-Za-z]|\r")

# O que o Claude diz quando não aceita o código — e continua esperando outro.
_RECUSA = re.compile(rb"(?i)invalid|error|failed|expired")

_ESPERA_DO_LINK = 30.0
_ESPERA_DO_CODIGO = 90.0
# Um link pedido e esquecido não deixa processo pendurado para sempre.
_VALIDADE_DO_LINK = 15 * 60.0


class DonoInvalido(ProcessorError):
    code = "dono_invalido"
    http_status = status.HTTP_400_BAD_REQUEST


class ConexaoNaoIniciada(ProcessorError):
    code = "conexao_nao_iniciada"
    http_status = status.HTTP_409_CONFLICT


class CodigoRecusado(ProcessorError):
    code = "codigo_recusado"
    http_status = status.HTTP_422_UNPROCESSABLE_CONTENT


class ClaudeSemLink(ProcessorError):
    code = "claude_sem_link"
    http_status = status.HTTP_502_BAD_GATEWAY
    transient = True


@dataclass(frozen=True)
class Situacao:
    conectado: bool
    email: str = ""
    plano: str = ""


@dataclass
class _Sessao:
    proc: subprocess.Popen[bytes]
    mestre: int
    criada: float = field(default_factory=time.monotonic)
    saida: bytearray = field(default_factory=bytearray)
    mudou: threading.Condition = field(default_factory=threading.Condition)

    def ler(self) -> None:
        """Drena o terminal até o processo fechá-lo (sem isso, ele trava escrevendo)."""
        while True:
            try:
                pedaco = os.read(self.mestre, 4096)
            except OSError:
                pedaco = b""
            with self.mudou:
                if pedaco:
                    self.saida += pedaco
                self.mudou.notify_all()
            if not pedaco:
                return

    def esperar_link(self, prazo: float) -> str | None:
        fim = time.monotonic() + prazo
        with self.mudou:
            while True:
                achado = _LINK.search(self.saida)
                if achado:
                    return achado.group(0).decode()
                resta = fim - time.monotonic()
                if resta <= 0 or self.proc.poll() is not None:
                    return None
                self.mudou.wait(min(resta, 0.5))

    def texto(self) -> str:
        with self.mudou:
            bruto = self.saida.decode(errors="replace")
        return _SEQUENCIA.sub("", bruto).strip()

    def encerrar(self) -> None:
        if self.proc.poll() is None:
            with contextlib.suppress(ProcessLookupError):
                os.killpg(self.proc.pid, signal.SIGTERM)
            with contextlib.suppress(subprocess.TimeoutExpired):
                self.proc.wait(timeout=5)
        with contextlib.suppress(OSError):
            os.close(self.mestre)


class Conexoes:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._sessoes: dict[str, _Sessao] = {}
        self._trava = threading.Lock()

    def pasta(self, dono: str) -> Path:
        if not _DONO.fullmatch(dono):
            raise DonoInvalido("conta inválida")
        return Path(self._settings.claude_contas_dir) / dono

    def ambiente(self, dono: str) -> dict[str, str]:
        """O ambiente do Claude desta conta: a pasta dela, e nenhum token de fora."""
        amb = dict(os.environ)
        amb.pop("CLAUDE_CODE_OAUTH_TOKEN", None)
        amb["CLAUDE_CONFIG_DIR"] = str(self.pasta(dono))
        return amb

    async def situacao(self, dono: str) -> Situacao:
        amb = self.ambiente(dono)
        if not self.pasta(dono).exists():
            return Situacao(conectado=False)
        proc = await asyncio.create_subprocess_exec(
            self._settings.claude_bin,
            "auth",
            "status",
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.DEVNULL,
            env=amb,
        )
        try:
            saida, _ = await asyncio.wait_for(proc.communicate(), 30)
        except TimeoutError:
            proc.kill()
            await proc.wait()
            return Situacao(conectado=False)
        try:
            dados = json.loads(saida.decode(errors="replace"))
        except ValueError:
            return Situacao(conectado=False)
        if not isinstance(dados, dict) or not dados.get("loggedIn"):
            return Situacao(conectado=False)
        return Situacao(
            conectado=True,
            email=str(dados.get("email") or ""),
            plano=str(dados.get("subscriptionType") or ""),
        )

    async def iniciar(self, dono: str) -> str:
        """Começa o login e devolve o link de autorização."""
        self.pasta(dono).mkdir(parents=True, exist_ok=True)
        return await asyncio.to_thread(self._iniciar, dono)

    def _iniciar(self, dono: str) -> str:
        self._encerrar(dono)
        self._expirar()
        mestre, escravo = pty.openpty()
        # Largo o bastante para o link sair numa linha só.
        fcntl.ioctl(escravo, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 4000, 0, 0))
        try:
            proc = subprocess.Popen(
                [self._settings.claude_bin, "auth", "login", "--claudeai"],
                stdin=escravo,
                stdout=escravo,
                stderr=escravo,
                env=self.ambiente(dono),
                cwd=self.pasta(dono),
                start_new_session=True,
                close_fds=True,
            )
        finally:
            os.close(escravo)
        sessao = _Sessao(proc=proc, mestre=mestre)
        threading.Thread(target=sessao.ler, daemon=True).start()

        link = sessao.esperar_link(_ESPERA_DO_LINK)
        if link is None:
            texto = sessao.texto()
            sessao.encerrar()
            _log.warning("o login do Claude não mostrou o link", extra={"stage": "mapas"})
            raise ClaudeSemLink(f"o Claude não mostrou o link de autorização: {texto[-300:]}")
        with self._trava:
            self._sessoes[dono] = sessao
        return link

    async def concluir(self, dono: str, codigo: str) -> Situacao:
        """Entrega o código colado na tela ao login que espera por ele."""
        self.pasta(dono)
        with self._trava:
            sessao = self._sessoes.pop(dono, None)
        if sessao is None or sessao.proc.poll() is not None:
            if sessao is not None:
                sessao.encerrar()
            raise ConexaoNaoIniciada("peça o link de novo: este já não espera o código")
        resposta = await asyncio.to_thread(self._digitar, sessao, codigo)
        s = await self.situacao(dono)
        if s.conectado:
            sessao.encerrar()
            return s
        if sessao.proc.poll() is None:
            # O Claude recusou e voltou a pedir o código: o mesmo link serve.
            with self._trava:
                self._sessoes[dono] = sessao
            raise CodigoRecusado(
                "o Claude não aceitou o código. Copie o código inteiro da página de autorização "
                f"e cole de novo. ({resposta})"
            )
        sessao.encerrar()
        raise CodigoRecusado(
            "o Claude não aceitou o código. Peça um link novo e cole o código que a página "
            f"mostrar, inteiro. ({resposta})"
        )

    def _digitar(self, sessao: _Sessao, codigo: str) -> str:
        """Digita o código e espera o Claude terminar ou recusar."""
        with sessao.mudou:
            marca = len(sessao.saida)
        try:
            os.write(sessao.mestre, codigo.encode())
            # O Enter à parte: junto do código, a tela do Claude o toma por colagem.
            time.sleep(0.3)
            os.write(sessao.mestre, b"\r")
        except OSError:
            return "o login do Claude já tinha terminado"
        fim = time.monotonic() + _ESPERA_DO_CODIGO
        with sessao.mudou:
            while sessao.proc.poll() is None and time.monotonic() < fim:
                if _RECUSA.search(bytes(sessao.saida[marca:])):
                    break
                sessao.mudou.wait(0.5)
            novo = bytes(sessao.saida[marca:]).decode(errors="replace")
        linhas = [x.strip() for x in _SEQUENCIA.sub("", novo).splitlines() if x.strip()]
        linhas = [x for x in linhas if codigo not in x]
        return linhas[-1][-200:] if linhas else "sem resposta"

    async def desconectar(self, dono: str) -> None:
        pasta = self.pasta(dono)
        self._encerrar(dono)
        if pasta.exists():
            proc = await asyncio.create_subprocess_exec(
                self._settings.claude_bin,
                "auth",
                "logout",
                stdout=asyncio.subprocess.DEVNULL,
                stderr=asyncio.subprocess.DEVNULL,
                env=self.ambiente(dono),
            )
            try:
                await asyncio.wait_for(proc.wait(), 30)
            except TimeoutError:
                proc.kill()
                await proc.wait()
            # O logout tira a credencial; apagar a pasta tira o resto da conta.
            shutil.rmtree(pasta, ignore_errors=True)

    def _encerrar(self, dono: str) -> None:
        with self._trava:
            sessao = self._sessoes.pop(dono, None)
        if sessao is not None:
            sessao.encerrar()

    def _expirar(self) -> None:
        agora = time.monotonic()
        with self._trava:
            velhas = [d for d, s in self._sessoes.items() if agora - s.criada > _VALIDADE_DO_LINK]
            sessoes = [self._sessoes.pop(d) for d in velhas]
        for s in sessoes:
            s.encerrar()

    def encerrar_tudo(self) -> None:
        with self._trava:
            sessoes = list(self._sessoes.values())
            self._sessoes.clear()
        for s in sessoes:
            s.encerrar()
