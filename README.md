# Terraform Provider Query

`tpq` is an interactive terminal UI (TUI) for browsing the Terraform registry — searching providers, exploring their versions, reading release notes, and viewing provider documentation, all without leaving your terminal.

## Install

### macOS (Homebrew)

`tpq` is published as a [Homebrew](https://brew.sh) cask via the
[`mattias-fjellstrom/homebrew-tap`](https://github.com/mattias-fjellstrom/homebrew-tap)
tap:

```bash
brew install mattias-fjellstrom/tap/tpq
```

To upgrade to the latest release:

```bash
brew upgrade mattias-fjellstrom/tap/tpq
```

Note that the binary is currently not signed using an Apple Developer ID, which means it might get flagged as a security risk. Install at your own peril! 

### Linux and Windows (pre-built binary)

Pre-built binaries for Linux and Windows (both `amd64` and `arm64`) are
attached to each release on the
[releases page](https://github.com/mattias-fjellstrom/terraform-provider-query/releases).

1. Download the archive for your operating system and architecture
   (`.tar.gz` for Linux, `.zip` for Windows).
2. Extract the `tpq` binary from the archive.
3. Move it to a directory on your `PATH`, for example:

   ```bash
   # Linux
   sudo mv tpq /usr/local/bin/tpq
   ```

   On Windows, place `tpq.exe` in a folder that is included in your `PATH`
   environment variable.

Verify the installation:

```bash
tpq --help
```

### Build from source

If you'd rather build `tpq` yourself, you need **Go 1.24 or later** installed.
This project is only tested with the three latest minor versions of Go
(currently 1.24, 1.25, and 1.26). On macOS the easiest way to install Go is
via Homebrew (`brew install go`); on other systems use
[golang.org/dl](https://golang.org/dl/).

Clone the repository and build the binary:

```bash
git clone https://github.com/mattias-fjellstrom/terraform-provider-query.git
cd terraform-provider-query
go build -o tpq .
```

Then either install it to your Go binary directory (typically `~/go/bin`,
which should be on your `PATH`):

```bash
go install .
```

…or copy the binary you built to a directory that is already on your `PATH`:

```bash
cp tpq /usr/local/bin/tpq
```

## Usage

Launch `tpq` to open the terminal UI:

```bash
tpq
```

`tpq` is a TUI-only application — there are no command-line lookup modes or
flags for one-shot queries. The only flags are `--version` (print the version
and exit) and `--help`.

The TUI opens to a **browse** screen with a search input and a list of
official and partner providers from the Terraform Registry, grouped by tier
and sorted by downloads. Community providers are intentionally skipped to keep
the list focused and the load fast:

- Start typing to filter the list by `namespace/name` (case-insensitive
  substring match).
- Use the arrow keys to highlight a provider, and press `enter` to drop into
  the **versions** screen for that provider.
- On a version, press `enter` to view its release notes, or `d` to open the
  documentation page for that version on the Terraform Registry website
  (in your default browser).
- `esc` clears the filter (or returns to the previous screen).
- `ctrl+c` quits.

### GitHub authentication (optional)

Release notes and version publish dates are fetched from the GitHub API.
Unauthenticated requests are limited to 60 per hour, which is easy to hit
when browsing many providers. `tpq` will automatically use a GitHub token
to raise the limit to 5,000 requests per hour, looking in this order:

1. The `GITHUB_TOKEN` environment variable.
2. The `GH_TOKEN` environment variable.
3. The token from the [GitHub CLI](https://cli.github.com), if `gh` is
   installed and you are signed in (`gh auth login`). `tpq` runs
   `gh auth token` to retrieve it.

No token is required — `tpq` will fall back to unauthenticated requests if
none is found.
