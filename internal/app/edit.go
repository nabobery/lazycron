package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

// ValidationError wraps field-level validation errors for display in the editor.
type ValidationError struct {
	Fields []domain.FieldError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		msgs[i] = f.Error()
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// CreateJob appends a new job to the document and applies.
func (s *ApplyService) CreateJob(ctx context.Context, draft domain.JobDraft) error {
	if errs := domain.ValidateDraft(draft); len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}

	newDoc := s.cloneDoc()
	rawLine := draft.RawLine()

	// Add a blank separator if the document is non-empty and doesn't end with a blank line
	if len(newDoc.Lines) > 0 {
		last := newDoc.Lines[len(newDoc.Lines)-1]
		if last.Kind != domain.LineKindBlank {
			newDoc.Lines = append(newDoc.Lines, domain.CronLine{
				Index: len(newDoc.Lines),
				Raw:   "",
				Kind:  domain.LineKindBlank,
			})
		}
	}

	newDoc.Lines = append(newDoc.Lines, domain.CronLine{
		Index:   len(newDoc.Lines),
		Raw:     rawLine,
		Kind:    domain.LineKindJob,
		Enabled: draft.Enabled,
	})

	return s.applyDoc(ctx, newDoc)
}

// EditJob replaces the raw line for an existing job and applies.
func (s *ApplyService) EditJob(ctx context.Context, jobID string, draft domain.JobDraft) error {
	if errs := domain.ValidateDraft(draft); len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}

	lineIdx, err := s.findJobLineIndex(jobID)
	if err != nil {
		return err
	}

	newDoc := s.cloneDoc()
	line := &newDoc.Lines[lineIdx]

	rawLine := draft.RawLine()
	line.Raw = rawLine

	if draft.Enabled {
		line.Kind = domain.LineKindJob
		line.Enabled = true
	} else {
		line.Kind = domain.LineKindDisabled
		line.Enabled = false
	}

	return s.applyDoc(ctx, newDoc)
}

// DraftFromJob creates a JobDraft pre-populated from an existing CronJob.
func DraftFromJob(job domain.CronJob) domain.JobDraft {
	draft := domain.JobDraft{
		Enabled:     job.Enabled,
		SchedKind:   job.Schedule.Kind,
		Timezone:    job.Schedule.Timezone,
		TimezoneKey: detectTimezoneKey(job.RawLine),
		Command:     job.Command,
	}

	switch job.Schedule.Kind {
	case domain.ScheduleKindStandard:
		fields := strings.Fields(job.Schedule.Expression)
		if len(fields) == 5 {
			draft.Minute = fields[0]
			draft.Hour = fields[1]
			draft.DayOfMonth = fields[2]
			draft.Month = fields[3]
			draft.DayOfWeek = fields[4]
		}
	case domain.ScheduleKindDescriptor, domain.ScheduleKindReboot:
		draft.Descriptor = job.Schedule.Expression
	}

	return draft
}

// NewJobDraft returns a draft pre-populated with sensible defaults for creating a new job.
func NewJobDraft() domain.JobDraft {
	return domain.JobDraft{
		Enabled:    true,
		SchedKind:  domain.ScheduleKindStandard,
		Minute:     "0",
		Hour:       "*",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
		Command:    "",
	}
}

// FormatDraftErrors returns a human-readable summary of validation errors.
func FormatDraftErrors(errs []domain.FieldError) string {
	if len(errs) == 0 {
		return ""
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return strings.Join(msgs, "\n")
}

// detectTimezoneKey inspects a raw crontab line (possibly with disabled marker)
// and returns "TZ" if the line uses TZ=, "CRON_TZ" if it uses CRON_TZ=, or ""
// if no timezone prefix is present.
func detectTimezoneKey(rawLine string) string {
	line := rawLine
	line = strings.TrimPrefix(line, domain.DisabledMarker)
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "TZ=") {
		return "TZ"
	}
	if strings.HasPrefix(line, "CRON_TZ=") {
		return "CRON_TZ"
	}
	return ""
}
