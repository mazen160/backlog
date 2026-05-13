package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
	"github.com/mazen160/backlog/internal/timeutil"
)

func newExportCmd() *cobra.Command {
	var (
		format      string
		projectFlag string
		outFile     string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks to JSON, CSV, or Markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			planSvc := service.NewPlanService(app.DB)
			labelSvc := service.NewLabelService(app.DB)
			taskSvc := service.NewTaskService(app.DB, planSvc, labelSvc)
			projectFlag = projectOrDefault(cmd, projectFlag)

			tasks, _, err := taskSvc.List(cmd.Context(), models.TaskFilter{
				ProjectAlias: projectFlag,
				Limit:        0,
			})
			if err != nil {
				return err
			}

			var out *os.File
			if outFile != "" {
				out, err = os.Create(outFile)
				if err != nil {
					return err
				}
				defer out.Close()
			} else {
				out = os.Stdout
			}

			switch format {
			case "json":
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{"tasks": tasks})
			case "csv":
				return exportCSV(out, tasks)
			case "md":
				return exportMarkdown(out, tasks)
			default:
				return fmt.Errorf("unknown format %q (use json|csv|md)", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "output format: json|csv|md")
	cmd.Flags().StringVarP(&projectFlag, "project", "p", "", "filter by project alias")
	cmd.Flags().StringVarP(&outFile, "out", "o", "", "output file (default: stdout)")
	return cmd
}

func exportCSV(out *os.File, tasks []*models.Task) error {
	w := csv.NewWriter(out)
	w.Write([]string{"id", "project", "title", "type", "status", "priority", "assignee", "actor_kind", "actor_name", "source", "external_ref", "created_at"}) //nolint:errcheck
	for _, t := range tasks {
		proj := ""
		if t.Project != nil {
			proj = t.Project.Alias
		}
		w.Write([]string{ //nolint:errcheck
			t.ID, proj, t.Title, string(t.Type), string(t.Status),
			"P" + strconv.Itoa(t.Priority), t.Assignee,
			string(t.Actor.Kind), t.Actor.Name, t.Source, t.ExternalRef,
			timeutil.FormatNanoISO(t.CreatedAt),
		})
	}
	w.Flush()
	return w.Error()
}

func exportMarkdown(out *os.File, tasks []*models.Task) error {
	fmt.Fprintf(out, "# Backlog Export\n\n")
	for _, t := range tasks {
		proj := ""
		if t.Project != nil {
			proj = " [" + t.Project.Alias + "]"
		}
		fmt.Fprintf(out, "## [P%d][%s]%s %s\n\n", t.Priority, t.Status, proj, t.Title)
		if t.Description != "" {
			fmt.Fprintf(out, "%s\n\n", t.Description)
		}
		fmt.Fprintf(out, "- ID: `%s`\n- Type: %s\n- Actor: %s:%s\n\n---\n\n",
			t.ID, t.Type, t.Actor.Kind, t.Actor.Name)
	}
	return nil
}
