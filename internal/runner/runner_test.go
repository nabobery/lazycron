package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

func makeJob(command string) domain.CronJob {
	return domain.CronJob{
		ID:      "test:job:0",
		Command: command,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "* * * * *",
		},
	}
}

func TestRun_Success(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo hello")
	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", rec.ExitCode)
	}
	if rec.Status != domain.RunStatusSuccess {
		t.Fatalf("expected success status, got %s", rec.Status)
	}
	if !strings.Contains(rec.Stdout, "hello") {
		t.Fatalf("expected stdout to contain 'hello', got %q", rec.Stdout)
	}
	if rec.Duration <= 0 {
		t.Fatal("duration should be positive")
	}
	if rec.Mode != domain.EnvModeShellInherit {
		t.Fatalf("expected shell_inherit mode, got %s", rec.Mode)
	}
}

func TestRun_Failure(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("exit 42")
	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ExitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", rec.ExitCode)
	}
	if rec.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed status, got %s", rec.Status)
	}
}

func TestRun_Stderr(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo error >&2")
	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Stderr, "error") {
		t.Fatalf("expected stderr to contain 'error', got %q", rec.Stderr)
	}
}

func TestRun_Cancel(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("sleep 60")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var rec domain.RunRecord
	var runErr error

	go func() {
		rec, runErr = r.Run(ctx, job, domain.EnvModeShellInherit)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not complete after cancel")
	}

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if rec.Status != domain.RunStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", rec.Status)
	}
}

func TestRun_LargeOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxOutputBytes = 100

	r := New(cfg)
	// Generate output larger than MaxOutputBytes
	job := makeJob("yes hello | head -100")
	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.Stdout) > cfg.MaxOutputBytes+100 {
		t.Fatalf("stdout should be bounded, got %d bytes", len(rec.Stdout))
	}
	if !rec.Truncated {
		t.Fatal("expected truncated flag to be set")
	}
}

func TestRun_CronLikeEnv(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("env | sort")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "LAZYCRON_TEST_VAR", Value: "test_value"},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Stdout, "LAZYCRON_TEST_VAR=test_value") {
		t.Fatalf("expected LAZYCRON_TEST_VAR in env output, got:\n%s", rec.Stdout)
	}
	if rec.Mode != domain.EnvModeCronLike {
		t.Fatalf("expected cron_like mode, got %s", rec.Mode)
	}
}

func TestRun_CronLikeEnvOverridesDefaults(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo $PATH")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "PATH", Value: "/custom/bin:/usr/bin"},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Stdout, "/custom/bin:/usr/bin") {
		t.Fatalf("expected PATH override in output, got:\n%s", rec.Stdout)
	}
}

func TestRun_CronLikeEnvNoDuplicateKeys(t *testing.T) {
	env := buildCronLikeEnv([]domain.EnvAssignment{
		{Key: "PATH", Value: "/custom/bin"},
	})
	pathCount := 0
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathCount++
		}
	}
	if pathCount != 1 {
		t.Fatalf("expected exactly 1 PATH entry, got %d in env: %v", pathCount, env)
	}
	// Verify the override won
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			if e != "PATH=/custom/bin" {
				t.Fatalf("expected PATH=/custom/bin, got %s", e)
			}
		}
	}
}

func TestRun_CronLikeHonorsSHELL(t *testing.T) {
	r := New(DefaultConfig())
	// Create a temp script that acts as a custom shell
	dir := t.TempDir()
	shellPath := dir + "/myshell"
	if err := os.WriteFile(shellPath, []byte("#!/bin/sh\necho CUSTOM_SHELL_USED\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	job := makeJob("unused-arg")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "SHELL", Value: shellPath},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Stdout, "CUSTOM_SHELL_USED") {
		t.Fatalf("expected SHELL override to be honored, got stdout:\n%s\nstderr:\n%s", rec.Stdout, rec.Stderr)
	}
}

func TestRun_CronLikeSetsCwdToHOME(t *testing.T) {
	r := New(DefaultConfig())
	dir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	job := makeJob("pwd")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "HOME", Value: realDir},
	}
	rec, runErr := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	got := strings.TrimSpace(rec.Stdout)
	if got != realDir {
		t.Fatalf("expected cwd %q, got %q", realDir, got)
	}
}

