"""O PDF de uma aula virando mapa, do lado do processador.

Como isto quebra (escrito antes do código):

- o backend entrega o PDF e a rota responde 202, mas ninguém roda o Claude;
- a rota aceita a chamada sem o token de serviço (é interna, mas o token é a
  segunda camada) ou sem dono;
- o mesmo pedido entregue duas vezes (redespacho ao subir) roda duas vezes;
- o backend recusa a importação (422) e o processador desiste, em vez de
  devolver o motivo ao Claude na mesma sessão (--resume);
- o Claude termina com erro e o pedido fica "processando" para sempre, sem
  ninguém avisar o backend;
- um processamento que falha mata a fila, e o próximo PDF nunca roda;
- o PDF (material pago) fica no disco depois do pedido;
- o token guardado em Configurações não chega ao Claude;
- as instruções copiadas da skill e do README do formato ficam velhas;
- o pedido sem aula (só o assunto) é recusado por falta de PDF, chega ao Claude
  com um PDF vazio ou sem o assunto, ou sem as ferramentas da web — e o mapa
  sai da memória do modelo, não das fontes oficiais;
- o pedido de PDF ganha acesso à web, que ele não precisa.
- o Claude que acha o login da conta sendo renovado por outro processo (ou
  por um que morreu no meio) derruba o pedido na hora, quando ele mesmo diz
  que é passageiro; ou repete a primeira chamada com a sessão que já ficou
  registrada; ou, se não passa, falha sem dizer o que fazer.

O Claude e o backend são a fronteira de fora: entram como dublês.
"""

from __future__ import annotations

import io
import json
import time
from collections.abc import Iterator
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from app.domain.mapas.pedido import Resposta, Saida, Trabalho
from app.main import Componentes, create_app
from app.platform.config import Settings
from app.service.mapas.fila import Mapas
from app.service.mapas.sessao import INSTRUCOES, ler_saida

RAIZ = Path(__file__).resolve().parents[3]

MAPA = "# Aula de teste\nslug: aula-de-teste\n\n- Ramo\n  - item\n"


class Claude:
    """Dublê do `claude -p`: escreve a saída que lhe mandam e anota as chamadas."""

    def __init__(self, codigo: int = 0, erro: str = "") -> None:
        self.codigo = codigo
        self.erro = erro
        self.chamadas: list[tuple[list[str], str, dict[str, str]]] = []
        self.pastas: list[Path] = []
        # O que havia na pasta do pedido quando o Claude rodou.
        self.arquivos: list[list[str]] = []

    async def __call__(
        self, args: list[str], mensagem: str, pasta: Path, ambiente: dict[str, str]
    ) -> tuple[int, str]:
        self.chamadas.append((args, mensagem, ambiente))
        self.pastas.append(pasta)
        self.arquivos.append(sorted(p.name for p in pasta.iterdir()))
        if self.codigo:
            return self.codigo, self.erro
        saida = pasta / "saida"
        (saida / "mapa.md").write_text(MAPA, encoding="utf-8")
        (saida / "topicos.json").write_text(json.dumps({"temas": ["Tema 1"]}), encoding="utf-8")
        (saida / "relatorio.md").write_text("1 ramo", encoding="utf-8")
        (saida / "imagens" / "fig.png").write_bytes(b"\x89PNG")
        return 0, "{}"


class Backend:
    """Dublê do backend interno: responde o que a fila de respostas mandar."""

    def __init__(self, *respostas: Resposta) -> None:
        self.respostas = list(respostas) or [Resposta(200, "")]
        self.entregues: list[tuple[str, Saida]] = []
        self.falhas: list[tuple[str, str]] = []

    async def entregar(self, pedido: str, saida: Saida) -> Resposta:
        self.entregues.append((pedido, saida))
        return self.respostas.pop(0) if len(self.respostas) > 1 else self.respostas[0]

    async def avisar_falha(self, pedido: str, relatorio: str) -> None:
        self.falhas.append((pedido, relatorio))


def _trabalho(pedido: str = "p1", **kw: str) -> Trabalho:
    return Trabalho(pedido=pedido, dono="u1", arquivo="aula.pdf", pdf=b"%PDF-1.7 x", **kw)


