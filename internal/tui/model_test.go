package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/avinashchangrani/lazycron/internal/app"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/crontab"
	"github.com/avinashchangrani/lazycron/internal/runner"
	"github.com/avinashchangrani/lazycron/internal/schedule"
)

var testSource = domain.CronSource{
	Kind: domain.SourceKindUserCrontab,
	Path: "crontab://current-user",
	User: "testuser",
}

func newTestModel(content string) Model {
	fc := crontab.NewFakeClient(content, content != "")
	svc := app.NewApplyService(fc, testSource)
	schedSvc := schedule.NewService()
	r := runner.New(runner.DefaultConfig())
	m := NewModel(svc, schedSvc, r)
	m.width = 120
	m.height = 40
	return m
}

func press(code rune, text string, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text, Mod: mod}
}

func loadModel(m Model) Model {
	cmd := m.Init()
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return m
}

func TestModel_LoadAndRender(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	if len(m.jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(m.jobs))
	}
	if len(m.filteredIdx) != 2 {
		t.Fatalf("expected 2 filtered indices, got %d", len(m.filteredIdx))
	}

	view := m.View()
	if view.Content == "" {
		t.Fatal("view should not be empty")
	}
	if !view.AltScreen {
		t.Fatal("view should enable alt screen in v2")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected mouse cell motion, got %v", view.MouseMode)
	}
}

func TestModel_NavigationPreservesSelection(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n0 5 * * * /usr/local/bin/third\n")
	m = loadModel(m)

	if m.selected != 0 {
		t.Fatalf("initial selection should be 0, got %d", m.selected)
	}

	// Move down
	updated, _ := m.Update(press('j', "j", 0))
	m = updated.(Model)
	if m.selected != 1 {
		t.Fatalf("after j, selection should be 1, got %d", m.selected)
	}

	// Move down again
	updated, _ = m.Update(press('j', "j", 0))
	m = updated.(Model)
	if m.selected != 2 {
		t.Fatalf("after second j, selection should be 2, got %d", m.selected)
	}

	// Move down past end (should clamp)
	updated, _ = m.Update(press('j', "j", 0))
	m = updated.(Model)
	if m.selected != 2 {
		t.Fatalf("should clamp at 2, got %d", m.selected)
	}

	// Move up
	updated, _ = m.Update(press('k', "k", 0))
	m = updated.(Model)
	if m.selected != 1 {
		t.Fatalf("after k, selection should be 1, got %d", m.selected)
	}
}

func TestModel_FilterNarrowsList(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	// Enter filter mode
	updated, _ := m.Update(press('/', "/", 0))
	m = updated.(Model)
	if !m.filtering {
		t.Fatal("should be in filtering mode")
	}

	// Type "backup"
	for _, ch := range "backup" {
		updated, _ = m.Update(press(ch, string(ch), 0))
		m = updated.(Model)
	}

	if len(m.filteredIdx) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(m.filteredIdx))
	}

	// Selection should be clamped
	if m.selected != 0 {
		t.Fatalf("selection should be 0, got %d", m.selected)
	}

	// Escape clears filter
	updated, _ = m.Update(press(tea.KeyEsc, "esc", 0))
	m = updated.(Model)
	if m.filtering {
		t.Fatal("should not be filtering after escape")
	}
	if len(m.filteredIdx) != 2 {
		t.Fatalf("expected 2 results after clearing filter, got %d", len(m.filteredIdx))
	}
}

func TestModel_FilterAllowsSpaces(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup db --full\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	updated, _ := m.Update(press('/', "/", 0))
	m = updated.(Model)
	for _, ch := range "backup db" {
		text := string(ch)
		if ch == ' ' {
			text = "space"
		}
		updated, _ = m.Update(press(ch, text, 0))
		m = updated.(Model)
	}

	if len(m.filteredIdx) != 1 {
		t.Fatalf("expected 1 filtered result after typing a space, got %d", len(m.filteredIdx))
	}
}

func TestModel_ConfirmDeleteModal(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	// Press d to enter delete confirmation
	updated, _ := m.Update(press('d', "d", 0))
	m = updated.(Model)
	if m.state != stateConfirmDelete {
		t.Fatalf("expected stateConfirmDelete, got %d", m.state)
	}

	// Press n to cancel
	updated, _ = m.Update(press('n', "n", 0))
	m = updated.(Model)
	if m.state != stateReady {
		t.Fatalf("expected stateReady after cancel, got %d", m.state)
	}
}

func TestModel_EmptyCrontab(t *testing.T) {
	m := newTestModel("")
	m = loadModel(m)

	if len(m.jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(m.jobs))
	}

	view := m.View()
	if view.Content == "" {
		t.Fatal("view should render even with no jobs")
	}
}

