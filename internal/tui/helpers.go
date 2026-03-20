package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func toLower(s string) string {
	return strings.ToLower(s)
}

func containsLower(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), needle)
}

// truncate truncates s to maxLen visible cells, accounting for ANSI escape
// sequences and wide (East Asian) characters. Appends "…" when truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxLen {
		return s
	}
	return ansi.Truncate(s, maxLen, "…")
}

func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// stripControlCodes removes ANSI escape sequences and non-printable control
// characters (except \n and \t) to prevent log output from corrupting the TUI.
func stripControlCodes(s string) string {
	s = ansi.Strip(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var secretKeyPatterns = []string{
	"TOKEN", "SECRET", "PASSWORD", "PASSWD", "KEY", "CREDENTIAL",
	"AUTH", "API_KEY", "APIKEY",
}

// maskSecretValue returns "******" if the env key looks like it holds a secret.
func maskSecretValue(key, value string) string {
	upper := strings.ToUpper(key)
	for _, pattern := range secretKeyPatterns {
		if strings.Contains(upper, pattern) {
			return "******"
		}
	}
	return value
}
