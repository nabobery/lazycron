package domain

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// PeriodicInterval represents the cadence of a periodic directory.
type PeriodicInterval string

const (
	PeriodicHourly  PeriodicInterval = "hourly"
	PeriodicDaily   PeriodicInterval = "daily"
	PeriodicWeekly  PeriodicInterval = "weekly"
	PeriodicMonthly PeriodicInterval = "monthly"
)

// SourceKey returns a short stable identifier for a CronSource suitable for
// use in job IDs. User crontab sources return "user_crontab:<user>"; system
// sources return a short hash of the path to avoid collisions.
func SourceKey(s CronSource) string {
	if s.Kind == SourceKindUserCrontab {
		return fmt.Sprintf("%s:%s", s.Kind, s.User)
	}
	h := sha256.Sum256([]byte(s.Path))
	return fmt.Sprintf("sys:%x", h[:4])
}

type SourceKind string

const (
	SourceKindUserCrontab SourceKind = "user_crontab"
	SourceKindSystem      SourceKind = "system"
)

type SourceSubkind string

const (
	SubkindNone          SourceSubkind = ""
	SubkindSystemCrontab SourceSubkind = "system_crontab"
	SubkindCronD         SourceSubkind = "cron_d"
	SubkindPeriodicDir   SourceSubkind = "periodic_dir"
)

// SourceAccess captures OS-level read/write status for a cron source.
type SourceAccess struct {
	Readable bool
	Writable bool
	Reason   string // human-readable explanation when access is limited
}

type CronSource struct {
	Kind    SourceKind
	Subkind SourceSubkind
	Path    string
	User    string
	Label   string // short display label, e.g. "/etc/crontab" or "cron.d/backups"
	Owner   string // best-effort file owner username or uid
	Access  SourceAccess
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
	ScheduleKindPeriodic   ScheduleKind = "periodic"
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
	RunAsUser   string // system cron user field; empty for user crontab jobs
	ReadOnly    bool   // true for system sources (not mutable via lazycron)
}

type EnvAssignment struct {
	Key   string
	Value string
}

type ValidationIssue struct {
	LineIndex  int
	SourcePath string
	Message    string
	Severity   IssueSeverity
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
