package cli

import (
	"github.com/spf13/cobra"
	"github.com/mazen160/backlog/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server subcommands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpserver.Serve(app.DB, app.Actor)
			return nil
		},
	})
	return cmd
}
