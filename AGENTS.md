# LAZYCRON KNOWLEDGE BASE

**Generated:** 2026-03-20
**Branch:** main
**Module:** github.com/avinashchangrani/lazycron

## OVERVIEW

Terminal UI for managing cron jobs. Go + Bubble Tea v2 TUI with document-preserving parser. Targets user crontab read/write and system cron read-only discovery.

## STRUCTURE

```
lazycron/
├── cmd/lazycron/       # Entry point (main.go) — dispatches CLI or TUI
├── internal/
│   ├── app/            # ApplyService: load, toggle, delete, create, edit with drift detection
│   ├── cli/            # Non-interactive CLI subcommands (list, validate, run, doctor)
│   ├── cronparse/      # Document-preserving cron parser (Parse, Render)
│   ├── domain/         # Core types (CronJob, CronDocument, ScheduleSpec, JobDraft, etc.)
│   ├── platform/crontab/  # crontab Client interface + system adapter
│   ├── runner/         # Subprocess execution with bounded output
│   ├── schedule/       # Next-run calculation + human descriptions
│   ├── testutil/       # Shared test helpers
│   └── tui/            # Bubble Tea Model/Update/View + editor (10 files)
├── docs/
│   ├── specs/          # Technical specification
│   └── plans/          # PRD
└── justfile            # Task runner (fmt, lint, test, build, run)
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add TUI feature | `internal/tui/` | Model/Update/View pattern |
| Change parsing | `internal/cronparse/parser.go` | Document-preserving, must preserve raw fidelity |
| Safe crontab write | `internal/app/apply.go` | Drift detection via baseline comparison |
| Create/edit jobs | `internal/app/edit.go` | CreateJob/EditJob patch only targeted lines |
| Add domain type | `internal/domain/types.go` | Shared across all packages |
| Validation helpers | `internal/domain/validation.go` | Field-level + full-expression validation |
| Schedule math | `internal/schedule/service.go` | Uses robfig/cron/v3 |
| Job execution | `internal/runner/runner.go` | Bounded buffer, process group for cancellation |
| CLI subcommands | `internal/cli/cli.go` | list, validate, run, doctor via flag.NewFlagSet |
| TUI editor | `internal/tui/editor.go` | Modal create/edit form with preview |

## CONVENTIONS

- **Two-layer parser**: raw document (classified lines) → normalized job projection
- **Disabled marker**: `# [lazycron-disabled] ` prefix, reversible round-trip
- **Drift detection**: Compare baseline before apply, refuse silent overwrite
- **Interface-first platform**: `crontab.Client` interface enables fake for testing
- **MVU for TUI**: `Model` struct, `Update` handles `tea.Msg`, `View` renders

## ANTI-PATTERNS (THIS PROJECT)

- Never suppress crontab read errors as empty state without checking exit code
- Never flatten document to jobs-only—preserve comments/env/blank lines for round-trip
- Never execute jobs automatically—manual run requires explicit user action
- Never bypass `crontab.Client` interface for direct system calls

## COMMANDS

```bash
just run          # go run ./cmd/lazycron
just test         # go test ./...
just test-race    # go test -race ./...
just build        # go build ./...
just fmt          # gofmt -w on all .go files
just lint         # golangci-lint or go vet fallback
just check        # fmt-check + lint + test
just ci           # check + build
```

## NOTES

- Go 1.25.0, Bubble Tea v2 (`charm.land/bubbletea/v2`), robfig/cron/v3
- Platform targets: macOS (latest), Ubuntu 24.04 LTS
- Three-pane layout: jobs list (left), details (top-right), logs (bottom-right)
- Cron-like env mode: minimal PATH/SHELL/HOME vs shell-inherit mode
