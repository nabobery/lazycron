package domain

import (
	"testing"
)

func TestValidateDraft_ValidStandard(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/usr/local/bin/backup-db",
	}
	errs := ValidateDraft(draft)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDraft_ValidDescriptor(t *testing.T) {
	tests := []struct {
		name string
		desc string
	}{
		{"daily", "@daily"},
		{"hourly", "@hourly"},
		{"weekly", "@weekly"},
		{"monthly", "@monthly"},
		{"yearly", "@yearly"},
		{"annually", "@annually"},
		{"midnight", "@midnight"},
		{"every_5m", "@every 5m"},
		{"every_30s", "@every 30s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := JobDraft{
				Enabled:    true,
				SchedKind:  ScheduleKindDescriptor,
				Descriptor: tt.desc,
				Command:    "/usr/local/bin/test",
			}
			errs := ValidateDraft(draft)
			if len(errs) != 0 {
				t.Fatalf("expected no errors for %q, got %v", tt.desc, errs)
			}
		})
	}
}

func TestValidateDraft_ValidReboot(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindReboot,
		Descriptor: "@reboot",
		Command:    "/usr/local/bin/start-agent",
	}
	errs := ValidateDraft(draft)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDraft_EmptyCommand(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for empty command")
	}
	found := false
	for _, e := range errs {
		if e.Field == "command" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected command error, got %v", errs)
	}
}

func TestValidateDraft_EmptyMinute(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for empty minute")
	}
	if errs[0].Field != "minute" {
		t.Fatalf("expected minute error, got %v", errs)
	}
}

func TestValidateDraft_InvalidCharInField(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0;rm -rf",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid char in minute")
	}
}

func TestValidateDraft_InvalidTimezone(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Timezone:   "Not/A/Real/Zone",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid timezone")
	}
	found := false
	for _, e := range errs {
		if e.Field == "timezone" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected timezone error, got %v", errs)
	}
}

func TestValidateDraft_ValidTimezone(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "9",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "MON-FRI",
		Timezone:   "America/New_York",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateDraft_UnknownDescriptor(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindDescriptor,
		Descriptor: "@bogus",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown descriptor")
	}
}

func TestValidateDraft_EveryWithoutDuration(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindDescriptor,
		Descriptor: "@every ",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for @every without duration")
	}
}

func TestJobDraft_Expression(t *testing.T) {
	tests := []struct {
		name  string
		draft JobDraft
		want  string
	}{
		{
			"standard",
			JobDraft{SchedKind: ScheduleKindStandard, Minute: "0", Hour: "3", DayOfMonth: "*", Month: "*", DayOfWeek: "*"},
			"0 3 * * *",
		},
		{
			"descriptor",
			JobDraft{SchedKind: ScheduleKindDescriptor, Descriptor: "@daily"},
			"@daily",
		},
		{
			"reboot",
			JobDraft{SchedKind: ScheduleKindReboot, Descriptor: "@reboot"},
			"@reboot",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.draft.Expression()
			if got != tt.want {
				t.Errorf("Expression() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJobDraft_RawLine(t *testing.T) {
	tests := []struct {
		name  string
		draft JobDraft
		want  string
	}{
		{
			"standard enabled",
			JobDraft{Enabled: true, SchedKind: ScheduleKindStandard, Minute: "0", Hour: "3", DayOfMonth: "*", Month: "*", DayOfWeek: "*", Command: "/bin/backup"},
			"0 3 * * * /bin/backup",
		},
		{
			"standard disabled",
			JobDraft{Enabled: false, SchedKind: ScheduleKindStandard, Minute: "0", Hour: "3", DayOfMonth: "*", Month: "*", DayOfWeek: "*", Command: "/bin/backup"},
			"# [lazycron-disabled] 0 3 * * * /bin/backup",
		},
		{
			"with timezone",
			JobDraft{Enabled: true, SchedKind: ScheduleKindStandard, Minute: "0", Hour: "9", DayOfMonth: "*", Month: "*", DayOfWeek: "MON-FRI", Timezone: "America/New_York", Command: "/bin/check"},
			"CRON_TZ=America/New_York 0 9 * * MON-FRI /bin/check",
		},
		{
			"with TZ= key",
			JobDraft{Enabled: true, SchedKind: ScheduleKindStandard, Minute: "0", Hour: "9", DayOfMonth: "*", Month: "*", DayOfWeek: "MON-FRI", Timezone: "Europe/London", TimezoneKey: "TZ", Command: "/bin/check"},
			"TZ=Europe/London 0 9 * * MON-FRI /bin/check",
		},
		{
			"descriptor",
			JobDraft{Enabled: true, SchedKind: ScheduleKindDescriptor, Descriptor: "@daily", Command: "/bin/cleanup"},
			"@daily /bin/cleanup",
		},
		{
			"reboot",
			JobDraft{Enabled: true, SchedKind: ScheduleKindReboot, Descriptor: "@reboot", Command: "/bin/start"},
			"@reboot /bin/start",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.draft.RawLine()
			if got != tt.want {
				t.Errorf("RawLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateDraft_CommandWithNewline(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/echo hello\n0 * * * * /bin/evil",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for command with newline")
	}
	found := false
	for _, e := range errs {
		if e.Field == "command" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected command error, got %v", errs)
	}
}

func TestValidateDraft_CommandWithCarriageReturn(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/echo\r/bin/evil",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for command with carriage return")
	}
}

func TestValidateDraft_CommandWithNUL(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "0",
		Hour:       "3",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "/bin/echo\x00evil",
	}
	errs := ValidateDraft(draft)
	if len(errs) == 0 {
		t.Fatal("expected error for command with NUL byte")
	}
}

func TestValidateDraft_StandardWithRanges(t *testing.T) {
	draft := JobDraft{
		Enabled:    true,
		SchedKind:  ScheduleKindStandard,
		Minute:     "*/15",
		Hour:       "9-17",
		DayOfMonth: "1,15",
		Month:      "*",
		DayOfWeek:  "MON-FRI",
		Command:    "/bin/echo",
	}
	errs := ValidateDraft(draft)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for ranges/steps, got %v", errs)
	}
}
