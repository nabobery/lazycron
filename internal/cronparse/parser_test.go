package cronparse

import (
	"testing"

	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/testutil"
)

var testSource = domain.CronSource{
	Kind: domain.SourceKindUserCrontab,
	Path: "crontab://current-user",
	User: "testuser",
}

var testSystemSource = domain.CronSource{
	Kind:    domain.SourceKindSystem,
	Subkind: domain.SubkindSystemCrontab,
	Path:    "/etc/crontab",
	Label:   "/etc/crontab",
}

var testCronDSource = domain.CronSource{
	Kind:    domain.SourceKindSystem,
	Subkind: domain.SubkindCronD,
	Path:    "/etc/cron.d/backups",
	Label:   "cron.d/backups",
}

func TestParse_ValidFixture(t *testing.T) {
	doc, jobs, issues := Parse(testutil.FixtureValid, testSource)

	if doc.Raw != testutil.FixtureValid {
		t.Fatal("raw text should be preserved")
	}

	if len(doc.Lines) != 7 {
		t.Fatalf("expected 7 lines, got %d", len(doc.Lines))
	}

	// Line 0: comment
	if doc.Lines[0].Kind != domain.LineKindComment {
		t.Errorf("line 0: expected comment, got %s", doc.Lines[0].Kind)
	}

	// Line 1: env assignment
	if doc.Lines[1].Kind != domain.LineKindEnv {
		t.Errorf("line 1: expected env, got %s", doc.Lines[1].Kind)
	}
	if doc.Lines[1].EnvKey != "MAILTO" || doc.Lines[1].EnvVal != "ops@example.com" {
		t.Errorf("line 1: expected MAILTO=ops@example.com, got %s=%s", doc.Lines[1].EnvKey, doc.Lines[1].EnvVal)
	}

	// Line 2: standard job
	if doc.Lines[2].Kind != domain.LineKindJob {
		t.Errorf("line 2: expected job, got %s", doc.Lines[2].Kind)
	}
	if doc.Lines[2].Job == nil {
		t.Fatal("line 2: job should not be nil")
	}
	if doc.Lines[2].Job.Schedule.Kind != domain.ScheduleKindStandard {
		t.Errorf("line 2: expected standard schedule, got %s", doc.Lines[2].Job.Schedule.Kind)
	}
	if doc.Lines[2].Job.Command != "/usr/local/bin/backup-db" {
		t.Errorf("line 2: expected command /usr/local/bin/backup-db, got %s", doc.Lines[2].Job.Command)
	}

	// Line 3: comment
	if doc.Lines[3].Kind != domain.LineKindComment {
		t.Errorf("line 3: expected comment, got %s", doc.Lines[3].Kind)
	}

	// Line 4: standard job with */15
	if doc.Lines[4].Kind != domain.LineKindJob {
		t.Errorf("line 4: expected job, got %s", doc.Lines[4].Kind)
	}

	// Line 5: @daily descriptor
	if doc.Lines[5].Kind != domain.LineKindJob {
		t.Errorf("line 5: expected job, got %s", doc.Lines[5].Kind)
	}
	if doc.Lines[5].Job.Schedule.Kind != domain.ScheduleKindDescriptor {
		t.Errorf("line 5: expected descriptor schedule, got %s", doc.Lines[5].Job.Schedule.Kind)
	}

	// Line 6: @reboot
	if doc.Lines[6].Kind != domain.LineKindJob {
		t.Errorf("line 6: expected job, got %s", doc.Lines[6].Kind)
	}
	if doc.Lines[6].Job.Schedule.Kind != domain.ScheduleKindReboot {
		t.Errorf("line 6: expected reboot schedule, got %s", doc.Lines[6].Job.Schedule.Kind)
	}

	if len(jobs) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(jobs))
	}

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}

	// Verify env context propagation
	backupJob := jobs[0]
	if len(backupJob.EnvContext) != 1 || backupJob.EnvContext[0].Key != "MAILTO" {
		t.Errorf("backup job should have MAILTO in env context, got %v", backupJob.EnvContext)
	}
}

