package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/cronparse"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/cronlogs"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
	"github.com/avinashchangrani/lazycron/internal/platform/systemcron"
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
  logs [id]   Fetch system cron log entries
  export      Export crontab to a file
  import      Import crontab from a file
  doctor      Show platform and adapter diagnostics

Run 'lazycron <command> -h' for command-specific help.
`

type Deps struct {
	Client       crontab.Client
	Source       domain.CronSource
	Runner       *runner.Runner
	ScheduleSvc  *schedule.Service
	Discoverer   *systemcron.Discoverer
	LogsProvider cronlogs.Provider
}

func DefaultDeps() Deps {
	return Deps{
		Client: crontab.NewSystemClient(),
		Source: domain.CronSource{
			Kind: domain.SourceKindUserCrontab,
			Path: "crontab://current-user",
		},
		Runner:       runner.New(runner.DefaultConfig()),
		ScheduleSvc:  schedule.NewService(),
		Discoverer:   systemcron.New(),
		LogsProvider: cronlogs.NewAutoProvider(),
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
	case "logs":
		return runLogs(args[2:], stdout, stderr, deps)
	case "export":
		return runExport(args[2:], stdout, stderr, deps)
	case "import":
		return runImport(args[2:], stdout, stderr, deps)
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

func loadJobs(ctx context.Context, deps Deps, includeSystem bool) ([]domain.CronJob, []domain.ValidationIssue, error) {
	applySvc := app.NewApplyService(deps.Client, deps.Source)
	if !includeSystem || deps.Discoverer == nil {
		if err := applySvc.Load(ctx); err != nil {
			return nil, nil, err
		}
		return applySvc.Jobs(), applySvc.Issues(), nil
	}

	invSvc := app.NewInventoryService(applySvc, deps.Discoverer)
	inv, err := invSvc.LoadAll(ctx)
	if err != nil {
		return nil, nil, err
	}
	return inv.Jobs, inv.Issues, nil
}

func runList(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "output as JSON")
	allFlag := fs.Bool("all", false, "include system cron sources")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	jobs, _, err := loadJobs(context.Background(), deps, *allFlag)
	if err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

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
		source := ""
		if job.Source.Label != "" {
			source = job.Source.Label
		}
		if source != "" {
			if err := writef(stdout, "%s\t%s\t%s\t%s\t%s\n", job.ID, status, desc, job.Command, source); err != nil {
				if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
					return 1
				}
				return 1
			}
		} else {
			if err := writef(stdout, "%s\t%s\t%s\t%s\n", job.ID, status, desc, job.Command); err != nil {
				if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
					return 1
				}
				return 1
			}
		}
	}
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allFlag := fs.Bool("all", false, "include system cron sources")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	_, issues, err := loadJobs(context.Background(), deps, *allFlag)
	if err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

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
		var line string
		prefix := ""
		if issue.SourcePath != "" {
			prefix = issue.SourcePath + ": "
		}
		if issue.LineIndex < 0 {
			line = fmt.Sprintf("%s[%s] %s\n", prefix, issue.Severity, issue.Message)
		} else {
			line = fmt.Sprintf("%sline %d: [%s] %s\n", prefix, issue.LineIndex+1, issue.Severity, issue.Message)
		}
		if err := writeString(stdout, line); err != nil {
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
	allFlag := fs.Bool("all", false, "search system sources for the job ID")
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

	jobs, _, err := loadJobs(context.Background(), deps, *allFlag)
	if err != nil {
		if writeErr := writef(stderr, "error: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	var target *domain.CronJob
	for i := range jobs {
		if jobs[i].ID == jobID {
			target = &jobs[i]
			break
		}
	}
	if target == nil {
		if err := writef(stderr, "error: job %q not found\n", jobID); err != nil {
			return 1
		}
		return 1
	}

	if target.RunAsUser != "" {
		if u, uErr := user.Current(); uErr == nil && u.Username != target.RunAsUser {
			_ = writef(stderr, "Note: job runs as %s in cron; running now as %s\n", target.RunAsUser, u.Username)
		}
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

	// System cron source diagnostics (always included in doctor)
	if err := writeln(stdout, ""); err != nil {
		return 1
	}
	if err := writeln(stdout, "System cron sources:"); err != nil {
		return 1
	}

	if deps.Discoverer == nil {
		if err := writeln(stdout, "  (discoverer not available)"); err != nil {
			return 1
		}
		return 0
	}

	sysSources, periodicEntries, sysIssues := deps.Discoverer.DiscoverAll()

	if len(sysSources) == 0 && len(periodicEntries) == 0 {
		if err := writeln(stdout, "  No system cron sources found (or not readable)"); err != nil {
			return 1
		}
	}

	for _, ds := range sysSources {
		status := "readable"
		if !ds.Source.Access.Readable {
			status = "NOT readable"
			if ds.Source.Access.Reason != "" {
				status += " (" + ds.Source.Access.Reason + ")"
			}
		}
		if err := writef(stdout, "  %-30s %s\n", ds.Source.Path, status); err != nil {
			return 1
		}
	}

	if len(periodicEntries) > 0 {
		if err := writef(stdout, "  periodic entries:  %d\n", len(periodicEntries)); err != nil {
			return 1
		}
	}

	if len(sysIssues) > 0 {
		if err := writef(stdout, "  system issues:     %d\n", len(sysIssues)); err != nil {
			return 1
		}
	}

	return 0
}

func runLogs(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sinceFlag := fs.String("since", "", "show logs since (e.g. '1 hour ago', '2024-01-01 00:00:00')")
	limitFlag := fs.Int("limit", 50, "maximum number of log lines")
	allFlag := fs.Bool("all", false, "search system sources for the job ID")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	provider := deps.LogsProvider
	if provider == nil {
		provider = cronlogs.NewAutoProvider()
	}

	q := cronlogs.Query{Limit: *limitFlag}

	if *sinceFlag != "" {
		t, err := parseSinceValue(*sinceFlag)
		if err != nil {
			_ = writef(stderr, "error: cannot parse --since %q\n", *sinceFlag)
			return 1
		}
		q.Since = t
	}

	if fs.NArg() > 0 {
		jobID := fs.Arg(0)
		jobs, _, err := loadJobs(context.Background(), deps, *allFlag)
		if err != nil {
			_ = writef(stderr, "error: %v\n", err)
			return 1
		}
		var target *domain.CronJob
		for i := range jobs {
			if jobs[i].ID == jobID {
				target = &jobs[i]
				break
			}
		}
		if target == nil {
			_ = writef(stderr, "error: job %q not found\n", jobID)
			return 1
		}
		q.Command = target.Command
		if target.RunAsUser != "" {
			q.User = target.RunAsUser
		}
	}

	result, err := provider.Fetch(context.Background(), q)
	if err != nil {
		_ = writef(stderr, "error: %v\n", err)
		return 1
	}

	if result.NotFound {
		_ = writef(stderr, "logs not available: %s\n", result.Reason)
		return 0
	}

	if len(result.Lines) == 0 {
		_ = writef(stdout, "No matching log entries found (source: %s)\n", result.Source)
		return 0
	}

	for _, line := range result.Lines {
		_ = writeln(stdout, line)
	}

	if result.Partial {
		_ = writef(stderr, "(output truncated at %d lines)\n", *limitFlag)
	}
	_ = writef(stderr, "source: %s\n", result.Source)
	return 0
}

func runExport(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outFlag := fs.String("out", "", "output file path (default: stdout)")
	allFlag := fs.Bool("all", false, "include system cron sources (read-only)")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	text, _, err := deps.Client.Read(context.Background())
	if err != nil {
		_ = writef(stderr, "error reading crontab: %v\n", err)
		return 1
	}

	var output string
	if *allFlag {
		var sb strings.Builder
		sb.WriteString("# === User crontab ===\n")
		sb.WriteString(text)

		if deps.Discoverer != nil {
			sources, _, _ := deps.Discoverer.DiscoverAll()
			for _, ds := range sources {
				if ds.Text == "" {
					continue
				}
				sb.WriteString("\n# === " + ds.Source.Path + " (read-only) ===\n")
				sb.WriteString(ds.Text)
			}
		}
		output = sb.String()
	} else {
		output = text
	}

	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(output), 0o600); err != nil {
			_ = writef(stderr, "error writing to %s: %v\n", *outFlag, err)
			return 1
		}
		_ = writef(stderr, "exported to %s\n", *outFlag)
		return 0
	}

	_ = writeString(stdout, output)
	return 0
}

func runImport(args []string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromFlag := fs.String("from", "", "file to import (required)")
	yesFlag := fs.Bool("yes", false, "actually apply (default is dry-run)")
	ok, help := parseFlags(fs, args)
	if !ok {
		if help {
			return 0
		}
		return 1
	}

	if *fromFlag == "" {
		_ = writeln(stderr, "error: --from is required")
		return 1
	}

	data, err := os.ReadFile(*fromFlag)
	if err != nil {
		_ = writef(stderr, "error reading %s: %v\n", *fromFlag, err)
		return 1
	}

	content := string(data)
	source := domain.CronSource{
		Kind: domain.SourceKindUserCrontab,
		Path: *fromFlag,
	}

	_, jobs, issues := cronparse.Parse(content, source)
	_ = writef(stdout, "Parsed %d jobs from %s\n", len(jobs), *fromFlag)

	if len(issues) > 0 {
		_ = writef(stdout, "%d issues found:\n", len(issues))
		for _, issue := range issues {
			_ = writef(stdout, "  line %d: [%s] %s\n", issue.LineIndex+1, issue.Severity, issue.Message)
		}
	}

	if !*yesFlag {
		_ = writeln(stdout, "\nDry run — no changes applied. Use --yes to apply.")
		return 0
	}

	result, err := deps.Client.Apply(context.Background(), content)
	if err != nil {
		_ = writef(stderr, "error applying: %v\n", err)
		if result.Stderr != "" {
			_ = writef(stderr, "crontab stderr: %s\n", result.Stderr)
		}
		return 1
	}

	_ = writeln(stdout, "Imported successfully.")
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
	Command   string      `json:"command"`
	EnvKeys   []string    `json:"env_keys,omitempty"`
	Source    *jsonSource `json:"source,omitempty"`
	RunAsUser string      `json:"run_as_user,omitempty"`
	ReadOnly  bool        `json:"read_only"`
	Mutable   bool        `json:"mutable"`
}

type jsonSource struct {
	Kind    string `json:"kind"`
	Subkind string `json:"subkind,omitempty"`
	Path    string `json:"path"`
	Label   string `json:"label,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

