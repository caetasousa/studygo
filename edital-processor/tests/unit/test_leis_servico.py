"""Captura de leis pela aplicação — cada teste cita o id de app/leis/README.md.

O serviço é a parte que a tela usa: o link colado vira uma captura em segundo
plano, consultada pelo id até ficar pronta. Nada aqui baixa de verdade: o HTTP
é trocado por uma função, e o relógio por um contador.
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest
from fastapi.testclient import TestClient

from app.leis.captura import capturar
from app.leis.fontes import FonteInvalida, Resposta, fonte_do_link, redirecionamento_permitido
from app.leis.servico import CapturaNaoEncontrada, Capturas, CapturasDemais

LEI = """<html><body>
<p>LEI Nº 1, DE 1º DE JANEIRO DE 2020</p>
<p>CAPÍTULO I<br>DISPOSIÇÕES PRELIMINARES</p>
<p>Art. 1º Esta Lei dispõe sobre o teste.</p>
<p>Art. 2º Compete ao órgão:</p>
<p>I - julgar;</p>
<p>Brasília, 1º de janeiro de 2020.</p>
</body></html>"""

PLANALTO = "https://www.planalto.gov.br/ccivil_03/leis/l0001.htm"


def _http(corpo: str = LEI) -> Any:
    return lambda url: Resposta(200, "text/html", corpo.encode("cp1252"))


async def _esperar(capturas: Capturas, cid: str, dono: str = "u1") -> dict[str, object]:
    for _ in range(200):
        estado = capturas.consultar(cid, dono)
        if estado["estado"] != "rodando":
            return estado
        await asyncio.sleep(0.01)
    raise AssertionError("a captura não terminou")


# ---------------------------------------------------------------- K29, K30


@pytest.mark.parametrize(
    "link",
    [
        "http://www.planalto.gov.br/ccivil_03/leis/l0001.htm",
        "https://www.exemplo.com/lei.htm",
        "https://127.0.0.1/lei.htm",
        "https://localhost/lei.htm",
        "https://10.0.0.5/lei.htm",
        "https://planalto.gov.br.exemplo.com/lei.htm",
        "https://user@www.planalto.gov.br/lei.htm",
        "https://www.planalto.gov.br:8443/lei.htm",
        "ftp://www.planalto.gov.br/lei.htm",
        "",
    ],
)
def test_k29_recusa_link_fora_das_fontes_oficiais(link: str) -> None:
    with pytest.raises(FonteInvalida):
        fonte_do_link(link)


def test_k29_reconhece_a_fonte_pelo_link() -> None:
    planalto = fonte_do_link(PLANALTO)
    assert (planalto.tipo, planalto.url, planalto.url_publica) == ("planalto", PLANALTO, PLANALTO)

    go = fonte_do_link("https://legisla.casacivil.go.gov.br/pesquisa_legislacao/86708/lei-16168")
    assert go.tipo == "casacivil-go"
    assert go.url == "https://legisla.casacivil.go.gov.br/api/v2/pesquisa/legislacoes/86708"
    assert go.url_publica == "https://legisla.casacivil.go.gov.br/pesquisa_legislacao/86708"

    tce = fonte_do_link("https://portal.tce.go.gov.br/documents/regimento.pdf")
    assert tce.tipo == "link"


def test_k30_redirecionamento_so_dentro_das_fontes_oficiais() -> None:
    assert redirecionamento_permitido("https://www2.planalto.gov.br/outra.htm")
    assert not redirecionamento_permitido("https://www.exemplo.com/lei.htm")
    assert not redirecionamento_permitido("http://www.planalto.gov.br/lei.htm")
    assert not redirecionamento_permitido("https://169.254.169.254/latest/meta-data")


# ---------------------------------------------------------------- K34, K35, K36


def test_k34_captura_com_bloqueio_nao_e_publicavel() -> None:
    corpo = "<html><body><p>Art. 1º A.</p><p>Art. 3º C.</p><p>Texto solto</p></body></html>"
    # Salto de numeração é aviso; o que torna o bloqueio certo é a fonte sem
    # artigo nenhum, que não deixa nem montar a lei.
    vazia = asyncio.run(capturar(fonte_do_link(PLANALTO), _http("<html><p>Erro</p></html>"), None))
    assert vazia.bloqueios and not vazia.publicavel
    assert vazia.dispositivos is None

    com_salto = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(corpo), None))
    assert not com_salto.bloqueios
    assert com_salto.dispositivos is not None


def test_k35_salto_de_numeracao_e_aviso_para_revisar() -> None:
    corpo = "<html><body><p>Art. 1º A.</p><p>Art. 3º C.</p></body></html>"
    c = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(corpo), None))
    saltos = [a for a in c.avisos if "art1 → art3" in a.texto]
    assert len(saltos) == 1
    assert c.publicavel


def test_k35_divergencia_do_gemini_e_aviso_com_o_trecho() -> None:
    class Discorda:
        def available(self) -> bool:
            return True

        async def extract_structured(self, request: Any) -> dict[str, object]:
            itens = []
            for chunk in request.chunks:
                pid, texto = chunk.split(": ", 1)
                tipo = "solto" if texto.startswith("Art. 2º") else _regra(texto)
                itens.append({"id": pid, "tipo": tipo})
            return {"itens": itens}

    c = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(), Discorda()))  # type: ignore[arg-type]
    assert not c.bloqueios
    divergencias = [a for a in c.avisos if "o Gemini diz solto" in a.texto]
    assert len(divergencias) == 1
    assert divergencias[0].trecho.startswith("Art. 2º Compete")
    # A regra prevaleceu: o art. 2º continua artigo na árvore.
    assert c.dispositivos is not None
    assert any(d["ref"] == "art2" and d["tipo"] == "artigo" for d in c.dispositivos)


def _regra(texto: str) -> str:
    if texto.startswith("Art."):
        return "artigo"
    if texto.startswith("CAPÍTULO"):
        return "capitulo"
    if texto.startswith("I -"):
        return "inciso"
    if texto.startswith("LEI"):
        return "preambulo"
    if texto.startswith("Brasília"):
        return "fecho"
    return "solto"


def test_k36_sem_gemini_vira_aviso() -> None:
    c = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(), None))
    assert not c.gemini
    assert any("Gemini" in a.texto for a in c.avisos)
    assert c.publicavel


def test_k26_ids_dos_avisos_sao_estaveis_entre_capturas() -> None:
    corpo = "<html><body><p>Art. 1º A.</p><p>Art. 3º C.</p></body></html>"
    a = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(corpo), None))
    b = asyncio.run(capturar(fonte_do_link(PLANALTO), _http(corpo), None))
    assert [x.id for x in a.avisos] == [x.id for x in b.avisos]
    assert a.versao == b.versao


# ---------------------------------------------------------------- K31, K32, K33, K37


def test_k31_outra_conta_nao_ve_a_captura() -> None:
    async def cenario() -> None:
        capturas = Capturas(http=_http(), provider=None)
        cid = capturas.iniciar(PLANALTO, "u1")
        await _esperar(capturas, cid)
        with pytest.raises(CapturaNaoEncontrada):
            capturas.consultar(cid, "u2")
        assert capturas.consultar(cid, "u1")["estado"] == "pronta"

    asyncio.run(cenario())


def test_k32_excecao_no_meio_vira_falha_e_nao_fica_rodando() -> None:
    def explode(url: str) -> Resposta:
        raise RuntimeError("conexão caiu")

    async def cenario() -> None:
        capturas = Capturas(http=explode, provider=None)
        cid = capturas.iniciar(PLANALTO, "u1")
        estado = await _esperar(capturas, cid)
        assert estado["estado"] == "falhou"
        assert "conexão caiu" not in str(estado["erro"])  # detalhe interno fica no log
        assert estado["erro"]

    asyncio.run(cenario())


def test_k33_limita_capturas_simultaneas() -> None:
    async def cenario() -> None:
        liberar = asyncio.Event()

        class Lento:
            """Segura a classificação: a primeira captura fica rodando."""

            def available(self) -> bool:
                return True

            async def extract_structured(self, request: Any) -> dict[str, object]:
                await liberar.wait()
                return {"itens": []}

        capturas = Capturas(http=_http(), provider=Lento(), simultaneas=1)  # type: ignore[arg-type]
        primeira = capturas.iniciar(PLANALTO, "u1")
        await asyncio.sleep(0.05)
        # A segunda é recusada, não enfileirada: quem espera é a tela, que
        # pode tentar de novo.
        with pytest.raises(CapturasDemais):
            capturas.iniciar(PLANALTO, "u2")
        liberar.set()
        await _esperar(capturas, primeira)
        # Terminada a primeira, a vaga volta.
        capturas.iniciar(PLANALTO, "u2")

    asyncio.run(cenario())


def test_k37_captura_pronta_expira() -> None:
    agora = [1000.0]

    async def cenario() -> None:
        capturas = Capturas(http=_http(), provider=None, validade=60, relogio=lambda: agora[0])
        cid = capturas.iniciar(PLANALTO, "u1")
        await _esperar(capturas, cid)
        agora[0] += 61
        with pytest.raises(CapturaNaoEncontrada):
            capturas.consultar(cid, "u1")

    asyncio.run(cenario())


# ---------------------------------------------------------------- rotas


def test_rotas_exigem_token_e_devolvem_a_previa(tmp_path: object) -> None:
    from app.api.leis import get_capturas
    from app.core.config import Settings, get_settings
    from app.main import create_app

    capturas = Capturas(http=_http(), provider=None)
    app = create_app()
    app.dependency_overrides[get_settings] = lambda: Settings(
        service_token="t",
        gemini_api_key="",
        work_dir=tmp_path / "work",  # type: ignore[operator]
    )
    app.dependency_overrides[get_capturas] = lambda: capturas
    with TestClient(app) as cliente:
        sem_token = cliente.post("/internal/leis/capturas", json={"link": PLANALTO})
        assert sem_token.status_code == 401

        cab = {"Authorization": "Bearer t", "X-Owner-Ref": "u1"}
        fora = cliente.post(
            "/internal/leis/capturas", json={"link": "https://x.com/a"}, headers=cab
        )
        assert fora.status_code == 422
        assert fora.json()["code"] == "fonte_invalida"
        assert fora.json()["transient"] is False

        cid = cliente.post("/internal/leis/capturas", json={"link": PLANALTO}, headers=cab).json()[
            "id"
        ]
        for _ in range(200):
            corpo = cliente.get(f"/internal/leis/capturas/{cid}", headers=cab).json()
            if corpo["estado"] != "rodando":
                break
        assert corpo["estado"] == "pronta"
        assert corpo["resultado"]["publicavel"] is True
        assert corpo["resultado"]["fonte"] == PLANALTO
        assert json.dumps(corpo["resultado"]["dispositivos"])

        outro = {"Authorization": "Bearer t", "X-Owner-Ref": "u2"}
        assert cliente.get(f"/internal/leis/capturas/{cid}", headers=outro).status_code == 404
