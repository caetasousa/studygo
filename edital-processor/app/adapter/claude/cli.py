"""O `claude -p` de verdade: um processo por chamada, com prazo."""

from __future__ import annotations

import asyncio
from pathlib import Path


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
