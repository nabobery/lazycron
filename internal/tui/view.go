package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nabobery/lazycron/internal/domain"
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

	// Editor overlay takes over the screen
	if m.state == stateEditing || m.state == stateCreating || m.state == stateConfirmDiscard {
		return m.renderEditor(m.width, m.height)
	}

	var sections []string

	// Title bar
	title := titleStyle.Render(" lazycron v0.2.0 ")
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

		prefix := badge
		if job.ReadOnly {
			prefix = badge + " " + readOnlyBadge.String()
		}

		prefixWidth := ansi.StringWidth(prefix + " ")
		cmdDisplay := truncate(job.Command, width-prefixWidth)
		line := fmt.Sprintf("%s %s", prefix, cmdDisplay)

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
	lines = append(lines, truncate(fmt.Sprintf("Schedule: %s", desc), width))
	lines = append(lines, truncate(fmt.Sprintf("Expression: %s", dimStyle.Render(job.Schedule.Expression)), width))

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
	} else if job.Schedule.Kind == domain.ScheduleKindPeriodic {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("(periodic directory; schedule is non-deterministic)"))
	}

	// Source info
	lines = append(lines, "")
	lines = append(lines, truncate(fmt.Sprintf("Command: %s", job.Command), width))
	lines = append(lines, truncate(fmt.Sprintf("Source: %s", dimStyle.Render(job.Source.Path)), width))

	if job.Source.Label != "" && job.Source.Label != job.Source.Path {
		lines = append(lines, truncate(fmt.Sprintf("Label: %s", dimStyle.Render(job.Source.Label)), width))
	}

	if job.Source.Owner != "" {
		lines = append(lines, truncate(fmt.Sprintf("Owner: %s", dimStyle.Render(job.Source.Owner)), width))
	}

	if job.RunAsUser != "" {
		lines = append(lines, truncate(fmt.Sprintf("Run as: %s", dimStyle.Render(job.RunAsUser)), width))
	}

	status := successStyle.Render("enabled")
	if !job.Enabled {
		status = dimStyle.Render("disabled")
	}
	lines = append(lines, fmt.Sprintf("Status: %s", status))

	if job.ReadOnly {
		lines = append(lines, fmt.Sprintf("Editable: %s", readOnlyLabelStyle.Render("no (system source, read-only)")))
	}

	if job.Schedule.Timezone != "" {
		lines = append(lines, truncate(fmt.Sprintf("Timezone: %s", job.Schedule.Timezone), width))
	}

	if job.Source.Kind == domain.SourceKindSystem {
		access := job.Source.Access
		if access.Readable {
			lines = append(lines, fmt.Sprintf("Access: %s", dimStyle.Render("readable")))
		}
		if access.Reason != "" {
			lines = append(lines, truncate(fmt.Sprintf("Note: %s", dimStyle.Render(access.Reason)), width))
		}
	}

	// Issues for this job's source
	jobIssues := m.issuesForJob(job)
	if len(jobIssues) > 0 {
		lines = append(lines, "")
		lines = append(lines, warningStyle.Render("Issues:"))
		for _, issue := range jobIssues {
			severity := dimStyle.Render("[" + string(issue.Severity) + "]")
			lines = append(lines, truncate(fmt.Sprintf("  %s %s", severity, issue.Message), width))
		}
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

	if len(m.logs) == 0 && m.systemLogs == nil {
		lines = append(lines, dimStyle.Render("No runs yet. Press 'x' to run, 'l' for system logs."))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:min(len(lines), height)], "\n")
	}

	if len(m.logs) > 0 {
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
	}

	if m.systemLogs != nil && !m.systemLogs.NotFound && len(m.systemLogs.Lines) > 0 {
		if len(m.logs) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, headerStyle.Render("System logs:")+" "+dimStyle.Render("("+m.systemLogs.Source+")"))
		for _, l := range m.systemLogs.Lines {
			lines = append(lines, truncate(l, width))
		}
		if m.systemLogs.Partial {
			lines = append(lines, warningStyle.Render("(output truncated)"))
		}
	} else if m.systemLogs != nil && len(m.logs) == 0 {
		if m.systemLogs.NotFound {
			lines = append(lines, dimStyle.Render("Logs: "+m.systemLogs.Reason))
		} else {
			lines = append(lines, dimStyle.Render("No matching log entries found ("+m.systemLogs.Source+")"))
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

func (m Model) issuesForJob(job *domain.CronJob) []domain.ValidationIssue {
	var result []domain.ValidationIssue
	jobPath := job.Source.Path
	for _, issue := range m.issues {
		if issue.SourcePath != "" && issue.SourcePath != jobPath {
			continue
		}
		if issue.LineIndex == job.LineIndex || issue.LineIndex < 0 {
			result = append(result, issue)
		}
	}
	return result
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
	case stateEditing, stateCreating:
		parts = append(parts, "tab/shift+tab:fields  enter:save  esc:cancel")
	case stateConfirmDiscard:
		parts = append(parts, "y:discard  n:keep editing")
	default:
		modeLabel := string(m.runEnvMode)
		parts = append(parts, fmt.Sprintf("j/k:navigate  space:toggle  d:delete  x:run(%s)  E:mode  l:logs  n:new  e:edit  /:search  r:reload  q:quit", modeLabel))
	}

	return helpStyle.Render(strings.Join(parts, "  "))
}
