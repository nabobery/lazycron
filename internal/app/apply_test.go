package app

import (
	"context"
	"strings"
	"testing"

	"github.com/avinashchangrani/lazycron/internal/cronparse"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
)

var testSource = domain.CronSource{
	Kind: domain.SourceKindUserCrontab,
	Path: "crontab://current-user",
	User: "testuser",
}

func TestToggleDisable(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	jobs := svc.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Toggle first job off
	err = svc.Toggle(ctx, jobs[0].ID)
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}

	// Verify the applied content has the disabled marker
	applied := fc.ApplyCalls[0]
	if !strings.Contains(applied, domain.DisabledMarker) {
		t.Fatalf("expected disabled marker in applied content:\n%s", applied)
	}

	// Second job should still be there
	if !strings.Contains(applied, "@daily /usr/local/bin/cleanup") {
		t.Fatal("second job should be preserved")
	}
}

func TestToggleEnable(t *testing.T) {
	content := "# [lazycron-disabled] 0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	jobs := svc.Jobs()
	disabledJob := jobs[0]
	if disabledJob.Enabled {
		t.Fatal("first job should be disabled")
	}

	err = svc.Toggle(ctx, disabledJob.ID)
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	if strings.Contains(applied, domain.DisabledMarker) {
		t.Fatalf("disabled marker should be removed:\n%s", applied)
	}
	if !strings.Contains(applied, "0 3 * * * /usr/local/bin/backup-db") {
		t.Fatal("original job line should be restored")
	}
}

func TestDeleteJob(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	jobs := svc.Jobs()
	err = svc.Delete(ctx, jobs[0].ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	if strings.Contains(applied, "backup-db") {
		t.Fatal("deleted job should not be in applied content")
	}
	if !strings.Contains(applied, "@daily /usr/local/bin/cleanup") {
		t.Fatal("other job should be preserved")
	}
}

func TestDeleteOnlyTargetedEntry(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n0 4 * * * /usr/local/bin/other\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	// Delete the middle job
	err := svc.Delete(ctx, jobs[1].ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	lines := strings.Split(strings.TrimRight(applied, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines after delete, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(applied, "backup-db") {
		t.Fatal("first job should be preserved")
	}
	if !strings.Contains(applied, "cleanup") {
		t.Fatal("third job should be preserved")
	}
}

func TestApplyFailurePreservesBaseline(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	fc := crontab.NewFakeClient(content, true)
	fc.ApplyErr = crontab.NewFailingClient("bad syntax").Err
	fc.ApplyStderr = "errors in crontab file"
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	err := svc.Toggle(ctx, jobs[0].ID)
	if err == nil {
		t.Fatal("expected error from apply failure")
	}

	// Baseline should be unchanged
	text, _, _ := fc.Read(ctx)
	if text != content {
		t.Fatalf("baseline should be preserved, got %q", text)
	}
}

func TestDriftDetection(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	// Simulate external modification
	fc.Content = "0 5 * * * /usr/local/bin/changed\n"

	jobs := svc.Jobs()
	err := svc.Toggle(ctx, jobs[0].ID)
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !IsDriftError(err) {
		t.Fatalf("expected drift error, got: %v", err)
	}
}

func TestLoadEmptyCrontab(t *testing.T) {
	fc := crontab.NewFakeClient("", false)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("load should succeed for empty crontab: %v", err)
	}
	if len(svc.Jobs()) != 0 {
		t.Fatal("expected 0 jobs for empty crontab")
	}
}

func TestReloadPreservesState(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	if len(svc.Jobs()) != 2 {
		t.Fatal("expected 2 jobs")
	}

	// Reload
	err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if len(svc.Jobs()) != 2 {
		t.Fatal("expected 2 jobs after reload")
	}

	_ = cronparse.Parse // ensure import used
}

func TestTogglePreservesExactRawLine(t *testing.T) {
	original := "  0 3 * * *   /usr/local/bin/backup-db  \n"
	fc := crontab.NewFakeClient(original, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// Toggle disable
	err := svc.Toggle(ctx, jobs[0].ID)
	if err != nil {
		t.Fatalf("toggle disable failed: %v", err)
	}

	applied := fc.ApplyCalls[0]
	expectedDisabled := domain.DisabledMarker + "  0 3 * * *   /usr/local/bin/backup-db  "
	if !strings.Contains(applied, expectedDisabled) {
		t.Fatalf("disabled line should preserve original whitespace.\nexpected: %q\ngot applied:\n%s", expectedDisabled, applied)
	}

	// Toggle enable again
	jobs = svc.Jobs()
	var disabledJob *domain.CronJob
	for i := range jobs {
		if !jobs[i].Enabled {
			disabledJob = &jobs[i]
			break
		}
	}
	if disabledJob == nil {
		t.Fatal("expected a disabled job after toggle")
	}

	err = svc.Toggle(ctx, disabledJob.ID)
	if err != nil {
		t.Fatalf("toggle enable failed: %v", err)
	}

	applied = fc.ApplyCalls[len(fc.ApplyCalls)-1]
	if !strings.Contains(applied, "  0 3 * * *   /usr/local/bin/backup-db  ") {
		t.Fatalf("re-enabled line should restore exact original.\ngot applied:\n%s", applied)
	}
}

func TestCloneDocDoesNotAliasPointers(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n"
	fc := crontab.NewFakeClient(content, true)
	svc := NewApplyService(fc, testSource)

	ctx := context.Background()
	_ = svc.Load(ctx)

	originalJobs := svc.Jobs()
	originalEnabled := originalJobs[0].Enabled

	// Toggle first job — should not mutate the original jobs slice
	_ = svc.Toggle(ctx, originalJobs[0].ID)

	// After toggle + reload, check that the original slice wasn't mutated
	// (This tests that cloneDoc deep-copies Job pointers)
	if originalJobs[0].Enabled != originalEnabled {
		t.Fatal("original jobs slice should not be mutated by toggle")
	}
}

func TestLoadPlumbsUserFromReadMeta(t *testing.T) {
	content := "0 3 * * * /usr/local/bin/backup-db\n"
	fc := crontab.NewFakeClient(content, true)
	source := domain.CronSource{
		Kind: domain.SourceKindUserCrontab,
		Path: "crontab://current-user",
	}
	svc := NewApplyService(fc, source)

	ctx := context.Background()
	_ = svc.Load(ctx)

	jobs := svc.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Source.User != "testuser" {
		t.Errorf("expected source user 'testuser' from ReadMeta, got %q", jobs[0].Source.User)
	}
}
