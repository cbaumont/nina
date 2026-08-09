from nina.cli import _extract_auto_flag, _extract_model_flag, run_args


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


def test_extract_auto_flag_absent() -> None:
    rest, auto = _extract_auto_flag(["learn", "python"])
    assert rest == ["learn", "python"]
    assert auto is False


def test_extract_auto_flag_present() -> None:
    rest, auto = _extract_auto_flag(["--auto", "learn", "python"])
    assert rest == ["learn", "python"]
    assert auto is True


def test_extract_model_and_auto_flags_any_order() -> None:
    rest, model = _extract_model_flag(["--auto", "--model", "opus", "learn", "python"])
    rest, auto = _extract_auto_flag(rest)
    assert rest == ["learn", "python"]
    assert model == "opus"
    assert auto is True
