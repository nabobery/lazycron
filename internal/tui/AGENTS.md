# TUI MODULE

Terminal UI implementation using Bubble Tea v2 MVU pattern.

## FILES

| File | Role |
|------|------|
| `model.go` | Model struct, Init, loadCmd, filter logic |
| `update.go` | Message dispatch, state transitions |
| `update_keyboard.go` | Keyboard handlers (toggle, delete, run, search) |
| `update_mouse.go` | Mouse click handlers for pane focus |
| `view.go` | Three-pane rendering (jobs, details, logs) |
| `styles.go` | Lip Gloss style definitions |
| `types.go` | Internal types (appState, focusedPane, banner) |
| `helpers.go` | String utilities (containsLower, toLower) |
| `model_test.go` | Model state transition tests |

## PATTERNS

- **State machine**: `appState` enum (Loading → Ready → Filtering/Running/Applying)
- **Pane focus**: `focusedPane` enum (Jobs, Details, Logs) with tab/mouse switching
- **Async ops**: `tea.Cmd` functions for Load, Apply, Run results
- **Filter**: Client-side substring match on Command, Expression, ID

## ANTI-PATTERNS

- Never block the UI thread—always use `tea.Cmd` for I/O
- Never mutate Model directly in View—only in Update
- Never assume pane dimensions—always subtract border width
