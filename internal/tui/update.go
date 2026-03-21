package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/nabobery/lazycron/internal/app"
	"github.com/nabobery/lazycron/internal/domain"
	"github.com/nabobery/lazycron/internal/platform/cronlogs"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case loadResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.bannerMsg = &banner{message: "Load failed: " + msg.err.Error(), isError: true}
			return m, nil
		}
		m.jobs = msg.jobs
		m.issues = msg.issues
		m.rebuildFilter()
		if len(msg.issues) > 0 {
			m.bannerMsg = &banner{
				message: pluralize(len(msg.issues), "validation issue") + " found",
				isError: false,
			}
		} else {
			m.bannerMsg = nil
		}
		m.state = stateReady
		return m, nil

	case applyResultMsg:
		m.state = stateReady
		if msg.err != nil {
			if app.IsDriftError(msg.err) {
				m.state = stateDriftDetected
				m.bannerMsg = &banner{message: "Crontab modified externally. Press 'r' to reload.", isError: true}
				return m, nil
			}
			m.bannerMsg = &banner{message: "Apply failed: " + msg.err.Error(), isError: true}
			return m, nil
		}
		m.bannerMsg = &banner{message: "Applied successfully", isError: false}
		return m, m.loadCmd()

	case runResultMsg:
		m.state = stateReady
		m.cancelRun = nil
		if msg.err != nil {
			m.bannerMsg = &banner{message: "Run error: " + msg.err.Error(), isError: true}
			return m, nil
		}
		m.logs = append(m.logs, msg.record)
		return m, nil

	case sysLogResultMsg:
		if msg.err != nil {
			m.bannerMsg = &banner{message: "Log fetch error: " + msg.err.Error(), isError: true}
			return m, nil
		}
		m.systemLogs = &msg.result
		if msg.result.NotFound {
			m.bannerMsg = &banner{message: "Logs: " + msg.result.Reason}
		} else if len(msg.result.Lines) == 0 {
			m.bannerMsg = &banner{message: "No matching log entries found"}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg, tea.MouseWheelMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m Model) toggleCmd(jobID string) tea.Cmd {
	svc := m.applySvc
	return func() tea.Msg {
		err := svc.Toggle(context.Background(), jobID)
		return applyResultMsg{err: err}
	}
}

func (m Model) deleteCmd(jobID string) tea.Cmd {
	svc := m.applySvc
	return func() tea.Msg {
		err := svc.Delete(context.Background(), jobID)
		return applyResultMsg{err: err}
	}
}

func (m *Model) startRun(job domain.CronJob) tea.Cmd {
	r := m.runner
	mode := m.runEnvMode
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRun = cancel
	return func() tea.Msg {
		rec, err := r.Run(ctx, job, mode)
		return runResultMsg{record: rec, err: err}
	}
}

func (m *Model) fetchSystemLogs(job domain.CronJob) tea.Cmd {
	provider := m.logsProvider
	return func() tea.Msg {
		q := cronlogs.Query{
			Command: job.Command,
			Limit:   50,
		}
		if job.RunAsUser != "" {
			q.User = job.RunAsUser
		}
		result, err := provider.Fetch(context.Background(), q)
		return sysLogResultMsg{result: result, err: err}
	}
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