func TestRun_CronLikePercentSemantics(t *testing.T) {
	r := New(DefaultConfig())

	tests := []struct {
		name    string
		command string
		stdout  string
	}{
		{
			name:    "first percent splits stdin",
			command: "cat%hello",
			stdout:  "hello\n",
		},
		{
			name:    "multiple percents become newlines in stdin",
			command: "cat%hello%world",
			stdout:  "hello\nworld\n",
		},
		{
			name:    "escaped percent is literal",
			command: `echo 100\%`,
			stdout:  "100%\n",
		},
		{
			name:    "no percent passes normally",
			command: "echo no-percent",
			stdout:  "no-percent\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := makeJob(tt.command)
			rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Stdout != tt.stdout {
				t.Fatalf("expected stdout %q, got %q\nstderr: %s", tt.stdout, rec.Stdout, rec.Stderr)
			}
		})
	}
}

func TestRun_CronLikeLOGNAMENotOverridable(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo $LOGNAME")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "LOGNAME", Value: "hacker"},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(rec.Stdout)
	if got == "hacker" {
		t.Fatal("LOGNAME should not be overridable via env context")
	}
	// Should be the current user
	expected := currentUser()
	if got != expected {
		t.Fatalf("expected LOGNAME %q, got %q", expected, got)
	}
}

func TestRun_CronLikeTZPropagation(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo $TZ")
	job.Schedule.Timezone = "America/New_York"
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(rec.Stdout)
	if got != "America/New_York" {
		t.Fatalf("expected TZ=America/New_York, got %q", got)
	}
}

func TestRun_ShellInheritIgnoresPercentSemantics(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo hello%world")
	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// In shell_inherit mode, % should be treated literally by the shell
	if !strings.Contains(rec.Stdout, "hello%world") {
		t.Fatalf("expected literal %% in shell_inherit mode, got %q", rec.Stdout)
	}
}

func TestRun_CronLikeShellLastAssignmentWins(t *testing.T) {
	r := New(DefaultConfig())
	dir := t.TempDir()
	shellPath := dir + "/lastshell"
	if err := os.WriteFile(shellPath, []byte("#!/bin/sh\necho LAST_SHELL_WINS\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	job := makeJob("unused-arg")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "SHELL", Value: "/bin/sh"},
		{Key: "SHELL", Value: shellPath},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Stdout, "LAST_SHELL_WINS") {
		t.Fatalf("expected last SHELL assignment to win, got stdout:\n%s\nstderr:\n%s", rec.Stdout, rec.Stderr)
	}
}

func TestBuildCronLikeEnv_ShellLastAssignmentInEnvList(t *testing.T) {
	env := buildCronLikeEnv([]domain.EnvAssignment{
		{Key: "SHELL", Value: "/bin/bash"},
		{Key: "SHELL", Value: "/bin/zsh"},
	})
	got := envListValue(env, "SHELL")
	if got != "/bin/zsh" {
		t.Fatalf("expected SHELL=/bin/zsh in env list, got SHELL=%s\nfull env: %v", got, env)
	}
}

func TestRun_CronLikeUSERNotOverridable(t *testing.T) {
	r := New(DefaultConfig())
	job := makeJob("echo $USER")
	job.EnvContext = []domain.EnvAssignment{
		{Key: "USER", Value: "hacker"},
	}
	rec, err := r.Run(context.Background(), job, domain.EnvModeCronLike)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(rec.Stdout)
	if got == "hacker" {
		t.Fatal("USER should not be overridable via env context")
	}
	expected := currentUser()
	if got != expected {
		t.Fatalf("expected USER %q, got %q", expected, got)
	}
}

func TestBuildCronLikeEnv_USERAlwaysPresent(t *testing.T) {
	env := buildCronLikeEnv(nil)
	got := envListValue(env, "USER")
	expected := currentUser()
	if got != expected {
		t.Fatalf("expected USER=%s in env, got USER=%s\nfull env: %v", expected, got, env)
	}
}

func TestBuildCronLikeEnv_USERNotEmpty(t *testing.T) {
	env := buildCronLikeEnv([]domain.EnvAssignment{
		{Key: "USER", Value: "attacker"},
	})
	got := envListValue(env, "USER")
	if got == "" {
		t.Fatalf("USER should never be empty, full env: %v", env)
	}
	if got == "attacker" {
		t.Fatalf("USER should be pinned, not overridable, full env: %v", env)
	}
}

func TestRun_CancelKillsChildProcesses(t *testing.T) {
	r := New(DefaultConfig())
	// Spawn a child that itself spawns a sleep — the whole group should be killed
	job := makeJob("sleep 60 & wait")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var rec domain.RunRecord
	var runErr error

	go func() {
		rec, runErr = r.Run(ctx, job, domain.EnvModeShellInherit)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not complete after cancel — child processes may be orphaned")
	}

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if rec.Status != domain.RunStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", rec.Status)
	}
}
