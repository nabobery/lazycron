package cronparse

import (
	"fmt"
	"strings"

	"github.com/avinashchangrani/lazycron/internal/domain"
	cron "github.com/robfig/cron/v3"
)

var standardParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

func Parse(text string, source domain.CronSource) (domain.CronDocument, []domain.CronJob, []domain.ValidationIssue) {
	doc := domain.CronDocument{
		Source: source,
		Raw:    text,
	}

	if text == "" {
		return doc, nil, nil
	}

	rawLines := splitLines(text)
	var jobs []domain.CronJob
	var issues []domain.ValidationIssue
	var envContext []domain.EnvAssignment

	for i, raw := range rawLines {
		line := classifyLine(i, raw, source, envContext)
		doc.Lines = append(doc.Lines, line)

		if line.Kind == domain.LineKindEnv {
			envContext = append(envContext, domain.EnvAssignment{
				Key:   line.EnvKey,
				Value: line.EnvVal,
			})
		}

		if line.Job != nil {
			jobs = append(jobs, *line.Job)
		}
		if line.Issue != nil {
			issues = append(issues, *line.Issue)
		}
	}

	return doc, jobs, issues
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func classifyLine(index int, raw string, source domain.CronSource, envContext []domain.EnvAssignment) domain.CronLine {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return domain.CronLine{Index: index, Raw: raw, Kind: domain.LineKindBlank}
	}

	if strings.HasPrefix(trimmed, domain.DisabledMarker) {
		return parseDisabledLine(index, raw, trimmed, source, envContext)
	}

	if strings.HasPrefix(trimmed, "#") {
		return domain.CronLine{Index: index, Raw: raw, Kind: domain.LineKindComment}
	}

	// Check for CRON_TZ= or TZ= prefixed job lines before env assignment
	if strings.HasPrefix(trimmed, "CRON_TZ=") || strings.HasPrefix(trimmed, "TZ=") {
		if indexAnyWhitespace(trimmed) != -1 {
			return parseJobLine(index, raw, trimmed, source, envContext, true)
		}
	}

	if key, val, ok := parseEnvAssignment(trimmed); ok {
		return domain.CronLine{
			Index:  index,
			Raw:    raw,
			Kind:   domain.LineKindEnv,
			EnvKey: key,
			EnvVal: val,
		}
	}

	return parseJobLine(index, raw, trimmed, source, envContext, true)
}

func parseDisabledLine(index int, raw, trimmed string, source domain.CronSource, envContext []domain.EnvAssignment) domain.CronLine {
	originalLine := strings.TrimPrefix(trimmed, domain.DisabledMarker)
	jobLine := parseJobLine(index, raw, originalLine, source, envContext, false)
	jobLine.Kind = domain.LineKindDisabled
	jobLine.Enabled = false
	if jobLine.Job != nil {
		jobLine.Job.Enabled = false
	}
	return jobLine
}

func parseJobLine(index int, raw, trimmed string, source domain.CronSource, envContext []domain.EnvAssignment, enabled bool) domain.CronLine {
	line := domain.CronLine{
		Index:   index,
		Raw:     raw,
		Enabled: enabled,
	}

	expr, command, tz, err := extractScheduleAndCommand(trimmed)
	if err != nil {
		line.Kind = domain.LineKindInvalid
		line.Issue = &domain.ValidationIssue{
			LineIndex: index,
			Message:   err.Error(),
			Severity:  domain.IssueSeverityWarning,
		}
		return line
	}

	schedKind := classifyScheduleKind(expr)

	if schedKind == domain.ScheduleKindReboot {
		line.Kind = domain.LineKindJob
		job := buildJob(index, raw, source, envContext, enabled, domain.ScheduleSpec{
			Kind:       domain.ScheduleKindReboot,
			Expression: expr,
			Timezone:   tz,
		}, command)
		line.Job = &job
		return line
	}

	// Validate with robfig/cron parser
	_, parseErr := standardParser.Parse(expr)
	if parseErr != nil {
		line.Kind = domain.LineKindInvalid
		line.Issue = &domain.ValidationIssue{
			LineIndex: index,
			Message:   fmt.Sprintf("invalid cron expression %q: %v", expr, parseErr),
			Severity:  domain.IssueSeverityWarning,
		}
		return line
	}

	line.Kind = domain.LineKindJob
	job := buildJob(index, raw, source, envContext, enabled, domain.ScheduleSpec{
		Kind:       schedKind,
		Expression: expr,
		Timezone:   tz,
	}, command)
	line.Job = &job
	return line
}

