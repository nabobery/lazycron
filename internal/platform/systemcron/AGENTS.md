# SYSTEM CRON DISCOVERY

Discovers read-only system cron sources: /etc/crontab, /etc/cron.d/*, periodic dirs.

## FILES

| File | Role |
|------|------|
| `discovery.go` | Discoverer: enumerates all system cron sources |
| `discovery_test.go` | Tests with FS mock |

## ARCHITECTURE

```
Discoverer.DiscoverAll()
  ├── candidates()         # Platform-aware source list
  ├── discoverFile()       # Single file (e.g., /etc/crontab)
  ├── discoverCronDDir()   # /etc/cron.d/* files
  └── discoverPeriodicDir() # /etc/cron.{hourly,daily,weekly,monthly}
```

## FS INTERFACE

```go
type FS interface {
    ReadFile(name string) ([]byte, error)
    Stat(name string) (fs.FileInfo, error)
    ReadDir(name string) ([]fs.DirEntry, error)
}
```

Use `New()` for production, `NewWithFS(f)` for tests.

## OUTPUT TYPES

| Type | Description |
|------|-------------|
| `DiscoveredSource` | Source + raw text content |
| `PeriodicEntry` | Script in periodic dir (name, path, interval) |

## CANDIDATES

| Path | Subkind | Platform |
|------|---------|----------|
| `/etc/crontab` | `SubkindSystemCrontab` | Linux |
| `/etc/cron.d` | `SubkindCronD` | Both |
| `/etc/cron.hourly` | `SubkindPeriodicDir` | Both |
| `/etc/cron.daily` | `SubkindPeriodicDir` | Both |
| `/etc/cron.weekly` | `SubkindPeriodicDir` | Both |
| `/etc/cron.monthly` | `SubkindPeriodicDir` | Both |
| `~/Library/LaunchAgents` | `SubkindCronD` | macOS |

## ANTI-PATTERNS

- Never attempt writes to system cron sources
- Never skip permission errors—report as validation issues
- Never assume all candidates exist—handle missing gracefully