async def test_entrega_o_que_o_claude_escreveu(settings: Settings) -> None:
    claude, backend = Claude(), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho(token_claude="tok-da-conta"))
    await mapas.esperar_fila()

    assert [p for p, _ in backend.entregues] == ["p1"]
    saida = backend.entregues[0][1]
    assert saida.mapa == MAPA
    assert saida.temas == ["Tema 1"]
    assert saida.relatorio == "1 ramo"
    assert saida.imagens == [("fig.png", b"\x89PNG")]
    assert backend.falhas == []
    # O token da conta chega ao Claude.
    assert claude.chamadas[0][2]["CLAUDE_CODE_OAUTH_TOKEN"] == "tok-da-conta"
    # O PDF não fica no disco.
    assert not claude.pastas[0].exists()


async def test_assunto_sem_aula_vai_ao_claude_com_as_fontes_da_web(settings: Settings) -> None:
    claude, backend = Claude(), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(
        Trabalho(
            pedido="p1",
            dono="u1",
            arquivo="Transações: ACID",
            pdf=b"",
            assunto="Transações: ACID e níveis de isolamento",
            materia="BANDA — Banco de Dados",
            token_claude="tok",
        )
    )
    await mapas.esperar_fila()

    assert [p for p, _ in backend.entregues] == ["p1"]
    args, mensagem, _ = claude.chamadas[0]
    # Sem PDF na pasta: o mapa nasce das fontes, não de uma aula vazia.
    assert "aula.pdf" not in claude.arquivos[0]
    assert "Transações: ACID e níveis de isolamento" in mensagem
    assert "BANDA — Banco de Dados" in mensagem
    assert "fontes oficiais" in mensagem
    ferramentas = args[args.index("--allowedTools") + 1 :]
    assert "WebSearch" in ferramentas
    assert "WebFetch" in ferramentas


async def test_pdf_da_aula_nao_ganha_a_web(settings: Settings) -> None:
    claude, backend = Claude(), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho(token_claude="tok"))
    await mapas.esperar_fila()

    args = claude.chamadas[0][0]
    assert "aula.pdf" in claude.arquivos[0]
    assert "WebSearch" not in args
    assert "WebFetch" not in args


async def test_recusa_volta_ao_claude_na_mesma_sessao(settings: Settings) -> None:
    claude = Claude()
    backend = Backend(Resposta(422, "linha 3: marca desconhecida"), Resposta(200, ""))
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho())
    await mapas.esperar_fila()

    assert len(claude.chamadas) == 2
    primeira, segunda = claude.chamadas[0][0], claude.chamadas[1][0]
    sessao = primeira[primeira.index("--session-id") + 1]
    assert segunda[segunda.index("--resume") + 1] == sessao
    assert "linha 3: marca desconhecida" in claude.chamadas[1][1]
    assert backend.falhas == []


# O que o Claude Code 2.1 responde quando a trava da renovação do login está
# ocupada (pedido de 13/10/2026 no staging).
LOGIN_OCUPADO = (
    '{"type":"result","is_error":true,"terminal_reason":"api_error","result":"Failed to '
    "refresh OAuth token: another Claude Code process is refreshing it or exited "
    'mid-refresh. This is usually transient; retry in a minute, and if it persists close '
    'other Claude Code processes or sign in again"}'
)


class ClaudeComLoginOcupado(Claude):
    """O Claude que, nas primeiras `vezes` chamadas, acha o login em renovação."""

    def __init__(self, vezes: int) -> None:
        super().__init__()
        self.vezes = vezes

    async def __call__(
        self, args: list[str], mensagem: str, pasta: Path, ambiente: dict[str, str]
    ) -> tuple[int, str]:
        if len(self.chamadas) < self.vezes:
            self.chamadas.append((args, mensagem, ambiente))
            return 1, LOGIN_OCUPADO
        return await super().__call__(args, mensagem, pasta, ambiente)


