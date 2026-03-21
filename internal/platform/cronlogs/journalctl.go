package cronlogs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// JournalctlProvider fetches cron logs via journalctl on Linux systems.
type JournalctlProvider struct {
	execFn    func(ctx context.Context, args ...string) ([]byte, error)
	available bool
}

func NewJournalctlProvider() *JournalctlProvider {
	_, err := exec.LookPath("journalctl")
	return &JournalctlProvider{
		execFn:    defaultExec,
		available: err == nil,
	}
}

func defaultExec(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	return cmd.CombinedOutput()
}

func (p *JournalctlProvider) Name() string { return "journalctl" }

var cronUnits = []string{"cron", "crond"}

func (p *JournalctlProvider) Fetch(ctx context.Context, q Query) (Result, error) {
	if !p.available {
		return Result{NotFound: true, Reason: "journalctl not found on this system"}, nil
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	var lastReason string
	for _, unit := range cronUnits {
		result, err := p.fetchUnit(ctx, unit, q, limit)
		if err != nil {
			return Result{}, err
		}
		if !result.NotFound {
			return result, nil
		}
		lastReason = result.Reason
	}

	if lastReason == "" {
		lastReason = "no cron unit found in journal"
	}
	return Result{NotFound: true, Reason: lastReason}, nil
}

func (p *JournalctlProvider) fetchUnit(ctx context.Context, unit string, q Query, limit int) (Result, error) {
	args := []string{"journalctl", "--no-pager", "-u", unit}
	args = append(args, fmt.Sprintf("--lines=%d", limit))

	if !q.Since.IsZero() {
		args = append(args, "--since", q.Since.Format("2006-01-02 15:04:05"))
	}
	if !q.Until.IsZero() {
		args = append(args, "--until", q.Until.Format("2006-01-02 15:04:05"))
	}

	source := "journalctl -u " + unit

	out, err := p.execFn(ctx, args...)
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "No journal files") || strings.Contains(outStr, "Failed to") {
			return Result{NotFound: true, Reason: "journalctl: " + outStr}, nil
		}
		if strings.Contains(outStr, "could not be found") || strings.Contains(outStr, "not found") {
			return Result{NotFound: true, Reason: fmt.Sprintf("journalctl: unit %s not found", unit)}, nil
		}
		return Result{}, fmt.Errorf("journalctl: %w (%s)", err, outStr)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" || raw == "-- No entries --" {
		return Result{Source: source}, nil
	}

	lines := strings.Split(raw, "\n")

	if q.Command != "" {
		filtered := lines[:0]
		lower := strings.ToLower(q.Command)
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l), lower) {
				filtered = append(filtered, l)
			}
		}
		lines = filtered
	}

	partial := len(lines) >= limit
	return Result{Lines: lines, Source: source, Partial: partial}, nil
}
