package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/nabobery/lazycron/internal/app"
	"github.com/nabobery/lazycron/internal/domain"
	"github.com/nabobery/lazycron/internal/platform/cronlogs"
	"github.com/nabobery/lazycron/internal/runner"
	"github.com/nabobery/lazycron/internal/schedule"
)

type Model struct {
	applySvc     *app.ApplyService
	inventorySvc *app.InventoryService
	scheduleSvc  *schedule.Service
	runner       *runner.Runner
	logsProvider cronlogs.Provider
	cancelRun    context.CancelFunc

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

	runEnvMode domain.EnvMode

	logs       []domain.RunRecord
	systemLogs *cronlogs.Result
	logScroll  int
	detScroll  int

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
		runEnvMode:  domain.EnvModeCronLike,
	}
}

// SetInventoryService enables multi-source loading (user + system cron).
func (m *Model) SetInventoryService(invSvc *app.InventoryService) {
	m.inventorySvc = invSvc
}

// SetLogsProvider enables system log fetching in the TUI.
func (m *Model) SetLogsProvider(p cronlogs.Provider) {
	m.logsProvider = p
}

func (m Model) Init() tea.Cmd {
	return m.loadCmd()
}

func (m Model) loadCmd() tea.Cmd {
	if m.inventorySvc != nil {
		invSvc := m.inventorySvc
		return func() tea.Msg {
			inv, err := invSvc.LoadAll(context.Background())
			if err != nil {
				return loadResultMsg{err: err}
			}
			return loadResultMsg{
				jobs:   inv.Jobs,
				issues: inv.Issues,
			}
		}
	}

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
	tokens, freeText := parseFilterTokens(filter)

	for _, tok := range tokens {
		if !matchesToken(job, tok) {
			return false
		}
	}

	if freeText != "" {
		lower := toLower(freeText)
		if !containsLower(job.Command, lower) &&
			!containsLower(job.Schedule.Expression, lower) &&
			!containsLower(job.ID, lower) &&
			!containsLower(job.Source.Label, lower) &&
			!containsLower(job.Source.Path, lower) &&
			!containsLower(job.RunAsUser, lower) {
			return false
		}
	}

	return true
}

type filterToken struct {
	key   string
	value string
}

func parseFilterTokens(filter string) (tokens []filterToken, freeText string) {
	parts := strings.Fields(filter)
	var free []string
	for _, p := range parts {
		if idx := strings.IndexByte(p, ':'); idx > 0 && idx < len(p)-1 {
			key := toLower(p[:idx])
			value := toLower(p[idx+1:])
			switch key {
			case "kind", "subkind", "owner", "source", "runas", "tz":
				tokens = append(tokens, filterToken{key: key, value: value})
				continue
			case "enabled", "readonly":
				if value == "true" || value == "false" {
					tokens = append(tokens, filterToken{key: key, value: value})
					continue
				}
			}
		}
		free = append(free, p)
	}
	freeText = strings.Join(free, " ")
	return tokens, freeText
}

func matchesToken(job domain.CronJob, tok filterToken) bool {
	switch tok.key {
	case "kind":
		return containsLower(string(job.Source.Kind), tok.value)
	case "subkind":
		return containsLower(string(job.Source.Subkind), tok.value)
	case "enabled":
		if tok.value == "true" {
			return job.Enabled
		}
		return !job.Enabled
	case "readonly":
		if tok.value == "true" {
			return job.ReadOnly
		}
		return !job.ReadOnly
	case "owner":
		return containsLower(job.Source.Owner, tok.value)
	case "source":
		return containsLower(job.Source.Label, tok.value) || containsLower(job.Source.Path, tok.value)
	case "runas":
		return containsLower(job.RunAsUser, tok.value)
	case "tz":
		return containsLower(job.Schedule.Timezone, tok.value)
	}
	return false
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