func buildJob(index int, raw string, source domain.CronSource, envContext []domain.EnvAssignment, enabled bool, spec domain.ScheduleSpec, command string) domain.CronJob {
	envCopy := make([]domain.EnvAssignment, len(envContext))
	copy(envCopy, envContext)

	return domain.CronJob{
		ID:          fmt.Sprintf("%s:%s:line-%d", source.Kind, source.User, index),
		LineIndex:   index,
		Source:      source,
		Enabled:     enabled,
		RawLine:     raw,
		Schedule:    spec,
		Command:     command,
		EnvContext:  envCopy,
		Fingerprint: domain.ComputeFingerprint(spec.Expression, command),
	}
}

func indexAnyWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return i
		}
	}
	return -1
}

func extractScheduleAndCommand(trimmed string) (expr, command, tz string, err error) {
	// Handle CRON_TZ= or TZ= prefix
	if strings.HasPrefix(trimmed, "CRON_TZ=") || strings.HasPrefix(trimmed, "TZ=") {
		wsIdx := indexAnyWhitespace(trimmed)
		if wsIdx == -1 {
			return "", "", "", fmt.Errorf("malformed timezone prefix: %q", trimmed)
		}
		prefix := trimmed[:wsIdx]
		trimmed = strings.TrimSpace(trimmed[wsIdx+1:])

		eqIdx := strings.IndexByte(prefix, '=')
		tz = prefix[eqIdx+1:]
	}

	// Handle @reboot, @daily, @hourly, @every, etc.
	if strings.HasPrefix(trimmed, "@") {
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return "", "", "", fmt.Errorf("descriptor %q has no command", trimmed)
		}

		// @every requires two tokens for the expression: "@every <duration>"
		if strings.EqualFold(fields[0], "@every") {
			if len(fields) < 3 {
				return "", "", "", fmt.Errorf("descriptor %q has no command", trimmed)
			}
			expr = fields[0] + " " + fields[1]
			cmd := strings.TrimSpace(trimmed[findCommandStartAfterFields(trimmed, 2):])
			return expr, cmd, tz, nil
		}

		cmd := strings.TrimSpace(trimmed[findCommandStartAfterFields(trimmed, 1):])
		if cmd == "" {
			return "", "", "", fmt.Errorf("descriptor %q has no command", trimmed)
		}
		return fields[0], cmd, tz, nil
	}

	// Standard 5-field: min hour dom month dow command...
	fields := strings.Fields(trimmed)
	if len(fields) < 6 {
		return "", "", "", fmt.Errorf("too few fields (%d) for standard cron line", len(fields))
	}

	expr = strings.Join(fields[:5], " ")
	command = strings.TrimSpace(trimmed[findCommandStart(trimmed, 5):])
	return expr, command, tz, nil
}

func findCommandStartAfterFields(line string, skipFields int) int {
	return findCommandStart(line, skipFields)
}

func findCommandStart(line string, skipFields int) int {
	i := 0
	n := len(line)

	for field := 0; field < skipFields; field++ {
		// skip leading whitespace
		for i < n && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		// skip field content
		for i < n && line[i] != ' ' && line[i] != '\t' {
			i++
		}
	}

	// skip whitespace between last schedule field and command
	for i < n && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	return i
}

func classifyScheduleKind(expr string) domain.ScheduleKind {
	if strings.EqualFold(expr, "@reboot") {
		return domain.ScheduleKindReboot
	}
	if strings.HasPrefix(expr, "@") {
		return domain.ScheduleKindDescriptor
	}
	return domain.ScheduleKindStandard
}

func parseEnvAssignment(trimmed string) (key, val string, ok bool) {
	if strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	eqIdx := strings.IndexByte(trimmed, '=')
	if eqIdx == -1 {
		return "", "", false
	}

	key = trimmed[:eqIdx]
	// Env keys must be valid identifiers (letters, digits, underscores, starting with letter/underscore)
	if !isValidEnvKey(key) {
		return "", "", false
	}

	val = trimmed[eqIdx+1:]
	// Strip surrounding quotes if present
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		val = val[1 : len(val)-1]
	}
	return key, val, true
}

func isValidEnvKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i, ch := range key {
		if i == 0 {
			if isAlpha(ch) || ch == '_' {
				continue
			}
			return false
		} else {
			if isAlpha(ch) || isDigit(ch) || ch == '_' {
				continue
			}
			return false
		}
	}
	return true
}

func isAlpha(ch rune) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// Render reconstructs the document text from its lines.
func Render(doc domain.CronDocument) string {
	if len(doc.Lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, line := range doc.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line.Raw)
	}
	sb.WriteByte('\n')
	return sb.String()
}
