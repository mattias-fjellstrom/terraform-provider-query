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

- **Go 1.24 or later** (only the three latest minor versions are tested) — see [golang.org/dl](https://golang.org/dl/) or install via Homebrew:
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

`tpq` is a TUI-only application; running it always launches the interactive
terminal UI.

Build and run directly without installing:

```bash
go run .
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
   e.g. `feat: add docs viewport scrollbar` or `fix: handle missing namespace`.
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

## Recording the README demo

The animated/video demo embedded in the README is generated from
[`demo.tape`](./demo.tape) using [VHS](https://github.com/charmbracelet/vhs).
The script is committed; the rendered output is git-ignored.

Install VHS (macOS):

```bash
brew install vhs
```

Render the demo from the repo root:

```bash
vhs demo.tape
```

This produces `demo.gif`, `demo.mp4`, and `demo.webm`. None of these are
committed — pick one of the following hosting options and reference it from
`README.md`:

- **`vhs publish demo.gif`** — uploads to Charm's CDN and prints a
  `https://vhs.charm.sh/...gif` URL. Simplest option, used by the current
  README.
- **GitHub user-attachments** — open a new issue or PR comment and drag
  `demo.mp4` into it. GitHub uploads the file and yields a
  `https://github.com/user-attachments/assets/<id>` URL that auto-embeds as a
  `<video>` player when pasted on its own line.

Prefer MP4 over GIF when self-hosting — it is typically 5–10× smaller for the
same clip. Keep the recording short (≤ 20 s) and the terminal modestly sized
so the file stays small.
