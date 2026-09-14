#!/usr/bin/env python3
"""Cópia do banco e do volume de provas antes do deploy, sob o mesmo lock.

O banco diz quais arquivos de prova existem; o volume `provas_data` guarda os
arquivos. Copiados em momentos diferentes, a limpeza poderia apagar, entre um e
outro, um arquivo que o dump ainda referencia — e a restauração voltaria com
imagens faltando. O advisory lock 704403 é o mesmo que
ProvaRepo.LimparReferencias segura: com ele preso aqui, a limpeza espera.

Roda na VPS, no diretório do compose da aplicação. O ansible/deploy.yml o chama
antes de cada deploy; imprime o caminho das cópias, e os avisos saem na mesma
saída, que o deploy mostra.

Com --restaurar, faz o caminho de volta do volume: apaga o conteúdo atual e
extrai a cópia. O banco se restaura à parte, com o dump de mesmo nome (ver
docs/ci-cd.md).
"""

from __future__ import annotations

import argparse
import gzip
import os
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path

LOCK_ARQUIVOS_PROVAS = 704403
MANTER = 5
COMPOSE = ["docker", "compose", "exec", "-T"]

# O processador tem Python e enxerga o volume; as imagens do backend são
# distroless, sem shell nem tar.
TAR_DO_VOLUME = (
    "import sys, tarfile\n"
    "with tarfile.open(fileobj=sys.stdout.buffer, mode='w|gz') as t:\n"
    "    t.add('/var/lib/provas', arcname='provas')\n"
)

# O filtro "data" recusa link, caminho absoluto e ".." dentro da cópia.
RESTAURAR_VOLUME = (
    "import pathlib, shutil, sys, tarfile\n"
    "raiz = pathlib.Path('/var/lib/provas')\n"
    "for p in raiz.iterdir():\n"
    "    shutil.rmtree(p) if p.is_dir() else p.unlink()\n"
    "with tarfile.open(fileobj=sys.stdin.buffer, mode='r|gz') as t:\n"
    "    for m in t:\n"
    "        if m.isfile():\n"
    "            m.name = m.name.removeprefix('provas/')\n"
    "            t.extract(m, raiz, filter='data')\n"
)


def servicos_de_pe() -> set[str]:
    saida = subprocess.run(
        ["docker", "compose", "ps", "--status", "running", "--services"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return set(saida.split())


def prender_lock(usuario: str, banco: str) -> subprocess.Popen[str]:
    """Abre uma sessão psql que segura o lock até ser fechada."""
    sessao = subprocess.Popen(
        [*COMPOSE, "postgres", "psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-U", usuario, banco],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        text=True,
    )
    assert sessao.stdin and sessao.stdout
    sessao.stdin.write(f"SELECT pg_advisory_lock({LOCK_ARQUIVOS_PROVAS}); SELECT 'preso';\n")
    sessao.stdin.flush()
    for linha in sessao.stdout:
        if linha.strip() == "preso":
            return sessao
    raise RuntimeError("não consegui prender o lock dos arquivos de provas")


def volume_de_provas_existe() -> bool:
    teste = [*COMPOSE, "edital-processor", "test", "-d", "/var/lib/provas"]
    return subprocess.run(teste, check=False).returncode == 0


def gravar(destino: Path, escrever) -> None:  # type: ignore[no-untyped-def]
    """Grava num .parcial e só renomeia no fim: uma cópia interrompida nunca
    fica com cara de cópia válida."""
    parcial = destino.with_name(destino.name + ".parcial")
    try:
        with parcial.open("xb") as saida:
            parcial.chmod(0o600)
            escrever(saida)
        parcial.replace(destino)
    finally:
        parcial.unlink(missing_ok=True)


def backup(app_dir: str, backup_dir: str, usuario: str, banco: str, pipeline: str) -> None:
    os.chdir(app_dir)
    raiz = Path(backup_dir)
    raiz.mkdir(parents=True, exist_ok=True)
    # A data no nome impede que reexecutar o job da mesma pipeline — é assim que
    # o rollback funciona — sobrescreva a cópia anterior.
    carimbo = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    dump = raiz / f"pre-deploy-{carimbo}-p{pipeline}.sql.gz"
    volume = raiz / f"pre-deploy-{carimbo}-p{pipeline}.provas.tar.gz"

    sessao = prender_lock(usuario, banco)
    try:

        def escrever_dump(saida) -> None:  # type: ignore[no-untyped-def]
            with gzip.GzipFile(fileobj=saida, mode="wb") as comprimido:
                pg = subprocess.Popen(
                    [*COMPOSE, "postgres", "pg_dump", "-U", usuario, banco], stdout=subprocess.PIPE
                )
                assert pg.stdout
                shutil.copyfileobj(pg.stdout, comprimido)
                if pg.wait() != 0:
                    raise RuntimeError("pg_dump falhou")

        gravar(dump, escrever_dump)
        print(dump)

        if "edital-processor" not in servicos_de_pe():
            print(
                "AVISO: edital-processor fora do ar; o volume de provas NÃO foi copiado "
                f"junto de {dump.name}."
            )
        elif not volume_de_provas_existe():
            # A versão no ar ainda não tem o catálogo de provas: não há volume.
            print("Sem volume de provas nesta versão; só o banco foi copiado.")
        else:
            gravar(
                volume,
                lambda saida: subprocess.run(
                    [*COMPOSE, "edital-processor", "python", "-c", TAR_DO_VOLUME],
                    stdout=saida,
                    check=True,
                ),
            )
            print(volume)
    finally:
        # Fechar a sessão solta o lock; a limpeza de provas volta a andar.
        assert sessao.stdin
        sessao.stdin.close()
        sessao.wait(timeout=15)

    for antigo in sorted(raiz.glob("pre-deploy-*.sql.gz"), reverse=True)[MANTER:]:
        antigo.with_name(antigo.name.removesuffix(".sql.gz") + ".provas.tar.gz").unlink(
            missing_ok=True
        )
        antigo.unlink()


def restaurar(app_dir: str, copia: str) -> None:
    """Troca o conteúdo do volume de provas pela cópia. Pare backend e worker
    antes: um recorte gravado no meio se perderia."""
    os.chdir(app_dir)
    with Path(copia).open("rb") as entrada:
        subprocess.run(
            [
                "docker",
                "compose",
                "run",
                "--rm",
                "-T",
                "--no-deps",
                "--entrypoint",
                "python",
                "edital-processor",
                "-c",
                RESTAURAR_VOLUME,
            ],
            stdin=entrada,
            check=True,
        )
    print(f"volume de provas restaurado de {copia}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app-dir", required=True)
    parser.add_argument("--restaurar", metavar="COPIA", help="<data>.provas.tar.gz a restaurar")
    for nome in ["backup-dir", "user", "database", "pipeline"]:
        parser.add_argument("--" + nome)
    args = parser.parse_args()
    if args.restaurar:
        restaurar(args.app_dir, args.restaurar)
    elif not all([args.backup_dir, args.user, args.database, args.pipeline]):
        parser.error("a cópia exige --backup-dir, --user, --database e --pipeline")
    else:
        backup(args.app_dir, args.backup_dir, args.user, args.database, args.pipeline)
