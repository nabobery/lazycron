# CLI MODULE

Non-interactive CLI subcommands for scripting and diagnostics.

## FILES

| File | Role |
|------|------|
| `cli.go` | Entry point: `Run()` dispatches to subcommands |
| `cli_test.go` | Tests for all subcommands |

## SUBCOMMANDS

| Command | Function | Output |
|---------|----------|--------|
| `lazycron list` | `runList()` | Human-readable or `--json` |
| `lazycron validate` | `runValidate()` | Exit code 1 if issues found |
| `lazycron run <id>` | `runRun()` | Streams stdout/stderr |
| `lazycron doctor` | `runDoctor()` | Platform diagnostics |
| `lazycron` (no args) | Returns `-1` | Signal to launch TUI |

## DEPENDENCY INJECTION

```go
type Deps struct {
    Client      crontab.Client
    Source      domain.CronSource
    Runner      *runner.Runner
    ScheduleSvc *schedule.Service
}
```

Use `DefaultDeps()` for production, inject fakes for tests.

## CONVENTIONS

- Uses Go stdlib `flag` package (no cobra/urfave)
- JSON output: `--json` flag on `list` command
- Return codes: `0` = success, `1` = error, `-1` = launch TUI

## ANTI-PATTERNS

- Never block on stdin for CLI commands
- Never write to stdout for errors—use stderr
