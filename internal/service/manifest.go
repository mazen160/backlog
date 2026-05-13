package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mazen160/backlog/internal/manifest"
	"github.com/mazen160/backlog/internal/models"
)

type ManifestService struct {
	projects *ProjectService
}

func NewManifestService(db *sql.DB) *ManifestService {
	return &ManifestService{projects: NewProjectService(db)}
}

// SyncResult reports what changed when syncing the manifest.
type SyncResult struct {
	Added   []string
	Skipped []string
}

// Sync reads backlog.json from dir and ensures all listed projects exist in the DB.
// It never deletes; removed projects get a warning only.
func (s *ManifestService) Sync(ctx context.Context, dir string, actor models.Actor) (*SyncResult, error) {
	m, err := manifest.Load(dir)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}
	for _, mp := range m.Projects {
		existing, _ := s.projects.GetByAlias(ctx, mp.Alias)
		if existing != nil {
			result.Skipped = append(result.Skipped, mp.Alias)
			continue
		}
		_, err := s.projects.Create(ctx, models.CreateProjectInput{
			Alias:       mp.Alias,
			Name:        mp.Name,
			Description: mp.Description,
			RepoPath:    mp.RepoPath,
			Actor:       actor,
		})
		if err != nil {
			return nil, fmt.Errorf("sync project %q: %w", mp.Alias, err)
		}
		result.Added = append(result.Added, mp.Alias)
	}
	return result, nil
}
