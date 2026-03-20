package systemcron

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/avinashchangrani/lazycron/internal/domain"
)

// FS abstracts filesystem operations for testability.
type FS interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

// osFS implements FS using the real filesystem.
type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error)       { return os.ReadFile(name) }
func (osFS) Stat(name string) (fs.FileInfo, error)      { return os.Stat(name) }
func (osFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

// DiscoveredSource holds a system cron source with its raw text content.
type DiscoveredSource struct {
	Source domain.CronSource
	Text   string // raw file content; empty if unreadable
}

// PeriodicEntry represents a script found in a periodic directory.
type PeriodicEntry struct {
	Source     domain.CronSource
	Name       string
	Path       string
	Executable bool
	ValidName  bool // matches run-parts naming conventions
	Interval   domain.PeriodicInterval
}

// Discoverer enumerates system cron sources.
type Discoverer struct {
	fs FS
}

// New creates a Discoverer using the real filesystem.
func New() *Discoverer {
	return &Discoverer{fs: osFS{}}
}

// NewWithFS creates a Discoverer with an injected FS for testing.
func NewWithFS(f FS) *Discoverer {
	return &Discoverer{fs: f}
}

// DiscoverAll returns all discoverable system cron sources for the current OS.
func (d *Discoverer) DiscoverAll() ([]DiscoveredSource, []PeriodicEntry, []domain.ValidationIssue) {
	var sources []DiscoveredSource
	var periodic []PeriodicEntry
	var issues []domain.ValidationIssue

	candidates := d.candidates()

	for _, c := range candidates {
		if c.subkind == domain.SubkindPeriodicDir {
			entries, dirIssues := d.discoverPeriodicDir(c)
			periodic = append(periodic, entries...)
			issues = append(issues, dirIssues...)
			continue
		}

		if c.isDir {
			dirSources, dirIssues := d.discoverCronDDir(c)
			sources = append(sources, dirSources...)
			issues = append(issues, dirIssues...)
			continue
		}

		src, srcIssues := d.discoverFile(c)
		if src != nil {
			sources = append(sources, *src)
		}
		issues = append(issues, srcIssues...)
	}

	return sources, periodic, issues
}

type candidate struct {
	path     string
	subkind  domain.SourceSubkind
	isDir    bool
	interval domain.PeriodicInterval // only for periodic dirs
}

func (d *Discoverer) candidates() []candidate {
	var cs []candidate

	cs = append(cs, candidate{
		path:    "/etc/crontab",
		subkind: domain.SubkindSystemCrontab,
	})

	cs = append(cs, candidate{
		path:    "/etc/cron.d",
		subkind: domain.SubkindCronD,
		isDir:   true,
	})

	periodicDirs := []struct {
		path     string
		interval domain.PeriodicInterval
	}{
		{"/etc/cron.hourly", domain.PeriodicHourly},
		{"/etc/cron.daily", domain.PeriodicDaily},
		{"/etc/cron.weekly", domain.PeriodicWeekly},
		{"/etc/cron.monthly", domain.PeriodicMonthly},
	}

	for _, pd := range periodicDirs {
		cs = append(cs, candidate{
			path:     pd.path,
			subkind:  domain.SubkindPeriodicDir,
			isDir:    true,
			interval: pd.interval,
		})
	}

	if runtime.GOOS == "darwin" {
		macPeriodic := []struct {
			path     string
			interval domain.PeriodicInterval
		}{
			{"/etc/periodic/hourly", domain.PeriodicHourly},
			{"/etc/periodic/daily", domain.PeriodicDaily},
			{"/etc/periodic/weekly", domain.PeriodicWeekly},
			{"/etc/periodic/monthly", domain.PeriodicMonthly},
		}
		for _, pd := range macPeriodic {
			cs = append(cs, candidate{
				path:     pd.path,
				subkind:  domain.SubkindPeriodicDir,
				isDir:    true,
				interval: pd.interval,
			})
		}
	}

	return cs
}

func (d *Discoverer) discoverFile(c candidate) (*DiscoveredSource, []domain.ValidationIssue) {
	info, err := d.fs.Stat(c.path)
	if err != nil {
		return nil, nil // file doesn't exist, silently skip
	}

	owner := fileOwner(info)
	label := c.path

	// Single read attempt — derive access from the result.
	data, readErr := d.fs.ReadFile(c.path)
	readable := readErr == nil
	reason := ""
	if !readable {
		reason = "permission denied"
	}

	src := domain.CronSource{
		Kind:    domain.SourceKindSystem,
		Subkind: c.subkind,
		Path:    c.path,
		Label:   label,
		Owner:   owner,
		Access: domain.SourceAccess{
			Readable: readable,
			Writable: false,
			Reason:   reason,
		},
	}

	if !readable {
		return &DiscoveredSource{Source: src}, []domain.ValidationIssue{{
			LineIndex: -1,
			Message:   fmt.Sprintf("cannot read %s: %s", c.path, reason),
			Severity:  domain.IssueSeverityWarning,
		}}
	}

	return &DiscoveredSource{Source: src, Text: string(data)}, nil
}

func (d *Discoverer) discoverCronDDir(c candidate) ([]DiscoveredSource, []domain.ValidationIssue) {
	entries, err := d.fs.ReadDir(c.path)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, []domain.ValidationIssue{{
			LineIndex: -1,
			Message:   fmt.Sprintf("cannot read directory %s: %v", c.path, err),
			Severity:  domain.IssueSeverityWarning,
		}}
	}

	var sources []DiscoveredSource
	var issues []domain.ValidationIssue

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldSkipCronDFile(name) {
			continue
		}

		filePath := filepath.Join(c.path, name)
		fc := candidate{
			path:    filePath,
			subkind: domain.SubkindCronD,
		}
		src, fileIssues := d.discoverFile(fc)
		if src != nil {
			src.Source.Label = "cron.d/" + name
			sources = append(sources, *src)
		}
		issues = append(issues, fileIssues...)
	}

	return sources, issues
}

