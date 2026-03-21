package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nabobery/lazycron/internal/domain"
	"github.com/nabobery/lazycron/internal/platform/cronlogs"
	"github.com/nabobery/lazycron/internal/platform/crontab"
	"github.com/nabobery/lazycron/internal/platform/systemcron"
	"github.com/nabobery/lazycron/internal/runner"
	"github.com/nabobery/lazycron/internal/schedule"
)

func testDeps(content string, hasCrontab bool) Deps {
	return Deps{
		Client: crontab.NewFakeClient(content, hasCrontab),
		Source: domain.CronSource{
			Kind: domain.SourceKindUserCrontab,
			Path: "crontab://current-user",
		},
		Runner:      runner.New(runner.DefaultConfig()),
		ScheduleSvc: schedule.NewService(),
	}
}

func failingDeps(msg string) Deps {
	return Deps{
		Client: crontab.NewFailingClient(msg),
		Source: domain.CronSource{
			Kind: domain.SourceKindUserCrontab,
			Path: "crontab://current-user",
		},
		Runner:      runner.New(runner.DefaultConfig()),
		ScheduleSvc: schedule.NewService(),
	}
}

func TestRun_NoArgs_ReturnsTUISignal(t *testing.T) {
	deps := testDeps("", false)
	code := Run([]string{"lazycron"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	if code != -1 {
		t.Fatalf("expected -1 (TUI signal), got %d", code)
	}
}

func TestRun_UnknownCommand_ReturnsTUISignal(t *testing.T) {
	deps := testDeps("", false)
	code := Run([]string{"lazycron", "unknown-cmd"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	if code != -1 {
		t.Fatalf("expected -1 (TUI signal), got %d", code)
	}
}

func TestRun_Help(t *testing.T) {
	deps := testDeps("", false)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "help"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatal("help output should contain Usage:")
	}
}

func TestList_WithJobs(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "backup-db") {
		t.Fatalf("output should contain backup-db, got: %s", out)
	}
	if !strings.Contains(out, "cleanup") {
		t.Fatalf("output should contain cleanup, got: %s", out)
	}
}

func TestList_EmptyCrontab(t *testing.T) {
	deps := testDeps("", false)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "No cron jobs found") {
		t.Fatalf("expected 'No cron jobs found', got: %s", stdout.String())
	}
}

func TestList_JSON(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--json"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "backup-db") {
		t.Fatalf("JSON output should contain backup-db, got: %s", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatalf("JSON output should start with [, got: %s", out)
	}
}

func TestList_ReadError(t *testing.T) {
	deps := failingDeps("permission denied")
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "list"}, &bytes.Buffer{}, &stderr, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "permission denied") {
		t.Fatalf("stderr should contain error, got: %s", stderr.String())
	}
}

func TestValidate_NoIssues(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "validate"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "No issues found") {
		t.Fatalf("expected 'No issues found', got: %s", stdout.String())
	}
}

func TestValidate_WithIssues(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\nnot a valid line\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "validate"}, &stdout, &bytes.Buffer{}, deps)
	if code != 1 {
		t.Fatalf("expected exit 1 for issues, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "line 2") {
		t.Fatalf("should report line number, got: %s", out)
	}
}

func TestRunCmd_Success(t *testing.T) {
	content := "0 3 * * * /bin/echo hello\n"
	deps := testDeps(content, true)
	var stdout, stderr bytes.Buffer

	svc := deps.Client.(*crontab.FakeClient)
	_ = svc // ensure we have the right type

	code := Run([]string{"lazycron", "run", "user_crontab:testuser:line-0"}, &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("stdout should contain 'hello', got: %s", stdout.String())
	}
}

func TestRunCmd_NotFound(t *testing.T) {
	content := "0 3 * * * /bin/echo hello\n"
	deps := testDeps(content, true)
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "run", "nonexistent-id"}, &bytes.Buffer{}, &stderr, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr should say not found, got: %s", stderr.String())
	}
}

func TestRunCmd_NoArgs(t *testing.T) {
	deps := testDeps("0 3 * * * /bin/echo hello\n", true)
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "run"}, &bytes.Buffer{}, &stderr, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr should show usage, got: %s", stderr.String())
	}
}

