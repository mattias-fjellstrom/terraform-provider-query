# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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

### Changed
- TUI browse view no longer fetches community-tier providers; only official and partner providers are listed

### Removed
- TUI usage snippet view (`u` shortcut from the version list)
- `--hcl` CLI flag for outputting HCL required_providers blocks

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