async def test_login_em_renovacao_espera_e_tenta_de_novo(settings: Settings) -> None:
    settings.mapas_espera_do_login = 0
    claude, backend = ClaudeComLoginOcupado(vezes=1), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho(token_claude="tok"))
    await mapas.esperar_fila()

    assert backend.falhas == []
    assert [p for p, _ in backend.entregues] == ["p1"]
    primeira, segunda = claude.chamadas[0][0], claude.chamadas[1][0]
    # A sessão da chamada que falhou pode ter ficado registrada: a nova é outra.
    def sessao(args: list[str]) -> str:
        return args[args.index("--session-id") + 1]

    assert sessao(segunda) != sessao(primeira)


async def test_login_que_nao_libera_falha_dizendo_o_que_fazer(settings: Settings) -> None:
    settings.mapas_espera_do_login = 0
    claude, backend = ClaudeComLoginOcupado(vezes=99), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho(token_claude="tok"))
    await mapas.esperar_fila()

    assert len(claude.chamadas) == 3
    assert backend.entregues == []
    [(pedido, motivo)] = backend.falhas
    assert pedido == "p1"
    assert "Pôr na fila de novo" in motivo
    assert "Configurações" in motivo


async def test_recusa_que_nao_se_resolve_vira_falha(settings: Settings) -> None:
    claude = Claude()
    backend = Backend(Resposta(422, "slug de outro mapa"))
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho())
    await mapas.esperar_fila()

    assert len(claude.chamadas) == settings.mapas_tentativas
    assert len(backend.falhas) == 1
    assert "slug de outro mapa" in backend.falhas[0][1]


async def test_erro_do_claude_avisa_o_backend_e_a_fila_segue(settings: Settings) -> None:
    claude = Claude(codigo=1, erro='{"api_error_status": 401}')
    backend = Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho("p1"))
    mapas.iniciar(_trabalho("p2"))
    await mapas.esperar_fila()

    assert [p for p, _ in backend.falhas] == ["p1", "p2"]
    assert "401" in backend.falhas[0][1]
    assert "conecte de novo" in backend.falhas[0][1]
    assert backend.entregues == []


async def test_pedido_excluido_la_nao_e_falha(settings: Settings) -> None:
    backend = Backend(Resposta(404, "pedido não encontrado"))
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, Claude())

    mapas.iniciar(_trabalho())
    await mapas.esperar_fila()

    assert backend.falhas == []


async def test_mesmo_pedido_duas_vezes_roda_uma(settings: Settings) -> None:
    claude, backend = Claude(), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude)

    mapas.iniciar(_trabalho())
    mapas.iniciar(_trabalho())
    await mapas.esperar_fila()

    assert len(claude.chamadas) == 1


def test_sem_mapa_md_e_erro_para_o_claude(tmp_path: Path) -> None:
    (tmp_path / "imagens").mkdir()
    with pytest.raises(Exception, match=r"mapa\.md"):
        ler_saida(tmp_path)


@pytest.fixture
def cliente(tmp_path: Path) -> Iterator[tuple[TestClient, Claude, Backend]]:
    settings = Settings(
        service_token="s3cr3t",
        gemini_api_key="",
        work_dir=tmp_path / "work",
        claude_contas_dir=tmp_path / "contas",
    )
    claude, backend = Claude(), Backend()
    componentes = Componentes(
        settings, entregar=backend.entregar, avisar_falha=backend.avisar_falha, claude=claude
    )
    # A fila nasce no primeiro pedido, dentro do loop do TestClient, como no servidor.
    with TestClient(create_app(componentes)) as c:
        yield c, claude, backend


def _post(
    c: TestClient,
    token: str = "s3cr3t",
    pdf: bytes = b"%PDF-1.7 aula",
    token_claude: str = "tok-da-conta",
) -> int:
    dados = {"pedido": "p9", "arquivo": "a.pdf", "temas": None, "tokenClaude": token_claude}
    r = c.post(
        "/internal/mapas/processamentos",
        # Sem matéria, o Go manda os tópicos como null.
        data={"dados": json.dumps(dados)},
        files={"pdf": ("a.pdf", io.BytesIO(pdf), "application/pdf")},
        headers={"authorization": f"Bearer {token}", "x-owner-ref": "u1"},
    )
    return r.status_code


def test_rota_exige_token_de_servico(cliente: tuple[TestClient, Claude, Backend]) -> None:
    c, claude, _ = cliente
    assert _post(c, token="errado") == 401
    assert claude.chamadas == []


