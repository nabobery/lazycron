package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
	"github.com/avinashchangrani/lazycron/internal/runner"
	"github.com/avinashchangrani/lazycron/internal/schedule"
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
