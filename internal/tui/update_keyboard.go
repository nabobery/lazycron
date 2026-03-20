package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle filtering mode
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	// Handle confirm-delete modal
	if m.state == stateConfirmDelete {
		return m.handleConfirmDeleteKey(key)
	}

	// Handle drift state
	if m.state == stateDriftDetected {
		if key == "r" {
			m.state = stateLoading
			m.bannerMsg = nil
			return m, m.loadCmd()
		}
		if key == "q" || key == "esc" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Don't process keys during async operations
	if m.state == stateApplying || m.state == stateLoading {
		return m, nil
	}

	// Running state: only allow cancel and quit (which auto-cancels)
	if m.state == stateRunning {
		switch key {
		case "c":
			if m.cancelRun != nil {
				m.cancelRun()
			}
			return m, nil
		case "q", "esc":
			if m.cancelRun != nil {
				m.cancelRun()
			}
			m.bannerMsg = &banner{message: "Cancelling running job...", isError: false}
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.bannerMsg != nil {
			m.bannerMsg = nil
			return m, nil
		}
		return m, tea.Quit

	case "j", "down":
		if len(m.filteredIdx) > 0 {
			m.selected = min(m.selected+1, len(m.filteredIdx)-1)
			m.detScroll = 0
		}
	case "k", "up":
		if len(m.filteredIdx) > 0 {
			m.selected = max(m.selected-1, 0)
			m.detScroll = 0
		}

	case "tab":
		m.focused = (m.focused + 1) % 3
	case "shift+tab":
		m.focused = (m.focused + 2) % 3

	case "/":
		m.filtering = true
		m.filterText = ""
		m.state = stateFiltering

	case "space":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			m.state = stateApplying
			return m, m.toggleCmd(job.ID)
		}

	case "d":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			m.state = stateConfirmDelete
		}

	case "x":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			m.state = stateRunning
			cmd := m.startRun(*job)
			return m, cmd
		}

	case "r":
		if m.state == stateReady {
			m.state = stateLoading
			m.bannerMsg = nil
			return m, m.loadCmd()
		}

	case "g":
		m.selected = 0
	case "G":
		if len(m.filteredIdx) > 0 {
			m.selected = len(m.filteredIdx) - 1
		}
	}

	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.state = stateReady
		m.rebuildFilter()
		return m, nil
	case "enter":
		m.filtering = false
		m.state = stateReady
		return m, nil
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.rebuildFilter()
		}
		return m, nil
	case "space":
		m.filterText += " "
		m.rebuildFilter()
		return m, nil
	default:
		if len(key) == 1 {
			m.filterText += key
			m.rebuildFilter()
		}
		return m, nil
	}
}

func (m Model) handleConfirmDeleteKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		job := m.selectedJob()
		if job != nil {
			m.state = stateApplying
			return m, m.deleteCmd(job.ID)
		}
		m.state = stateReady
	case "n", "N", "esc":
		m.state = stateReady
	}
	return m, nil
}
