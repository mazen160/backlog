package cli

import (
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
	"github.com/spf13/cobra"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "label", Short: "Manage labels"}
	cmd.AddCommand(labelCreateCmd(), labelListCmd(), labelAttachCmd(), labelDetachCmd())
	return cmd
}

func labelCreateCmd() *cobra.Command {
	var projectAlias, color string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a label",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projSvc := service.NewProjectService(app.DB)
			projectAlias, err := requireProjectOrDefault(cmd, projectAlias)
			if err != nil {
				return err
			}
			p, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
			if err != nil {
				return err
			}
			svc := service.NewLabelService(app.DB)
			l, err := svc.Create(cmd.Context(), models.CreateLabelInput{
				ProjectID: p.ID,
				Name:      args[0],
				Color:     color,
				Actor:     app.Actor,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(l)
			} else {
				app.Out.Success("label created: " + l.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&projectAlias, "project", "p", "", "project alias")
	cmd.Flags().StringVar(&color, "color", "", "label color (e.g. #ff0000)")
	return cmd
}

func labelListCmd() *cobra.Command {
	var projectAlias string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projSvc := service.NewProjectService(app.DB)
			projectAlias, err := requireProjectOrDefault(cmd, projectAlias)
			if err != nil {
				return err
			}
			p, err := projSvc.GetByAlias(cmd.Context(), projectAlias)
			if err != nil {
				return err
			}
			svc := service.NewLabelService(app.DB)
			labels, err := svc.ListForProject(cmd.Context(), p.ID)
			if err != nil {
				return err
			}
			app.Out.Labels(labels)
			return nil
		},
	}
	cmd.Flags().StringVarP(&projectAlias, "project", "p", "", "project alias")
	return cmd
}

func labelAttachCmd() *cobra.Command {
	var taskID string
	cmd := &cobra.Command{
		Use:   "attach <label-name>",
		Short: "Attach a label to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get task's project ID
			planSvc := service.NewPlanService(app.DB)
			labelSvc := service.NewLabelService(app.DB)
			taskSvc := service.NewTaskService(app.DB, planSvc, labelSvc)
			t, err := taskSvc.Get(cmd.Context(), taskID, false, false)
			if err != nil {
				return err
			}
			return labelSvc.AttachByName(cmd.Context(), t.ProjectID, t.ID, args[0], app.Actor)
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "task ID")
	cmd.MarkFlagRequired("task")
	return cmd
}

func labelDetachCmd() *cobra.Command {
	var taskID string
	cmd := &cobra.Command{
		Use:   "detach <label-name>",
		Short: "Detach a label from a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planSvc := service.NewPlanService(app.DB)
			labelSvc := service.NewLabelService(app.DB)
			taskSvc := service.NewTaskService(app.DB, planSvc, labelSvc)
			t, err := taskSvc.Get(cmd.Context(), taskID, false, false)
			if err != nil {
				return err
			}
			return labelSvc.Detach(cmd.Context(), t.ID, t.ProjectID, args[0], app.Actor)
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "task ID")
	cmd.MarkFlagRequired("task")
	return cmd
}
