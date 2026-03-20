package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/runner"
	"github.com/avinashchangrani/lazycron/internal/schedule"
)

type Model struct {
	applySvc    *app.ApplyService
	scheduleSvc *schedule.Service
	runner      *runner.Runner
	cancelRun   context.CancelFunc

	width  int
	height int

	state       appState
	focused     focusedPane
	jobs        []domain.CronJob
	issues      []domain.ValidationIssue
	filteredIdx []int
	selected    int
	filterText  string
	filtering   bool

	logs      []domain.RunRecord
	logScroll int
	detScroll int

	bannerMsg *banner
	err       error

	editor *editorState
}

func NewModel(applySvc *app.ApplyService, scheduleSvc *schedule.Service, r *runner.Runner) Model {
	return Model{
		applySvc:    applySvc,
		scheduleSvc: scheduleSvc,
		runner:      r,
		state:       stateLoading,
		focused:     paneJobs,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadCmd()
}

func (m Model) loadCmd() tea.Cmd {
	svc := m.applySvc
	return func() tea.Msg {
		err := svc.Load(context.Background())
		if err != nil {
			return loadResultMsg{err: err}
		}
		return loadResultMsg{
			jobs:   svc.Jobs(),
			issues: svc.Issues(),
		}
	}
}

func (m *Model) rebuildFilter() {
	if m.filterText == "" {
		m.filteredIdx = nil
		for i := range m.jobs {
			m.filteredIdx = append(m.filteredIdx, i)
		}
	} else {
		m.filteredIdx = nil
		for i, job := range m.jobs {
			if matchesFilter(job, m.filterText) {
				m.filteredIdx = append(m.filteredIdx, i)
			}
		}
	}
	if m.selected >= len(m.filteredIdx) {
		m.selected = max(0, len(m.filteredIdx)-1)
	}
}

func matchesFilter(job domain.CronJob, filter string) bool {
	lower := toLower(filter)
	return containsLower(job.Command, lower) ||
		containsLower(job.Schedule.Expression, lower) ||
		containsLower(job.ID, lower)
}

func (m *Model) selectedJob() *domain.CronJob {
	if len(m.filteredIdx) == 0 {
		return nil
	}
	idx := m.filteredIdx[m.selected]
	if idx >= len(m.jobs) {
		return nil
	}
	return &m.jobs[idx]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
