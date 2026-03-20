package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
	"github.com/avinashchangrani/lazycron/internal/runner"
	"github.com/avinashchangrani/lazycron/internal/schedule"
)

func parseFlags(fs *flag.FlagSet, args []string) (ok bool, helpRequested bool) {
	if err := fs.Parse(args); err != nil {
		return false, errors.Is(err, flag.ErrHelp)
	}
	return true, false
}

func writeString(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeln(w io.Writer, args ...any) error {
	_, err := fmt.Fprintln(w, args...)
	return err
}

const usageText = `Usage: lazycron [command]

Commands:
  (none)      Launch the interactive TUI
  list        Print visible cron jobs
  validate    Check cron sources for issues
  run <id>    Run a job by ID outside the TUI
  doctor      Show platform and adapter diagnostics

Run 'lazycron <command> -h' for command-specific help.
`

type Deps struct {
	Client      crontab.Client
	Source      domain.CronSource
	Runner      *runner.Runner
	ScheduleSvc *schedule.Service
}

func DefaultDeps() Deps {
	return Deps{
		Client: crontab.NewSystemClient(),
		Source: domain.CronSource{
			Kind: domain.SourceKindUserCrontab,
			Path: "crontab://current-user",
		},
		Runner:      runner.New(runner.DefaultConfig()),
		ScheduleSvc: schedule.NewService(),
	}
}

func Run(args []string, stdout, stderr io.Writer, deps Deps) int {
	if len(args) < 2 {
		return -1 // signal: launch TUI
	}

	switch args[1] {
	case "list":
		return runList(args[2:], stdout, stderr, deps)
	case "validate":
		return runValidate(args[2:], stdout, stderr, deps)
	case "run":
		return runRun(args[2:], stdout, stderr, deps)
	case "doctor":
		return runDoctor(args[2:], stdout, stderr, deps)
	case "-h", "--help", "help":
		if err := writeString(stdout, usageText); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
		return 0
	default:
		return -1 // unknown command: fall through to TUI
	}
}

func runList(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "output as JSON")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	svc := app.NewApplyService(deps.Client, deps.Source)
	if err := svc.Load(context.Background()); err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	jobs := svc.Jobs()
	if *jsonFlag {
		safe := sanitizeJobsForJSON(jobs)
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(safe); err != nil {
			if writeErr := writef(stderr, "error encoding JSON: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
		return 0
	}

	if len(jobs) == 0 {
		if err := writeln(stdout, "No cron jobs found."); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
		return 0
	}

	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		desc := deps.ScheduleSvc.Describe(job.Schedule)
		if err := writef(stdout, "%s\t%s\t%s\t%s\n", job.ID, status, desc, job.Command); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
	}
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	svc := app.NewApplyService(deps.Client, deps.Source)
	if err := svc.Load(context.Background()); err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	issues := svc.Issues()
	if len(issues) == 0 {
		if err := writeln(stdout, "No issues found."); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
		return 0
	}

	for _, issue := range issues {
		if err := writef(stdout, "line %d: [%s] %s\n", issue.LineIndex+1, issue.Severity, issue.Message); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
	}
	return 1
}

func runRun(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envMode := fs.String("env", "cron_like", "environment mode: cron_like or shell_inherit")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	if fs.NArg() < 1 {
		if err := writeln(stderr, "usage: lazycron run <job-id>"); err != nil {
			return 1
		}
		return 1
	}
	jobID := fs.Arg(0)

	mode := domain.EnvModeCronLike
	if *envMode == "shell_inherit" {
		mode = domain.EnvModeShellInherit
	}

	svc := app.NewApplyService(deps.Client, deps.Source)
	if err := svc.Load(context.Background()); err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	var target *domain.CronJob
	for _, job := range svc.Jobs() {
		if job.ID == jobID {
			target = &job
			break
		}
	}
	if target == nil {
		if err := writef(stderr, "error: job %q not found\n", jobID); err != nil {
			return 1
		}
		return 1
	}

	rec, err := deps.Runner.Run(context.Background(), *target, mode)
	if err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	if rec.Stdout != "" {
		if err := writeString(stdout, rec.Stdout); err != nil {
			if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
	}
	if rec.Stderr != "" {
		if err := writeString(stderr, rec.Stderr); err != nil {
			return 1
		}
	}

	if err := writef(stderr, "\nexit=%d  duration=%s  mode=%s\n",
		rec.ExitCode,
		rec.Duration.Round(time.Millisecond),
		rec.Mode,
	); err != nil {
		return 1
	}
	return rec.ExitCode
}

func runDoctor(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	if err := writeln(stdout, "lazycron doctor"); err != nil {
		return 1
	}
	if err := writeln(stdout, strings.Repeat("-", 40)); err != nil {
		return 1
	}

	text, meta, err := deps.Client.Read(context.Background())
	if err != nil {
		if writeErr := writef(stdout, "crontab read:  ERROR (%v)\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	if meta.IsEmpty {
		if err := writeln(stdout, "crontab read:  OK (no crontab for user)"); err != nil {
			return 1
		}
	} else {
		lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
		if err := writef(stdout, "crontab read:  OK (%d lines)\n", len(lines)); err != nil {
			return 1
		}
	}

	if meta.User != "" {
		if err := writef(stdout, "current user:  %s\n", meta.User); err != nil {
			return 1
		}
	}

	svc := app.NewApplyService(deps.Client, deps.Source)
	if err := svc.Load(context.Background()); err != nil {
		if writeErr := writef(stdout, "parse:         ERROR (%v)\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	if err := writef(stdout, "jobs found:    %d\n", len(svc.Jobs())); err != nil {
		return 1
	}
	if err := writef(stdout, "issues found:  %d\n", len(svc.Issues())); err != nil {
		return 1
	}
	if err := writef(stdout, "source kind:   %s\n", deps.Source.Kind); err != nil {
		return 1
	}
	if err := writef(stdout, "source path:   %s\n", deps.Source.Path); err != nil {
		return 1
	}

	return 0
}

type jsonJob struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	Schedule struct {
		Kind       string `json:"kind"`
		Expression string `json:"expression"`
		Timezone   string `json:"timezone,omitempty"`
	} `json:"schedule"`
	Command string   `json:"command"`
	EnvKeys []string `json:"env_keys,omitempty"`
}

func sanitizeJobsForJSON(jobs []domain.CronJob) []jsonJob {
	out := make([]jsonJob, len(jobs))
	for i, j := range jobs {
		out[i] = jsonJob{
			ID:      j.ID,
			Enabled: j.Enabled,
			Command: j.Command,
		}
		out[i].Schedule.Kind = string(j.Schedule.Kind)
		out[i].Schedule.Expression = j.Schedule.Expression
		out[i].Schedule.Timezone = j.Schedule.Timezone
		if len(j.EnvContext) > 0 {
			keys := make([]string, len(j.EnvContext))
			for k, ea := range j.EnvContext {
				keys[k] = ea.Key
			}
			out[i].EnvKeys = keys
		}
	}
	return out
}
