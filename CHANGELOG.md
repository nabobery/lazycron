# Changelog

All notable changes to LazyCron will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- No unreleased changes yet.

## [0.1.0] - 2026-03-21

### Added

- Initial LazyCron release:
  - Terminal-first Bubble Tea v2 TUI for managing cron jobs.
  - CLI subcommands for listing, validating, running, and checking cron jobs.
  - Three-pane interactive layout with jobs, details, and logs.
  - Document-preserving cron parsing and rendering.
  - Safe apply flow with drift detection and validation before writes.
  - Create, edit, toggle, and delete job flows with read-only system cron awareness.
  - Schedule previews and next-run calculations for cron expressions and descriptors.
  - Manual execution with session-scoped logs and bounded subprocess output.
  - System cron discovery for read-only visibility into host cron sources.
  - Support for special cron syntax such as `@reboot`, `@daily`, `@hourly`, and disabled markers.
  - Community contribution infrastructure and release automation baseline:
    - CONTRIBUTING guide.
    - PR and issue templates.
    - Code of Conduct.
    - GitHub Actions CI and release workflows.
    - GolangCI and GoReleaser configuration.

[`Unreleased`]: https://github.com/nabobery/lazycron/compare/v0.1.0...HEAD
[`0.1.0`]: https://github.com/nabobery/lazycron/releases/tag/v0.1.0
