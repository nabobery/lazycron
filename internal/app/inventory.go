package app

import (
	"context"

	"github.com/avinashchangrani/lazycron/internal/cronparse"
	"github.com/avinashchangrani/lazycron/internal/domain"
	"github.com/avinashchangrani/lazycron/internal/platform/systemcron"
)

// Inventory holds the merged result of user + system cron sources.
type Inventory struct {
	Jobs   []domain.CronJob
	Issues []domain.ValidationIssue
}

// InventoryService loads a unified view of user crontab + system cron sources.
// Mutations still go exclusively through ApplyService for the user crontab.
type InventoryService struct {
	applySvc   *ApplyService
	discoverer *systemcron.Discoverer
}

// NewInventoryService creates an InventoryService.
func NewInventoryService(applySvc *ApplyService, discoverer *systemcron.Discoverer) *InventoryService {
	return &InventoryService{
		applySvc:   applySvc,
		discoverer: discoverer,
	}
}

// LoadAll loads user crontab jobs via ApplyService and discovers system sources,
// returning a merged inventory. User jobs come first, then system jobs.
func (s *InventoryService) LoadAll(ctx context.Context) (Inventory, error) {
	if err := s.applySvc.Load(ctx); err != nil {
		return Inventory{}, err
	}

	userJobs := s.applySvc.Jobs()
	userIssues := s.applySvc.Issues()

	sysJobs, sysIssues := s.discoverSystemJobs()

	allJobs := make([]domain.CronJob, 0, len(userJobs)+len(sysJobs))
	allJobs = append(allJobs, userJobs...)
	allJobs = append(allJobs, sysJobs...)

	allIssues := make([]domain.ValidationIssue, 0, len(userIssues)+len(sysIssues))
	allIssues = append(allIssues, userIssues...)
	allIssues = append(allIssues, sysIssues...)

	return Inventory{Jobs: allJobs, Issues: allIssues}, nil
}

func (s *InventoryService) discoverSystemJobs() ([]domain.CronJob, []domain.ValidationIssue) {
	if s.discoverer == nil {
		return nil, nil
	}

	sources, periodicEntries, discoverIssues := s.discoverer.DiscoverAll()

	var jobs []domain.CronJob
	var issues []domain.ValidationIssue
	issues = append(issues, discoverIssues...)

	for _, ds := range sources {
		if ds.Text == "" {
			continue
		}
		_, sourceJobs, sourceIssues := cronparse.Parse(ds.Text, ds.Source)
		jobs = append(jobs, sourceJobs...)
		issues = append(issues, sourceIssues...)
	}

	for _, pe := range periodicEntries {
		job := cronparse.BuildPeriodicJob(pe.Source, pe.Name, pe.Interval)
		jobs = append(jobs, job)
	}

	return jobs, issues
}

// IsJobMutable returns true if the job can be toggled/deleted/edited via lazycron.
func IsJobMutable(job domain.CronJob) bool {
	return !job.ReadOnly
}

// JobByID searches the inventory for a job by ID.
func (inv Inventory) JobByID(id string) *domain.CronJob {
	for i := range inv.Jobs {
		if inv.Jobs[i].ID == id {
			return &inv.Jobs[i]
		}
	}
	return nil
}
