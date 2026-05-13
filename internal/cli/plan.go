package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Manage plans"}
	cmd.AddCommand(
		planAddCmd(),
		planUpdateCmd(),
		planListCmd(),
		planShowCmd(),
		planHistoryCmd(),
		planDeleteCmd(),
	)
	return cmd
}

func planAddCmd() *cobra.Command {
	var (
		taskID     string
		title      string
		body       string
		fromFile   string
		source     string
		changeNote string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a plan to a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			resolvedID, err := resolveTaskRef(ctx, taskID)
			if err != nil {
				return err
			}
			svc := service.NewPlanService(app.DB)
			planBody := body
			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				planBody = string(data)
			}
			pl, err := svc.Create(ctx, models.CreatePlanInput{
				TaskID:     resolvedID,
				Title:      title,
				Body:       planBody,
				Source:     source,
				ChangeNote: changeNote,
				Actor:      app.Actor,
			})
			if err != nil {
				return err
			}
			app.Out.Plan(pl)
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "task ID")
	cmd.Flags().StringVar(&title, "title", "", "plan title")
	cmd.Flags().StringVar(&body, "content", "", "plan body (markdown)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read plan body from file")
	cmd.Flags().StringVar(&source, "source", "", "plan source")
	cmd.Flags().StringVar(&changeNote, "change-note", "", "change note")
	cmd.MarkFlagRequired("task")
	cmd.MarkFlagRequired("title")
	return cmd
}

func planUpdateCmd() *cobra.Command {
	var (
		title      string
		body       string
		fromFile   string
		changeNote string
	)
	cmd := &cobra.Command{
		Use:   "update <plan-id>",
		Short: "Update a plan (creates a new version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewPlanService(app.DB)
			planBody := body
			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				planBody = string(data)
			}
			pl, err := svc.Update(cmd.Context(), args[0], models.UpdatePlanInput{
				Title:      title,
				Body:       planBody,
				ChangeNote: changeNote,
				Actor:      app.Actor,
			})
			if err != nil {
				return err
			}
			app.Out.Plan(pl)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&body, "content", "", "new body")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read body from file")
	cmd.Flags().StringVar(&changeNote, "change-note", "", "reason for change")
	cmd.MarkFlagRequired("title")
	return cmd
}

func planListCmd() *cobra.Command {
	var taskID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List plans for a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			resolvedID, err := resolveTaskRef(ctx, taskID)
			if err != nil {
				return err
			}
			svc := service.NewPlanService(app.DB)
			plans, err := svc.ListForTask(ctx, resolvedID)
			if err != nil {
				return err
			}
			app.Out.Plans(plans)
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "task ID")
	cmd.MarkFlagRequired("task")
	return cmd
}

func planShowCmd() *cobra.Command {
	var versionNum int
	cmd := &cobra.Command{
		Use:   "show <plan-id>",
		Short: "Show plan content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewPlanService(app.DB)
			pl, err := svc.Get(cmd.Context(), args[0], versionNum)
			if err != nil {
				return err
			}
			app.Out.Plan(pl)
			return nil
		},
	}
	cmd.Flags().IntVar(&versionNum, "version", 0, "specific version (default: current)")
	return cmd
}

func planHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history <plan-id>",
		Short: "Show version history of a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewPlanService(app.DB)
			versions, err := svc.History(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			app.Out.PlanHistory(versions)
			return nil
		},
	}
}

func planDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <plan-id>",
		Short: "Delete a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewPlanService(app.DB)
			pl, err := svc.Get(cmd.Context(), args[0], 0)
			if err != nil {
				return err
			}
			if err := svc.Delete(cmd.Context(), args[0], app.Actor); err != nil {
				return err
			}
			app.Out.CommandResult("delete", "plan", pl.ID, pl.ID[:8], "plan deleted: "+pl.ID[:8])
			return nil
		},
	}
}