func TestParse_DisabledFixture(t *testing.T) {
	doc, jobs, _ := Parse(testutil.FixtureWithDisabled, testSource)

	if len(doc.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(doc.Lines))
	}

	// Line 1: disabled
	if doc.Lines[1].Kind != domain.LineKindDisabled {
		t.Errorf("line 1: expected disabled, got %s", doc.Lines[1].Kind)
	}
	if doc.Lines[1].Job == nil {
		t.Fatal("line 1: disabled line should still have a parsed job")
	}
	if doc.Lines[1].Job.Enabled {
		t.Error("line 1: disabled job should have Enabled=false")
	}

	// Should have 3 jobs total (1 active, 1 disabled, 1 active)
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	disabledJob := jobs[1]
	if disabledJob.Enabled {
		t.Error("disabled job should have Enabled=false")
	}
	if disabledJob.Command != "/usr/local/bin/healthcheck --quiet" {
		t.Errorf("disabled job command wrong: %s", disabledJob.Command)
	}
}

func TestParse_InvalidFixture(t *testing.T) {
	doc, jobs, issues := Parse(testutil.FixtureInvalid, testSource)

	if len(doc.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(doc.Lines))
	}

	// Line 1: invalid
	if doc.Lines[1].Kind != domain.LineKindInvalid {
		t.Errorf("line 1: expected invalid, got %s", doc.Lines[1].Kind)
	}

	// Line 2: invalid (only 3 fields)
	if doc.Lines[2].Kind != domain.LineKindInvalid {
		t.Errorf("line 2: expected invalid, got %s", doc.Lines[2].Kind)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 valid jobs, got %d", len(jobs))
	}

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
}

func TestParse_EmptyFixture(t *testing.T) {
	doc, jobs, issues := Parse(testutil.FixtureEmpty, testSource)

	if len(doc.Lines) != 0 {
		t.Fatalf("expected 0 lines, got %d", len(doc.Lines))
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestParse_OnlyComments(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureOnlyComments, testSource)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestParse_EnvOnly(t *testing.T) {
	doc, jobs, _ := Parse(testutil.FixtureEnvOnly, testSource)
	for _, line := range doc.Lines {
		if line.Kind != domain.LineKindEnv {
			t.Errorf("expected all env lines, got %s for %q", line.Kind, line.Raw)
		}
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestParse_MixedFixture(t *testing.T) {
	doc, jobs, issues := Parse(testutil.FixtureMixed, testSource)

	// Count line kinds
	kindCounts := map[domain.LineKind]int{}
	for _, line := range doc.Lines {
		kindCounts[line.Kind]++
	}

	if kindCounts[domain.LineKindEnv] != 3 {
		t.Errorf("expected 3 env lines, got %d", kindCounts[domain.LineKindEnv])
	}
	if kindCounts[domain.LineKindJob] < 4 {
		t.Errorf("expected at least 4 job lines, got %d", kindCounts[domain.LineKindJob])
	}
	if kindCounts[domain.LineKindDisabled] != 1 {
		t.Errorf("expected 1 disabled line, got %d", kindCounts[domain.LineKindDisabled])
	}
	if kindCounts[domain.LineKindInvalid] != 1 {
		t.Errorf("expected 1 invalid line, got %d", kindCounts[domain.LineKindInvalid])
	}

	// CRON_TZ job should have timezone
	tzJobIndex := -1
	for i := range jobs {
		if jobs[i].Schedule.Timezone != "" {
			tzJobIndex = i
			break
		}
	}
	if tzJobIndex < 0 {
		t.Fatal("expected a job with timezone")
	}
	if jobs[tzJobIndex].Schedule.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", jobs[tzJobIndex].Schedule.Timezone)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	_ = doc
}

func TestParse_FingerprintStability(t *testing.T) {
	_, jobs1, _ := Parse(testutil.FixtureValid, testSource)
	_, jobs2, _ := Parse(testutil.FixtureValid, testSource)

	if len(jobs1) != len(jobs2) {
		t.Fatal("job counts should match")
	}
	for i := range jobs1 {
		if jobs1[i].Fingerprint != jobs2[i].Fingerprint {
			t.Errorf("job %d fingerprint mismatch: %s != %s", i, jobs1[i].Fingerprint, jobs2[i].Fingerprint)
		}
	}
}

func TestParse_EveryDescriptor(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureEveryDescriptor, testSource)

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	tests := []struct {
		idx     int
		expr    string
		command string
		kind    domain.ScheduleKind
	}{
		{0, "@every 5m", "/usr/local/bin/poll-queue", domain.ScheduleKindDescriptor},
		{1, "@every 30s", "/usr/local/bin/heartbeat --fast", domain.ScheduleKindDescriptor},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			job := jobs[tt.idx]
			if job.Schedule.Expression != tt.expr {
				t.Errorf("expected expression %q, got %q", tt.expr, job.Schedule.Expression)
			}
			if job.Command != tt.command {
				t.Errorf("expected command %q, got %q", tt.command, job.Command)
			}
			if job.Schedule.Kind != tt.kind {
				t.Errorf("expected kind %s, got %s", tt.kind, job.Schedule.Kind)
			}
		})
	}
}

func TestParse_TabSeparated(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureTabSeparated, testSource)

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	t.Run("CRON_TZ with tabs", func(t *testing.T) {
		job := jobs[0]
		if job.Schedule.Timezone != "UTC" {
			t.Errorf("expected timezone UTC, got %q", job.Schedule.Timezone)
		}
		if job.Schedule.Expression != "@daily" {
			t.Errorf("expected expression @daily, got %q", job.Schedule.Expression)
		}
		if job.Command != "/usr/local/bin/tz-daily" {
			t.Errorf("expected command /usr/local/bin/tz-daily, got %q", job.Command)
		}
	})

	t.Run("TZ with tabs and standard expr", func(t *testing.T) {
		job := jobs[1]
		if job.Schedule.Timezone != "Europe/Berlin" {
			t.Errorf("expected timezone Europe/Berlin, got %q", job.Schedule.Timezone)
		}
		if job.Schedule.Expression != "0 3 * * *" {
			t.Errorf("expected expression '0 3 * * *', got %q", job.Schedule.Expression)
		}
		if job.Command != "/usr/local/bin/tz-standard" {
			t.Errorf("expected command /usr/local/bin/tz-standard, got %q", job.Command)
		}
	})

	t.Run("tab-separated standard fields", func(t *testing.T) {
		job := jobs[2]
		if job.Schedule.Expression != "0 3 * * *" {
			t.Errorf("expected expression '0 3 * * *', got %q", job.Schedule.Expression)
		}
		if job.Command != "/usr/local/bin/tab-fields" {
			t.Errorf("expected command /usr/local/bin/tab-fields, got %q", job.Command)
		}
	})
}

