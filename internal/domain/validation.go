package domain

import (
	"fmt"
	"strings"
	"time"

	cron "github.com/robfig/cron/v3"
)

var fieldParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// FieldError represents a validation error for a specific form field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// JobDraft captures the structured input for creating or editing a cron job.
type JobDraft struct {
	Enabled     bool
	SchedKind   ScheduleKind
	Minute      string
	Hour        string
	DayOfMonth  string
	Month       string
	DayOfWeek   string
	Descriptor  string // for @daily, @hourly, @every 5m, @reboot
	Timezone    string
	TimezoneKey string // "CRON_TZ" or "TZ"; defaults to "CRON_TZ" if empty
	Command     string
}

// Expression builds the full cron expression from the draft fields.
func (d JobDraft) Expression() string {
	switch d.SchedKind {
	case ScheduleKindDescriptor, ScheduleKindReboot:
		return d.Descriptor
	default:
		return strings.Join([]string{d.Minute, d.Hour, d.DayOfMonth, d.Month, d.DayOfWeek}, " ")
	}
}

// RawLine builds the full crontab line from the draft.
func (d JobDraft) RawLine() string {
	var sb strings.Builder
	if d.Timezone != "" {
		key := d.TimezoneKey
		if key == "" {
			key = "CRON_TZ"
		}
		sb.WriteString(key)
		sb.WriteByte('=')
		sb.WriteString(d.Timezone)
		sb.WriteByte(' ')
	}
	sb.WriteString(d.Expression())
	sb.WriteByte(' ')
	sb.WriteString(d.Command)
	raw := sb.String()
	if !d.Enabled {
		raw = DisabledMarker + raw
	}
	return raw
}

// ValidateDraft checks all fields of a JobDraft and returns any errors.
// Returns nil if the draft is valid.
func ValidateDraft(draft JobDraft) []FieldError {
	var errs []FieldError

	switch draft.SchedKind {
	case ScheduleKindStandard:
		errs = append(errs, validateStandardFields(draft)...)
	case ScheduleKindDescriptor:
		errs = append(errs, validateDescriptor(draft.Descriptor)...)
	case ScheduleKindReboot:
		if !strings.EqualFold(draft.Descriptor, "@reboot") {
			errs = append(errs, FieldError{Field: "descriptor", Message: "reboot kind requires @reboot"})
		}
	default:
		errs = append(errs, FieldError{Field: "schedule_kind", Message: fmt.Sprintf("unknown schedule kind %q", draft.SchedKind)})
	}

	if draft.Timezone != "" {
		if _, err := time.LoadLocation(draft.Timezone); err != nil {
			errs = append(errs, FieldError{Field: "timezone", Message: fmt.Sprintf("invalid timezone %q", draft.Timezone)})
		}
	}

	cmd := strings.TrimSpace(draft.Command)
	if cmd == "" {
		errs = append(errs, FieldError{Field: "command", Message: "command is required"})
	} else if containsControlChar(cmd) {
		errs = append(errs, FieldError{Field: "command", Message: "command must not contain newlines or control characters"})
	}

	// Full expression validation for non-reboot schedules
	if draft.SchedKind != ScheduleKindReboot && len(errs) == 0 {
		expr := draft.Expression()
		if _, err := fieldParser.Parse(expr); err != nil {
			errs = append(errs, FieldError{Field: "expression", Message: fmt.Sprintf("invalid expression %q: %v", expr, err)})
		}
	}

	return errs
}

func validateStandardFields(d JobDraft) []FieldError {
	var errs []FieldError
	if err := validateCronField("minute", d.Minute, 0, 59); err != nil {
		errs = append(errs, *err)
	}
	if err := validateCronField("hour", d.Hour, 0, 23); err != nil {
		errs = append(errs, *err)
	}
	if err := validateCronField("day_of_month", d.DayOfMonth, 1, 31); err != nil {
		errs = append(errs, *err)
	}
	if err := validateCronField("month", d.Month, 1, 12); err != nil {
		errs = append(errs, *err)
	}
	if err := validateCronField("day_of_week", d.DayOfWeek, 0, 7); err != nil {
		errs = append(errs, *err)
	}
	return errs
}

func validateCronField(name, value string, minVal, maxVal int) *FieldError {
	v := strings.TrimSpace(value)
	if v == "" {
		return &FieldError{Field: name, Message: "field is required"}
	}
	// Allow *, */N, N, N-M, N,M, and named months/days
	for _, ch := range v {
		if !isCronFieldChar(ch) {
			return &FieldError{Field: name, Message: fmt.Sprintf("invalid character %q in %s", ch, name)}
		}
	}
	return nil
}

func isCronFieldChar(ch rune) bool {
	if ch >= '0' && ch <= '9' {
		return true
	}
	if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
		return true
	}
	switch ch {
	case '*', '/', '-', ',', '?', '#', 'L', 'W':
		return true
	}
	return false
}

func containsControlChar(s string) bool {
	for _, ch := range s {
		if ch == '\n' || ch == '\r' || ch == 0 {
			return true
		}
	}
	return false
}

func validateDescriptor(desc string) []FieldError {
	d := strings.TrimSpace(desc)
	if d == "" {
		return []FieldError{{Field: "descriptor", Message: "descriptor is required"}}
	}
	if !strings.HasPrefix(d, "@") {
		return []FieldError{{Field: "descriptor", Message: "descriptor must start with @"}}
	}

	lower := strings.ToLower(d)
	switch {
	case lower == "@yearly", lower == "@annually",
		lower == "@monthly", lower == "@weekly",
		lower == "@daily", lower == "@midnight",
		lower == "@hourly":
		return nil
	case strings.HasPrefix(lower, "@every "):
		dur := strings.TrimPrefix(lower, "@every ")
		if strings.TrimSpace(dur) == "" {
			return []FieldError{{Field: "descriptor", Message: "@every requires a duration (e.g. @every 5m)"}}
		}
		return nil
	default:
		return []FieldError{{Field: "descriptor", Message: fmt.Sprintf("unknown descriptor %q", d)}}
	}
}
