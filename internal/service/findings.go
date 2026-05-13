package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mazen160/backlog/internal/models"
)

type FindingsFile struct {
	Version int           `json:"version"`
	Project string        `json:"project"`
	Items   []FindingItem `json:"items"`
}

type FindingItem struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        models.TaskType   `json:"type"`
	Priority    interface{}       `json:"priority"` // accepts "P1" or 1
	Status      models.TaskStatus `json:"status"`
	Assignee    string            `json:"assignee"`
	Source      string            `json:"source"`
	ExternalRef string            `json:"external_ref"`
	Labels      []string          `json:"labels"`
	Plans       []struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"plans"`
}

// FindingsResult summarises an import-findings run.
type FindingsResult struct {
	Tasks  int
	Plans  int
	Errors []string
}

func ImportFindings(ctx context.Context, db *sql.DB, filePath string, projectAlias string, defaultProject string, actor models.Actor, dryRun bool) (*FindingsResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read findings file: %w", err)
	}
	var ff FindingsFile
	if err := json.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("parse findings file: %w", err)
	}

	// Resolve in priority order: CLI flag, findings file, saved default project.
	alias := projectAlias
	if alias == "" {
		alias = ff.Project
	}
	if alias == "" {
		alias = defaultProject
	}
	if alias == "" {
		return nil, fmt.Errorf("project alias is required (use --project, set 'project' in the file, or run `backlog project set-default <alias>`)")
	}

	projSvc := NewProjectService(db)
	proj, err := projSvc.GetByAlias(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("project %q not found: %w", alias, err)
	}

	planSvc := NewPlanService(db)
	labelSvc := NewLabelService(db)
	taskSvc := NewTaskService(db, planSvc, labelSvc)

	result := &FindingsResult{}

	for _, item := range ff.Items {
		priority := parsePriority(item.Priority)
		taskType := item.Type
		if taskType == "" {
			taskType = models.TaskTypeTask
		}
		status := item.Status
		if status == "" {
			status = models.TaskStatusTodo
		}

		in := models.CreateTaskInput{
			ProjectID:   proj.ID,
			Title:       item.Title,
			Description: item.Description,
			Type:        taskType,
			Status:      status,
			Priority:    priority,
			Assignee:    item.Assignee,
			Actor:       actor,
			Source:      item.Source,
			ExternalRef: item.ExternalRef,
			Labels:      item.Labels,
		}
		for _, p := range item.Plans {
			in.Plans = append(in.Plans, models.CreatePlanInput{
				Title: p.Title,
				Body:  p.Body,
				Actor: actor,
			})
		}

		if !dryRun {
			_, err := taskSvc.Create(ctx, in)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%q: %v", item.Title, err))
				continue
			}
		}
		result.Tasks++
		result.Plans += len(item.Plans)
	}

	return result, nil
}

func parsePriority(v interface{}) int {
	switch val := v.(type) {
	case float64:
		p := int(val)
		if p < 1 || p > 5 {
			return 3
		}
		return p
	case string:
		s := strings.TrimPrefix(strings.ToUpper(val), "P")
		switch s {
		case "1":
			return 1
		case "2":
			return 2
		case "3":
			return 3
		case "4":
			return 4
		case "5":
			return 5
		}
	}
	return 3
}
