package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

func newImportCmd() *cobra.Command {
	var (
		projectAlias string
		dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "import <other-backlog.db>",
		Short: "Import tasks from another backlog workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcDB, err := repo.Open(args[0])
			if err != nil {
				return fmt.Errorf("open source db: %w", err)
			}
			defer srcDB.Close()

			result, err := service.ImportFromDB(cmd.Context(), app.DB, srcDB, projectAlias, app.Actor, dryRun)
			if err != nil {
				return err
			}

			if flagJSON {
				app.Out.PrintJSON(result)
				return nil
			}
			if dryRun {
				fmt.Print("[dry-run] ")
			}
			fmt.Printf("imported: %d projects, %d tasks, %d plans, %d comments, %d labels\n",
				result.Projects, result.Tasks, result.Plans, result.Comments, result.Labels)
			return nil
		},
	}
	cmd.Flags().StringVarP(&projectAlias, "project", "p", "", "only import this project alias from source")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report counts without writing")
	return cmd
}

func newImportFindingsCmd() *cobra.Command {
	var (
		projectAlias string
		dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "import-findings <findings.json>",
		Short: "Import tasks from a structured findings file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defaultProject := ""
			if !cmd.Flags().Changed("project") {
				defaultProject = app.DefaultProject
			}
			result, err := service.ImportFindings(cmd.Context(), app.DB, args[0], projectAlias, defaultProject, app.Actor, dryRun)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(result)
				return nil
			}
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}
			fmt.Printf("%simported %d tasks · %d plans · %d errors\n",
				prefix, result.Tasks, result.Plans, len(result.Errors))
			for _, e := range result.Errors {
				app.Out.Error(e)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&projectAlias, "project", "p", "", "project alias (overrides file)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report counts without writing")
	return cmd
}
