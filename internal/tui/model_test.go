package tui

import (
	"strings"
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

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello w…"},
		{"zero length", "hello", 0, ""},
		{"empty string", "", 10, ""},
		{"ANSI preserved", "\x1b[31mred\x1b[0m", 10, "\x1b[31mred\x1b[0m"},
		{"ANSI truncated", "\x1b[31mhello world\x1b[0m", 8, "\x1b[31mhello w…\x1b[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
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

func TestModel_NKeyOpensCreateEditor(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	if m.state != stateCreating {
		t.Fatalf("expected stateCreating, got %d", m.state)
	}
	if m.editor == nil {
		t.Fatal("editor should be initialized")
	}
	if m.editor.mode != editorModeCreate {
		t.Fatalf("expected create mode, got %d", m.editor.mode)
	}
}

func TestModel_EKeyOpensEditEditor(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	updated, _ := m.Update(press('e', "e", 0))
	m = updated.(Model)

	if m.state != stateEditing {
		t.Fatalf("expected stateEditing, got %d", m.state)
	}
	if m.editor == nil {
		t.Fatal("editor should be initialized")
	}
	if m.editor.mode != editorModeEdit {
		t.Fatalf("expected edit mode, got %d", m.editor.mode)
	}
	if m.editor.draft.Command != "/usr/local/bin/backup-db" {
		t.Fatalf("expected command from job, got %q", m.editor.draft.Command)
	}
}

func TestModel_EditorEscCancelClean(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Esc should cancel without confirm since not dirty
	updated, _ = m.Update(press(tea.KeyEsc, "esc", 0))
	m = updated.(Model)

	if m.state != stateReady {
		t.Fatalf("expected stateReady after clean cancel, got %d", m.state)
	}
	if m.editor != nil {
		t.Fatal("editor should be nil after cancel")
	}
}

func TestModel_EditorDirtyCancelTriggersConfirm(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Type something to make it dirty
	updated, _ = m.Update(press('5', "5", 0))
	m = updated.(Model)

	if !m.isDirty() {
		t.Fatal("editor should be dirty after typing")
	}

	// Esc should trigger confirm discard
	updated, _ = m.Update(press(tea.KeyEsc, "esc", 0))
	m = updated.(Model)

	if m.state != stateConfirmDiscard {
		t.Fatalf("expected stateConfirmDiscard, got %d", m.state)
	}

	// Press y to discard
	updated, _ = m.Update(press('y', "y", 0))
	m = updated.(Model)

	if m.state != stateReady {
		t.Fatalf("expected stateReady after discard, got %d", m.state)
	}
}

func TestModel_EditorConfirmDiscardNo(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Make dirty
	updated, _ = m.Update(press('5', "5", 0))
	m = updated.(Model)

	// Esc -> confirm discard
	updated, _ = m.Update(press(tea.KeyEsc, "esc", 0))
	m = updated.(Model)

	// Press n to keep editing
	updated, _ = m.Update(press('n', "n", 0))
	m = updated.(Model)

	if m.state != stateCreating {
		t.Fatalf("expected stateCreating after keeping, got %d", m.state)
	}
	if m.editor == nil {
		t.Fatal("editor should still exist")
	}
}

func TestModel_EditorSaveTriggersApply(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Navigate to command field and type something
	// Tab through fields: schedKind -> minute -> hour -> dom -> month -> dow -> tz -> command
	for i := 0; i < 7; i++ {
		updated, _ = m.Update(press(tea.KeyTab, "tab", 0))
		m = updated.(Model)
	}

	// Type a command
	for _, ch := range "/bin/echo" {
		updated, _ = m.Update(press(ch, string(ch), 0))
		m = updated.(Model)
	}

	// Press enter to save
	updated, cmd := m.Update(press(tea.KeyEnter, "enter", 0))
	m = updated.(Model)

	if m.state != stateApplying {
		t.Fatalf("expected stateApplying after save, got %d", m.state)
	}
	if cmd == nil {
		t.Fatal("should have returned a command for the apply")
	}
}

func TestModel_EditorRenderNoPanic(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("renderEditor panicked: %v", r)
		}
	}()

	view := m.View()
	if view.Content == "" {
		t.Fatal("view should not be empty in editor state")
	}
}

