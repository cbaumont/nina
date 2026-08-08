from nina.cli import run_args


def test_no_args_prints_usage() -> None:
    run_args([])


def test_version() -> None:
    run_args(["version"])
