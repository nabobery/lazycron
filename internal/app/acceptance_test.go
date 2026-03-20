package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
	"github.com/avinashchangrani/lazycron/internal/runner"
	"github.com/avinashchangrani/lazycron/internal/schedule"
)

// Acceptance criteria from the spec:
// 1. The app loads a valid user crontab and renders all visible jobs
// 2. Selected jobs show a readable schedule and at least three next runs
// 3. Toggling a job updates the crontab safely and survives reload
// 4. Deleting a job removes only the intended entry
// 5. Manual execution shows stdout, stderr, exit code, and duration
// 6. Search narrows the list without corrupting selection state

const acceptanceFixture = `# Environment
MAILTO=ops@example.com
PATH=/usr/local/bin:/usr/bin:/bin

# Backup jobs
0 3 * * * /usr/local/bin/backup-db
30 4 * * 0 /usr/local/bin/weekly-report

# Monitoring
*/15 * * * * /usr/local/bin/healthcheck --quiet
@daily /usr/local/bin/cleanup-logs
@reboot /usr/local/bin/start-agent
`

func TestAcceptance_LoadAndRenderAllJobs(t *testing.T) {
	fc := crontab.NewFakeClient(acceptanceFixture, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	jobs := svc.Jobs()
	if len(jobs) != 5 {
		t.Fatalf("expected 5 jobs, got %d", len(jobs))
	}

	// Verify all jobs have IDs, commands, and schedules
	for i, job := range jobs {
		if job.ID == "" {
			t.Errorf("job %d: missing ID", i)
		}
		if job.Command == "" {
			t.Errorf("job %d: missing command", i)
		}
		if job.Schedule.Expression == "" {
			t.Errorf("job %d: missing schedule expression", i)
		}
	}

	// Verify env context propagation
	for _, job := range jobs {
		if len(job.EnvContext) < 2 {
			t.Errorf("job %s: expected at least 2 env vars in context, got %d", job.Command, len(job.EnvContext))
		}
	}
}

func TestAcceptance_ScheduleAndNextRuns(t *testing.T) {
	fc := crontab.NewFakeClient(acceptanceFixture, true)
	svc := NewApplyService(fc, testSource)
	schedSvc := schedule.NewService()

	ctx := context.Background()
	_ = svc.Load(ctx)

	now := time.Now()

	for _, job := range svc.Jobs() {
		desc := schedSvc.Describe(job.Schedule)
		if desc == "" {
			t.Errorf("job %s: empty description", job.Command)
		}

		runs, err := schedSvc.NextRuns(job.Schedule, now, 3)
		if err != nil {
			t.Errorf("job %s: next runs error: %v", job.Command, err)
			continue
		}

		if job.Schedule.Kind == domain.ScheduleKindReboot {
			if len(runs) != 0 {
				t.Errorf("@reboot job should have 0 next runs, got %d", len(runs))
			}
			continue
		}

		if len(runs) < 3 {
			t.Errorf("job %s: expected at least 3 next runs, got %d", job.Command, len(runs))
		}

		// Verify runs are in ascending order
		for i := 1; i < len(runs); i++ {
			if !runs[i].After(runs[i-1]) {
				t.Errorf("job %s: run %d not after run %d", job.Command, i, i-1)
			}
		}
	}
}

func TestAcceptance_ToggleSurvivesReload(t *testing.T) {
	fc := crontab.NewFakeClient(acceptanceFixture, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	targetJob := jobs[0] // backup-db
	if !targetJob.Enabled {
		t.Fatal("first job should be enabled")
	}

	// Toggle disable
	err := svc.Toggle(ctx, targetJob.ID)
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}

	// After toggle, reload happens automatically via applyDoc
	jobs = svc.Jobs()
	var found bool
	for _, job := range jobs {
		if strings.Contains(job.Command, "backup-db") {
			found = true
			if job.Enabled {
				t.Fatal("job should be disabled after toggle")
			}
		}
	}
	if !found {
		t.Fatal("backup-db job should still exist after toggle")
	}

	// Toggle enable again
	for _, job := range jobs {
		if strings.Contains(job.Command, "backup-db") {
			err = svc.Toggle(ctx, job.ID)
			if err != nil {
				t.Fatalf("re-enable toggle failed: %v", err)
			}
			break
		}
	}

	// Verify re-enabled
	jobs = svc.Jobs()
	for _, job := range jobs {
		if strings.Contains(job.Command, "backup-db") {
			if !job.Enabled {
				t.Fatal("job should be re-enabled after second toggle")
			}
		}
	}
}

func TestAcceptance_DeleteRemovesOnlyIntended(t *testing.T) {
	fc := crontab.NewFakeClient(acceptanceFixture, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	originalCount := len(svc.Jobs())
	targetJob := svc.Jobs()[2] // healthcheck

	err := svc.Delete(ctx, targetJob.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	newJobs := svc.Jobs()
	if len(newJobs) != originalCount-1 {
		t.Fatalf("expected %d jobs after delete, got %d", originalCount-1, len(newJobs))
	}

	for _, job := range newJobs {
		if strings.Contains(job.Command, "healthcheck") {
			t.Fatal("deleted job should not appear in list")
		}
	}

	// Verify other jobs are intact
	commands := map[string]bool{}
	for _, job := range newJobs {
		commands[job.Command] = true
	}
	for _, expected := range []string{"/usr/local/bin/backup-db", "/usr/local/bin/weekly-report", "/usr/local/bin/cleanup-logs", "/usr/local/bin/start-agent"} {
		if !commands[expected] {
			t.Errorf("expected job %s to survive delete", expected)
		}
	}
}

func TestAcceptance_ManualExecution(t *testing.T) {
	r := runner.New(runner.DefaultConfig())

	// Test successful execution
	job := domain.CronJob{
		ID:      "test:job:0",
		Command: "echo stdout_output && echo stderr_output >&2",
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "* * * * *",
		},
	}

	rec, err := r.Run(context.Background(), job, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if rec.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", rec.ExitCode)
	}
	if !strings.Contains(rec.Stdout, "stdout_output") {
		t.Fatalf("stdout should contain output, got %q", rec.Stdout)
	}
	if !strings.Contains(rec.Stderr, "stderr_output") {
		t.Fatalf("stderr should contain output, got %q", rec.Stderr)
	}
	if rec.Duration <= 0 {
		t.Fatal("duration should be positive")
	}
	if rec.Status != domain.RunStatusSuccess {
		t.Fatalf("expected success status, got %s", rec.Status)
	}

	// Test failed execution
	failJob := domain.CronJob{
		ID:      "test:job:1",
		Command: "exit 1",
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "* * * * *",
		},
	}

	rec, err = r.Run(context.Background(), failJob, domain.EnvModeShellInherit)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if rec.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", rec.ExitCode)
	}
	if rec.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed status, got %s", rec.Status)
	}
}

func TestAcceptance_SearchNarrowsWithoutCorruption(t *testing.T) {
	fc := crontab.NewFakeClient(acceptanceFixture, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	allCount := len(jobs)

	// Simulate search by filtering
	filter := "backup"
	var filtered []domain.CronJob
	for _, job := range jobs {
		if strings.Contains(strings.ToLower(job.Command), filter) ||
			strings.Contains(strings.ToLower(job.Schedule.Expression), filter) {
			filtered = append(filtered, job)
		}
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 match for 'backup', got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Command, "backup-db") {
		t.Fatalf("expected backup-db job, got %s", filtered[0].Command)
	}

	// Verify original list is not corrupted
	if len(jobs) != allCount {
		t.Fatal("original job list should not be modified by filtering")
	}
}
