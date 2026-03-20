package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/domain"
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
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRun = cancel
	return func() tea.Msg {
		rec, err := r.Run(ctx, job, domain.EnvModeCronLike)
		return runResultMsg{record: rec, err: err}
	}
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
