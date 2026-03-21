package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

	command, stdinData := job.Command, ""
	if mode == domain.EnvModeCronLike {
		command, stdinData = applyPercentSemantics(job.Command)
	}

	var env []string
	shell := "/bin/sh"
	if mode == domain.EnvModeCronLike {
		env = buildCronLikeEnv(job.EnvContext)
		if job.Schedule.Timezone != "" {
			env = appendOrReplace(env, "TZ", job.Schedule.Timezone)
		}
		if s := envListValue(env, "SHELL"); s != "" {
			shell = s
		}
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}

	var stdout, stderr boundedBuffer
	stdout.max = r.cfg.MaxOutputBytes
	stderr.max = r.cfg.MaxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if mode == domain.EnvModeCronLike {
		cmd.Env = env
		if home := envListValue(env, "HOME"); home != "" {
			cmd.Dir = home
		}
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

// pinnedKeys are env vars that cron does not allow users to override.
var pinnedKeys = map[string]bool{
	"LOGNAME": true,
	"USER":    true,
}

func buildCronLikeEnv(envContext []domain.EnvAssignment) []string {
	user := currentUser()
	defaults := map[string]string{
		"HOME":    homeDir(),
		"LOGNAME": user,
		"USER":    user,
		"PATH":    "/usr/bin:/bin",
		"SHELL":   "/bin/sh",
	}

	for _, ea := range envContext {
		if pinnedKeys[ea.Key] {
			continue
		}
		defaults[ea.Key] = ea.Value
	}

	seen := make(map[string]bool, len(defaults))
	env := make([]string, 0, len(defaults))

	for _, key := range []string{"HOME", "LOGNAME", "USER", "PATH", "SHELL"} {
		if val, ok := defaults[key]; ok {
			env = append(env, key+"="+val)
			seen[key] = true
		}
	}

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

// applyPercentSemantics implements cron's % handling: the first unescaped %
// splits the command from stdin data; subsequent unescaped %s become newlines
// in the stdin portion. Escaped \% becomes literal %.
func applyPercentSemantics(raw string) (command, stdin string) {
	var cmd, rest strings.Builder
	inStdin := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch == '\\' && i+1 < len(raw) && raw[i+1] == '%' {
			if inStdin {
				rest.WriteByte('%')
			} else {
				cmd.WriteByte('%')
			}
			i++
			continue
		}
		if ch == '%' {
			if !inStdin {
				inStdin = true
			} else {
				rest.WriteByte('\n')
			}
			continue
		}
		if inStdin {
			rest.WriteByte(ch)
		} else {
			cmd.WriteByte(ch)
		}
	}
	if rest.Len() > 0 {
		return cmd.String(), rest.String() + "\n"
	}
	return cmd.String(), ""
}

func envListValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

func appendOrReplace(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}
