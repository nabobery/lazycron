package tui

import (
	"fmt"
	"os/user"

	tea "charm.land/bubbletea/v2"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle editor states
	if m.state == stateEditing || m.state == stateCreating {
		return m.handleEditorKey(msg)
	}

	// Handle confirm-discard modal
	if m.state == stateConfirmDiscard {
		return m.handleConfirmDiscardKey(key)
	}

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
			if job.ReadOnly {
				m.bannerMsg = &banner{message: "Cannot toggle: system source is read-only", isError: false}
				return m, nil
			}
			m.state = stateApplying
			return m, m.toggleCmd(job.ID)
		}

	case "d":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			if job.ReadOnly {
				m.bannerMsg = &banner{message: "Cannot delete: system source is read-only", isError: false}
				return m, nil
			}
			m.state = stateConfirmDelete
		}

	case "x":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			if job.RunAsUser != "" {
				if u, err := user.Current(); err == nil && u.Username != job.RunAsUser {
					m.bannerMsg = &banner{
						message: fmt.Sprintf("Note: job runs as %s in cron; running now as %s", job.RunAsUser, u.Username),
					}
				}
			}
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

	case "n":
		if m.state == stateReady {
			m.openCreateEditor()
			return m, nil
		}

	case "e":
		job := m.selectedJob()
		if job != nil && m.state == stateReady {
			if job.ReadOnly {
				m.bannerMsg = &banner{message: "Cannot edit: system source is read-only", isError: false}
				return m, nil
			}
			m.openEditEditor(*job)
			return m, nil
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

func (m Model) handleEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.editor == nil {
		m.state = stateReady
		return m, nil
	}

	// Schedule kind field uses left/right to cycle, not text input
	if m.editor.focusField == fieldSchedKind {
		switch key {
		case "left", "right", "h", "l", "space":
			m.cycleSchedKind()
			return m, nil
		case "tab", "down", "j":
			m.editorNextField()
			return m, nil
		case "shift+tab", "up", "k":
			m.editorPrevField()
			return m, nil
		case "enter":
			return m.editorTrySave()
		case "esc":
			return m.editorTryCancel()
		}
		return m, nil
	}

	switch key {
	case "tab", "down":
		m.editorNextField()
		return m, nil
	case "shift+tab", "up":
		m.editorPrevField()
		return m, nil
	case "enter":
		return m.editorTrySave()
	case "esc":
		return m.editorTryCancel()
	case "backspace":
		if len(m.editor.fieldBuf) > 0 {
			m.editor.fieldBuf = m.editor.fieldBuf[:len(m.editor.fieldBuf)-1]
		}
		return m, nil
	default:
		if len(key) == 1 || key == "space" {
			ch := key
			if key == "space" {
				ch = " "
			}
			m.editor.fieldBuf += ch
		}
		return m, nil
	}
}

func (m Model) editorTrySave() (tea.Model, tea.Cmd) {
	errs := m.validateEditor()
	m.editor.fieldErrs = make(map[editorField]string)

	if len(errs) > 0 {
		kind := m.editor.draft.SchedKind
		for _, e := range errs {
			f := fieldFromName(e.Field, kind)
			if f >= 0 {
				m.editor.fieldErrs[f] = e.Message
			}
		}
		return m, nil
	}

	m.state = stateApplying
	cmd := m.editorSaveCmd()
	m.editor = nil
	return m, cmd
}

func (m Model) editorTryCancel() (tea.Model, tea.Cmd) {
	if m.isDirty() {
		m.state = stateConfirmDiscard
		return m, nil
	}
	m.editor = nil
	m.state = stateReady
	return m, nil
}

func (m Model) handleConfirmDiscardKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		m.editor = nil
		m.state = stateReady
	case "n", "N", "esc":
		if m.editor != nil {
			if m.editor.mode == editorModeCreate {
				m.state = stateCreating
			} else {
				m.state = stateEditing
			}
		} else {
			m.state = stateReady
		}
	}
	return m, nil
}

func fieldFromName(name string, kind domain.ScheduleKind) editorField {
	switch name {
	case "minute":
		return fieldMinute
	case "hour":
		return fieldHour
	case "day_of_month":
		return fieldDayOfMonth
	case "month":
		return fieldMonth
	case "day_of_week":
		return fieldDayOfWeek
	case "descriptor":
		return fieldDescriptor
	case "timezone":
		return fieldTimezone
	case "command":
		return fieldCommand
	case "expression":
		if kind == domain.ScheduleKindDescriptor || kind == domain.ScheduleKindReboot {
			return fieldDescriptor
		}
		return fieldMinute
	case "schedule_kind":
		return fieldSchedKind
	}
	return -1
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
