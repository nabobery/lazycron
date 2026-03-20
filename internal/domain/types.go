package domain

import "time"

type SourceKind string

const (
	SourceKindUserCrontab SourceKind = "user_crontab"
	SourceKindSystem      SourceKind = "system"
)

type CronSource struct {
	Kind SourceKind
	Path string
	User string
}

type LineKind string

const (
	LineKindBlank    LineKind = "blank"
	LineKindComment  LineKind = "comment"
	LineKindEnv      LineKind = "env"
	LineKindJob      LineKind = "job"
	LineKindDisabled LineKind = "disabled"
	LineKindInvalid  LineKind = "invalid"
)

type CronLine struct {
	Index   int
	Raw     string
	Kind    LineKind
	EnvKey  string // populated when Kind == LineKindEnv
	EnvVal  string
	Job     *CronJob         // populated when Kind == LineKindJob
	Issue   *ValidationIssue // populated when Kind == LineKindInvalid
	Enabled bool             // true for job lines, false for disabled
}

type CronDocument struct {
	Source CronSource
	Lines  []CronLine
	Raw    string // original full text for drift comparison
}

type ScheduleKind string

const (
	ScheduleKindStandard   ScheduleKind = "standard_5_field"
	ScheduleKindDescriptor ScheduleKind = "descriptor"
	ScheduleKindReboot     ScheduleKind = "reboot"
)

type ScheduleSpec struct {
	Kind       ScheduleKind
	Expression string
	Timezone   string // from CRON_TZ= or TZ= prefix, empty if none
}

type CronJob struct {
	ID          string
	LineIndex   int
	Source      CronSource
	Enabled     bool
	RawLine     string
	Schedule    ScheduleSpec
	Command     string
	EnvContext  []EnvAssignment // env assignments in scope above this line
	Fingerprint string
}

type EnvAssignment struct {
	Key   string
	Value string
}

type ValidationIssue struct {
	LineIndex int
	Message   string
	Severity  IssueSeverity
}

type IssueSeverity string

const (
	IssueSeverityWarning IssueSeverity = "warning"
	IssueSeverityError   IssueSeverity = "error"
)

type EnvMode string

const (
	EnvModeCronLike     EnvMode = "cron_like"
	EnvModeShellInherit EnvMode = "shell_inherit"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSuccess   RunStatus = "success"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

type RunRecord struct {
	JobID      string
	StartedAt  time.Time
	FinishedAt time.Time
	Duration   time.Duration
	ExitCode   int
	Mode       EnvMode
	Stdout     string
	Stderr     string
	Status     RunStatus
	Truncated  bool
}

const DisabledMarker = "# [lazycron-disabled] "
