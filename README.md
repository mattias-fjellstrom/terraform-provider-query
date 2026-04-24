# Terraform Provider Query

`tpq` is a CLI tool for querying the Terraform registry for provider versions. It supports an interactive TUI mode as well as direct command-line lookups.

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

### Interactive TUI

Launch the interactive terminal UI to browse and search Terraform providers:

```bash
tpq
```

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