def test_rota_recusa_o_que_nao_e_pdf(cliente: tuple[TestClient, Claude, Backend]) -> None:
    c, claude, _ = cliente
    assert _post(c, pdf=b"<html>") == 400
    assert claude.chamadas == []


def test_rota_aceita_e_processa(cliente: tuple[TestClient, Claude, Backend]) -> None:
    c, _, backend = cliente
    assert _post(c) == 202
    # O TestClient roda o loop numa thread; o processamento termina logo, com o dublê.
    for _ in range(100):
        if backend.entregues:
            break
        time.sleep(0.02)
    assert [p for p, _ in backend.entregues] == ["p9"]


def test_rota_aceita_o_assunto_sem_pdf_e_recusa_o_pedido_vazio(
    cliente: tuple[TestClient, Claude, Backend],
) -> None:
    c, claude, backend = cliente
    sem_nada = {"pedido": "p8", "arquivo": "x", "tokenClaude": "tok"}
    r = c.post(
        "/internal/mapas/processamentos",
        data={"dados": json.dumps(sem_nada)},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "u1"},
    )
    assert r.status_code == 400

    assunto = {**sem_nada, "pedido": "p9", "assunto": "Transações: ACID"}
    r = c.post(
        "/internal/mapas/processamentos",
        data={"dados": json.dumps(assunto)},
        headers={"authorization": "Bearer s3cr3t", "x-owner-ref": "u1"},
    )
    assert r.status_code == 202
    for _ in range(100):
        if backend.entregues:
            break
        time.sleep(0.02)
    assert [p for p, _ in backend.entregues] == ["p9"]
    assert "Transações: ACID" in claude.chamadas[0][1]


def test_rota_sem_claude_da_conta_avisa_a_falha(
    cliente: tuple[TestClient, Claude, Backend],
) -> None:
    """Sem token e sem conexão, o pedido falha na hora dizendo o que fazer, sem
    subir o Claude: a montagem real liga a fila à conexão de cada conta."""
    c, claude, backend = cliente
    assert _post(c, token_claude="") == 202
    for _ in range(100):
        if backend.falhas:
            break
        time.sleep(0.02)
    assert [p for p, _ in backend.falhas] == ["p9"]
    assert "não está conectado" in backend.falhas[0][1]
    assert claude.chamadas == []


@pytest.mark.parametrize(
    ("copia", "original"),
    [
        ("skill.md", ".claude/skills/mapa-mental/SKILL.md"),
        ("formato.md", "conteudo/mapas/README.md"),
    ],
)
def test_instrucoes_iguais_as_do_repositorio(copia: str, original: str) -> None:
    """A imagem do processador não enxerga o resto do repositório: leva cópias.
    Se a skill ou o formato mudarem, a cópia tem de mudar junto."""
    fonte = RAIZ / original
    if not fonte.exists():
        pytest.skip("fora do repositório (dentro da imagem)")
    assert (INSTRUCOES / copia).read_text(encoding="utf-8") == fonte.read_text(encoding="utf-8")


async def test_sem_conexao_nem_token_falha_dizendo_o_que_fazer(tmp_path: Path) -> None:
    """O PDF de quem não conectou o Claude não roda: o pedido diz para conectar."""
    from app.adapter.claude.conexao import Conexoes

    settings = Settings(
        service_token="",
        gemini_api_key="",
        work_dir=tmp_path / "work",
        claude_contas_dir=tmp_path / "contas",
    )
    claude, backend = Claude(), Backend()
    mapas = Mapas(settings, backend.entregar, backend.avisar_falha, claude, Conexoes(settings))

    mapas.iniciar(_trabalho())
    await mapas.esperar_fila()

    assert claude.chamadas == []
    assert "Conecte-o em Configurações" in backend.falhas[0][1]

    # Com o token da conta, roda — e o Claude usa a pasta da conta.
    mapas.iniciar(_trabalho("p2", token_claude="tok"))
    await mapas.esperar_fila()
    assert claude.chamadas[0][2]["CLAUDE_CONFIG_DIR"] == str(tmp_path / "contas" / "u1")
