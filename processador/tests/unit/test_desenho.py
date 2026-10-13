"""As figuras didáticas do mapa sem aula (ferramentas/desenho.py).

Sem PDF de onde recortar, o Claude desenha as figuras do mapa — e quem estuda
não pode notar diferença entre elas e as que já estão nos mapas (fundo branco,
título em negrito, caixas na paleta de sempre).

Como isto quebra (escrito antes do código):
- a ferramenta não roda como o Claude a chama (`python -I`, de outra pasta);
- um modelo sai com fundo transparente ou escuro, ilegível no tema escuro do app;
- a figura passa dos 2 MB que o mapa aceita e é recusada na importação;
- um texto longo empurra o desenho para fora da imagem, cortado;
- a especificação errada (modelo que não existe, campo faltando) gera uma
  figura vazia em silêncio, em vez de dizer o que está errado;
- o prompt do mapa sem aula não manda usar a ferramenta, e cada mapa sai com
  um visual diferente.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import pymupdf
import pytest

from app.service.mapas.sessao import FERRAMENTAS, INSTRUCOES

LONGO = (
    "um texto bem mais longo do que cabe numa linha só, para obrigar a quebra e "
    "conferir que a caixa cresce junto em vez de cortar o que sobra"
)

MODELOS: dict[str, dict[str, object]] = {
    "fluxo": {
        "etapas": [["Planejamento", "objetivos"], ["Análise", LONGO], ["Execução", "roda"]],
        "faixa": ["Monitoramento e controle", "acompanha todas as atividades"],
    },
    "fluxo_longo": {
        "modelo": "fluxo",
        "etapas": [[f"Etapa {i}", "o que acontece nela"] for i in range(1, 10)],
    },
    "camadas": {"niveis": [["Aceitação", "negócio"], ["Sistema", LONGO], ["Unidade", "isolado"]]},
    "piramide": {"camadas": [["UI", "poucos"], ["Serviço", "médio"], ["Unidade", "muitos"]]},
    "escada": {"degraus": [["1 — Inicial", "imprevisível"], ["2 — Gerenciado", LONGO]]},
    "hierarquia": {
        "raiz": "Qualidade do produto",
        "filhos": [["Usabilidade", LONGO], ["Segurança", "c-i-a"]],
    },
    "grupos": {
        "grupos": [["Acordo", ["Aquisição", "Fornecimento"]], ["Técnicos", ["Requisitos", LONGO]]]
    },
    "quadrantes": {
        "quadrantes": [
            ["Q2", "negócio"],
            ["Q3", "exploratório"],
            ["Q1", "unitário"],
            ["Q4", LONGO],
        ],
        "eixo_x": ["apoiam o time", "criticam o produto"],
        "eixo_y": ["negócio", "tecnologia"],
    },
    "cartoes": {"cartoes": [["S — SRP", "um motivo para mudar"], ["O — OCP", LONGO]]},
    "ciclo": {
        "etapas": [["Coletar contexto", "lê"], ["Agir", "edita"], ["Verificar", LONGO]],
        "centro": "Humano",
    },
}


def _desenhar(tmp_path: Path, spec: dict[str, object]) -> subprocess.CompletedProcess[str]:
    """Roda a ferramenta como o Claude roda: `python -I`, de outra pasta."""
    entrada = tmp_path / "figura.json"
    entrada.write_text(json.dumps(spec, ensure_ascii=False), encoding="utf-8")
    return subprocess.run(
        [
            sys.executable,
            "-I",
            str(FERRAMENTAS / "desenho.py"),
            str(entrada),
            str(tmp_path / "figura.png"),
        ],
        cwd=tmp_path,
        capture_output=True,
        text=True,
        check=False,
    )


@pytest.mark.parametrize("nome", sorted(MODELOS))
def test_cada_modelo_sai_no_padrao_das_figuras(tmp_path: Path, nome: str) -> None:
    spec = {
        "modelo": nome,
        "titulo": "Título da figura",
        "nota": "Uma nota de rodapé.",
        **MODELOS[nome],
    }
    r = _desenhar(tmp_path, spec)
    assert r.returncode == 0, r.stderr

    png = tmp_path / "figura.png"
    assert png.stat().st_size < 2 * 1024 * 1024
    pix = pymupdf.Pixmap(str(png))
    assert pix.alpha == 0
    # Fundo branco nos quatro cantos: o desenho não encosta na borda (nada cortado).
    for x, y in [(2, 2), (pix.width - 3, 2), (2, pix.height - 3), (pix.width - 3, pix.height - 3)]:
        assert pix.pixel(x, y) == (255, 255, 255), f"canto ({x}, {y}) não é branco"
    # Tem desenho de fato, e colorido: não é uma imagem em branco.
    assert (
        len({pix.pixel(x, y) for x in range(0, pix.width, 23) for y in range(0, pix.height, 23)})
        > 10
    )


def test_especificacao_errada_diz_o_que_falta(tmp_path: Path) -> None:
    r = _desenhar(tmp_path, {"modelo": "nao-existe", "titulo": "x"})
    assert r.returncode != 0
    assert "fluxo" in r.stderr  # lista os modelos que existem
    assert not (tmp_path / "figura.png").exists()

    r = _desenhar(tmp_path, {"modelo": "fluxo", "titulo": "x"})
    assert r.returncode != 0
    assert "etapas" in r.stderr


def test_o_prompt_do_mapa_sem_aula_manda_usar_a_ferramenta() -> None:
    prompt = (INSTRUCOES / "prompt_assunto.md").read_text(encoding="utf-8")
    assert "desenho.py" in prompt
