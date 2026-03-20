package app

import (
	"context"
	"strings"
	"testing"

	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
)

var editTestSource = domain.CronSource{
	Kind: domain.SourceKindUserCrontab,
	Path: "crontab://current-user",
}

func setupEditSvc(content string, hasCrontab bool) (*ApplyService, *crontab.FakeClient) {
	fc := crontab.NewFakeClient(content, hasCrontab)
	svc := NewApplyService(fc, editTestSource)
	if err := svc.Load(context.Background()); err != nil {
		panic(err)
	}
	return svc, fc
}

func TestCreateJob_EmptyDoc(t *testing.T) {
	svc, fc := setupEditSvc("", false)

	draft := domain.JobDraft{
		Enabled:    true,
		SchedKind:  domain.ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/usr/local/bin/backup-db",
	}

	if err := svc.CreateJob(context.Background(), draft); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if len(fc.ApplyCalls) != 1 {
		t.Fatalf("expected 1 apply call, got %d", len(fc.ApplyCalls))
	}

	applied := fc.ApplyCalls[0]
	if !strings.Contains(applied, "0 3 * * * /usr/local/bin/backup-db") {
		t.Fatalf("applied content should contain the new job, got: %q", applied)
	}

	// After reload, should have 1 job
	if len(svc.Jobs()) != 1 {
		t.Fatalf("expected 1 job after create, got %d", len(svc.Jobs()))
	}
}

func TestCreateJob_NonEmptyDoc_PreservesExisting(t *testing.T) {
	original := "# Daily backup\n0 3 * * * /usr/local/bin/backup-db\n"
	svc, fc := setupEditSvc(original, true)

	draft := domain.JobDraft{
		Enabled:    true,
		SchedKind:  domain.ScheduleKindDescriptor,
		Descriptor: "@daily",
		Command:    "/usr/local/bin/cleanup",
	}

	if err := svc.CreateJob(context.Background(), draft); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]

	// Original lines must be preserved byte-for-byte
	if !strings.HasPrefix(applied, "# Daily backup\n0 3 * * * /usr/local/bin/backup-db\n") {
		t.Fatalf("original lines not preserved, got: %q", applied)
	}

	// New job should be appended
	if !strings.Contains(applied, "@daily /usr/local/bin/cleanup") {
		t.Fatalf("new job not found in applied content, got: %q", applied)
	}

	if len(svc.Jobs()) != 2 {
		t.Fatalf("expected 2 jobs after create, got %d", len(svc.Jobs()))
	}
}

