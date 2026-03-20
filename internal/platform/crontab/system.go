package crontab

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

type SystemClient struct{}

func NewSystemClient() *SystemClient {
	return &SystemClient{}
}

func (s *SystemClient) Read(ctx context.Context) (string, ReadMeta, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", ReadMeta{}, fmt.Errorf("cannot determine current user: %w", err)
	}
	meta := ReadMeta{User: currentUser.Username}

	cmd := exec.CommandContext(ctx, "crontab", "-l")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if isNoCrontabError(stderrStr) {
			meta.IsEmpty = true
			return "", meta, nil
		}
		return "", meta, fmt.Errorf("crontab -l failed: %s", stderrStr)
	}

	return stdout.String(), meta, nil
}

func (s *SystemClient) Apply(ctx context.Context, content string) (ApplyResult, error) {
	tmpFile, err := os.CreateTemp("", "lazycron-apply-*.crontab")
	if err != nil {
		return ApplyResult{}, fmt.Errorf("cannot create temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return ApplyResult{}, fmt.Errorf("cannot write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return ApplyResult{}, fmt.Errorf("cannot close temp file: %w", err)
	}

	cmd := exec.CommandContext(ctx, "crontab", tmpFile.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	stderrStr := strings.TrimSpace(stderr.String())
	result := ApplyResult{Stderr: stderrStr}

	if err != nil {
		return result, fmt.Errorf("crontab apply failed: %s", stderrStr)
	}

	return result, nil
}

func isNoCrontabError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "no crontab for")
}

var _ Client = (*SystemClient)(nil)