// shouldSkipCronDFile returns true for files cron(8) would skip.
func shouldSkipCronDFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	if strings.HasSuffix(name, "~") {
		return true
	}
	ext := filepath.Ext(name)
	switch ext {
	case ".dpkg-dist", ".dpkg-old", ".dpkg-new", ".rpmsave", ".rpmnew":
		return true
	}
	return false
}

// runPartsNameRe matches names that run-parts will execute by default.
var runPartsNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (d *Discoverer) discoverPeriodicDir(c candidate) ([]PeriodicEntry, []domain.ValidationIssue) {
	entries, err := d.fs.ReadDir(c.path)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, []domain.ValidationIssue{{
			LineIndex: -1,
			Message:   fmt.Sprintf("cannot read directory %s: %v", c.path, err),
			Severity:  domain.IssueSeverityWarning,
		}}
	}

	var periodic []PeriodicEntry
	var issues []domain.ValidationIssue

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		filePath := filepath.Join(c.path, name)
		info, err := d.fs.Stat(filePath)
		if err != nil {
			continue
		}

		executable := info.Mode()&0111 != 0
		validName := runPartsNameRe.MatchString(name)
		owner := fileOwner(info)

		src := domain.CronSource{
			Kind:    domain.SourceKindSystem,
			Subkind: domain.SubkindPeriodicDir,
			Path:    filePath,
			Label:   fmt.Sprintf("%s/%s", filepath.Base(c.path), name),
			Owner:   owner,
			Access:  domain.SourceAccess{Readable: true, Writable: false},
		}

		pe := PeriodicEntry{
			Source:     src,
			Name:       name,
			Path:       filePath,
			Executable: executable,
			ValidName:  validName,
			Interval:   c.interval,
		}
		periodic = append(periodic, pe)

		if !validName {
			issues = append(issues, domain.ValidationIssue{
				LineIndex: -1,
				Message:   fmt.Sprintf("%s: name %q may be skipped by run-parts (contains dots or special chars)", filePath, name),
				Severity:  domain.IssueSeverityWarning,
			})
		}
		if !executable {
			issues = append(issues, domain.ValidationIssue{
				LineIndex: -1,
				Message:   fmt.Sprintf("%s: not executable", filePath),
				Severity:  domain.IssueSeverityWarning,
			})
		}
	}

	return periodic, issues
}

func isNotExist(err error) bool {
	return os.IsNotExist(err)
}

func fileOwner(info fs.FileInfo) string {
	if info == nil {
		return ""
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	uid := strconv.Itoa(int(stat.Uid))
	u, err := user.LookupId(uid)
	if err != nil {
		return uid
	}
	return u.Username
}
