# CRONTAB PLATFORM ADAPTER

Interface + implementations for reading/writing crontab.

## FILES

| File | Role |
|------|------|
| `client.go` | `Client` interface: Read, Apply |
| `system.go` | Real adapter using `crontab -l` / `crontab <file>` |
| `fake.go` | In-memory fake for testing |
| `client_test.go` | Interface contract tests |

## INTERFACE

```go
type Client interface {
    Read(ctx context.Context) (text string, meta ReadMeta, err error)
    Apply(ctx context.Context, content string) (ApplyResult, error)
}
```

## CONVENTIONS

- `ReadMeta.IsEmpty = true` when user has no crontab (not an error)
- `Apply` writes via temp file, invokes `crontab <tempfile`
- `fake.go` holds state in memory—use for unit tests

## ANTI-PATTERNS

- Never call `crontab` directly—always through Client interface
- Never treat "no crontab" as fatal error
