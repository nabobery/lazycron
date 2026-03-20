package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/avinashchangrani/lazycron/internal/cronparse"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
)

type DriftError struct {
	Message string
}

func (e *DriftError) Error() string { return e.Message }

func IsDriftError(err error) bool {
	_, ok := err.(*DriftError)
	return ok
}

type ApplyService struct {
	client   crontab.Client
	source   domain.CronSource
	doc      domain.CronDocument
	jobs     []domain.CronJob
	issues   []domain.ValidationIssue
	baseline string // raw text at last load, for drift detection
}

func NewApplyService(client crontab.Client, source domain.CronSource) *ApplyService {
	return &ApplyService{
		client: client,
		source: source,
	}
}

func (s *ApplyService) Load(ctx context.Context) error {
	text, meta, err := s.client.Read(ctx)
	if err != nil {
		return fmt.Errorf("read crontab: %w", err)
	}

	if meta.User != "" {
		s.source.User = meta.User
	}

	if meta.IsEmpty {
		s.doc = domain.CronDocument{Source: s.source, Raw: ""}
		s.jobs = nil
		s.issues = nil
		s.baseline = ""
		return nil
	}

	doc, jobs, issues := cronparse.Parse(text, s.source)
	s.doc = doc
	s.jobs = jobs
	s.issues = issues
	s.baseline = text
	return nil
}

func (s *ApplyService) Jobs() []domain.CronJob {
	return s.jobs
}

func (s *ApplyService) Issues() []domain.ValidationIssue {
	return s.issues
}

func (s *ApplyService) Document() domain.CronDocument {
	return s.doc
}

func (s *ApplyService) Baseline() string {
	return s.baseline
}

func (s *ApplyService) Toggle(ctx context.Context, jobID string) error {
	lineIdx, err := s.findJobLineIndex(jobID)
	if err != nil {
		return err
	}

	newDoc := s.cloneDoc()
	line := &newDoc.Lines[lineIdx]

	switch line.Kind {
	case domain.LineKindDisabled:
		// Enable: strip the disabled marker, restoring the exact original line
		original := strings.TrimPrefix(line.Raw, domain.DisabledMarker)
		line.Raw = original
		line.Kind = domain.LineKindJob
		line.Enabled = true
		if line.Job != nil {
			line.Job.Enabled = true
		}
	case domain.LineKindJob:
		// Disable: prepend marker to the exact raw line
		line.Raw = domain.DisabledMarker + line.Raw
		line.Kind = domain.LineKindDisabled
		line.Enabled = false
		if line.Job != nil {
			line.Job.Enabled = false
		}
	default:
		return fmt.Errorf("cannot toggle line of kind %s", line.Kind)
	}

	return s.applyDoc(ctx, newDoc)
}

func (s *ApplyService) Delete(ctx context.Context, jobID string) error {
	lineIdx, err := s.findJobLineIndex(jobID)
	if err != nil {
		return err
	}

	newDoc := s.cloneDoc()
	newDoc.Lines = append(newDoc.Lines[:lineIdx], newDoc.Lines[lineIdx+1:]...)

	return s.applyDoc(ctx, newDoc)
}

func (s *ApplyService) applyDoc(ctx context.Context, newDoc domain.CronDocument) error {
	// Drift detection: re-read current crontab and compare to baseline
	currentText, meta, err := s.client.Read(ctx)
	if err != nil {
		return fmt.Errorf("drift check read: %w", err)
	}

	currentRaw := ""
	if !meta.IsEmpty {
		currentRaw = currentText
	}

	if currentRaw != s.baseline {
		return &DriftError{
			Message: "crontab has been modified externally since last load",
		}
	}

	rendered := cronparse.Render(newDoc)

	_, err = s.client.Apply(ctx, rendered)
	if err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	// Reload from source of truth
	return s.Load(ctx)
}

func (s *ApplyService) findJobLineIndex(jobID string) (int, error) {
	for _, job := range s.jobs {
		if job.ID == jobID {
			return job.LineIndex, nil
		}
	}
	return -1, fmt.Errorf("job %q not found", jobID)
}

func (s *ApplyService) cloneDoc() domain.CronDocument {
	lines := make([]domain.CronLine, len(s.doc.Lines))
	copy(lines, s.doc.Lines)
	return domain.CronDocument{
		Source: s.doc.Source,
		Lines:  lines,
		Raw:    s.doc.Raw,
	}
}
