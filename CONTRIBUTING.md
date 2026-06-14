# Contributing to goXpress

Thanks for your interest in improving goXpress! This document explains how to
get set up and the conventions we follow.

## Getting started

```sh
git clone https://github.com/chmenegatti/goxpress
cd goxpress
make test
```

Requires Go 1.26+.

## Development workflow

Common tasks are wrapped in the `Makefile`:

| Command | Description |
| ------- | ----------- |
| `make test` | Run all tests with the race detector |
| `make cover` | Run tests and produce a coverage report |
| `make bench` | Run benchmarks |
| `make lint` | Run `golangci-lint` |
| `make vet` | Run `go vet` |
| `make check` | Run `vet`, `lint` and `test` (run this before pushing) |

## Conventions

- **Zero dependencies in the core package.** Anything that needs a third-party
  library lives in an opt-in sub-package.
- **`net/http` compatibility is non-negotiable.** The router is an
  `http.Handler` and integrates with the standard ecosystem.
- **Document every exported symbol** with a proper godoc comment.
- **Table-driven tests** for new behavior; keep coverage high.
- Run `make check` before opening a pull request.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <description>
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`, `ci`.

## Pull requests

1. Fork and create a feature branch.
2. Make your change with tests and docs.
3. Ensure `make check` passes.
4. Open a PR describing the change and the motivation.
