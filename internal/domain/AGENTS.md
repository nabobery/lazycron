# DOMAIN MODULE

Core types and validation for lazycron.

## FILES

| File | Role |
|------|------|
| `types.go` | Core types: CronJob, CronDocument, CronLine, ScheduleSpec, RunRecord |
| `validation.go` | JobDraft, FieldError, ValidateDraft |
| `fingerprint.go` | Job fingerprint computation |
| `fingerprint_test.go` | Fingerprint tests |
| `validation_test.go` | Draft validation tests |

## JOB DRAFT

```go
type JobDraft struct {
    Enabled     bool
    SchedKind   ScheduleKind   // standard / descriptor / reboot
    Minute      string         // 5-field cron components
    Hour        string
    DayOfMonth  string
    Month       string
    DayOfWeek   string
    Descriptor  string         // @daily, @hourly, @every Nm, @reboot
    Timezone    string
    TimezoneKey string         // CRON_TZ or TZ
    Command     string
}
```

## BUILDERS

- `draft.Expression()` → Full cron expression string
- `draft.RawLine()` → Complete crontab line (with DisabledMarker if disabled)

## VALIDATION

`ValidateDraft()` returns `[]FieldError` for:
- Standard fields: numeric ranges, cron syntax chars
- Descriptor: must start with @, valid keywords
- Command: must not be empty

## ANTI-PATTERNS

- Never skip validation before apply
- Never allow control characters in fields
- Never mutate JobDraft in View layer
