package cronlogs

import "context"

// NoopProvider always reports that logs are not available.
// Used on platforms (e.g. macOS) where cron logging is not easily accessible.
type NoopProvider struct {
	reason string
}

func NewNoopProvider(reason string) *NoopProvider {
	return &NoopProvider{reason: reason}
}

func (p *NoopProvider) Name() string { return "noop" }

func (p *NoopProvider) Fetch(_ context.Context, _ Query) (Result, error) {
	return Result{NotFound: true, Reason: p.reason}, nil
}
