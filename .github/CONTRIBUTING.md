# Contributing to LazyCron

## Welcome

Thanks for your interest in helping improve LazyCron.
This project provides a terminal-first cron management experience with a strict
focus on safety and document preservation.
Your contributions are welcome, whether they are:

- Bug reports and reproducible cases
- Documentation improvements
- Parser and scheduling fixes
- TUI refinements
- Test coverage and quality improvements
- CI, release, or developer-experience improvements

This guide explains how to contribute effectively and follow project conventions.

## Scope and expectations

Contributions should prioritize:

- Reliable behavior over clever shortcuts.
- Readability and maintainability over novelty.
- Documentation that explains not only *what* changed, but *why*.
- Conservative risk around cron file mutations and user data preservation.
- Consistent and predictable behavior in both TUI and CLI paths.

Please avoid:

- Breaking public CLI output formats without documenting the migration path.
- Non-reproducible changes without test coverage.
- Changes that introduce destructive behavior around `crontab` or discovery sources.

## Before you start

Before opening any pull request:

1. Confirm you can run the project locally using current dependencies.
2. Ensure no untracked environment assumptions are required.
3. Check whether an existing issue already covers your change.
4. Be ready to include tests (or explain why not applicable).
5. Provide reproducible steps in your issue or PR description.

## Prerequisites

- Go 1.25.0+
- `git` and a POSIX-like shell
- `just` task runner
- A working terminal for TUI interaction

## Setup

Clone and enter the repository:

```bash
git clone https://github.com/nabobery/lazycron.git
cd lazycron
```

Install dependencies through normal Go module commands:

```bash
go mod download
```

Verify baseline tooling:

```bash
go version
just --version
```

## Build and validation commands

The project uses the following canonical commands:

- Format: `just fmt`
- Strict formatting check: `just fmt-check`
- Lint: `just lint`
- Tests: `just test`
- Race tests: `just test-race`
- Build: `just build`
- Release prep workflow: `just check`
- Local CI baseline: `just ci`
- GitHub Actions CI also runs race tests.

Please run at least these commands before asking for review:

- `just fmt-check`
- `just lint`
- `just test`
- `just build`

## Development workflow

### 1) Branching

Create a dedicated branch from `main`:

```bash
git checkout -b <type>/<short-description>
```

Recommended prefix examples:

- `feat/` for feature additions
- `fix/` for bug fixes
- `docs/` for documentation-only changes
- `chore/` for build/release/process updates

### 2) Make code changes

Prefer small, targeted commits.
Avoid broad changes that mix unrelated workstreams.
For example, do not combine:

- parser rewrites with UI polish
- workflow changes with runtime behavior changes
- docs updates with release packaging edits

### 3) Validate locally

Run the quality checks before pushing:

```bash
just fmt
just lint
just test-race
just build
```

If you're touching markdown-only files:

```bash
just check
```

For any new behavior, add tests in the relevant package and include:

- Positive and negative cases
- Reproducible error handling paths
- CLI or parser input/output expectations

## Code style

LazyCron uses idiomatic Go conventions and project-specific safety patterns.

### Formatting

- Run `gofmt` via `just fmt`.
- Do not hand-format spacing, imports, or blocks.
- Keep line length readable while not forcing artificial wrapping.

### Linting

- Use `golangci-lint` behavior via `just lint`.
- Prefer straightforward control flow in parser and state transition code.
- Keep new imports organized and avoid unnecessary aliases.

### Error handling

- Return context-rich errors where appropriate.
- Avoid swallowing errors during file and process operations.
- Preserve previous document fidelity unless an explicit transformation is desired.

### Logging and observability

- Use consistent log messages in helper services.
- Surface actionable errors to callers instead of silent fallback paths.

## Testing requirements

### Minimum baseline

Before opening a PR, include evidence that:

- `just lint` passes.
- `just test` passes.
- `just test-race` either passes or is explicitly documented why not.
- `just build` passes.

### Test coverage expectations

For non-trivial changes, include unit tests that cover:

- Parsing and rendering edge cases
- Validation and normalization behavior
- Branches around mutable vs read-only jobs
- CLI and TUI interactions (if behavior changed)
- Scheduling calculations for schedule expressions

### Snapshot and regression safety

For parser behavior changes, include fixtures that prove:

- Existing formatting is preserved where expected.
- New behavior does not corrupt blank lines, comments, and env entries.
- Document-level roundtrip invariants remain intact.

## Pull request process

Open a draft PR early when you are still validating behavior.
Use `main` as your target branch.

Every PR should include:

- Clear summary of behavior changed.
- Why this change is needed.
- Test steps and results.
- Risks and follow-up tasks (if any).

Checklist before opening:

- [ ] `just fmt-check` passes.
- [ ] `just lint` passes.
- [ ] `just test` passes.
- [ ] `just build` passes.
- [ ] PR description includes test output.

PRs are expected to keep changes scoped and reviewable.
Large PRs should be split where practical.

## Review process

Reviewers look for:

- Behavioral correctness
- Potential parser regressions
- Safety checks around mutable system operations
- Test quality and confidence
- Quality of docs and migration notes

Please be responsive to follow-up questions and avoid force-pushing through an
active review without explanation.

## Bug reporting

Use `.github/ISSUE_TEMPLATE/bug_report.yml` for structured bug reports.
Include at least:

- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment details
- Repro log or screenshot where possible

If applicable:

- cron syntax used
- command output snippets
- OS and Go version

For high-impact defects, include a minimal cron snippet that demonstrates the issue.

## Feature requests

Use `.github/ISSUE_TEMPLATE/feature_request.yml` for proposals.
Include:

- Problem statement
- Proposed approach
- Alternatives considered
- Trade-offs or compatibility concerns
- Suggested validation strategy

Community contributions that include feature flags, rollout notes, and migration impact
are prioritized for review.

## Documentation

For any user-visible feature changes, update:

- `README.md` (as needed)
- Any command examples
- Help text or usage docs if CLI output changes
- Relevant tests for docs-driven workflows

## Release process expectations

Releases are currently managed with `goreleaser` on tagged versions.
Follow project maintainers for release cut timing.
Contributors should focus on clean commits, clear changelog notes, and stable test
coverage before merge.

## Community and communication

- Be respectful and assume positive intent.
- Keep discussion technical and actionable.
- Ask questions when unclear.

## License and attribution

By contributing, you agree that your contributions may be used under the project
license (MIT) and that you have the right to submit the work.

Questions? Create an issue and we will route it to the right maintainers.
