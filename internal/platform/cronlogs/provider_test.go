package cronlogs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoopProvider(t *testing.T) {
	p := NewNoopProvider("test reason")
	if p.Name() != "noop" {
		t.Fatalf("expected name 'noop', got %q", p.Name())
	}

	result, err := p.Fetch(context.Background(), Query{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NotFound {
		t.Fatal("expected NotFound=true")
	}
	if result.Reason != "test reason" {
		t.Fatalf("expected reason 'test reason', got %q", result.Reason)
	}
}

func TestJournalctlProvider_WithFakeExec(t *testing.T) {
	p := &JournalctlProvider{
		available: true,
		execFn: func(_ context.Context, args ...string) ([]byte, error) {
			return []byte("Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)\nMar 21 10:05:01 host CRON[1235]: (root) CMD (/usr/local/bin/cleanup)\n"), nil
		},
	}

	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotFound {
		t.Fatal("expected NotFound=false")
	}
	if len(result.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result.Lines))
	}
}

func TestJournalctlProvider_CommandFilter(t *testing.T) {
	p := &JournalctlProvider{
		available: true,
		execFn: func(_ context.Context, args ...string) ([]byte, error) {
			return []byte("Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)\nMar 21 10:05:01 host CRON[1235]: (root) CMD (/usr/local/bin/cleanup)\n"), nil
		},
	}

	result, err := p.Fetch(context.Background(), Query{Command: "backup", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("expected 1 line after command filter, got %d", len(result.Lines))
	}
	if !strings.Contains(result.Lines[0], "backup") {
		t.Fatalf("expected line to contain 'backup', got %q", result.Lines[0])
	}
}

func TestJournalctlProvider_EmptyOutput(t *testing.T) {
	p := &JournalctlProvider{
		available: true,
		execFn: func(_ context.Context, args ...string) ([]byte, error) {
			return []byte("-- No entries --"), nil
		},
	}

	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 0 {
		t.Fatalf("expected 0 lines for empty journal, got %d", len(result.Lines))
	}
}

func TestJournalctlProvider_UnitNotFound(t *testing.T) {
	p := &JournalctlProvider{
		available: true,
		execFn: func(_ context.Context, args ...string) ([]byte, error) {
			for _, a := range args {
				if a == "cron" || a == "crond" {
					return []byte("Unit cron.service could not be found."), fmt.Errorf("exit status 1")
				}
			}
			return nil, fmt.Errorf("unexpected args")
		},
	}

	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NotFound {
		t.Fatal("expected NotFound=true when unit not found")
	}
	if !strings.Contains(result.Reason, "not found") {
		t.Fatalf("expected reason to mention 'not found', got %q", result.Reason)
	}
}

func TestJournalctlProvider_FallbackToCrond(t *testing.T) {
	callCount := 0
	p := &JournalctlProvider{
		available: true,
		execFn: func(_ context.Context, args ...string) ([]byte, error) {
			callCount++
			for i, a := range args {
				if a == "-u" && i+1 < len(args) {
					unit := args[i+1]
					if unit == "cron" {
						return []byte("Unit cron.service could not be found."), fmt.Errorf("exit status 1")
					}
					if unit == "crond" {
						return []byte("Mar 21 10:00:01 host crond[1234]: (root) CMD (/usr/local/bin/backup)\n"), nil
					}
				}
			}
			return nil, fmt.Errorf("unexpected args: %v", args)
		},
	}

	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotFound {
		t.Fatal("expected NotFound=false when crond unit works")
	}
	if len(result.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result.Lines))
	}
	if callCount < 2 {
		t.Fatalf("expected at least 2 exec calls (cron then crond), got %d", callCount)
	}
}

func TestSyslogProvider_ReadsCronLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")
	content := `Mar 21 10:00:01 host kernel: something
Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)
Mar 21 10:05:01 host sshd: connection from 1.2.3.4
Mar 21 10:05:01 host CRON[1235]: (root) CMD (/usr/local/bin/cleanup)
`
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &SyslogProvider{paths: []string{logFile}}
	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotFound {
		t.Fatal("expected NotFound=false")
	}
	if len(result.Lines) != 2 {
		t.Fatalf("expected 2 cron lines, got %d: %v", len(result.Lines), result.Lines)
	}
}

func TestSyslogProvider_CommandFilter(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")
	content := `Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)
Mar 21 10:05:01 host CRON[1235]: (root) CMD (/usr/local/bin/cleanup)
`
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &SyslogProvider{paths: []string{logFile}}
	result, err := p.Fetch(context.Background(), Query{Command: "cleanup", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("expected 1 line after command filter, got %d", len(result.Lines))
	}
}

func TestSyslogProvider_NoReadableFiles(t *testing.T) {
	p := &SyslogProvider{paths: []string{"/nonexistent/path/syslog"}}
	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NotFound {
		t.Fatal("expected NotFound=true when no files readable")
	}
}

func TestSyslogProvider_LimitTruncation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)")
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &SyslogProvider{paths: []string{logFile}}
	result, err := p.Fetch(context.Background(), Query{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 10 {
		t.Fatalf("expected 10 lines (limit), got %d", len(result.Lines))
	}
}

func TestSyslogProvider_ReadableButNoMatches(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")
	content := "Mar 21 10:00:01 host kernel: something\nMar 21 10:05:01 host sshd: connection\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &SyslogProvider{paths: []string{logFile}}
	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotFound {
		t.Fatal("expected NotFound=false for readable file with no matches")
	}
	if len(result.Lines) != 0 {
		t.Fatalf("expected 0 lines, got %d", len(result.Lines))
	}
	if result.Source == "" {
		t.Fatal("expected Source to be set for readable file")
	}
}

func TestSyslogProvider_LimitTruncationSetsPartial(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)")
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &SyslogProvider{paths: []string{logFile}}
	result, err := p.Fetch(context.Background(), Query{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Fatal("expected Partial=true when truncating to limit")
	}
}

func TestSyslogProvider_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "syslog")

	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, "Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup)")
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := &SyslogProvider{paths: []string{logFile}}
	_, err := p.Fetch(ctx, Query{Limit: 50})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
}

func TestAutoProvider_Name(t *testing.T) {
	p := NewAutoProvider()
	if p.Name() != "auto" {
		t.Fatalf("expected name 'auto', got %q", p.Name())
	}
}

func TestAutoProvider_PreservesNotFoundReasons(t *testing.T) {
	p := &AutoProvider{
		providers: []Provider{
			NewNoopProvider("macOS: cron logging not available"),
		},
	}
	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NotFound {
		t.Fatal("expected NotFound=true")
	}
	if !strings.Contains(result.Reason, "macOS") {
		t.Fatalf("expected reason to contain provider-specific reason 'macOS', got %q", result.Reason)
	}
}

func TestAutoProvider_AggregatesMultipleReasons(t *testing.T) {
	p := &AutoProvider{
		providers: []Provider{
			NewNoopProvider("journalctl: unit cron not found"),
			NewNoopProvider("syslog: no readable files"),
		},
	}
	result, err := p.Fetch(context.Background(), Query{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NotFound {
		t.Fatal("expected NotFound=true")
	}
	if !strings.Contains(result.Reason, "journalctl") {
		t.Fatalf("expected reason to contain 'journalctl', got %q", result.Reason)
	}
	if !strings.Contains(result.Reason, "syslog") {
		t.Fatalf("expected reason to contain 'syslog', got %q", result.Reason)
	}
}
