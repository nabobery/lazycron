package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

const defaultMaxOutputBytes = 1024 * 1024 // 1MB

type Config struct {
	MaxOutputBytes int
}

func DefaultConfig() Config {
	return Config{MaxOutputBytes: defaultMaxOutputBytes}
}

type Runner struct {
	cfg Config
}

func New(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Run(ctx context.Context, job domain.CronJob, mode domain.EnvMode) (domain.RunRecord, error) {
	rec := domain.RunRecord{
		JobID:     job.ID,
		StartedAt: time.Now(),
		Mode:      mode,
		Status:    domain.RunStatusRunning,
	}

	cmd := exec.Command("/bin/sh", "-c", job.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr boundedBuffer
	stdout.max = r.cfg.MaxOutputBytes
	stderr.max = r.cfg.MaxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if mode == domain.EnvModeCronLike {
		cmd.Env = buildCronLikeEnv(job.EnvContext)
	}

	if err := cmd.Start(); err != nil {
		return rec, fmt.Errorf("start command: %w", err)
	}

	// Wait for completion or cancellation
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	var err error
	select {
	case <-ctx.Done():
		// Kill the entire process group
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-waitDone
		rec.FinishedAt = time.Now()
		rec.Duration = rec.FinishedAt.Sub(rec.StartedAt)
		rec.Stdout = stdout.String()
		rec.Stderr = stderr.String()
		rec.Truncated = stdout.truncated || stderr.truncated
		rec.Status = domain.RunStatusCancelled
		rec.ExitCode = -1
		return rec, nil
	case err = <-waitDone:
	}

	rec.FinishedAt = time.Now()
	rec.Duration = rec.FinishedAt.Sub(rec.StartedAt)
	rec.Stdout = stdout.String()
	rec.Stderr = stderr.String()
	rec.Truncated = stdout.truncated || stderr.truncated

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			rec.ExitCode = exitErr.ExitCode()
			rec.Status = domain.RunStatusFailed
			return rec, nil
		}

		return rec, fmt.Errorf("run command: %w", err)
	}

	rec.ExitCode = 0
	rec.Status = domain.RunStatusSuccess
	return rec, nil
}

func buildCronLikeEnv(envContext []domain.EnvAssignment) []string {
	defaults := map[string]string{
		"HOME":    homeDir(),
		"LOGNAME": currentUser(),
		"PATH":    "/usr/bin:/bin",
		"SHELL":   "/bin/sh",
	}

	// envContext overrides defaults
	for _, ea := range envContext {
		defaults[ea.Key] = ea.Value
	}

	// Deterministic ordering: defaults first (sorted), then any extra keys from envContext
	seen := make(map[string]bool, len(defaults))
	env := make([]string, 0, len(defaults))

	// Emit in a stable order: known defaults first
	for _, key := range []string{"HOME", "LOGNAME", "PATH", "SHELL"} {
		if val, ok := defaults[key]; ok {
			env = append(env, key+"="+val)
			seen[key] = true
		}
	}

	// Then any extra keys from envContext in their original order
	for _, ea := range envContext {
		if !seen[ea.Key] {
			env = append(env, ea.Key+"="+defaults[ea.Key])
			seen[ea.Key] = true
		}
	}

	return env
}

func homeDir() string {
	if h := getenv("HOME"); h != "" {
		return h
	}
	return "/tmp"
}

func currentUser() string {
	if u := getenv("USER"); u != "" {
		return u
	}
	if u := getenv("LOGNAME"); u != "" {
		return u
	}
	return "unknown"
}

func getenv(key string) string {
	return os.Getenv(key)
}

type boundedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.max {
		remaining := b.max - b.buf.Len()
		if remaining > 0 {
			b.buf.Write(p[:remaining])
		}
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *boundedBuffer) String() string {
	return b.buf.String()
}
