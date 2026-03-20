package systemcron

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

// fakeFS implements FS for testing.
type fakeFS struct {
	files map[string]fakeFile
	dirs  map[string][]fakeDirEntry
}

type fakeFile struct {
	content string
	info    fakeFileInfo
}

type fakeFileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeDirEntry struct {
	name  string
	isDir bool
	mode  fs.FileMode
}

func (e fakeDirEntry) Name() string      { return e.name }
func (e fakeDirEntry) IsDir() bool       { return e.isDir }
func (e fakeDirEntry) Type() fs.FileMode { return e.mode }
func (e fakeDirEntry) Info() (fs.FileInfo, error) {
	return fakeFileInfo{name: e.name, mode: e.mode}, nil
}

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(file.content), nil
}

func (f *fakeFS) Stat(name string) (fs.FileInfo, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return file.info, nil
}

func (f *fakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, ok := f.dirs[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	result := make([]fs.DirEntry, len(entries))
	for i := range entries {
		result[i] = entries[i]
	}
	return result, nil
}

func TestDiscoverAll_EtcCrontab(t *testing.T) {
	content := "SHELL=/bin/bash\n0 3 * * * root /usr/local/bin/backup\n"
	fake := &fakeFS{
		files: map[string]fakeFile{
			"/etc/crontab": {
				content: content,
				info:    fakeFileInfo{name: "crontab", mode: 0644},
			},
		},
	}

	d := NewWithFS(fake)
	sources, _, issues := d.DiscoverAll()

	var found *DiscoveredSource
	for i := range sources {
		if sources[i].Source.Path == "/etc/crontab" {
			found = &sources[i]
			break
		}
	}

	if found == nil {
		t.Fatal("expected to discover /etc/crontab")
	}
	if found.Source.Subkind != domain.SubkindSystemCrontab {
		t.Errorf("expected subkind system_crontab, got %s", found.Source.Subkind)
	}
	if found.Text != content {
		t.Errorf("expected content to match, got %q", found.Text)
	}
	if found.Source.Kind != domain.SourceKindSystem {
		t.Errorf("expected kind system, got %s", found.Source.Kind)
	}

	for _, issue := range issues {
		if issue.Severity == domain.IssueSeverityError {
			t.Errorf("unexpected error issue: %s", issue.Message)
		}
	}
}

func TestDiscoverAll_CronDDir(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{
			"/etc/cron.d/backups": {
				content: "0 2 * * * root /usr/local/bin/backup\n",
				info:    fakeFileInfo{name: "backups", mode: 0644},
			},
			"/etc/cron.d/monitoring": {
				content: "*/5 * * * * root /usr/local/bin/check\n",
				info:    fakeFileInfo{name: "monitoring", mode: 0644},
			},
		},
		dirs: map[string][]fakeDirEntry{
			"/etc/cron.d": {
				{name: "backups", mode: 0644},
				{name: "monitoring", mode: 0644},
				{name: ".placeholder", mode: 0644},
			},
		},
	}

	d := NewWithFS(fake)
	sources, _, _ := d.DiscoverAll()

	cronDSources := 0
	for _, s := range sources {
		if s.Source.Subkind == domain.SubkindCronD {
			cronDSources++
		}
	}

	if cronDSources != 2 {
		t.Fatalf("expected 2 cron.d sources (skipping .placeholder), got %d", cronDSources)
	}
}