func TestDoctor_OK(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "doctor"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "lazycron doctor") {
		t.Fatalf("should contain header, got: %s", out)
	}
	if !strings.Contains(out, "crontab read:  OK") {
		t.Fatalf("should report OK, got: %s", out)
	}
	if !strings.Contains(out, "jobs found:    1") {
		t.Fatalf("should report 1 job, got: %s", out)
	}
}

func TestDoctor_EmptyCrontab(t *testing.T) {
	deps := testDeps("", false)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "doctor"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no crontab for user") {
		t.Fatalf("should report empty crontab, got: %s", stdout.String())
	}
}

func TestDoctor_ReadError(t *testing.T) {
	deps := failingDeps("crontab: not found")
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "doctor"}, &stdout, &bytes.Buffer{}, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "ERROR") {
		t.Fatalf("should report ERROR, got: %s", stdout.String())
	}
}

func TestRunCmd_FailingCommand(t *testing.T) {
	content := "0 3 * * * /bin/sh -c 'exit 2'\n"
	deps := testDeps(content, true)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lazycron", "run", "user_crontab:testuser:line-0"}, &stdout, &stderr, deps)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestList_JSON_NoSecretValues(t *testing.T) {
	content := "API_TOKEN=supersecret123\n0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--json"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if strings.Contains(out, "supersecret123") {
		t.Fatalf("JSON output should not contain secret value, got: %s", out)
	}
	if !strings.Contains(out, "API_TOKEN") {
		t.Fatalf("JSON output should contain env key name, got: %s", out)
	}
}

func TestList_AllFlag_NoDiscoverer(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--all"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "backup-db") {
		t.Fatalf("output should contain backup-db, got: %s", stdout.String())
	}
}

func TestList_JSON_IncludesMutabilityFields(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--json"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, `"mutable"`) {
		t.Fatalf("JSON output should contain 'mutable' field, got: %s", out)
	}
	if !strings.Contains(out, `"read_only"`) {
		t.Fatalf("JSON output should contain 'read_only' field, got: %s", out)
	}
}

func TestValidate_AllFlag_NoDiscoverer(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "validate", "--all"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "No issues found") {
		t.Fatalf("expected 'No issues found', got: %s", stdout.String())
	}
}

func TestDoctor_ShowsSystemSources(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "doctor"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "System cron sources:") {
		t.Fatalf("doctor should include system cron sources section, got: %s", out)
	}
}

func TestValidate_SourceLevelIssue_NoLineNumber(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\nnot a valid line\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "validate"}, &stdout, &bytes.Buffer{}, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	out := stdout.String()
	// Line-level issues should have "line N:" prefix
	if !strings.Contains(out, "line ") {
		t.Fatalf("line-level issues should have 'line N:' prefix, got: %s", out)
	}
}

func TestSubcommandHelp_ExitZero(t *testing.T) {
	deps := testDeps("0 3 * * * /bin/echo\n", true)
	cmds := []string{"list", "validate", "run", "doctor"}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{"lazycron", cmd, "-h"}, &stdout, &stderr, deps)
			if code != 0 {
				t.Fatalf("%s -h: expected exit 0, got %d; stderr: %s", cmd, code, stderr.String())
			}
		})
	}
}

// --- helpers for system cron discovery in CLI tests ---

type cliFakeFileInfo struct {
	name string
	mode fs.FileMode
}

func (f cliFakeFileInfo) Name() string       { return f.name }
func (f cliFakeFileInfo) Size() int64        { return 0 }
func (f cliFakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f cliFakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f cliFakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f cliFakeFileInfo) Sys() any           { return nil }

type cliFakeFile struct {
	content string
	info    cliFakeFileInfo
}

type cliFakeDirEntry struct {
	name  string
	isDir bool
	mode  fs.FileMode
}

