package domain

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func ComputeFingerprint(schedule, command string) string {
	normalized := strings.TrimSpace(schedule) + "\x00" + strings.TrimSpace(command)
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h[:8])
}
