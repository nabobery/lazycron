package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/domain"
)

func (m *Model) openCreateEditor() {
	draft := app.NewJobDraft()
	m.editor = &editorState{
		mode:          editorModeCreate,
		draft:         draft,
		originalDraft: draft,
		focusField:    fieldMinute,
		fieldBuf:      draft.Minute,
		fieldErrs:     make(map[editorField]string),
	}
	m.state = stateCreating
}

func (m *Model) openEditEditor(job domain.CronJob) {
	draft := app.DraftFromJob(job)
	es := &editorState{
		mode:          editorModeEdit,
		draft:         draft,
		originalDraft: draft,
		targetJobID:   job.ID,
		fieldErrs:     make(map[editorField]string),
	}

	switch draft.SchedKind {
	case domain.ScheduleKindStandard:
		es.focusField = fieldMinute
		es.fieldBuf = draft.Minute
	case domain.ScheduleKindDescriptor, domain.ScheduleKindReboot:
		es.focusField = fieldDescriptor
		es.fieldBuf = draft.Descriptor
	}

	m.editor = es
	m.state = stateEditing
}

func (m *Model) editorFieldValue(f editorField) string {
	d := m.editor.draft
	switch f {
	case fieldSchedKind:
		return string(d.SchedKind)
	case fieldMinute:
		return d.Minute
	case fieldHour:
		return d.Hour
	case fieldDayOfMonth:
		return d.DayOfMonth
	case fieldMonth:
		return d.Month
	case fieldDayOfWeek:
		return d.DayOfWeek
	case fieldDescriptor:
		return d.Descriptor
	case fieldTimezone:
		return d.Timezone
	case fieldCommand:
		return d.Command
	}
	return ""
}

func (m *Model) setEditorFieldValue(f editorField, val string) {
	d := &m.editor.draft
	switch f {
	case fieldMinute:
		d.Minute = val
	case fieldHour:
		d.Hour = val
	case fieldDayOfMonth:
		d.DayOfMonth = val
	case fieldMonth:
		d.Month = val
	case fieldDayOfWeek:
		d.DayOfWeek = val
	case fieldDescriptor:
		d.Descriptor = val
	case fieldTimezone:
		d.Timezone = val
	case fieldCommand:
		d.Command = val
	}
}

func (m *Model) commitFieldBuf() {
	if m.editor == nil {
		return
	}
	m.setEditorFieldValue(m.editor.focusField, m.editor.fieldBuf)
}

// isDirty returns true if the editor draft (including the current fieldBuf)
// differs from the original draft at open time. Pure — no state mutation.
func (m Model) isDirty() bool {
	if m.editor == nil {
		return false
	}
	return m.previewDraft() != m.editor.originalDraft
}

// previewDraft returns a copy of the editor draft with the current fieldBuf
// applied to the focused field, without mutating any state. Safe to call from View().
func (m Model) previewDraft() domain.JobDraft {
	if m.editor == nil {
		return domain.JobDraft{}
	}
	d := m.editor.draft
	applyFieldToDraft(&d, m.editor.focusField, m.editor.fieldBuf)
	return d
}

func applyFieldToDraft(d *domain.JobDraft, f editorField, val string) {
	switch f {
	case fieldMinute:
		d.Minute = val
	case fieldHour:
		d.Hour = val
	case fieldDayOfMonth:
		d.DayOfMonth = val
	case fieldMonth:
		d.Month = val
	case fieldDayOfWeek:
		d.DayOfWeek = val
	case fieldDescriptor:
		d.Descriptor = val
	case fieldTimezone:
		d.Timezone = val
	case fieldCommand:
		d.Command = val
	}
}

func (m *Model) loadFieldBuf(f editorField) {
	m.editor.focusField = f
	m.editor.fieldBuf = m.editorFieldValue(f)
}

func (m *Model) editorVisibleFields() []editorField {
	if m.editor == nil {
		return nil
	}
	fields := []editorField{fieldSchedKind}
	switch m.editor.draft.SchedKind {
	case domain.ScheduleKindStandard:
		fields = append(fields, fieldMinute, fieldHour, fieldDayOfMonth, fieldMonth, fieldDayOfWeek)
	case domain.ScheduleKindDescriptor, domain.ScheduleKindReboot:
		fields = append(fields, fieldDescriptor)
	}
	fields = append(fields, fieldTimezone, fieldCommand)
	return fields
}

func (m *Model) editorNextField() {
	fields := m.editorVisibleFields()
	for i, f := range fields {
		if f == m.editor.focusField && i+1 < len(fields) {
			m.commitFieldBuf()
			m.loadFieldBuf(fields[i+1])
			return
		}
	}
}

func (m *Model) editorPrevField() {
	fields := m.editorVisibleFields()
	for i, f := range fields {
		if f == m.editor.focusField && i > 0 {
			m.commitFieldBuf()
			m.loadFieldBuf(fields[i-1])
			return
		}
	}
}