func (e cliFakeDirEntry) Name() string      { return e.name }
func (e cliFakeDirEntry) IsDir() bool       { return e.isDir }
func (e cliFakeDirEntry) Type() fs.FileMode { return e.mode }
func (e cliFakeDirEntry) Info() (fs.FileInfo, error) {
	return cliFakeFileInfo{name: e.name, mode: e.mode}, nil
}

type cliFakeFS struct {
	files map[string]cliFakeFile
	dirs  map[string][]cliFakeDirEntry
}

func (f *cliFakeFS) ReadFile(name string) ([]byte, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(file.content), nil
}

func (f *cliFakeFS) Stat(name string) (fs.FileInfo, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return file.info, nil
}

func (f *cliFakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, ok := f.dirs[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	result := make([]fs.DirEntry, len(entries))
	for i := range entries {
		result[i] = entries[i]
	}
	return result, nil
}

func testDepsWithDiscoverer(content string, hasCrontab bool, fakeFS systemcron.FS) Deps {
	return Deps{
		Client: crontab.NewFakeClient(content, hasCrontab),
		Source: domain.CronSource{
			Kind: domain.SourceKindUserCrontab,
			Path: "crontab://current-user",
		},
		Runner:      runner.New(runner.DefaultConfig()),
		ScheduleSvc: schedule.NewService(),
		Discoverer:  systemcron.NewWithFS(fakeFS),
	}
}

func systemFS() *cliFakeFS {
	return &cliFakeFS{
		files: map[string]cliFakeFile{
			"/etc/crontab": {
				content: "0 5 * * * root /usr/sbin/logrotate\n",
				info:    cliFakeFileInfo{name: "crontab", mode: 0644},
			},
			"/etc/cron.d/sysstat": {
				content: "*/10 * * * * root /usr/lib/sysstat/sa1 1 1\n",
				info:    cliFakeFileInfo{name: "sysstat", mode: 0644},
			},
			"/etc/cron.daily":           {info: cliFakeFileInfo{name: "daily", mode: fs.ModeDir | 0755}},
			"/etc/cron.daily/logrotate": {content: "#!/bin/sh\nlogrotate /etc/logrotate.conf\n", info: cliFakeFileInfo{name: "logrotate", mode: 0755}},
		},
		dirs: map[string][]cliFakeDirEntry{
			"/etc/cron.d": {
				{name: "sysstat", mode: 0644},
			},
			"/etc/cron.daily": {
				{name: "logrotate", mode: 0755},
			},
		},
	}
}

func TestList_All_IncludesSystemJobs(t *testing.T) {
	userContent := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDepsWithDiscoverer(userContent, true, systemFS())
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--all"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "backup-db") {
		t.Fatalf("output should contain user job, got: %s", out)
	}
	if !strings.Contains(out, "logrotate") {
		t.Fatalf("output should contain system job from /etc/crontab, got: %s", out)
	}
	if !strings.Contains(out, "sysstat") {
		t.Fatalf("output should contain system job from cron.d, got: %s", out)
	}
}

func TestList_JSON_All_IncludesSystemFields(t *testing.T) {
	userContent := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDepsWithDiscoverer(userContent, true, systemFS())
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--json", "--all"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var jobs []json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &jobs); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(jobs) < 2 {
		t.Fatalf("expected at least 2 jobs (user + system), got %d", len(jobs))
	}

	hasSysSource := false
	for _, raw := range jobs {
		var j map[string]any
		if err := json.Unmarshal(raw, &j); err != nil {
			t.Fatalf("failed to parse job JSON: %v", err)
		}
		if _, ok := j["read_only"]; !ok {
			t.Error("expected read_only field in JSON")
		}
		if _, ok := j["mutable"]; !ok {
			t.Error("expected mutable field in JSON")
		}
		if src, ok := j["source"]; ok && src != nil {
			hasSysSource = true
			srcMap := src.(map[string]any)
			if _, ok := srcMap["kind"]; !ok {
				t.Error("expected source.kind in JSON")
			}
			if _, ok := srcMap["path"]; !ok {
				t.Error("expected source.path in JSON")
			}
		}
	}
	if !hasSysSource {
		t.Error("expected at least one job with a source field (system job)")
	}
}

