# Terraform Provider Query

`tpq` is a CLI tool for querying the Terraform registry for provider versions. It supports an interactive TUI mode as well as direct command-line lookups with optional HCL output.

## Prerequisites

You need **Go 1.21 or later** installed. The easiest way to install Go on a Mac is via [Homebrew](https://brew.sh):

```bash
brew install go
```

Alternatively, download the installer from [golang.org/dl](https://golang.org/dl/).

Verify your installation:

```bash
go version
```

## Build

Clone the repository and build the binary:

```bash
git clone https://github.com/mattias-fjellstrom/terraform-provider-query.git
cd terraform-provider-query
go build -o tpq .
```

This produces a `tpq` binary in the current directory.

## Install

To install `tpq` to your Go binary directory (typically `~/go/bin`, which should be on your `PATH`):

```bash
go install .
```

Or copy the binary you built to a directory that is already on your `PATH`, for example:

```bash
cp tpq /usr/local/bin/tpq
```

## Usage

### Interactive TUI

Launch the interactive terminal UI to browse and search Terraform providers:

```bash
tpq
```

### Look up a specific provider

Print the latest version of a provider:

```bash
tpq hashicorp/aws
```

### Output an HCL block

Use the `--hcl` flag to output a ready-to-paste `required_providers` block:

```bash
tpq --hcl hashicorp/aws
```

Example output:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.98.0"
    }
  }
}
```