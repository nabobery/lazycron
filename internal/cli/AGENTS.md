# CLI MODULE

Non-interactive CLI subcommands for scripting and diagnostics.

## FILES

| File | Role |
|------|------|
| `cli.go` | Entry point: `Run()` dispatches to subcommands |
| `cli_test.go` | Tests for all subcommands |

## SUBCOMMANDS

| Command | Function | Flags | Output |
|---------|----------|-------|--------|
| `lazycron list` | `runList()` | `--json`, `--all` | Human-readable or JSON |
| `lazycron validate` | `runValidate()` | `--all` | Exit code 1 if issues found |
| `lazycron run <id>` | `runRun()` | `--all`, `--env` | Streams stdout/stderr |
| `lazycron doctor` | `runDoctor()` | — | Platform diagnostics (includes system cron) |
| `lazycron` (no args) | Returns `-1` | — | Signal to launch TUI |

The `--all` flag includes system cron sources (via `Deps.Discoverer`) in addition to the user crontab. Without `--all`, only the user crontab is queried.

## DEPENDENCY INJECTION

```go
type Deps struct {
    Client      crontab.Client
    Source      domain.CronSource
    Runner      *runner.Runner
    ScheduleSvc *schedule.Service
    Discoverer  *systemcron.Discoverer // nil-safe; enables --all
}
```

Use `DefaultDeps()` for production, inject fakes for tests. When `Discoverer` is nil, `--all` falls back to user-only behavior.

## CONVENTIONS

- Uses Go stdlib `flag` package (no cobra/urfave)
- JSON output: `--json` flag on `list` command; `source` emitted for all jobs
- Return codes: `0` = success, `1` = error, `-1` = launch TUI
- Source-level issues (LineIndex < 0) print without a line number prefix
- Run-as-user mismatch warning printed to stderr (non-fatal) when running a system job

## ANTI-PATTERNS

- Never block on stdin for CLI commands
- Never write to stdout for errors—use stderr
