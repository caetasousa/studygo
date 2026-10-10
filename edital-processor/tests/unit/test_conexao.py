"""Conectar o Claude de uma conta pela tela, sem terminal.

Como isto quebra (escrito antes do código):

- o login do Claude Code espera um terminal: sem um, ele não imprime o link, e
  a tela fica esperando para sempre;
- o código colado não chega ao processo (ou chega sem o Enter), e a conexão
  nunca termina;
- o código errado é dado como aceito, porque o processo terminou;
- a conexão de uma conta vale para outra (a pasta do login é uma só), ou o
  `dono` vira caminho fora da pasta das contas (`../`);
- desconectar não apaga o login;
- o PDF de quem não conectou roda o Claude sem credencial, e o pedido falha
  com um erro que não diz o que fazer.

O Claude é a fronteira de fora: aqui é um script que faz o papel dele.
"""

from __future__ import annotations

import stat
import sys
from pathlib import Path

import pytest

from app.core.config import Settings
from app.mapas.conexao import CodigoRecusado, ConexaoNaoIniciada, Conexoes, DonoInvalido

CLAUDE_DE_MENTIRA = r'''#!PYTHON
"""Faz o papel do `claude auth …`: o link só sai num terminal, como no de verdade."""
import json, os, sys
pasta = os.environ["CLAUDE_CONFIG_DIR"]
cred = os.path.join(pasta, ".credentials.json")
cmd = sys.argv[1:]
if cmd[:2] == ["auth", "status"]:
    if os.path.exists(cred):
        print(json.dumps({"loggedIn": True, "email": "quem@estuda.dev", "subscriptionType": "max"}))
    else:
        print(json.dumps({"loggedIn": False, "authMethod": "none"}))
elif cmd[:2] == ["auth", "logout"]:
    if os.path.exists(cred):
        os.remove(cred)
elif cmd[:2] == ["auth", "login"]:
    if not os.isatty(1):
        sys.exit(3)
    url = "https://claude.com/cai/oauth/authorize?code=true&state=abc"
    sys.stdout.write("Opening browser to sign in…\r\n")
    link = "\x1b]8;;" + url + "\x07" + url + "\x1b]8;;\x07"
    sys.stdout.write("If the browser didn't open, visit: " + link + "\r\n")
    sys.stdout.write("Paste code here if prompted > ")
    sys.stdout.flush()
    while True:
        codigo = sys.stdin.readline().strip()
        if codigo == "codigo-certo":
            os.makedirs(pasta, exist_ok=True)
            open(cred, "w").write("{}")
            print("Login successful.")
            sys.exit(0)
        if codigo == "codigo-que-encerra":
            print("OAuth error: token exchange failed")
            sys.exit(1)
        # Como o de verdade: recusa e volta a pedir o código.
        sys.stdout.write("Invalid code. Please make sure the full code was copied.\r\n")
        sys.stdout.write("Paste code here if prompted > ")
        sys.stdout.flush()
'''


@pytest.fixture
def conexoes(tmp_path: Path) -> Conexoes:
    script = tmp_path / "claude"
    script.write_text(CLAUDE_DE_MENTIRA.replace("PYTHON", sys.executable), encoding="utf-8")
    script.chmod(script.stat().st_mode | stat.S_IEXEC)
    settings = Settings(
        service_token="",
        gemini_api_key="",
        work_dir=tmp_path / "work",
        claude_bin=str(script),
        claude_contas_dir=tmp_path / "contas",
    )
    return Conexoes(settings)


async def test_conectar_mostra_o_link_e_aceita_o_codigo(conexoes: Conexoes) -> None:
    assert (await conexoes.situacao("conta-a")).conectado is False

    url = await conexoes.iniciar("conta-a")
    assert url == "https://claude.com/cai/oauth/authorize?code=true&state=abc"

    s = await conexoes.concluir("conta-a", "codigo-certo")
    assert s.conectado is True
    assert s.email == "quem@estuda.dev"
    assert s.plano == "max"
    # A conexão é da conta: a outra continua sem.
    assert (await conexoes.situacao("conta-b")).conectado is False


async def test_codigo_errado_e_recusado_e_pode_tentar_de_novo(conexoes: Conexoes) -> None:
    await conexoes.iniciar("conta-a")
    with pytest.raises(CodigoRecusado, match="cole de novo"):
        await conexoes.concluir("conta-a", "codigo-errado")
    assert (await conexoes.situacao("conta-a")).conectado is False

    # O Claude voltou a pedir o código: o mesmo link serve.
    assert (await conexoes.concluir("conta-a", "codigo-certo")).conectado is True


async def test_codigo_que_encerra_o_login_pede_link_novo(conexoes: Conexoes) -> None:
    await conexoes.iniciar("conta-a")
    with pytest.raises(CodigoRecusado, match="link novo"):
        await conexoes.concluir("conta-a", "codigo-que-encerra")
    with pytest.raises(ConexaoNaoIniciada):
        await conexoes.concluir("conta-a", "codigo-certo")
    await conexoes.iniciar("conta-a")
    assert (await conexoes.concluir("conta-a", "codigo-certo")).conectado is True


async def test_codigo_sem_link_pedido(conexoes: Conexoes) -> None:
    with pytest.raises(ConexaoNaoIniciada):
        await conexoes.concluir("conta-a", "codigo-certo")


async def test_desconectar_apaga_o_login(conexoes: Conexoes) -> None:
    await conexoes.iniciar("conta-a")
    await conexoes.concluir("conta-a", "codigo-certo")
    await conexoes.desconectar("conta-a")
    assert (await conexoes.situacao("conta-a")).conectado is False


async def test_dono_que_vira_caminho_e_recusado(conexoes: Conexoes) -> None:
    for dono in ("../outra", "a/b", "", "."):
        with pytest.raises(DonoInvalido):
            await conexoes.situacao(dono)


def test_pasta_e_por_conta(conexoes: Conexoes, tmp_path: Path) -> None:
    assert conexoes.pasta("conta-a") == tmp_path / "contas" / "conta-a"
    assert conexoes.pasta("conta-a") != conexoes.pasta("conta-b")


def test_rotas_exigem_token_e_dono(tmp_path: Path) -> None:
    from fastapi.testclient import TestClient

    from app.core.config import get_settings
    from app.main import create_app

    settings = Settings(
        service_token="s3cr3t",
        gemini_api_key="",
        work_dir=tmp_path / "w",
        claude_contas_dir=tmp_path / "contas",
    )
    app = create_app()
    app.dependency_overrides[get_settings] = lambda: settings
    with TestClient(app) as c:
        assert c.get("/internal/claude", headers={"x-owner-ref": "u1"}).status_code == 401
        r = c.get(
            "/internal/claude", headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "u1"}
        )
        assert r.status_code == 200
        assert r.json() == {"conectado": False, "email": "", "plano": ""}
        r = c.post(
            "/internal/claude/conexao/codigo",
            json={"codigo": "x"},
            headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "u1"},
        )
        assert r.status_code == 409
        assert r.json()["code"] == "conexao_nao_iniciada"
    app.dependency_overrides.clear()