func TestCreateJob_ValidationFailure(t *testing.T) {
	svc, _ := setupEditSvc("0 3 * * * /bin/echo\n", true)

	draft := domain.JobDraft{
		Enabled:   true,
		SchedKind: domain.ScheduleKindStandard,
		Minute:    "0",
		Hour:      "3",
		Command:   "", // empty command
	}

	err := svc.CreateJob(context.Background(), draft)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !IsValidationError(err) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestEditJob_PreservesOtherLines(t *testing.T) {
	original := "# Comment\n0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	svc, fc := setupEditSvc(original, true)

	jobs := svc.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Edit the first job's command
	draft := DraftFromJob(jobs[0])
	draft.Command = "/usr/local/bin/backup-db --full"

	if err := svc.EditJob(context.Background(), jobs[0].ID, draft); err != nil {
		t.Fatalf("EditJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]

	// Comment must be preserved
	if !strings.HasPrefix(applied, "# Comment\n") {
		t.Fatalf("comment not preserved, got: %q", applied)
	}

	// Second job must be preserved
	if !strings.Contains(applied, "@daily /usr/local/bin/cleanup") {
		t.Fatalf("second job not preserved, got: %q", applied)
	}

	// Edited job should have new command
	if !strings.Contains(applied, "0 3 * * * /usr/local/bin/backup-db --full") {
		t.Fatalf("edited job not found, got: %q", applied)
	}
}

func TestEditJob_PreservesTimezone(t *testing.T) {
	original := "CRON_TZ=America/New_York 0 9 * * MON-FRI /usr/local/bin/morning-check\n"
	svc, fc := setupEditSvc(original, true)

	jobs := svc.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	draft := DraftFromJob(jobs[0])
	if draft.Timezone != "America/New_York" {
		t.Fatalf("expected timezone America/New_York, got %q", draft.Timezone)
	}

	// Change only the command
	draft.Command = "/usr/local/bin/morning-check --verbose"

	if err := svc.EditJob(context.Background(), jobs[0].ID, draft); err != nil {
		t.Fatalf("EditJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	if !strings.Contains(applied, "CRON_TZ=America/New_York") {
		t.Fatalf("timezone not preserved, got: %q", applied)
	}
	if !strings.Contains(applied, "--verbose") {
		t.Fatalf("new command not found, got: %q", applied)
	}
}

func TestEditJob_DisabledJob(t *testing.T) {
	original := "# [lazycron-disabled] */15 * * * * /usr/local/bin/healthcheck --quiet\n"
	svc, fc := setupEditSvc(original, true)

	jobs := svc.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Enabled {
		t.Fatal("job should be disabled")
	}

	draft := DraftFromJob(jobs[0])
	draft.Command = "/usr/local/bin/healthcheck --verbose"

	if err := svc.EditJob(context.Background(), jobs[0].ID, draft); err != nil {
		t.Fatalf("EditJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	// Should still be disabled
	if !strings.HasPrefix(strings.TrimSpace(applied), "# [lazycron-disabled]") {
		t.Fatalf("disabled marker not preserved, got: %q", applied)
	}
	if !strings.Contains(applied, "--verbose") {
		t.Fatalf("new command not found, got: %q", applied)
	}
}

func TestEditJob_DriftBlocks(t *testing.T) {
	original := "0 3 * * * /usr/local/bin/backup-db\n"
	svc, fc := setupEditSvc(original, true)

	// Simulate external modification
	fc.Content = "0 3 * * * /usr/local/bin/backup-db\n@hourly /usr/local/bin/new-job\n"

	jobs := svc.Jobs()
	draft := DraftFromJob(jobs[0])
	draft.Command = "/usr/local/bin/backup-db --full"

	err := svc.EditJob(context.Background(), jobs[0].ID, draft)
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !IsDriftError(err) {
		t.Fatalf("expected DriftError, got %T: %v", err, err)
	}
}

func TestDraftFromJob_Standard(t *testing.T) {
	job := domain.CronJob{
		Enabled: true,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "0 3 * * *",
			Timezone:   "UTC",
		},
		Command: "/bin/backup",
	}
	draft := DraftFromJob(job)
	if draft.Minute != "0" || draft.Hour != "3" || draft.DayOfMonth != "*" || draft.Month != "*" || draft.DayOfWeek != "*" {
		t.Fatalf("unexpected fields: %+v", draft)
	}
	if draft.Timezone != "UTC" {
		t.Fatalf("expected UTC, got %q", draft.Timezone)
	}
	if draft.Command != "/bin/backup" {
		t.Fatalf("expected /bin/backup, got %q", draft.Command)
	}
}

func TestDraftFromJob_Descriptor(t *testing.T) {
	job := domain.CronJob{
		Enabled: true,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindDescriptor,
			Expression: "@daily",
		},
		Command: "/bin/cleanup",
	}
	draft := DraftFromJob(job)
	if draft.Descriptor != "@daily" {
		t.Fatalf("expected @daily, got %q", draft.Descriptor)
	}
}

func TestNewJobDraft(t *testing.T) {
	draft := NewJobDraft()
	if !draft.Enabled {
		t.Fatal("new draft should be enabled")
	}
	if draft.SchedKind != domain.ScheduleKindStandard {
		t.Fatalf("expected standard kind, got %q", draft.SchedKind)
	}
	if draft.Minute != "0" {
		t.Fatalf("expected minute=0, got %q", draft.Minute)
	}
}

func TestEditJob_PreservesTZPrefix(t *testing.T) {
	original := "TZ=Europe/London 0 9 * * MON-FRI /usr/local/bin/morning-check\n"
	svc, fc := setupEditSvc(original, true)

	jobs := svc.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	draft := DraftFromJob(jobs[0])
	if draft.TimezoneKey != "TZ" {
		t.Fatalf("expected TimezoneKey=TZ, got %q", draft.TimezoneKey)
	}
	if draft.Timezone != "Europe/London" {
		t.Fatalf("expected timezone Europe/London, got %q", draft.Timezone)
	}

	draft.Command = "/usr/local/bin/morning-check --verbose"

	if err := svc.EditJob(context.Background(), jobs[0].ID, draft); err != nil {
		t.Fatalf("EditJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	if !strings.Contains(applied, "TZ=Europe/London") {
		t.Fatalf("TZ= prefix not preserved, got: %q", applied)
	}
	if strings.Contains(applied, "CRON_TZ=") {
		t.Fatalf("should not have CRON_TZ= when original used TZ=, got: %q", applied)
	}
	if !strings.Contains(applied, "--verbose") {
		t.Fatalf("new command not found, got: %q", applied)
	}
}

func TestDraftFromJob_CronTZKey(t *testing.T) {
	job := domain.CronJob{
		Enabled: true,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "0 9 * * MON-FRI",
			Timezone:   "America/New_York",
		},
		Command: "/bin/check",
		RawLine: "CRON_TZ=America/New_York 0 9 * * MON-FRI /bin/check",
	}
	draft := DraftFromJob(job)
	if draft.TimezoneKey != "CRON_TZ" {
		t.Fatalf("expected CRON_TZ, got %q", draft.TimezoneKey)
	}
}

func TestDraftFromJob_NoTimezoneKey(t *testing.T) {
	job := domain.CronJob{
		Enabled: true,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "0 3 * * *",
		},
		Command: "/bin/backup",
		RawLine: "0 3 * * * /bin/backup",
	}
	draft := DraftFromJob(job)
	if draft.TimezoneKey != "" {
		t.Fatalf("expected empty TimezoneKey, got %q", draft.TimezoneKey)
	}
}

func TestDraftFromJob_DisabledWithTZ(t *testing.T) {
	job := domain.CronJob{
		Enabled: false,
		Schedule: domain.ScheduleSpec{
			Kind:       domain.ScheduleKindStandard,
			Expression: "0 9 * * MON-FRI",
			Timezone:   "Europe/London",
		},
		Command: "/bin/check",
		RawLine: "# [lazycron-disabled] TZ=Europe/London 0 9 * * MON-FRI /bin/check",
	}
	draft := DraftFromJob(job)
	if draft.TimezoneKey != "TZ" {
		t.Fatalf("expected TZ for disabled job, got %q", draft.TimezoneKey)
	}
}

func TestCreateJob_Disabled(t *testing.T) {
	svc, fc := setupEditSvc("", false)

	draft := domain.JobDraft{
		Enabled:    false,
		SchedKind:  domain.ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/backup",
	}

	if err := svc.CreateJob(context.Background(), draft); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	if !strings.Contains(applied, domain.DisabledMarker) {
		t.Fatalf("expected disabled marker in applied content, got: %q", applied)
	}
}