func sanitizeJobsForJSON(jobs []domain.CronJob) []jsonJob {
	out := make([]jsonJob, len(jobs))
	for i, j := range jobs {
		out[i] = jsonJob{
			ID:        j.ID,
			Enabled:   j.Enabled,
			Command:   j.Command,
			RunAsUser: j.RunAsUser,
			ReadOnly:  j.ReadOnly,
			Mutable:   app.IsJobMutable(j),
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
		out[i].Source = &jsonSource{
			Kind:    string(j.Source.Kind),
			Subkind: string(j.Source.Subkind),
			Path:    j.Source.Path,
			Label:   j.Source.Label,
			Owner:   j.Source.Owner,
		}
	}
	return out
}

func parseSinceValue(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return parseRelativeTime(s)
}

func parseRelativeTime(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasSuffix(s, " ago") {
		return time.Time{}, fmt.Errorf("unsupported time format: %q", s)
	}
	s = strings.TrimSuffix(s, " ago")
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("unsupported relative time: %q", s)
	}

	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("invalid number in relative time: %q", parts[0])
	}

	unit := strings.TrimSuffix(parts[1], "s")
	var d time.Duration
	switch unit {
	case "minute":
		d = time.Duration(n) * time.Minute
	case "hour":
		d = time.Duration(n) * time.Hour
	case "day":
		d = time.Duration(n) * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("unsupported time unit: %q", parts[1])
	}

	return time.Now().Add(-d), nil
}
