# Contributing to terraform-provider-query

Thank you for your interest in contributing! This document explains how to get
started and what to expect when contributing to this project.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Running the Tool Locally](#running-the-tool-locally)
- [Running Tests](#running-tests)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Coding Conventions](#coding-conventions)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)

## Prerequisites

- **Go 1.21 or later** — see [golang.org/dl](https://golang.org/dl/) or install via Homebrew:
  ```bash
  brew install go
  ```
- **Git**

## Development Setup

```bash
git clone https://github.com/mattias-fjellstrom/terraform-provider-query.git
cd terraform-provider-query
go mod download
```

## Running the Tool Locally

Build and run directly without installing:

```bash
go run . [provider...]
```

Or build the binary:

```bash
go build -o tpq .
./tpq
```

## Running Tests

```bash
go test ./...
```

To run tests with the race detector enabled:

```bash
go test -race ./...
```

To check for common issues:

```bash
go vet ./...
```

## Submitting a Pull Request

1. Fork the repository and create a new branch from `main`:
   ```bash
   git checkout -b your-feature-or-fix
   ```
2. Make your changes and add tests where applicable.
3. Ensure `go vet ./...` and `go test ./...` both pass.
4. Commit with a clear message using [Conventional Commits](https://www.conventionalcommits.org/) style,
   e.g. `feat: add --json flag` or `fix: handle missing namespace`.
5. Push your branch and open a pull request against `main`.
6. Fill in the pull request template and describe what problem your change solves.

## Coding Conventions

- Format all code with `gofmt` (or `goimports`).
- Keep functions small and focused; prefer explicit error returns over panics.
- Exported symbols must have Go doc comments.
- Avoid adding new dependencies unless necessary — the dependency footprint should stay small.

## Reporting Bugs

Use the **Bug report** issue template. Include:
- The `tpq` version or git commit you are running.
- Steps to reproduce the problem.
- Expected vs. actual behaviour.
- Any relevant error output.

## Requesting Features

Use the **Feature request** issue template. Describe the use-case and the
behaviour you would like to see before discussing implementation.
