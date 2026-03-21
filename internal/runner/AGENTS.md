# RUNNER MODULE

Subprocess execution with bounded output and environment control.

## FILES

| File | Role |
|------|------|
| `runner.go` | Runner: executes cron jobs as subprocesses |
| `runner_test.go` | Tests for success, failure, cancellation, large output |

## CONFIGURATION

```go
type Config struct {
    MaxOutputBytes int  // default: 1MB
}
```

## EXECUTION MODES

| Mode | Behavior |
|------|----------|
| `EnvModeCronLike` | Minimal env (PATH=/usr/bin:/bin, SHELL=/bin/sh) |
| `EnvModeShellInherit` | Full shell environment |

## PERCENT SEMANTICS

In cron-like mode, `%` in commands is converted to newline and passed via stdin:
- `echo foo%bar` → `echo foo\nbar` via stdin
- First `%` starts stdin data, subsequent `%` are newlines

## ENVIRONMENT HANDLING

- `HOME`, `LOGNAME`, `USER`, `PATH`, `SHELL` are set in cron-like mode
- `LOGNAME` and `USER` are pinned (cannot be overridden by job env)
- `TZ` is set from `Schedule.Timezone` if present
- Working directory is set to `HOME` in cron-like mode

## PROCESS MANAGEMENT

- Jobs run in a new process group (`Setpgid: true`)
- Cancellation sends `SIGKILL` to the entire process group
- Output is bounded (default 1MB per stream) with truncation flag

## ANTI-PATTERNS

- Never block UI thread—always run via tea.Cmd
- Never leak child processes on cancel—use process groups
- Never assume command runs in shell—explicitly invoke `/bin/sh -c`
