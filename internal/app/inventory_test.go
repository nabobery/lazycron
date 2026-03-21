package app

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/nabobery/lazycron/internal/domain"
	"github.com/nabobery/lazycron/internal/platform/crontab"
	"github.com/nabobery/lazycron/internal/platform/systemcron"
)

var userSource = domain.CronSource{
	Kind: domain.SourceKindUserCrontab,
	Path: "crontab://current-user",
	User: "testuser",
}

func TestInventoryService_LoadAll_UserOnly(t *testing.T) {
	fc := crontab.NewFakeClient("0 3 * * * /usr/local/bin/backup\n@daily /usr/local/bin/cleanup\n", true)
	applySvc := NewApplyService(fc, userSource)
	invSvc := NewInventoryService(applySvc, nil)

	inv, err := invSvc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(inv.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(inv.Jobs))
	}
	for _, job := range inv.Jobs {
		if job.ReadOnly {
			t.Errorf("user job %q should not be read-only", job.ID)
		}
		if !IsJobMutable(job) {
			t.Errorf("user job %q should be mutable", job.ID)
		}
	}
}

func TestInventoryService_LoadAll_WithSystemSources(t *testing.T) {
	fc := crontab.NewFakeClient("0 3 * * * /usr/local/bin/backup\n", true)
	applySvc := NewApplyService(fc, userSource)

	fake := &fakeDiscoveryFS{
		files: map[string]fakeFile{
			"/etc/crontab": {
				content: "0 2 * * * root /usr/local/bin/sys-backup\n",
				info:    fakeFileInfo{name: "crontab", mode: 0644},
			},
		},
	}
	disc := systemcron.NewWithFS(fake)
	invSvc := NewInventoryService(applySvc, disc)

	inv, err := invSvc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(inv.Jobs) < 2 {
		t.Fatalf("expected at least 2 jobs (user + system), got %d", len(inv.Jobs))
	}

	// First job should be user job
	if inv.Jobs[0].ReadOnly {
		t.Error("first job (user) should not be read-only")
	}

	hasSystemJob := false
	for _, job := range inv.Jobs {
		if job.Source.Kind == domain.SourceKindSystem {
			hasSystemJob = true
			if !job.ReadOnly {
				t.Errorf("system job %q should be read-only", job.ID)
			}
			if IsJobMutable(job) {
				t.Errorf("system job %q should not be mutable", job.ID)
			}
		}
	}
	if !hasSystemJob {
		t.Error("expected at least one system job in inventory")
	}
}

func TestInventoryService_LoadAll_MergesIssues(t *testing.T) {
	fc := crontab.NewFakeClient("invalid line\n", true)
	applySvc := NewApplyService(fc, userSource)
	invSvc := NewInventoryService(applySvc, nil)

	inv, err := invSvc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(inv.Issues) == 0 {
		t.Error("expected at least one issue from invalid user crontab line")
	}
}

func TestInventory_JobByID(t *testing.T) {
	inv := Inventory{
		Jobs: []domain.CronJob{
			{ID: "user_crontab:test:line-0", Command: "/bin/first"},
			{ID: "sys:abc12345:line-0", Command: "/bin/second"},
		},
	}

	found := inv.JobByID("sys:abc12345:line-0")
	if found == nil {
		t.Fatal("expected to find job by ID")
	}
	if found.Command != "/bin/second" {
		t.Errorf("expected command /bin/second, got %q", found.Command)
	}

	notFound := inv.JobByID("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestIsJobMutable(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
		want     bool
	}{
		{"user job", false, true},
		{"system job", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := domain.CronJob{ReadOnly: tt.readOnly}
			if got := IsJobMutable(job); got != tt.want {
				t.Errorf("IsJobMutable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeDiscoveryFS implements systemcron.FS for inventory tests.
type fakeDiscoveryFS struct {
	files map[string]fakeFile
	dirs  map[string][]fakeDirEntry
}

type fakeFile struct {
	content string
	info    fakeFileInfo
}

type fakeFileInfo struct {
	name string
	mode fs.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
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

func (f *fakeDiscoveryFS) ReadFile(name string) ([]byte, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(file.content), nil
}

func (f *fakeDiscoveryFS) Stat(name string) (fs.FileInfo, error) {
	file, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return file.info, nil
}

func (f *fakeDiscoveryFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.dirs == nil {
		return nil, fs.ErrNotExist
	}
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
