from nina.cli import _extract_model_flag, run_args


def test_no_args_prints_usage() -> None:
    run_args([])


def test_version() -> None:
    run_args(["version"])


def test_extract_model_flag_absent() -> None:
    rest, model = _extract_model_flag(["learn", "python"])
    assert rest == ["learn", "python"]
    assert model is None


def test_extract_model_flag_space_form() -> None:
    rest, model = _extract_model_flag(["--model", "opus", "learn", "python"])
    assert rest == ["learn", "python"]
    assert model == "opus"


def test_extract_model_flag_equals_form() -> None:
    rest, model = _extract_model_flag(["learn", "--model=haiku", "python"])
    assert rest == ["learn", "python"]
    assert model == "haiku"
