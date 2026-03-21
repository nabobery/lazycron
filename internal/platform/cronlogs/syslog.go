package cronlogs

import (
	"bufio"
	"context"
	"os"
	"strings"
)

var syslogPaths = []string{
	"/var/log/syslog",
	"/var/log/cron",
	"/var/log/messages",
}

// SyslogProvider reads cron log entries from syslog files as a fallback.
type SyslogProvider struct {
	paths []string
}

func NewSyslogProvider() *SyslogProvider {
	return &SyslogProvider{paths: syslogPaths}
}

func (p *SyslogProvider) Name() string { return "syslog" }

func (p *SyslogProvider) Fetch(ctx context.Context, q Query) (Result, error) {
	for _, path := range p.paths {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		lines, total, err := readCronLines(ctx, path, q)
		if err != nil {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			continue
		}
		if len(lines) > 0 {
			partial := total > len(lines)
			return Result{Lines: lines, Source: path, Partial: partial}, nil
		}
		return Result{Lines: nil, Source: path}, nil
	}
	return Result{NotFound: true, Reason: "no readable syslog files found"}, nil
}

func readCronLines(ctx context.Context, path string, q Query) (lines []string, totalMatched int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	var matched []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		line := scanner.Text()
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "cron") {
			continue
		}
		if q.Command != "" && !strings.Contains(lower, strings.ToLower(q.Command)) {
			continue
		}
		matched = append(matched, line)
	}

	total := len(matched)
	if len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, total, scanner.Err()
}
