package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

const minUsableWidth = 40
const minUsableHeight = 10

func (m Model) View() tea.View {
	v := tea.NewView(m.renderContent())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderContent() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.width < minUsableWidth || m.height < minUsableHeight {
		return fmt.Sprintf("Window too small (%dx%d). Need at least %dx%d.",
			m.width, m.height, minUsableWidth, minUsableHeight)
	}

	var sections []string

	// Title bar
	title := titleStyle.Render(" lazycron v0.1.0 ")
	sections = append(sections, title)

	// Banner
	if m.bannerMsg != nil {
		style := bannerInfoStyle
		if m.bannerMsg.isError {
			style = bannerErrorStyle
		}
		sections = append(sections, style.Width(m.width).Render(m.bannerMsg.message))
	}

	// Confirm delete modal
	if m.state == stateConfirmDelete {
		job := m.selectedJob()
		if job != nil {
			msg := fmt.Sprintf("Delete job: %s ? (y/n)", truncate(job.Command, 40))
			sections = append(sections, warningStyle.Render(msg))
		}
	}

	// Main content area
	headerHeight := len(sections)
	helpHeight := 1
	contentHeight := m.height - headerHeight - helpHeight - 1

	if contentHeight < 5 {
		contentHeight = 5
	}

	leftWidth := m.leftPaneWidth()
	rightWidth := m.width - leftWidth
	if rightWidth < 4 {
		rightWidth = 4
	}

	jobsPaneW := max(1, leftWidth-2)
	jobsPaneH := max(1, contentHeight-2)
	detailsHeight := max(1, contentHeight/2-2)
	logsHeight := max(1, contentHeight-contentHeight/2-2)
	rightPaneW := max(1, rightWidth-2)

	// Build panes
	jobsPane := m.renderJobsPane(jobsPaneW, jobsPaneH)
	detailsPane := m.renderDetailsPane(rightPaneW, detailsHeight)
	logsPane := m.renderLogsPane(rightPaneW, logsHeight)

	// Style panes
	jobsBorder := paneStyle
	detBorder := paneStyle
	logBorder := paneStyle
	switch m.focused {
	case paneJobs:
		jobsBorder = activePaneStyle
	case paneDetails:
		detBorder = activePaneStyle
	case paneLogs:
		logBorder = activePaneStyle
	}

	left := jobsBorder.Width(jobsPaneW).Height(jobsPaneH).Render(jobsPane)
	rightTop := detBorder.Width(rightPaneW).Height(detailsHeight).Render(detailsPane)
	rightBottom := logBorder.Width(rightPaneW).Height(logsHeight).Render(logsPane)

	right := lipgloss.JoinVertical(lipgloss.Left, rightTop, rightBottom)
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	sections = append(sections, mainContent)

	// Help bar
	help := m.renderHelp()
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) leftPaneWidth() int {
	if m.width < 60 {
		return m.width
	}
	return m.width * 40 / 100
}