func (m *Model) cycleSchedKind() {
	d := &m.editor.draft
	switch d.SchedKind {
	case domain.ScheduleKindStandard:
		d.SchedKind = domain.ScheduleKindDescriptor
		d.Descriptor = "@daily"
	case domain.ScheduleKindDescriptor:
		d.SchedKind = domain.ScheduleKindReboot
		d.Descriptor = "@reboot"
	case domain.ScheduleKindReboot:
		d.SchedKind = domain.ScheduleKindStandard
	}
	// Re-focus to first editable field after kind
	fields := m.editorVisibleFields()
	if len(fields) > 1 {
		m.loadFieldBuf(fields[1])
	}
}

func (m *Model) validateEditor() []domain.FieldError {
	m.commitFieldBuf()
	return domain.ValidateDraft(m.editor.draft)
}

func (m Model) editorSaveCmd() tea.Cmd {
	svc := m.applySvc
	es := m.editor
	draft := es.draft
	mode := es.mode
	jobID := es.targetJobID

	return func() tea.Msg {
		var err error
		if mode == editorModeCreate {
			err = svc.CreateJob(context.Background(), draft)
		} else {
			err = svc.EditJob(context.Background(), jobID, draft)
		}
		return applyResultMsg{err: err}
	}
}

func (m Model) renderEditor(width, height int) string {
	if m.editor == nil {
		return ""
	}

	es := m.editor
	modalW := min(width-4, 76)
	if modalW < 30 {
		modalW = 30
	}
	innerW := modalW - 4

	var lines []string

	// Title
	title := "Create New Job"
	if es.mode == editorModeEdit {
		title = "Edit Job"
	}
	lines = append(lines, editorTitleStyle.Width(innerW).Render(title))
	lines = append(lines, "")

	// Fields
	fields := m.editorVisibleFields()
	for _, f := range fields {
		isFocused := f == es.focusField
		label := f.label()

		var val string
		if isFocused && f != fieldSchedKind {
			val = es.fieldBuf + "█"
		} else if f == fieldSchedKind {
			val = schedKindDisplay(es.draft.SchedKind)
			if isFocused {
				val += dimStyle.Render("  ←/→ to change")
			}
		} else {
			val = m.editorFieldValue(f)
		}

		if val == "" && !isFocused {
			val = dimStyle.Render("(empty)")
		}

		labelStr := dimStyle.Render(label + ": ")
		if isFocused {
			labelStr = editorFocusStyle.Render("▸ " + label + ": ")
		}

		line := labelStr + val
		lines = append(lines, truncate(line, innerW))

		if errMsg, ok := es.fieldErrs[f]; ok {
			lines = append(lines, "  "+editorErrorStyle.Render(errMsg))
		}
	}

	// Preview (pure — no state mutation)
	lines = append(lines, "")
	lines = append(lines, editorPreviewHeader.Render("Preview"))

	preview := m.previewDraft()
	expr := preview.Expression()
	rawLine := preview.RawLine()
	lines = append(lines, dimStyle.Render("  "+truncate(rawLine, innerW-2)))

	desc := m.scheduleSvc.Describe(domain.ScheduleSpec{
		Kind:       preview.SchedKind,
		Expression: expr,
		Timezone:   preview.Timezone,
	})
	lines = append(lines, "  "+desc)

	if preview.SchedKind != domain.ScheduleKindReboot {
		nextRuns, err := m.scheduleSvc.NextRuns(domain.ScheduleSpec{
			Kind:       preview.SchedKind,
			Expression: expr,
			Timezone:   preview.Timezone,
		}, time.Now(), 3)
		if err == nil && len(nextRuns) > 0 {
			lines = append(lines, dimStyle.Render("  Next runs:"))
			for _, run := range nextRuns {
				lines = append(lines, dimStyle.Render(fmt.Sprintf("    %s", run.Local().Format("Mon Jan 2 15:04:05"))))
			}
		}
	}

	// Help
	lines = append(lines, "")
	helpText := "tab/shift+tab:fields  enter:save  esc:cancel"
	if m.state == stateConfirmDiscard {
		helpText = "y:discard  n:keep editing"
	}
	lines = append(lines, helpStyle.Render(helpText))

	if m.state == stateConfirmDiscard {
		lines = append(lines, warningStyle.Render("Discard unsaved changes?"))
	}

	content := strings.Join(lines, "\n")

	modal := editorModalStyle.
		Width(modalW).
		Render(content)

	// Center the modal
	padTop := max(0, (height-lipgloss.Height(modal))/2)
	padLeft := max(0, (width-lipgloss.Width(modal))/2)

	return strings.Repeat("\n", padTop) + strings.Repeat(" ", padLeft) + modal
}

func schedKindDisplay(k domain.ScheduleKind) string {
	switch k {
	case domain.ScheduleKindStandard:
		return "Standard (5-field)"
	case domain.ScheduleKindDescriptor:
		return "Descriptor (@daily, @every...)"
	case domain.ScheduleKindReboot:
		return "@reboot"
	}
	return string(k)
}

var (
	editorModalStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	editorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62")).
				Padding(0, 1)

	editorFocusStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("12"))

	editorErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9"))

	editorPreviewHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("10"))
)
