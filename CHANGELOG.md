# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.9.0] - 2026-10-13

### Added
- `--json` flag that prints the provider configuration in Terraform JSON
  syntax (e.g. `tpq --json aws azurerm tfe > providers.tf.json`)

### Changed
- Generating provider configuration no longer requires `--hcl`: passing
  providers (`tpq aws azurerm tfe`) prints HCL by default
- Flags may now appear after the providers (e.g. `tpq aws tfe --json`)

## [0.8.0] - 2026-10-13

### Added
- `--hcl PROVIDER...` one-shot mode that prints a `terraform` block pinning
  the latest stable versions of the given providers, plus a `provider` block
  for each (e.g. `tpq --hcl aws azurerm tfe > providers.tf`)

## [0.7.0] - 2026-10-13

### Added
- In-memory session cache for provider lists, versions, publish dates, and
  release notes so data is only fetched once per session (failed requests
  are not cached)

### Fixed
- Opening release notes could hang on the loading screen for several
  seconds, even for cached notes: Markdown rendering re-detected the
  terminal background on every render, racing Bubble Tea for stdin. The
  style is now detected once at startup and rendering happens off the UI
  thread

## [0.6.0] - 2026-04-30

### Added
- MIT license
- `CONTRIBUTING.md` contributor guide
- `SECURITY.md` vulnerability disclosure policy
- `CHANGELOG.md` (this file)
- `.gitignore` for Go projects
- CI workflow (build + test on every push and pull request)
- Release workflow with GoReleaser for cross-platform binaries
- GitHub issue templates (bug report, feature request)
- GitHub pull request template
- Unit tests for registry and command packages
- TUI documentation browser: press `d` on a version to list its docs
  (resources, data sources, ephemeral resources, actions, functions,
  guides, overview), fuzzy-filter with `/`, and render any page in a
  scrollable Markdown viewport
- TUI browse view now also lists providers from HashiCorp's new
  `partner-premier` tier, ranked between official and partner with a
  distinct badge

### Changed
- TUI browse view no longer fetches community-tier providers; only
  official, partner-premier, and partner providers are listed

### Removed
- TUI usage snippet view (`u` shortcut from the version list)
- `--hcl` CLI flag for outputting HCL required_providers blocks
- All non-interactive CLI lookup modes; `tpq` is now a TUI-only application
  (only `--version` and `--help` flags remain)

## [0.5.0] - 2026-04-22

### Added
- Support multiple providers in a single CLI invocation (`tpq hashicorp/aws hashicorp/azurerm`)

## [0.4.0] - 2026-04-22

### Added
- Support `namespace/name` syntax for provider queries

## [0.3.0] - 2026-04-22

### Fixed
- Reset version list cursor position when switching providers in TUI mode

## [0.2.0] - 2026-04-22

### Added
- Render Markdown in release-notes viewport in TUI mode

### Fixed
- Populate published dates for versions in TUI mode

## [0.1.0] - 2026-04-22

### Added
- Initial implementation with interactive TUI and CLI modes
- Query the Terraform registry for the latest provider version
