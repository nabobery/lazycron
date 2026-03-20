package tui

import (
	"github.com/avinashchangrani/lazycron/internal/domain"
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
