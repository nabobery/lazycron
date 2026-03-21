# CRON LOG PROVIDERS

Fetches cron-related log entries from system log sources.

## FILES

| File | Role |
|------|------|
| `provider.go` | Provider interface + Query/Result types |
| `auto.go` | AutoProvider: platform-aware provider chain |
| `journalctl.go` | Linux: journalctl integration |
| `syslog.go` | Fallback: reads /var/log/{syslog,cron,messages} |
| `noop.go` | No-op for unsupported platforms |
| `provider_test.go` | Interface contract tests |

## PROVIDER INTERFACE

```go
type Provider interface {
    Fetch(ctx context.Context, q Query) (Result, error)
    Name() string
}
```

## QUERY / RESULT

```go
type Query struct {
    Since   time.Time
    Until   time.Time
    Limit   int
    Command string  // substring match
    User    string  // filter by cron user
}

type Result struct {
    Lines    []string
    Source   string  // where logs came from
    Partial  bool    // output truncated
    NotFound bool    // no source available
    Reason   string  // explanation when NotFound
}
```

## PLATFORM BEHAVIOR

| OS | Provider Chain | Notes |
|----|----------------|-------|
| Linux | journalctl → syslog | journalctl preferred, syslog fallback |
| macOS | noop | Console.app recommended |
| Other | noop | Not supported |

## ANTI-PATTERNS

- Never block on log fetch—always async via tea.Cmd
- Never crash if log source unavailable—return NotFound with Reason
- Never parse journalctl output with fragile regex—use line-based filtering
