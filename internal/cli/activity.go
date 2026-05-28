package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

func newActivityCmd() *cobra.Command {
	var limit int
	var projectAlias string
	var entityKind string
	var actorKind string
	var actorName string

	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Show recent activity log",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := repo.NewActivityRepo(app.DB)
			projectAlias = projectOrDefault(cmd, projectAlias)

			// Resolve project alias to ID if provided
			projectID := ""
			if projectAlias != "" {
				projSvc := service.NewProjectService(app.DB)
				p, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
				if err != nil {
					return fmt.Errorf("project %q: %w", projectAlias, err)
				}
				projectID = p.ID
			}

			events, err := r.List(cmd.Context(), projectID, entityKind, actorKind, actorName, limit, 0)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"events": events})
				return nil
			}
			if len(events) == 0 {
				fmt.Fprintln(os.Stdout, "no activity recorded")
				return nil
			}
			for _, e := range events {
				ts := time.Unix(0, e.CreatedAt).UTC().Format("2006-01-02 15:04:05")
				summary := e.Summary
				if summary == "" {
					summary = e.Action
				}
				fmt.Fprintf(os.Stdout, "%s  [%s] %s by %s:%s\n",
					ts, e.Entity, summary, e.Actor.Kind, e.Actor.Name)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max events to show")
	cmd.Flags().StringVar(&projectAlias, "project", "", "filter by project alias")
	cmd.Flags().StringVar(&entityKind, "kind", "", "filter by entity kind (task, plan, doc, comment, memory, project, label, attachment)")
	cmd.Flags().StringVar(&actorKind, "actor-kind", "", "filter by actor kind (human|ai)")
	cmd.Flags().StringVar(&actorName, "actor-name", "", "filter by actor name")
	cmd.AddCommand(activityAnalyzeCmd())
	return cmd
}
