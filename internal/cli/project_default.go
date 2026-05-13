package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func projectOrDefault(cmd *cobra.Command, project string) string {
	if cmd != nil && cmd.Flags().Changed("project") {
		return project
	}
	if project != "" {
		return project
	}
	return app.DefaultProject
}

func requireProjectOrDefault(cmd *cobra.Command, project string) (string, error) {
	project = projectOrDefault(cmd, project)
	if project == "" {
		return "", fmt.Errorf("project is required (pass --project or run `backlog project set-default <alias>`)")
	}
	return project, nil
}
