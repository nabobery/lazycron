package tui

import (
	"github.com/nabobery/lazycron/internal/domain"
	"github.com/nabobery/lazycron/internal/platform/cronlogs"
)

type appState int

const (
	stateReady appState = iota
	stateLoading
	stateFiltering
	stateConfirmDelete
	stateRunning
	stateApplying
	stateDriftDetected
	stateEditing
	stateCreating
	stateConfirmDiscard
)

type focusedPane int

const (
	paneJobs focusedPane = iota
	paneDetails
	paneLogs
)

type banner struct {
	message string
	isError bool
}

type loadResultMsg struct {
	jobs   []domain.CronJob
	issues []domain.ValidationIssue
	err    error
}

type applyResultMsg struct {
	err error
}

type runResultMsg struct {
	record domain.RunRecord
	err    error
}

type sysLogResultMsg struct {
	result cronlogs.Result
	err    error
}

type editorMode int

const (
	editorModeCreate editorMode = iota
	editorModeEdit
)

type editorField int

const (
	fieldSchedKind editorField = iota
	fieldMinute
	fieldHour
	fieldDayOfMonth
	fieldMonth
	fieldDayOfWeek
	fieldDescriptor
	fieldTimezone
	fieldCommand
	fieldCount // sentinel
)

func (f editorField) label() string {
	switch f {
	case fieldSchedKind:
		return "Schedule Type"
	case fieldMinute:
		return "Minute"
	case fieldHour:
		return "Hour"
	case fieldDayOfMonth:
		return "Day of Month"
	case fieldMonth:
		return "Month"
	case fieldDayOfWeek:
		return "Day of Week"
	case fieldDescriptor:
		return "Descriptor"
	case fieldTimezone:
		return "Timezone"
	case fieldCommand:
		return "Command"
	default:
		return ""
	}
}

type editorState struct {
	mode          editorMode
	draft         domain.JobDraft
	originalDraft domain.JobDraft // snapshot at open, for dirty comparison
	targetJobID   string
	focusField    editorField
	fieldBuf      string // current editing buffer for the focused field
	fieldErrs     map[editorField]string
}