func (m Model) renderJobsPane(width, height int) string {
	var lines []string

	header := headerStyle.Render("Jobs")
	if m.filtering || m.filterText != "" {
		header += " " + searchStyle.Render("/"+m.filterText)
		if m.filtering {
			header += searchStyle.Render("_")
		}
	}
	header += dimStyle.Render(fmt.Sprintf(" (%d)", len(m.filteredIdx)))
	lines = append(lines, truncate(header, width))

	if len(m.filteredIdx) == 0 {
		lines = append(lines, dimStyle.Render("No jobs found"))
	}

	visibleStart := 0
	visibleCount := height - 1
	if visibleCount < 1 {
		visibleCount = 1
	}

	if m.selected >= visibleStart+visibleCount {
		visibleStart = m.selected - visibleCount + 1
	}
	if m.selected < visibleStart {
		visibleStart = m.selected
	}

	for i := visibleStart; i < len(m.filteredIdx) && i < visibleStart+visibleCount; i++ {
		jobIdx := m.filteredIdx[i]
		job := m.jobs[jobIdx]

		badge := enabledBadge.String()
		if !job.Enabled {
			badge = disabledBadge.String()
		}

		cmdDisplay := truncate(job.Command, width-4)
		line := fmt.Sprintf("%s %s", badge, cmdDisplay)

		if i == m.selected {
			line = selectedStyle.Width(width).Render(line)
		}

		lines = append(lines, line)
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m Model) renderDetailsPane(width, height int) string {
	var lines []string

	lines = append(lines, headerStyle.Render("Details"))

	job := m.selectedJob()
	if job == nil {
		lines = append(lines, dimStyle.Render("No job selected"))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:min(len(lines), height)], "\n")
	}

	// Schedule description
	desc := m.scheduleSvc.Describe(job.Schedule)
	lines = append(lines, fmt.Sprintf("Schedule: %s", desc))
	lines = append(lines, fmt.Sprintf("Expression: %s", dimStyle.Render(job.Schedule.Expression)))

	// Next runs
	nextRuns, err := m.scheduleSvc.NextRuns(job.Schedule, time.Now(), 3)
	if err == nil && len(nextRuns) > 0 {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Next runs:"))
		for _, run := range nextRuns {
			lines = append(lines, fmt.Sprintf("  %s", run.Local().Format("Mon Jan 2 15:04:05")))
		}
	} else if job.Schedule.Kind == domain.ScheduleKindReboot {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("(event-based, no scheduled runs)"))
	}

	// Source info
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Command: %s", truncate(job.Command, width-10)))
	lines = append(lines, fmt.Sprintf("Source: %s", dimStyle.Render(job.Source.Path)))

	status := successStyle.Render("enabled")
	if !job.Enabled {
		status = dimStyle.Render("disabled")
	}
	lines = append(lines, fmt.Sprintf("Status: %s", status))

	if job.Schedule.Timezone != "" {
		lines = append(lines, fmt.Sprintf("Timezone: %s", job.Schedule.Timezone))
	}

	// Env context
	if len(job.EnvContext) > 0 {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Environment:"))
		for _, env := range job.EnvContext {
			displayVal := maskSecretValue(env.Key, env.Value)
			lines = append(lines, fmt.Sprintf("  %s=%s", env.Key, truncate(displayVal, width-len(env.Key)-4)))
		}
	}

	// Apply scroll offset
	if m.detScroll > 0 && m.detScroll < len(lines) {
		lines = lines[m.detScroll:]
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m Model) renderLogsPane(width, height int) string {
	var lines []string

	header := headerStyle.Render("Logs")
	if m.state == stateRunning {
		header += " " + warningStyle.Render("(running...)")
	}
	lines = append(lines, header)

	if len(m.logs) == 0 {
		lines = append(lines, dimStyle.Render("No runs yet. Press 'x' to run a job."))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:min(len(lines), height)], "\n")
	}

	// Show most recent log
	rec := m.logs[len(m.logs)-1]

	statusStr := successStyle.Render("SUCCESS")
	switch rec.Status {
	case domain.RunStatusFailed:
		statusStr = errorStyle.Render("FAILED")
	case domain.RunStatusCancelled:
		statusStr = warningStyle.Render("CANCELLED")
	case domain.RunStatusRunning:
		statusStr = warningStyle.Render("RUNNING")
	}

	lines = append(lines, fmt.Sprintf("%s  exit=%d  %s  mode=%s",
		statusStr,
		rec.ExitCode,
		dimStyle.Render(rec.Duration.Round(time.Millisecond).String()),
		dimStyle.Render(string(rec.Mode)),
	))

	if rec.Mode == domain.EnvModeCronLike {
		lines = append(lines, warningStyle.Render("Note: ran with minimal cron-like environment"))
	}

	if rec.Truncated {
		lines = append(lines, warningStyle.Render("(output truncated)"))
	}

	if rec.Stdout != "" {
		lines = append(lines, headerStyle.Render("stdout:"))
		sanitized := stripControlCodes(strings.TrimRight(rec.Stdout, "\n"))
		for _, l := range strings.Split(sanitized, "\n") {
			lines = append(lines, truncate(l, width))
		}
	}

	if rec.Stderr != "" {
		lines = append(lines, errorStyle.Render("stderr:"))
		sanitized := stripControlCodes(strings.TrimRight(rec.Stderr, "\n"))
		for _, l := range strings.Split(sanitized, "\n") {
			lines = append(lines, truncate(l, width))
		}
	}

	// Apply scroll
	if m.logScroll > 0 && m.logScroll < len(lines) {
		lines = lines[m.logScroll:]
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m Model) renderHelp() string {
	var parts []string

	switch m.state {
	case stateConfirmDelete:
		parts = append(parts, "y:confirm  n:cancel")
	case stateRunning:
		parts = append(parts, "c:cancel run  q:cancel+quit")
	case stateApplying:
		parts = append(parts, "applying...")
	case stateLoading:
		parts = append(parts, "loading...")
	case stateDriftDetected:
		parts = append(parts, "r:reload  q:quit")
	default:
		parts = append(parts, "j/k:navigate  space:toggle  d:delete  x:run  /:search  r:reload  q:quit")
	}

	return helpStyle.Render(strings.Join(parts, "  "))
}