func TestModel_TabCyclesFocus(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	if m.focused != paneJobs {
		t.Fatal("initial focus should be paneJobs")
	}

	updated, _ := m.Update(press(tea.KeyTab, "tab", 0))
	m = updated.(Model)
	if m.focused != paneDetails {
		t.Fatalf("after tab, focus should be paneDetails, got %d", m.focused)
	}

	updated, _ = m.Update(press(tea.KeyTab, "tab", 0))
	m = updated.(Model)
	if m.focused != paneLogs {
		t.Fatalf("after second tab, focus should be paneLogs, got %d", m.focused)
	}

	updated, _ = m.Update(press(tea.KeyTab, "tab", 0))
	m = updated.(Model)
	if m.focused != paneJobs {
		t.Fatalf("after third tab, focus should wrap to paneJobs, got %d", m.focused)
	}
}

func TestModel_RunCancelStoresCancel(t *testing.T) {
	m := newTestModel("0 3 * * * /bin/echo hello\n")
	m = loadModel(m)

	// Press x to start a run
	updated, cmd := m.Update(press('x', "x", 0))
	m = updated.(Model)

	if m.state != stateRunning {
		t.Fatalf("expected stateRunning, got %d", m.state)
	}
	if m.cancelRun == nil {
		t.Fatal("cancelRun should be set after pressing x")
	}
	if cmd == nil {
		t.Fatal("should have returned a command for the run")
	}

	// Press c to cancel
	updated, _ = m.Update(press('c', "c", 0))
	m = updated.(Model)

	// The cancel function should have been called (we can't easily verify the goroutine
	// but we can verify the model still has the cancel func until the result comes back)
}

func TestModel_QuitBlockedDuringRun(t *testing.T) {
	m := newTestModel("0 3 * * * /bin/echo hello\n")
	m = loadModel(m)

	// Start a run
	updated, _ := m.Update(press('x', "x", 0))
	m = updated.(Model)

	if m.state != stateRunning {
		t.Fatalf("expected stateRunning, got %d", m.state)
	}

	// Try to quit — should auto-cancel, not quit immediately
	updated, cmd := m.Update(press('q', "q", 0))
	m = updated.(Model)

	// Should not have returned tea.Quit
	if cmd != nil {
		// Execute the cmd to check it's not a quit
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("q during stateRunning should not quit immediately")
		}
	}
}

func TestModel_LoadingStateBlocksActions(t *testing.T) {
	m := newTestModel("0 3 * * * /bin/echo hello\n")
	m = loadModel(m)

	// Simulate stateLoading
	m.state = stateLoading

	// Try to toggle — should be blocked
	updated, cmd := m.Update(press(' ', "space", 0))
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("space during stateLoading should not trigger toggle")
	}
	if m.state != stateLoading {
		t.Fatalf("state should remain stateLoading, got %d", m.state)
	}
}

func TestModel_SmallWindowNoPanic(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"zero", 0, 0},
		{"tiny", 10, 5},
		{"narrow", 20, 8},
		{"short", 80, 3},
		{"minimal", 30, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.width = tt.width
			m.height = tt.height
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("View() panicked at %dx%d: %v", tt.width, tt.height, r)
				}
			}()
			view := m.View()
			if view.Content == "" {
				t.Fatal("view should not be empty")
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		n    int
		word string
		want string
	}{
		{0, "issue", "0 issues"},
		{1, "issue", "1 issue"},
		{2, "issue", "2 issues"},
		{10, "issue", "10 issues"},
		{123, "warning", "123 warnings"},
	}
	for _, tt := range tests {
		got := pluralize(tt.n, tt.word)
		if got != tt.want {
			t.Errorf("pluralize(%d, %q) = %q, want %q", tt.n, tt.word, got, tt.want)
		}
	}
}

func TestStripControlCodes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"ANSI color", "\x1b[31mred text\x1b[0m", "red text"},
		{"ANSI bold", "\x1b[1mbold\x1b[0m normal", "bold normal"},
		{"null bytes", "hello\x00world", "helloworld"},
		{"bell char", "hello\x07world", "helloworld"},
		{"preserves newlines", "line1\nline2", "line1\nline2"},
		{"preserves tabs", "col1\tcol2", "col1\tcol2"},
		{"mixed escapes", "\x1b[32m\x07ok\x1b[0m\n", "ok\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripControlCodes(tt.input)
			if got != tt.want {
				t.Errorf("stripControlCodes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskSecretValue(t *testing.T) {
	tests := []struct {
		key  string
		val  string
		want string
	}{
		{"PATH", "/usr/bin", "/usr/bin"},
		{"HOME", "/home/user", "/home/user"},
		{"API_TOKEN", "abc123", "******"},
		{"AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI", "******"},
		{"DB_PASSWORD", "hunter2", "******"},
		{"GITHUB_TOKEN", "ghp_xxx", "******"},
		{"MY_API_KEY", "key123", "******"},
		{"auth_credential", "cred", "******"},
		{"MAILTO", "ops@example.com", "ops@example.com"},
		{"SHELL", "/bin/bash", "/bin/bash"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := maskSecretValue(tt.key, tt.val)
			if got != tt.want {
				t.Errorf("maskSecretValue(%q, %q) = %q, want %q", tt.key, tt.val, got, tt.want)
			}
		})
	}
}