func TestList_JSON_UserJobHasSource(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "list", "--json"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var jobs []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &jobs); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job")
	}
	src, ok := jobs[0]["source"]
	if !ok || src == nil {
		t.Fatal("expected user job to have a source field in JSON")
	}
	srcMap := src.(map[string]any)
	if srcMap["kind"] != "user_crontab" {
		t.Errorf("expected source.kind=user_crontab, got %v", srcMap["kind"])
	}
	if srcMap["path"] != "crontab://current-user" {
		t.Errorf("expected source.path=crontab://current-user, got %v", srcMap["path"])
	}
}

// --- Logs command tests ---

type fakeLogsProvider struct {
	result    cronlogs.Result
	err       error
	lastQuery cronlogs.Query
}

func (f *fakeLogsProvider) Name() string { return "fake" }
func (f *fakeLogsProvider) Fetch(_ context.Context, q cronlogs.Query) (cronlogs.Result, error) {
	f.lastQuery = q
	return f.result, f.err
}

func TestLogs_ShowsEntries(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	deps.LogsProvider = &fakeLogsProvider{
		result: cronlogs.Result{
			Lines:  []string{"Mar 21 10:00:01 host CRON[1234]: (root) CMD (/usr/local/bin/backup-db)"},
			Source: "fake-source",
		},
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lazycron", "logs"}, &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "backup-db") {
		t.Fatalf("expected log output to contain backup-db, got: %s", stdout.String())
	}
}

func TestLogs_NotAvailable(t *testing.T) {
	deps := testDeps("", false)
	deps.LogsProvider = &fakeLogsProvider{
		result: cronlogs.Result{NotFound: true, Reason: "test not available"},
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lazycron", "logs"}, &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "test not available") {
		t.Fatalf("expected not-available reason in stderr, got: %s", stderr.String())
	}
}

func TestLogs_WithJobID(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	deps.LogsProvider = &fakeLogsProvider{
		result: cronlogs.Result{
			Lines:  []string{"matched log line"},
			Source: "fake-source",
		},
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lazycron", "logs", "user_crontab:testuser:line-0"}, &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "matched log line") {
		t.Fatalf("expected log output, got: %s", stdout.String())
	}
}

func TestLogs_JobNotFound(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	deps.LogsProvider = &fakeLogsProvider{}
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "logs", "nonexistent-id"}, &bytes.Buffer{}, &stderr, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("expected 'not found' in stderr, got: %s", stderr.String())
	}
}

func TestLogs_SinceRelative(t *testing.T) {
	tests := []struct {
		name     string
		since    string
		wantErr  bool
		checkAge time.Duration
	}{
		{name: "1 hour ago", since: "1 hour ago", checkAge: time.Hour},
		{name: "2 hours ago", since: "2 hours ago", checkAge: 2 * time.Hour},
		{name: "30 minutes ago", since: "30 minutes ago", checkAge: 30 * time.Minute},
		{name: "1 minute ago", since: "1 minute ago", checkAge: time.Minute},
		{name: "7 days ago", since: "7 days ago", checkAge: 7 * 24 * time.Hour},
		{name: "1 day ago", since: "1 day ago", checkAge: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := testDeps("", false)
			provider := &fakeLogsProvider{
				result: cronlogs.Result{Source: "fake"},
			}
			deps.LogsProvider = provider
			var stdout, stderr bytes.Buffer
			code := Run([]string{"lazycron", "logs", "--since", tt.since}, &stdout, &stderr, deps)
			if tt.wantErr {
				if code == 0 {
					t.Fatal("expected error exit code")
				}
				return
			}
			if code != 0 {
				t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
			}
			if provider.lastQuery.Since.IsZero() {
				t.Fatal("expected Since to be set")
			}
			elapsed := time.Since(provider.lastQuery.Since)
			tolerance := 5 * time.Second
			if elapsed < tt.checkAge-tolerance || elapsed > tt.checkAge+tolerance {
				t.Fatalf("expected Since ~%v ago, got %v ago", tt.checkAge, elapsed)
			}
		})
	}
}

