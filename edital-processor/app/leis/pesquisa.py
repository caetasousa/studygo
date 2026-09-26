"""A pesquisa pelo tópico do edital: de "Constituição…: Administração Pública;
fiscalização…" à estrutura da lei, para o backend ler que divisões o tópico
pede antes de importar qualquer coisa.

Rápida de propósito: baixa e organiza só pelas regras, sem Gemini. A
conferência fica para a captura, e só no recorte escolhido.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field

from app.leis.captura import _paginas_do_pdf
from app.leis.classificar import AGRUPAMENTOS, por_regras
from app.leis.fontes import FonteInvalida, Http, baixar, descobrir_fonte, fonte_do_link
from app.leis.limpeza import paragrafos_de_html, paragrafos_de_pdf
from app.leis.montar import montar


class FonteNaoEncontrada(Exception):
    """O tópico não diz, com certeza, que norma é e onde está (K39)."""


@dataclass
class Pesquisa:
    fonte: str
    link: str
    epigrafe: str
    # As divisões, os artigos (com o começo do texto) e a epígrafe: o que o
    # recorte precisa, sem o texto inteiro.
    estrutura: list[dict[str, object]] = field(default_factory=list)


async def pesquisar(tema: str, link: str | None, http: Http) -> Pesquisa:
    if link:
        fonte = fonte_do_link(link)
    else:
        achada = await asyncio.to_thread(descobrir_fonte, tema, http)
        if achada is None:
            raise FonteNaoEncontrada(
                "não achei a fonte oficial desta norma pelo tópico: cole o link dela"
            )
        fonte = achada

    original = await asyncio.to_thread(baixar, fonte, http)
    if original.pdf:
        paragrafos = paragrafos_de_pdf(await asyncio.to_thread(_paginas_do_pdf, original.bruto))
    else:
        assert original.html is not None
        paragrafos = await asyncio.to_thread(paragrafos_de_html, original.html)
    montagem = await asyncio.to_thread(montar, paragrafos, por_regras(paragrafos))
    if not any(d.tipo == "artigo" for d in montagem.dispositivos):
        raise FonteInvalida("a página não tem nenhum artigo que as regras reconheçam")

    estrutura: list[dict[str, object]] = []
    epigrafe = ""
    for d in montagem.dispositivos:
        if d.tipo == "preambulo" and not epigrafe:
            epigrafe = d.texto.strip().rstrip(".")
        elif d.tipo not in (*AGRUPAMENTOS, "artigo"):
            continue
        estrutura.append(
            {
                "ref": d.ref,
                "pai": d.pai,
                "tipo": d.tipo,
                "rotulo": d.rotulo,
                "nome": d.nome,
                # A epígrafe inteira: é por ela que o backend reconhece a lei
                # no próprio tópico ("Constituição da República… de 1988").
                "texto": d.texto
                if d.tipo == "preambulo"
                else d.texto[:160]
                if d.tipo == "artigo"
                else "",
                "revogado": d.revogado,
            }
        )
    return Pesquisa(
        fonte=fonte.url_publica,
        link=link or fonte.url_publica,
        epigrafe=epigrafe,
        estrutura=estrutura,
    )