func TestParse_SystemCrontab(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureSystemCrontab, testSystemSource)

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
	if len(jobs) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(jobs))
	}

	tests := []struct {
		idx      int
		expr     string
		user     string
		readOnly bool
	}{
		{0, "17 * * * *", "root", true},
		{1, "25 6 * * *", "root", true},
		{2, "47 6 * * 0", "root", true},
		{3, "52 6 1 * *", "root", true},
	}

	for _, tt := range tests {
		t.Run(jobs[tt.idx].Command[:20], func(t *testing.T) {
			job := jobs[tt.idx]
			if job.Schedule.Expression != tt.expr {
				t.Errorf("expected expression %q, got %q", tt.expr, job.Schedule.Expression)
			}
			if job.RunAsUser != tt.user {
				t.Errorf("expected RunAsUser %q, got %q", tt.user, job.RunAsUser)
			}
			if job.ReadOnly != tt.readOnly {
				t.Errorf("expected ReadOnly=%v, got %v", tt.readOnly, job.ReadOnly)
			}
			if job.Source.Kind != domain.SourceKindSystem {
				t.Errorf("expected system source kind, got %s", job.Source.Kind)
			}
		})
	}
}

func TestParse_SystemCronD(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureSystemCronD, testCronDSource)

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
	if len(jobs) != 4 {
		t.Fatalf("expected 4 jobs, got %d", len(jobs))
	}

	t.Run("standard 6-field", func(t *testing.T) {
		job := jobs[0]
		if job.Schedule.Expression != "0 2 * * *" {
			t.Errorf("expected expression '0 2 * * *', got %q", job.Schedule.Expression)
		}
		if job.RunAsUser != "root" {
			t.Errorf("expected RunAsUser root, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/backup --full" {
			t.Errorf("expected command '/usr/local/bin/backup --full', got %q", job.Command)
		}
	})

	t.Run("non-root user", func(t *testing.T) {
		job := jobs[1]
		if job.RunAsUser != "monitor" {
			t.Errorf("expected RunAsUser monitor, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/check-health" {
			t.Errorf("expected command /usr/local/bin/check-health, got %q", job.Command)
		}
	})

	t.Run("descriptor with user", func(t *testing.T) {
		job := jobs[2]
		if job.Schedule.Kind != domain.ScheduleKindDescriptor {
			t.Errorf("expected descriptor kind, got %s", job.Schedule.Kind)
		}
		if job.RunAsUser != "root" {
			t.Errorf("expected RunAsUser root, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/cleanup-old-logs" {
			t.Errorf("expected command /usr/local/bin/cleanup-old-logs, got %q", job.Command)
		}
	})

	t.Run("reboot with user", func(t *testing.T) {
		job := jobs[3]
		if job.Schedule.Kind != domain.ScheduleKindReboot {
			t.Errorf("expected reboot kind, got %s", job.Schedule.Kind)
		}
		if job.RunAsUser != "www-data" {
			t.Errorf("expected RunAsUser www-data, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/start-webapp" {
			t.Errorf("expected command /usr/local/bin/start-webapp, got %q", job.Command)
		}
	})
}

func TestParse_SystemTabSeparated(t *testing.T) {
	_, jobs, issues := Parse(testutil.FixtureSystemTabSeparated, testSystemSource)

	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	t.Run("tab-separated system fields", func(t *testing.T) {
		job := jobs[0]
		if job.Schedule.Expression != "0 3 * * *" {
			t.Errorf("expected expression '0 3 * * *', got %q", job.Schedule.Expression)
		}
		if job.RunAsUser != "root" {
			t.Errorf("expected RunAsUser root, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/backup" {
			t.Errorf("expected command /usr/local/bin/backup, got %q", job.Command)
		}
	})

	t.Run("CRON_TZ with system format", func(t *testing.T) {
		job := jobs[1]
		if job.Schedule.Timezone != "UTC" {
			t.Errorf("expected timezone UTC, got %q", job.Schedule.Timezone)
		}
		if job.RunAsUser != "root" {
			t.Errorf("expected RunAsUser root, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/tz-daily" {
			t.Errorf("expected command /usr/local/bin/tz-daily, got %q", job.Command)
		}
	})

	t.Run("TZ with system format and non-root user", func(t *testing.T) {
		job := jobs[2]
		if job.Schedule.Timezone != "Europe/Berlin" {
			t.Errorf("expected timezone Europe/Berlin, got %q", job.Schedule.Timezone)
		}
		if job.RunAsUser != "backup" {
			t.Errorf("expected RunAsUser backup, got %q", job.RunAsUser)
		}
		if job.Command != "/usr/local/bin/tz-standard" {
			t.Errorf("expected command /usr/local/bin/tz-standard, got %q", job.Command)
		}
	})
}

func TestParse_SystemJobIDsUseSourceKey(t *testing.T) {
	_, jobs, _ := Parse(testutil.FixtureSystemCrontab, testSystemSource)

	for _, job := range jobs {
		if job.ID == "" {
			t.Error("job ID should not be empty")
		}
		// System source IDs should start with "sys:" prefix
		if len(job.ID) < 4 || job.ID[:4] != "sys:" {
			t.Errorf("system job ID should start with 'sys:', got %q", job.ID)
		}
	}
}

func TestParse_UserJobIDsPreserveFormat(t *testing.T) {
	_, jobs, _ := Parse(testutil.FixtureValid, testSource)

	prefix := "user_crontab:"
	for _, job := range jobs {
		if job.ID == "" {
			t.Error("job ID should not be empty")
		}
		if len(job.ID) < len(prefix) || job.ID[:len(prefix)] != prefix {
			t.Errorf("user job ID should start with %q, got %q", prefix, job.ID)
		}
	}
}

func TestBuildPeriodicJob(t *testing.T) {
	src := domain.CronSource{
		Kind:    domain.SourceKindSystem,
		Subkind: domain.SubkindPeriodicDir,
		Path:    "/etc/cron.daily/logrotate",
		Label:   "daily/logrotate",
	}

	job := BuildPeriodicJob(src, "logrotate", domain.PeriodicDaily)

	if job.Schedule.Kind != domain.ScheduleKindPeriodic {
		t.Errorf("expected periodic kind, got %s", job.Schedule.Kind)
	}
	if job.Schedule.Expression != "daily" {
		t.Errorf("expected expression 'daily', got %q", job.Schedule.Expression)
	}
	if job.RunAsUser != "root" {
		t.Errorf("expected RunAsUser root, got %q", job.RunAsUser)
	}
	if !job.ReadOnly {
		t.Error("expected ReadOnly=true for periodic job")
	}
	if !job.Enabled {
		t.Error("expected Enabled=true for periodic job")
	}
	if job.ID == "" {
		t.Error("expected non-empty ID")
	}
}
