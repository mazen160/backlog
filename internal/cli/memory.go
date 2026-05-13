package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
)

func newMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "memory", Short: "Manage project memory"}
	cmd.AddCommand(memoryAddCmd(), memoryAppendCmd(), memoryListCmd(), memoryDeleteCmd())
	return cmd
}

func memoryAddCmd() *cobra.Command {
	var project, tags string
	cmd := &cobra.Command{
		Use:   "add <body>",
		Short: "Add a memory entry to a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewMemoryService(app.DB)
			projSvc := service.NewProjectService(app.DB)
			project, err := requireProjectOrDefault(cmd, project)
			if err != nil {
				return err
			}
			p, err := projSvc.GetByAlias(cmd.Context(), project)
			if err != nil {
				return err
			}
			m, err := svc.Add(cmd.Context(), models.CreateMemoryInput{
				ProjectID: p.ID,
				Body:      args[0],
				Tags:      tags,
				Actor:     app.Actor,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(m)
			} else {
				app.Out.Success(fmt.Sprintf("memory added: %s", m.ID[:8]))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project alias")
	cmd.Flags().StringVar(&tags, "tag", "", "comma-separated tags")
	return cmd
}

func memoryAppendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "append <id> <text>",
		Short: "Append text to a memory entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewMemoryService(app.DB)
			m, err := svc.Append(cmd.Context(), args[0], args[1], app.Actor)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(m)
			} else {
				app.Out.Success(fmt.Sprintf("memory updated: %s", m.ID[:8]))
			}
			return nil
		},
	}
	return cmd
}

func memoryListCmd() *cobra.Command {
	var project, tag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memory entries for a project (newest first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewMemoryService(app.DB)
			project = projectOrDefault(cmd, project)
			entries, err := svc.List(cmd.Context(), project, tag)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"entries": entries})
				return nil
			}
			if len(entries) == 0 {
				fmt.Fprintln(os.Stdout, "no memory entries")
				return nil
			}
			fmt.Fprintf(os.Stdout, "%-20s  %-16s  %-50s  %s\n", "DATE", "TAGS", "BODY", "ACTOR")
			fmt.Fprintf(os.Stdout, "%s  %s  %s  %s\n",
				repeat("─", 20), repeat("─", 16), repeat("─", 50), repeat("─", 20))
			for _, e := range entries {
				ts := time.Unix(0, e.CreatedAt).UTC().Format("2006-01-02 15:04:05")
				body := e.Body
				if len(body) > 50 {
					body = body[:47] + "..."
				}
				actor := string(e.Actor.Kind) + ":" + e.Actor.Name
				fmt.Fprintf(os.Stdout, "%-20s  %-16s  %-50s  %s\n", ts, e.Tags, body, actor)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project alias")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	return cmd
}

func memoryDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewMemoryService(app.DB)
			if err := svc.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			ref := args[0]
			if len(ref) > 8 {
				ref = ref[:8]
			}
			app.Out.CommandResult("delete", "memory", args[0], ref, "memory entry deleted: "+ref)
			return nil
		},
	}
}

func repeat(s string, n int) string {
	out := make([]byte, len(s)*n)
	for i := 0; i < n; i++ {
		copy(out[i*len(s):], s)
	}
	return string(out)
}
