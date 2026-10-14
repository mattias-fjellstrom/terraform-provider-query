# Terraform Provider Query

`tpq` is an interactive terminal UI (TUI) for browsing the Terraform registry — searching providers, exploring their versions, reading release notes, and viewing provider documentation, all without leaving your terminal.

![Made with VHS](https://vhs.charm.sh/vhs-4G3eH1NqimkvlCIMI3HoOh.gif)

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

Pass one or more providers to print Terraform configuration for them instead
of opening the TUI (see [Generate provider configuration](#generate-provider-configuration)):

```bash
tpq aws azurerm tfe
```

Run `tpq check` in a directory containing a Terraform configuration to see
which providers are out of date, and `tpq check --upgrade` to pin them to
the latest versions (see [Check a configuration for outdated providers](#check-a-configuration-for-outdated-providers)):

```bash
tpq check
tpq check --upgrade
```

`tpq` supports a few flags, which may appear before or after the providers:

- `--hcl` — print the configuration in HCL syntax (the default).
- `--json` — print the configuration in Terraform JSON syntax.
- `--version` — print the version and exit.
- `--help` — show usage.

The TUI opens to a **browse** screen with a search input and a list of
official, partner-premier, and partner providers from the Terraform Registry, grouped by tier
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

Provider lists, versions, publish dates, and release notes are cached in
memory for the rest of the session, so revisiting a provider or version is
instant and doesn't make additional API requests. Failed requests are not
cached and will be retried the next time. Restart `tpq` to fetch fresh data.

### Generate provider configuration

Run `tpq` followed by one or more providers to print a `terraform` block that
pins each provider to its latest stable version, plus an empty `provider`
block for each one. Providers without a namespace default to `hashicorp`; use
`namespace/name` for others (e.g. `integrations/github`).

```bash
tpq aws azurerm tfe
```

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.68.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "5.9.0"
    }
    tfe = {
      source  = "hashicorp/tfe"
      version = "0.82.0"
    }
  }
}

provider "aws" {
  # add configuration here
}

provider "azurerm" {
  # add configuration here
}

provider "tfe" {
  # add configuration here
}
```

The configuration is written to standard output (errors go to standard
error), so you can redirect it straight into a file:

```bash
tpq aws azurerm tfe > providers.tf
```

HCL is the default output format (`--hcl` selects it explicitly). Use
`--json` to get the same configuration in
[Terraform JSON syntax](https://developer.hashicorp.com/terraform/language/syntax/json)
instead:

```bash
tpq --json aws azurerm tfe > providers.tf.json
```

```json
{
  "terraform": {
    "required_providers": {
      "aws": {
        "source": "hashicorp/aws",
        "version": "6.68.0"
      },
      "azurerm": {
        "source": "hashicorp/azurerm",
        "version": "5.9.0"
      },
      "tfe": {
        "source": "hashicorp/tfe",
        "version": "0.82.0"
      }
    }
  },
  "provider": {
    "aws": {
      "//": "add configuration here"
    },
    "azurerm": {
      "//": "add configuration here"
    },
    "tfe": {
      "//": "add configuration here"
    }
  }
}
```

### Check a configuration for outdated providers

Run `tpq check` in a directory containing a Terraform configuration to
compare the version constraint of every provider in its
`terraform { required_providers { ... } }` blocks with the latest stable
version in the Terraform Registry. Both `*.tf` and `*.tf.json` files in the
current directory are read (subdirectories, such as nested modules, are not).

```bash
tpq check
```

```text
PROVIDER             CONSTRAINT  LATEST  STATUS      LOCATION
integrations/github  6.13.0      6.13.0  up to date  github.tf:5
hashicorp/aws        ~> 5.0      6.68.0  outdated    versions.tf:5
hashicorp/random     >= 3.0      3.9.1   not pinned  versions.tf:9

2 of 3 providers can be upgraded. Run "tpq check --upgrade" to pin them to the latest versions.
```

Each provider gets one of these statuses:

- `up to date` — the constraint pins exactly the latest version (e.g. `"6.68.0"`).
- `outdated` — the constraint does not allow the latest version.
- `not pinned` — the constraint allows the latest version but does not pin it
  exactly (e.g. `">= 3.0"`).
- `no constraint` — the provider has no `version` argument.
- `error` — the provider could not be checked, e.g. because its source is not
  on `registry.terraform.io`.

Run `tpq check --upgrade` to rewrite every `outdated` and `not pinned`
constraint so that it pins the latest version:

```bash
tpq check --upgrade
```

```text
versions.tf:5: hashicorp/aws "~> 5.0" -> "6.68.0"
versions.tf:9: hashicorp/random ">= 3.0" -> "3.9.1"

Upgraded 2 providers. Run "terraform init -upgrade" to update the dependency lock file.
```

Only the quoted version strings are changed: files, formatting, comments,
and the order of blocks and arguments are left exactly as they were.
Providers without a `version` argument are skipped. Afterwards, run
`terraform init -upgrade` to update `.terraform.lock.hcl`.

> [!NOTE]
> `tpq check` is deliberately opinionated: it prefers **exact version pins**.
> A constraint is only considered up to date when it pins exactly the latest
> version, and `--upgrade` replaces the whole constraint (e.g. `"~> 5.0"` or
> `">= 3.0, < 5.0"`) with the latest version (e.g. `"6.68.0"`) rather than
> preserving its operators. Review the changes before applying them, as a
> new major version may contain breaking changes.

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