func TestModel_EditorFieldNavigation(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	if m.editor.focusField != fieldMinute {
		t.Fatalf("initial focus should be fieldMinute, got %d", m.editor.focusField)
	}

	// Tab to next field
	updated, _ = m.Update(press(tea.KeyTab, "tab", 0))
	m = updated.(Model)
	if m.editor.focusField != fieldHour {
		t.Fatalf("after tab, focus should be fieldHour, got %d", m.editor.focusField)
	}

	// Shift+tab back
	updated, _ = m.Update(press(tea.KeyTab, "shift+tab", tea.ModShift))
	m = updated.(Model)
	// Note: shift+tab sends different key string
}

func TestModel_EditorValidationErrors(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Clear the minute field
	m.editor.fieldBuf = ""
	m.commitFieldBuf()

	// Also clear command
	m.editor.draft.Command = ""

	// Try to save
	updated, _ = m.Update(press(tea.KeyEnter, "enter", 0))
	m = updated.(Model)

	// Should still be in editor state with errors
	if m.state == stateApplying {
		t.Fatal("should not apply with validation errors")
	}
	if len(m.editor.fieldErrs) == 0 {
		t.Fatal("should have field errors")
	}
}

func TestModel_EditorDescriptorValidationMapsToVisibleField(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Cycle to descriptor mode
	m.cycleSchedKind()
	if m.editor.draft.SchedKind != domain.ScheduleKindDescriptor {
		t.Fatalf("expected descriptor kind, got %q", m.editor.draft.SchedKind)
	}

	// Set an invalid descriptor that will fail full-expression validation
	m.editor.draft.Descriptor = "@bogus"
	m.editor.fieldBuf = "@bogus"
	m.editor.draft.Command = "/bin/echo"

	// Try to save
	updated, _ = m.Update(press(tea.KeyEnter, "enter", 0))
	m = updated.(Model)

	if m.state == stateApplying {
		t.Fatal("should not apply with invalid descriptor")
	}

	// The error should be mapped to fieldDescriptor, which is visible
	if _, ok := m.editor.fieldErrs[fieldDescriptor]; !ok {
		t.Fatalf("expected error on fieldDescriptor, got errors: %v", m.editor.fieldErrs)
	}
}

func TestModel_FieldFromNameExpressionMapping(t *testing.T) {
	tests := []struct {
		name  string
		kind  domain.ScheduleKind
		field string
		want  editorField
	}{
		{"standard expression", domain.ScheduleKindStandard, "expression", fieldMinute},
		{"descriptor expression", domain.ScheduleKindDescriptor, "expression", fieldDescriptor},
		{"reboot expression", domain.ScheduleKindReboot, "expression", fieldDescriptor},
		{"minute field", domain.ScheduleKindStandard, "minute", fieldMinute},
		{"command field", domain.ScheduleKindStandard, "command", fieldCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldFromName(tt.field, tt.kind)
			if got != tt.want {
				t.Errorf("fieldFromName(%q, %q) = %d, want %d", tt.field, tt.kind, got, tt.want)
			}
		})
	}
}

func TestModel_EKeyNoJobNoop(t *testing.T) {
	m := newTestModel("")
	m = loadModel(m)

	updated, _ := m.Update(press('e', "e", 0))
	m = updated.(Model)

	if m.state != stateReady {
		t.Fatalf("e with no jobs should stay in stateReady, got %d", m.state)
	}
	if m.editor != nil {
		t.Fatal("editor should not be opened with no jobs")
	}
}

func TestModel_EditorViewDoesNotMutateDirty(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	if m.isDirty() {
		t.Fatal("editor should not be dirty immediately after opening")
	}

	// Capture draft state before View
	draftBefore := m.editor.draft

	// Call View() — must not mutate state
	_ = m.View()

	if m.isDirty() {
		t.Fatal("View() should not make editor dirty")
	}
	if m.editor.draft != draftBefore {
		t.Fatal("View() should not mutate editor draft")
	}
}

func TestModel_EditorCleanEscDoesNotPrompt(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open create editor
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)

	// Esc immediately (no edits made)
	updated, _ = m.Update(press(tea.KeyEsc, "esc", 0))
	m = updated.(Model)

	if m.state == stateConfirmDiscard {
		t.Fatal("clean cancel should not trigger discard prompt")
	}
	if m.state != stateReady {
		t.Fatalf("expected stateReady, got %d", m.state)
	}
	if m.editor != nil {
		t.Fatal("editor should be nil after clean cancel")
	}
}

