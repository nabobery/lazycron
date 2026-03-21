package cronlogs

import (
	"context"
	"runtime"
	"strings"
)

// AutoProvider tries providers in order and returns the first successful result.
type AutoProvider struct {
	providers []Provider
}

// NewAutoProvider creates a provider chain appropriate for the current platform.
func NewAutoProvider() *AutoProvider {
	var providers []Provider

	switch runtime.GOOS {
	case "linux":
		providers = append(providers, NewJournalctlProvider(), NewSyslogProvider())
	case "darwin":
		providers = append(providers, NewNoopProvider("macOS: cron logging requires 'log show --process cron' which may be slow; use Console.app for detailed logs"))
	default:
		providers = append(providers, NewNoopProvider("cron log retrieval not supported on "+runtime.GOOS))
	}

	return &AutoProvider{providers: providers}
}

func (p *AutoProvider) Name() string { return "auto" }

func (p *AutoProvider) Fetch(ctx context.Context, q Query) (Result, error) {
	var reasons []string
	for _, prov := range p.providers {
		result, err := prov.Fetch(ctx, q)
		if err != nil {
			continue
		}
		if !result.NotFound {
			result.Source = prov.Name() + ": " + result.Source
			return result, nil
		}
		if result.Reason != "" {
			reasons = append(reasons, result.Reason)
		}
	}

	reason := "no cron log source available on this system"
	if len(reasons) > 0 {
		reason = strings.Join(reasons, "; ")
	}

	return Result{
		NotFound: true,
		Reason:   reason,
	}, nil
}
