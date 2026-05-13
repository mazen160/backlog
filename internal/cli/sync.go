package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/service"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync backlog.json manifest with the projects table",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewManifestService(app.DB)
			result, err := svc.Sync(cmd.Context(), app.WorkDir, app.Actor)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(result)
				return nil
			}
			if len(result.Added) > 0 {
				app.Out.Success(fmt.Sprintf("added projects: %v", result.Added))
			}
			if len(result.Skipped) > 0 {
				app.Out.Println(fmt.Sprintf("already exist: %v", result.Skipped))
			}
			if len(result.Added) == 0 && len(result.Skipped) == 0 {
				app.Out.Println("nothing to sync")
			}
			return nil
		},
	}
}
