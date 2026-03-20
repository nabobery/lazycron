package tui

import tea "charm.land/bubbletea/v2"

func (m Model) handleMouse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Mouse().Button == tea.MouseLeft {
			return m.handleMouseClick(msg)
		}
	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			return m.handleScroll(-3), nil
		case tea.MouseWheelDown:
			return m.handleScroll(3), nil
		}
	}
	return m, nil
}

func (m Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	leftWidth := m.leftPaneWidth()

	if mouse.X < leftWidth {
		m.focused = paneJobs
		// Calculate which job was clicked based on Y position
		contentStart := 3 // title + border
		if m.bannerMsg != nil {
			contentStart++
		}
		clickedRow := mouse.Y - contentStart
		if clickedRow >= 0 && clickedRow < len(m.filteredIdx) {
			m.selected = clickedRow
			m.detScroll = 0
		}
	} else {
		rightTop := m.height / 2
		if mouse.Y < rightTop {
			m.focused = paneDetails
		} else {
			m.focused = paneLogs
		}
	}

	return m, nil
}

func (m Model) handleScroll(delta int) Model {
	switch m.focused {
	case paneJobs:
		m.selected = clamp(m.selected+delta, 0, max(0, len(m.filteredIdx)-1))
	case paneDetails:
		m.detScroll = max(0, m.detScroll+delta)
	case paneLogs:
		m.logScroll = max(0, m.logScroll+delta)
	}
	return m
}
