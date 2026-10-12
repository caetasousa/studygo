#!/usr/bin/env python3
"""Limpa o registry-local, o registry da distro de desenvolvimento por onde a
esteira passa as imagens do build ao publish.

Uma imagem só serve ali durante a própria pipeline: o publish a envia ao
GitLab, e é de lá — pelo digest — que o deploy e o rollback a puxam. Mas nada
apagava as antigas, e o volume crescia a cada push (11 GB em outubro de 2026).

O que sai: toda imagem com mais de DIAS dias (padrão 2), de qualquer
repositório. O registry é dos runners de grupo, então a regra é por idade, e
não pelos commits deste projeto. O que fica:

- cache/*: o cache de camadas do build e a imagem do Playwright do e2e. A tag
  é sobrescrita a cada build; as versões que ela deixa para trás saem no
  garbage-collect.
- ci/*: a imagem do job de deploy. Um rollback reexecuta o deploy de uma
  pipeline antiga, que pede a imagem da época pela tag.

Depois de apagar, roda o garbage-collect (que é quem libera o disco) e
reinicia o registry: ele guarda em memória quais blobs existem, e um push logo
depois do GC confiaria nessa lista velha.

Só roda sem job em andamento — o GC com um push no meio corromperia a imagem
— e com o delete ligado no container (docs/ci-cd.md, "Runners").

Uso: make registry-limpar [DIAS=2] [SIMULAR=1]
"""

import json
import os
import subprocess
import sys
import urllib.request
from datetime import UTC, datetime, timedelta

REGISTRY = "http://localhost:5000"
CONTAINER = "registry-local"
MANTER = ("cache/", "ci/")
ACEITA = ", ".join(
    [
        "application/vnd.docker.distribution.manifest.v2+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.oci.image.index.v1+json",
    ]
)


def pedir(caminho, metodo="GET", aceita=None):
    req = urllib.request.Request(REGISTRY + caminho, method=metodo)
    if aceita:
        req.add_header("Accept", aceita)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.headers, resp.read()


def docker(*args):
    return subprocess.run(["docker", *args], capture_output=True, text=True, check=True).stdout


def criada_em(repo, manifesto):
    """Data de criação da imagem, que vem no config; None para um índice
    (multiplataforma), que não tem config próprio."""
    config = manifesto.get("config")
    if not config:
        return None
    _, corpo = pedir(f"/v2/{repo}/blobs/{config['digest']}")
    criada = json.loads(corpo).get("created")
    if not criada:
        return None
    # "2026-10-12T22:09:31.966123456Z": o Python não lê nanossegundos.
    data = criada.rstrip("Z").partition(".")[0]
    return datetime.fromisoformat(data).replace(tzinfo=UTC)


def main():
    dias = int(os.environ.get("DIAS") or 2)
    simular = os.environ.get("SIMULAR") == "1"
    limite = datetime.now(UTC) - timedelta(days=dias)

    env = docker("inspect", CONTAINER, "--format", "{{json .Config.Env}}")
    if "REGISTRY_STORAGE_DELETE_ENABLED=true" not in env and not simular:
        sys.exit(
            "FALHA - o registry-local não aceita DELETE. Recrie o container com o\n"
            'delete ligado (o volume e as imagens ficam): docs/ci-cd.md, "Runners".'
        )

    jobs = docker("ps", "--filter", "name=runner-", "--format", "{{.Names}}").split()
    if jobs and not simular:
        sys.exit(f"FALHA - há job da esteira rodando ({len(jobs)} containers runner-*).")

    _, corpo = pedir("/v2/_catalog?n=1000")
    repos = json.loads(corpo)["repositories"]
    apagar = []  # (repo, digest, tags)
    for repo in repos:
        if repo.startswith(MANTER):
            continue
        _, corpo = pedir(f"/v2/{repo}/tags/list")
        por_digest = {}
        for tag in json.loads(corpo).get("tags") or []:
            cab, corpo = pedir(f"/v2/{repo}/manifests/{tag}", aceita=ACEITA)
            digest = cab["Docker-Content-Digest"]
            item = por_digest.setdefault(digest, {"tags": [], "criada": None})
            item["tags"].append(tag)
            item["criada"] = criada_em(repo, json.loads(corpo))
        for digest, item in por_digest.items():
            # Apagar o manifesto leva todas as tags que apontam para ele: só sai
            # se a imagem é velha, e uma sem data fica.
            if item["criada"] and item["criada"] < limite:
                apagar.append((repo, digest, item["tags"]))

    print(f"{len(apagar)} imagens com mais de {dias} dias:")
    for repo, _, tags in apagar:
        print(f"  {repo}:{','.join(tags)[:60]}")
    if simular or not apagar:
        print("nada apagado" + (" (SIMULAR=1)" if simular else ""))
        if simular:
            return

    for repo, digest, _ in apagar:
        pedir(f"/v2/{repo}/manifests/{digest}", metodo="DELETE")

    antes = docker("exec", CONTAINER, "du", "-sh", "/var/lib/registry").split()[0]
    subprocess.run(
        [
            "docker",
            "exec",
            CONTAINER,
            "registry",
            "garbage-collect",
            "--delete-untagged",
            "/etc/docker/registry/config.yml",
        ],
        check=True,
        stdout=subprocess.DEVNULL,
    )
    docker("restart", CONTAINER)
    depois = docker("exec", CONTAINER, "du", "-sh", "/var/lib/registry").split()[0]
    print(f"registry-local: {antes} → {depois}")


if __name__ == "__main__":
    main()