func TestDiscoverAll_PeriodicDir(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{
			"/etc/cron.daily":            {info: fakeFileInfo{name: "daily", mode: fs.ModeDir | 0755}},
			"/etc/cron.daily/logrotate":  {content: "#!/bin/sh\nlogrotate /etc/logrotate.conf\n", info: fakeFileInfo{name: "logrotate", mode: 0755}},
			"/etc/cron.daily/apt-compat": {content: "#!/bin/sh\napt-get update\n", info: fakeFileInfo{name: "apt-compat", mode: 0755}},
			"/etc/cron.daily/bad.name":   {content: "#!/bin/sh\necho bad\n", info: fakeFileInfo{name: "bad.name", mode: 0755}},
		},
		dirs: map[string][]fakeDirEntry{
			"/etc/cron.daily": {
				{name: "logrotate", mode: 0755},
				{name: "apt-compat", mode: 0755},
				{name: "bad.name", mode: 0755},
			},
		},
	}

	d := NewWithFS(fake)
	_, periodic, issues := d.DiscoverAll()

	dailyEntries := 0
	for _, p := range periodic {
		if p.Interval == domain.PeriodicDaily {
			dailyEntries++
		}
	}
	if dailyEntries != 3 {
		t.Fatalf("expected 3 daily periodic entries, got %d", dailyEntries)
	}

	hasNameWarning := false
	for _, issue := range issues {
		if issue.Severity == domain.IssueSeverityWarning {
			if strings.Contains(issue.Message, "bad.name") && strings.Contains(issue.Message, "run-parts") {
				hasNameWarning = true
			}
		}
	}
	if !hasNameWarning {
		t.Error("expected a warning about bad.name not matching run-parts conventions")
	}
}

func TestDiscoverAll_MissingPaths(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{},
		dirs:  map[string][]fakeDirEntry{},
	}

	d := NewWithFS(fake)
	sources, periodic, issues := d.DiscoverAll()

	if len(sources) != 0 {
		t.Errorf("expected 0 sources for empty fs, got %d", len(sources))
	}
	if len(periodic) != 0 {
		t.Errorf("expected 0 periodic entries for empty fs, got %d", len(periodic))
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty fs, got %d", len(issues))
	}
}

func TestDiscoverAll_UnreadableFile(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{
			"/etc/crontab": {
				content: "", // ReadFile will return ErrNotExist since content is empty but stat succeeds
				info:    fakeFileInfo{name: "crontab", mode: 0600},
			},
		},
	}

	// Override ReadFile to simulate permission denied
	permDeniedFS := &permDeniedFakeFS{inner: fake, denyRead: "/etc/crontab"}

	d := NewWithFS(permDeniedFS)
	sources, _, issues := d.DiscoverAll()

	if len(sources) != 1 {
		t.Fatalf("expected 1 source (unreadable), got %d", len(sources))
	}
	if sources[0].Source.Access.Readable {
		t.Error("expected source to be marked as not readable")
	}

	hasPermIssue := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "cannot read") {
			hasPermIssue = true
		}
	}
	if !hasPermIssue {
		t.Error("expected a permission warning issue")
	}
}

func TestShouldSkipCronDFile(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{"backups", false},
		{"monitoring", false},
		{".placeholder", true},
		{"file~", true},
		{"pkg.dpkg-dist", true},
		{"pkg.dpkg-old", true},
		{"pkg.rpmsave", true},
		{"pkg.rpmnew", true},
		{"valid-name", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipCronDFile(tt.name)
			if got != tt.skip {
				t.Errorf("shouldSkipCronDFile(%q) = %v, want %v", tt.name, got, tt.skip)
			}
		})
	}
}

func TestRunPartsNameRe(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"logrotate", true},
		{"apt-compat", true},
		{"my_script", true},
		{"bad.name", false},
		{"has space", false},
		{"file.sh", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runPartsNameRe.MatchString(tt.name)
			if got != tt.valid {
				t.Errorf("runPartsNameRe.MatchString(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestDiscoverAll_NonExecutablePeriodicEntry(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{
			"/etc/cron.hourly":        {info: fakeFileInfo{name: "hourly", mode: fs.ModeDir | 0755}},
			"/etc/cron.hourly/noexec": {content: "#!/bin/sh\necho test\n", info: fakeFileInfo{name: "noexec", mode: 0644}},
		},
		dirs: map[string][]fakeDirEntry{
			"/etc/cron.hourly": {
				{name: "noexec", mode: 0644},
			},
		},
	}

	d := NewWithFS(fake)
	_, periodic, issues := d.DiscoverAll()

	if len(periodic) != 1 {
		t.Fatalf("expected 1 periodic entry, got %d", len(periodic))
	}
	if periodic[0].Executable {
		t.Error("expected entry to be marked as not executable")
	}

	hasExecWarning := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "not executable") {
			hasExecWarning = true
		}
	}
	if !hasExecWarning {
		t.Error("expected a warning about non-executable entry")
	}
}

