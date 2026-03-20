package runner

import (
	"context"
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
