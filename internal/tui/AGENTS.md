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
| `editor.go` | Modal create/edit form with field navigation + validation |
| `model_test.go` | Model state transition tests |

## MODEL FIELDS

| Field | Type | Notes |
|-------|------|-------|
| `logsProvider` | `cronlogs.Provider` | System log fetcher (set via SetLogsProvider) |
| `systemLogs` | `*cronlogs.Result` | Cached system log results |
| `runEnvMode` | `domain.EnvMode` | CronLike (default) vs ShellInherit |

## SYSTEM LOG FETCHING

- `sysLogResultMsg` received after async log fetch
- Logs pane shows both run output and system logs
- Provider chain: journalctl → syslog → noop

## FILTER TOKENS

Enhanced filter supports:
- `user:foo` — filter by run-as user
- `source:bar` — filter by source path
- Free text — matches command, schedule, ID, label

## STATE MACHINE

```
stateLoading → stateReady → stateFiltering/Running/Applying/Editing/Creating
stateEditing/Creating → stateConfirmDiscard (on dirty escape)
```

## EDITOR (create/edit)

| Key | Action |
|-----|--------|
| `n` | Open create editor |
| `e` | Open edit editor (selected job) |
| Tab/Shift+Tab | Navigate fields |
| Enter | Save draft |
| Esc | Cancel (confirms if dirty) |
| Left/Right | Cycle schedule kind |

## FIELD TYPES

| Field | Type | Notes |
|-------|------|-------|
| SchedKind | Enum | standard / descriptor / reboot |
| Minute-Hour-DOM-Month-DOW | Text | Standard cron fields |
| Descriptor | Text | @daily, @hourly, @every Nm |
| Timezone | Text | Optional CRON_TZ= prefix |
| Command | Text | The command to run |

## ANTI-PATTERNS

- Never block the UI thread—always use `tea.Cmd` for I/O
- Never mutate Model directly in View—only in Update
- Never assume pane dimensions—always subtract border width
- Never save without validation—show field errors inline
