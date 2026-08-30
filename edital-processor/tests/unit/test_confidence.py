from __future__ import annotations

from app.services.confidence import Signals, overall, score, source_quality_for


def test_native_text_beats_ocr_beats_low_conf_ocr() -> None:
    assert source_quality_for("native_text", None) == 1.0
    assert source_quality_for("ocr", 0.95) > source_quality_for("ocr", 0.5)
    assert source_quality_for("ocr", None) == 0.6
    assert source_quality_for("llm", None) == 0.5


def test_no_signals_gives_none() -> None:
    assert score(Signals()) is None


def test_all_positive_signals_score_high() -> None:
    s = Signals(
        source_quality=1.0,
        snippet_valid=True,
        rule_agreement=True,
        arithmetic_ok=True,
        page_conflict=False,
    )
    assert score(s) == 1.0


def test_invalid_snippet_and_conflict_drag_score_down() -> None:
    good = score(Signals(source_quality=1.0, snippet_valid=True))
    bad = score(Signals(source_quality=1.0, snippet_valid=False, page_conflict=True))
    assert good is not None and bad is not None
    assert bad < good


def test_partial_signals_are_weighted_by_what_applies() -> None:
    # only source_quality present → the result is just that value
    assert score(Signals(source_quality=0.8)) == 0.8


def test_overall_is_mean_of_present_confidences() -> None:
    assert overall([0.8, 0.6, None, 1.0]) == 0.8
    assert overall([None, None]) is None
