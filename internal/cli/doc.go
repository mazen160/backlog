package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
)

func newDocCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "doc", Short: "Manage project documentation"}
	cmd.AddCommand(
		docAddCmd(),
		docListCmd(),
		docShowCmd(),
		docUpdateCmd(),
		docAppendCmd(),
		docHistoryCmd(),
		docDeleteCmd(),
	)
	return cmd
}

func docAddCmd() *cobra.Command {
	var project, title, body, fromFile string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a doc to a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				body = string(data)
			}
			svc := service.NewDocService(app.DB)
			project, err := requireProjectOrDefault(cmd, project)
			if err != nil {
				return err
			}
			d, err := svc.Create(cmd.Context(), project, models.CreateDocInput{
				Title: title,
				Body:  body,
				Actor: app.Actor,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(d)
			} else {
				fmt.Fprintf(os.Stdout, "doc created: %s (ID: %s)\n", d.Title, d.ID[:8])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project alias")
	cmd.Flags().StringVar(&title, "title", "", "doc title")
	cmd.Flags().StringVar(&body, "content", "", "doc body (markdown)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read body from file")
	cmd.MarkFlagRequired("title")
	return cmd
}

func docListCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List docs for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewDocService(app.DB)
			project = projectOrDefault(cmd, project)
			docs, err := svc.List(cmd.Context(), project)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"docs": docs})
				return nil
			}
			if len(docs) == 0 {
				fmt.Fprintln(os.Stdout, "no docs")
				return nil
			}
			fmt.Fprintf(os.Stdout, "%-10s  %-40s  %s\n", "ID", "TITLE", "UPDATED")
			fmt.Fprintf(os.Stdout, "%s  %s  %s\n", repeat("─", 10), repeat("─", 40), repeat("─", 19))
			for _, d := range docs {
				fmt.Fprintf(os.Stdout, "%-10s  %-40s  %s\n",
					d.ID[:8],
					truncateStr(d.Title, 40),
					formatNano(d.UpdatedAt))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project alias")
	return cmd
}

func docShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <doc-id>",
		Short: "Show a doc with its current body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewDocService(app.DB)
			d, err := svc.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(d)
				return nil
			}
			fmt.Fprintf(os.Stdout, "ID:       %s\nTitle:    %s\nVersion:  v%d\nUpdated:  %s\n\n%s\n",
				d.ID, d.Title, d.CurrentVersion, formatNano(d.UpdatedAt), d.Version.Body)
			return nil
		},
	}
	return cmd
}

func docUpdateCmd() *cobra.Command {
	var title, body, changeNote, fromFile string
	cmd := &cobra.Command{
		Use:   "update <doc-id>",
		Short: "Update a doc (creates a new version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				body = string(data)
			}
			svc := service.NewDocService(app.DB)
			d, err := svc.Update(cmd.Context(), args[0], models.UpdateDocInput{
				Title:      title,
				Body:       body,
				ChangeNote: changeNote,
				Actor:      app.Actor,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(d)
			} else {
				fmt.Fprintf(os.Stdout, "doc updated to v%d\n", d.CurrentVersion)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&body, "content", "", "new body (markdown)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read body from file")
	cmd.Flags().StringVar(&changeNote, "change-note", "", "reason for change")
	return cmd
}

func docAppendCmd() *cobra.Command {
	var body, changeNote, fromFile string
	cmd := &cobra.Command{
		Use:   "append <doc-id>",
		Short: "Append content to a doc (creates a new version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				body = string(data)
			}
			if body == "" {
				return fmt.Errorf("--content or --from-file is required")
			}
			svc := service.NewDocService(app.DB)
			d, err := svc.Append(cmd.Context(), args[0], body, changeNote, app.Actor)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(d)
			} else {
				fmt.Fprintf(os.Stdout, "doc updated to v%d\n", d.CurrentVersion)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "content", "", "content to append (markdown)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read content to append from file")
	cmd.Flags().StringVar(&changeNote, "change-note", "", "reason for change")
	return cmd
}

func docHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <doc-id>",
		Short: "Show version history of a doc",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewDocService(app.DB)
			versions, err := svc.History(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"versions": versions})
				return nil
			}
			for _, v := range versions {
				note := ""
				if v.ChangeNote != "" {
					note = " — " + v.ChangeNote
				}
				fmt.Fprintf(os.Stdout, "v%d  %s  %s:%s%s\n",
					v.Version, formatNano(v.CreatedAt),
					v.Actor.Kind, v.Actor.Name, note)
			}
			return nil
		},
	}
	return cmd
}

func docDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <doc-id>",
		Short: "Delete a doc",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewDocService(app.DB)
			d, err := svc.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := svc.Delete(cmd.Context(), args[0], app.Actor); err != nil {
				return err
			}
			app.Out.CommandResult("delete", "doc", d.ID, d.ID[:8], "doc deleted: "+d.ID[:8])
			return nil
		},
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func formatNano(ns int64) string {
	if ns == 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format("2006-01-02 15:04:05")
}