// permDeniedFakeFS wraps a fakeFS but denies ReadFile/ReadDir for specific paths.
type permDeniedFakeFS struct {
	inner       *fakeFS
	denyRead    string
	denyReadDir string
}

func (f *permDeniedFakeFS) ReadFile(name string) ([]byte, error) {
	if name == f.denyRead {
		return nil, fs.ErrPermission
	}
	return f.inner.ReadFile(name)
}

func (f *permDeniedFakeFS) Stat(name string) (fs.FileInfo, error) {
	return f.inner.Stat(name)
}

func (f *permDeniedFakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == f.denyReadDir {
		return nil, fs.ErrPermission
	}
	return f.inner.ReadDir(name)
}

func TestDiscoverAll_PermissionDeniedCronDDir(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{},
		dirs: map[string][]fakeDirEntry{
			"/etc/cron.d": {{name: "job", mode: 0644}},
		},
	}
	pdFS := &permDeniedFakeFS{inner: fake, denyReadDir: "/etc/cron.d"}

	d := NewWithFS(pdFS)
	sources, _, issues := d.DiscoverAll()

	for _, s := range sources {
		if s.Source.Subkind == domain.SubkindCronD {
			t.Error("expected no cron.d sources when directory is unreadable")
		}
	}

	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "cannot read directory") && strings.Contains(issue.Message, "/etc/cron.d") {
			found = true
			if issue.LineIndex != -1 {
				t.Errorf("expected LineIndex -1 for source-level issue, got %d", issue.LineIndex)
			}
		}
	}
	if !found {
		t.Error("expected a warning issue about unreadable /etc/cron.d directory")
	}
}

func TestDiscoverAll_PermissionDeniedPeriodicDir(t *testing.T) {
	fake := &fakeFS{
		files: map[string]fakeFile{},
		dirs: map[string][]fakeDirEntry{
			"/etc/cron.daily": {{name: "script", mode: 0755}},
		},
	}
	pdFS := &permDeniedFakeFS{inner: fake, denyReadDir: "/etc/cron.daily"}

	d := NewWithFS(pdFS)
	_, periodic, issues := d.DiscoverAll()

	if len(periodic) != 0 {
		t.Errorf("expected 0 periodic entries when directory is unreadable, got %d", len(periodic))
	}

	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "cannot read directory") && strings.Contains(issue.Message, "/etc/cron.daily") {
			found = true
			if issue.LineIndex != -1 {
				t.Errorf("expected LineIndex -1 for source-level issue, got %d", issue.LineIndex)
			}
		}
	}
	if !found {
		t.Error("expected a warning issue about unreadable /etc/cron.daily directory")
	}
}

func TestDiscoverAll_SourceLevelIssueLineIndex(t *testing.T) {
	pdFS := &permDeniedFakeFS{
		inner: &fakeFS{
			files: map[string]fakeFile{
				"/etc/crontab": {content: "", info: fakeFileInfo{name: "crontab", mode: 0600}},
			},
		},
		denyRead: "/etc/crontab",
	}

	d := NewWithFS(pdFS)
	_, _, issues := d.DiscoverAll()

	for _, issue := range issues {
		if issue.LineIndex != -1 {
			t.Errorf("expected all discovery issues to have LineIndex -1, got %d for: %s", issue.LineIndex, issue.Message)
		}
	}
}
