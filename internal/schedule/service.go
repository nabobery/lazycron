package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/nabobery/lazycron/internal/domain"
	cron "github.com/robfig/cron/v3"
)

var standardParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) NextRuns(spec domain.ScheduleSpec, now time.Time, n int) ([]time.Time, error) {
	if spec.Kind == domain.ScheduleKindReboot || spec.Kind == domain.ScheduleKindPeriodic {
		return nil, nil
	}

	schedule, err := standardParser.Parse(spec.Expression)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", spec.Expression, err)
	}

	refTime := now
	if spec.Timezone != "" {
		loc, err := time.LoadLocation(spec.Timezone)
		if err != nil {
			return nil, fmt.Errorf("unknown timezone %q: %w", spec.Timezone, err)
		}
		refTime = now.In(loc)
	}

	runs := make([]time.Time, 0, n)
	t := refTime
	for i := 0; i < n; i++ {
		t = schedule.Next(t)
		if t.IsZero() {
			break
		}
		runs = append(runs, t)
	}

	return runs, nil
}

func (s *Service) Describe(spec domain.ScheduleSpec) string {
	if spec.Kind == domain.ScheduleKindReboot {
		return "Runs on system reboot"
	}

	if spec.Kind == domain.ScheduleKindPeriodic {
		return describePeriodicInterval(spec.Expression)
	}

	desc := describeExpression(spec.Expression)
	if spec.Timezone != "" {
		desc += fmt.Sprintf(" (%s)", spec.Timezone)
	}
	return desc
}

func describePeriodicInterval(interval string) string {
	switch domain.PeriodicInterval(interval) {
	case domain.PeriodicHourly:
		return "Hourly (periodic directory; via run-parts)"
	case domain.PeriodicDaily:
		return "Daily (periodic directory; via anacron/run-parts)"
	case domain.PeriodicWeekly:
		return "Weekly (periodic directory; via anacron/run-parts)"
	case domain.PeriodicMonthly:
		return "Monthly (periodic directory; via anacron/run-parts)"
	default:
		return fmt.Sprintf("Periodic: %s (via run-parts)", interval)
	}
}

func describeExpression(expr string) string {
	lower := strings.ToLower(expr)

	switch lower {
	case "@yearly", "@annually":
		return "At midnight on January 1st"
	case "@monthly":
		return "At midnight on the 1st of every month"
	case "@weekly":
		return "At midnight on Sunday"
	case "@daily", "@midnight":
		return "At midnight every day"
	case "@hourly":
		return "At the start of every hour"
	}

	if strings.HasPrefix(lower, "@every ") {
		return "Every " + strings.TrimPrefix(lower, "@every ")
	}

	return describeStandard(expr)
}

func describeStandard(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return expr
	}

	min, hour, dom, mon, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	var parts []string

	// Time part
	if min != "*" && hour != "*" {
		parts = append(parts, fmt.Sprintf("At %s:%s", padTime(hour), padTime(min)))
	} else if min == "*" && hour == "*" {
		parts = append(parts, "Every minute")
	} else if min != "*" && hour == "*" {
		if strings.HasPrefix(min, "*/") {
			parts = append(parts, fmt.Sprintf("Every %s minutes", strings.TrimPrefix(min, "*/")))
		} else {
			parts = append(parts, fmt.Sprintf("At minute %s of every hour", min))
		}
	} else {
		if strings.HasPrefix(hour, "*/") {
			parts = append(parts, fmt.Sprintf("Every %s hours at minute %s", strings.TrimPrefix(hour, "*/"), min))
		} else {
			parts = append(parts, fmt.Sprintf("At %s:%s", padTime(hour), padTime(min)))
		}
	}

	// Day of month
	if dom != "*" {
		parts = append(parts, fmt.Sprintf("on day %s", dom))
	}

	// Month
	if mon != "*" {
		parts = append(parts, fmt.Sprintf("in %s", monthName(mon)))
	}

	// Day of week
	if dow != "*" {
		parts = append(parts, fmt.Sprintf("on %s", dowName(dow)))
	}

	return strings.Join(parts, " ")
}

func padTime(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func monthName(s string) string {
	months := map[string]string{
		"1": "January", "2": "February", "3": "March", "4": "April",
		"5": "May", "6": "June", "7": "July", "8": "August",
		"9": "September", "10": "October", "11": "November", "12": "December",
	}
	if name, ok := months[s]; ok {
		return name
	}
	return s
}

func dowName(s string) string {
	days := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday",
	}
	if name, ok := days[s]; ok {
		return name
	}
	return s
}