func TestModel_ReadOnlyJobBlocksToggle(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Mark the first job as read-only (simulating system source)
	m.jobs[0].ReadOnly = true

	updated, cmd := m.Update(press(' ', "space", 0))
	m = updated.(Model)

	if cmd != nil {
		t.Fatal("space on read-only job should not trigger a command")
	}
	if m.state != stateReady {
		t.Fatalf("state should remain stateReady, got %d", m.state)
	}
	if m.bannerMsg == nil {
		t.Fatal("expected a banner message about read-only")
	}
	if m.bannerMsg.isError {
		t.Fatal("read-only banner should not be an error banner")
	}
}

func TestModel_ReadOnlyJobBlocksDelete(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	m.jobs[0].ReadOnly = true

	updated, _ := m.Update(press('d', "d", 0))
	m = updated.(Model)

	if m.state == stateConfirmDelete {
		t.Fatal("d on read-only job should not enter confirm delete state")
	}
	if m.state != stateReady {
		t.Fatalf("state should remain stateReady, got %d", m.state)
	}
	if m.bannerMsg == nil {
		t.Fatal("expected a banner message about read-only")
	}
}

func TestModel_ReadOnlyJobBlocksEdit(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	m.jobs[0].ReadOnly = true

	updated, _ := m.Update(press('e', "e", 0))
	m = updated.(Model)

	if m.state == stateEditing {
		t.Fatal("e on read-only job should not enter editing state")
	}
	if m.state != stateReady {
		t.Fatalf("state should remain stateReady, got %d", m.state)
	}
	if m.editor != nil {
		t.Fatal("editor should not be opened for read-only job")
	}
	if m.bannerMsg == nil {
		t.Fatal("expected a banner message about read-only")
	}
}

func TestModel_ReadOnlyJobAllowsRun(t *testing.T) {
	m := newTestModel("0 3 * * * /bin/echo hello\n")
	m = loadModel(m)

	m.jobs[0].ReadOnly = true

	updated, cmd := m.Update(press('x', "x", 0))
	m = updated.(Model)

	if m.state != stateRunning {
		t.Fatalf("x on read-only job should still start a run, got state %d", m.state)
	}
	if cmd == nil {
		t.Fatal("should have returned a command for the run")
	}
}

func TestModel_FilterMatchesSourceLabel(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	// Set source label on first job
	m.jobs[0].Source.Label = "cron.d/backups"

	// Enter filter mode
	updated, _ := m.Update(press('/', "/", 0))
	m = updated.(Model)

	for _, ch := range "cron.d" {
		updated, _ = m.Update(press(ch, string(ch), 0))
		m = updated.(Model)
	}

	if len(m.filteredIdx) != 1 {
		t.Fatalf("expected 1 filtered result matching source label, got %d", len(m.filteredIdx))
	}
}

func TestModel_FilterMatchesRunAsUser(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n@daily /usr/local/bin/cleanup\n")
	m = loadModel(m)

	m.jobs[0].RunAsUser = "www-data"

	updated, _ := m.Update(press('/', "/", 0))
	m = updated.(Model)

	for _, ch := range "www-data" {
		updated, _ = m.Update(press(ch, string(ch), 0))
		m = updated.(Model)
	}

	if len(m.filteredIdx) != 1 {
		t.Fatalf("expected 1 filtered result matching RunAsUser, got %d", len(m.filteredIdx))
	}
}

func TestModel_EditorViewMultipleCalls(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	// Open editor and type something
	updated, _ := m.Update(press('n', "n", 0))
	m = updated.(Model)
	updated, _ = m.Update(press('5', "5", 0))
	m = updated.(Model)

	// Call View() multiple times — must be idempotent
	for i := 0; i < 5; i++ {
		_ = m.View()
	}

	// Draft should still match what we typed, not be corrupted
	preview := m.previewDraft()
	if preview.Minute != "05" {
		t.Fatalf("after multiple View() calls, minute should be '05', got %q", preview.Minute)
	}
}

func TestModel_PeriodicJobShowsNonDeterministicNote(t *testing.T) {
	m := newTestModel("0 3 * * * /usr/local/bin/backup-db\n")
	m = loadModel(m)

	m.jobs[0].Schedule.Kind = domain.ScheduleKindPeriodic
	m.jobs[0].Schedule.Expression = "daily"

	m.width = 120
	m.height = 40

	view := m.View()
	content := view.Content
	if !strings.Contains(content, "non-deterministic") {
		t.Fatal("expected periodic job details to contain 'non-deterministic' note")
	}
}