func TestLogs_SinceAbsolute(t *testing.T) {
	deps := testDeps("", false)
	provider := &fakeLogsProvider{
		result: cronlogs.Result{Source: "fake"},
	}
	deps.LogsProvider = provider
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lazycron", "logs", "--since", "2024-06-15"}, &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	expected, _ := time.Parse("2006-01-02", "2024-06-15")
	if !provider.lastQuery.Since.Equal(expected) {
		t.Fatalf("expected Since=%v, got %v", expected, provider.lastQuery.Since)
	}
}

// --- Export command tests ---

func TestExport_WritesToStdout(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "export"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if stdout.String() != content {
		t.Fatalf("expected exact crontab content, got: %q", stdout.String())
	}
}

func TestExport_WritesToFile(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps(content, true)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "backup.crontab")
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "export", "--out", outPath}, &bytes.Buffer{}, &stderr, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("expected exact crontab content in file, got: %q", string(data))
	}
}

// --- Import command tests ---

func TestImport_DryRun(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps("", false)
	dir := t.TempDir()
	importPath := filepath.Join(dir, "import.crontab")
	if err := os.WriteFile(importPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "import", "--from", importPath}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Parsed 1 jobs") {
		t.Fatalf("expected dry-run output, got: %s", out)
	}
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("expected 'Dry run' message, got: %s", out)
	}
	// Should NOT have applied
	fc := deps.Client.(*crontab.FakeClient)
	if len(fc.ApplyCalls) != 0 {
		t.Fatalf("dry run should not call Apply, got %d calls", len(fc.ApplyCalls))
	}
}

func TestImport_Apply(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	deps := testDeps("", false)
	dir := t.TempDir()
	importPath := filepath.Join(dir, "import.crontab")
	if err := os.WriteFile(importPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "import", "--from", importPath, "--yes"}, &stdout, &bytes.Buffer{}, deps)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Imported successfully") {
		t.Fatalf("expected success message, got: %s", stdout.String())
	}
	fc := deps.Client.(*crontab.FakeClient)
	if len(fc.ApplyCalls) != 1 {
		t.Fatalf("expected 1 Apply call, got %d", len(fc.ApplyCalls))
	}
	if fc.ApplyCalls[0] != content {
		t.Fatalf("expected applied content to match, got: %q", fc.ApplyCalls[0])
	}
}

func TestImport_MissingFromFlag(t *testing.T) {
	deps := testDeps("", false)
	var stderr bytes.Buffer
	code := Run([]string{"lazycron", "import"}, &bytes.Buffer{}, &stderr, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--from is required") {
		t.Fatalf("expected --from required error, got: %s", stderr.String())
	}
}

func TestValidate_All_IncludesSystemIssues(t *testing.T) {
	userContent := "0 3 * * * /usr/local/bin/backup-db\n"
	fakeFS := &cliFakeFS{
		files: map[string]cliFakeFile{
			"/etc/crontab": {
				content: "0 5 * * * root /usr/sbin/logrotate\n",
				info:    cliFakeFileInfo{name: "crontab", mode: 0644},
			},
			"/etc/cron.daily":          {info: cliFakeFileInfo{name: "daily", mode: fs.ModeDir | 0755}},
			"/etc/cron.daily/bad.name": {content: "#!/bin/sh\necho bad\n", info: cliFakeFileInfo{name: "bad.name", mode: 0755}},
		},
		dirs: map[string][]cliFakeDirEntry{
			"/etc/cron.daily": {
				{name: "bad.name", mode: 0755},
			},
		},
	}
	deps := testDepsWithDiscoverer(userContent, true, fakeFS)
	var stdout bytes.Buffer
	code := Run([]string{"lazycron", "validate", "--all"}, &stdout, &bytes.Buffer{}, deps)
	if code != 1 {
		t.Fatalf("expected exit 1 (issues found), got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "run-parts") {
		t.Fatalf("expected run-parts warning for bad.name, got: %s", out)
	}
	// Source-level issues should not have "line N:" prefix
	if strings.Contains(out, "line 0:") {
		t.Fatalf("source-level issues should not print 'line 0:', got: %s", out)
	}
}
