from __future__ import annotations

from app.services.quality import score_page_text


def test_empty_text_scores_zero() -> None:
    assert score_page_text("").score == 0.0
    assert score_page_text("   \n  ").score == 0.0


def test_clean_prose_scores_high() -> None:
    prose = (
        "O Tribunal de Contas do Estado de Goias torna publica a abertura de "
        "inscricoes para o concurso publico de provas, conforme as instrucoes "
        "especiais que fazem parte deste edital. As disciplinas e o conteudo "
        "programatico constam do Anexo II. " * 6
    )
    assert score_page_text(prose).score > 0.75


def test_broken_native_layer_scores_below_ocr_threshold() -> None:
    # What a mangled PDF text layer looks like: replacement chars, control
    # bytes, digit noise where letters should be, two-char fragments.
    garble = "�l�� 1O th3 �dital c0 nt��nt \x00\x01 pr0gr4m ��� " * 8
    assert score_page_text(garble).score < 0.55


def test_lightly_noisy_ocr_text_stays_usable() -> None:
    # Mild OCR substitution errors on otherwise-Portuguese prose must NOT drag
    # the score below the floor — the page already came from OCR, re-running it
    # would not help.
    noisy = (
        "Trlbunai de Ccntas do Est ado de Golas toma publlca a abenura de "
        "mscncces para o concLrso publlco conforme as mstrucces especiais "
    ) * 3
    assert score_page_text(noisy).score >= 0.55


def test_repeated_line_penalized() -> None:
    unique = "\n".join(f"linha distinta numero {i} com conteudo real" for i in range(20))
    repeated = "\n".join("a mesma linha repetida sem parar" for _ in range(20))
    assert score_page_text(unique).repetition == 1.0
    assert score_page_text(repeated).repetition < 0.1


def test_short_page_scores_lower_than_full_page() -> None:
    short = score_page_text("Anexo II")
    full = score_page_text("Conteudo programatico detalhado por disciplina. " * 40)
    assert short.length < full.length
    assert short.score < full.score
