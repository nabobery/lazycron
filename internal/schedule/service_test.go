package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

var fixedNow = time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)

func TestNextRuns_Standard(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindStandard,
		Expression: "0 3 * * *",
	}

	runs, err := svc.NextRuns(spec, fixedNow, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// Should be 03:00 on March 21, 22, 23
	for i, run := range runs {
		if run.Hour() != 3 || run.Minute() != 0 {
			t.Errorf("run %d: expected 03:00, got %02d:%02d", i, run.Hour(), run.Minute())
		}
		expectedDay := 21 + i
		if run.Day() != expectedDay {
			t.Errorf("run %d: expected day %d, got %d", i, expectedDay, run.Day())
		}
	}

	// Runs should be in ascending order
	for i := 1; i < len(runs); i++ {
		if !runs[i].After(runs[i-1]) {
			t.Errorf("run %d should be after run %d", i, i-1)
		}
	}
}

func TestNextRuns_Descriptor(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindDescriptor,
		Expression: "@daily",
	}

	runs, err := svc.NextRuns(spec, fixedNow, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// @daily = midnight
	for i, run := range runs {
		if run.Hour() != 0 || run.Minute() != 0 {
			t.Errorf("run %d: expected 00:00, got %02d:%02d", i, run.Hour(), run.Minute())
		}
	}
}

func TestNextRuns_Hourly(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindDescriptor,
		Expression: "@hourly",
	}

	runs, err := svc.NextRuns(spec, fixedNow, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// @hourly = top of each hour, next should be 13:00, 14:00, 15:00
	expected := []int{13, 14, 15}
	for i, run := range runs {
		if run.Hour() != expected[i] {
			t.Errorf("run %d: expected hour %d, got %d", i, expected[i], run.Hour())
		}
	}
}

func TestNextRuns_Reboot(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindReboot,
		Expression: "@reboot",
	}

	runs, err := svc.NextRuns(spec, fixedNow, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs for @reboot, got %d", len(runs))
	}
}

func TestNextRuns_Invalid(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindStandard,
		Expression: "not valid",
	}

	_, err := svc.NextRuns(spec, fixedNow, 3)
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestDescribe_Standard(t *testing.T) {
	svc := NewService()
	tests := []struct {
		expr     string
		contains string
	}{
		{"0 3 * * *", "03:00"},
		{"*/15 * * * *", "15 minutes"},
		{"@daily", "midnight"},
		{"@hourly", "hour"},
		{"@reboot", "reboot"},
		{"@every 5m", "5m"},
		{"0 9 * * MON-FRI", "09:00"},
		{"30 4 * * 0", "04:30"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			kind := domain.ScheduleKindStandard
			if tt.expr[0] == '@' {
				if tt.expr == "@reboot" {
					kind = domain.ScheduleKindReboot
				} else {
					kind = domain.ScheduleKindDescriptor
				}
			}
			desc := svc.Describe(domain.ScheduleSpec{Kind: kind, Expression: tt.expr})
			if desc == "" {
				t.Fatal("description should not be empty")
			}
			if !strings.Contains(strings.ToLower(desc), strings.ToLower(tt.contains)) {
				t.Errorf("expected description to contain %q, got %q", tt.contains, desc)
			}
		})
	}
}

func TestNextRuns_WithTimezone(t *testing.T) {
	svc := NewService()
	spec := domain.ScheduleSpec{
		Kind:       domain.ScheduleKindStandard,
		Expression: "0 9 * * MON-FRI",
		Timezone:   "America/New_York",
	}

	runs, err := svc.NextRuns(spec, fixedNow, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	loc, _ := time.LoadLocation("America/New_York")
	for i, run := range runs {
		inTZ := run.In(loc)
		if inTZ.Hour() != 9 {
			t.Errorf("run %d: expected 09:00 in New York, got %02d:%02d", i, inTZ.Hour(), inTZ.Minute())
		}
	}
}
