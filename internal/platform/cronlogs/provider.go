package cronlogs

import (
	"context"
	"time"
)

// Query specifies what log entries to retrieve.
type Query struct {
	Since   time.Time
	Until   time.Time
	Limit   int
	Command string // substring match against log lines
	User    string // filter by cron user if supported
}

// Result holds fetched log entries and metadata.
type Result struct {
	Lines    []string
	Source   string // human-readable description of where logs came from
	Partial  bool   // true if output was truncated
	NotFound bool   // true if no log source was available
	Reason   string // explanation when NotFound is true
}

// Provider fetches cron-related log entries from the system.
type Provider interface {
	Fetch(ctx context.Context, q Query) (Result, error)
	Name() string
}
