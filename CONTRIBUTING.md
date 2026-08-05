# Contributing

Thanks for considering a contribution to Nina.

## Getting started

Requires Go 1.25+ and git. Clone the repo and build:

```sh
go build ./...
```

## Before opening a PR

```sh
gofmt -l .      # should print nothing; run `gofmt -w` on anything it lists
go vet ./...
go test ./...
```

These are the same checks CI runs (`.github/workflows/ci.yml`), plus `go test -race`.

## Guidelines

* Keep changes small and focused; one logical change per PR.
* Add or update tests for any non-trivial behaviour change.
* No code comments unless the code genuinely can't be made obvious without one.
* Match existing naming and code style rather than introducing new patterns.
* Write commit messages in the imperative mood, without prefixes (e.g. `Add user profile validation`, not `feat: add...`).

## Reporting issues

Open a GitHub issue with steps to reproduce, what you expected, and what happened instead.
