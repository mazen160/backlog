package cli

import (
	"fmt"
	"os"

	"github.com/mazen160/backlog/internal/config"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Manage projects", Aliases: []string{"proj", "p"}}
	cmd.AddCommand(
		projectAddCmd(),
		projectListCmd(),
		projectShowCmd(),
		projectUpdateCmd(),
		projectArchiveCmd(),
		projectDeleteCmd(),
		projectSetDefaultCmd(),
		projectClearDefaultCmd(),
		projectUseCmd(),
		projectCurrentCmd(),
	)
	return cmd
}

func projectAddCmd() *cobra.Command {
	var alias, description, repoPath string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			p, err := svc.Create(cmd.Context(), models.CreateProjectInput{
				Alias:       alias,
				Name:        args[0],
				Description: description,
				RepoPath:    repoPath,
				Actor:       app.Actor,
			})
			if err != nil {
				return err
			}
			app.Out.Project(p)
			return nil
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "short alias (required)")
	cmd.Flags().StringVar(&description, "description", "", "description")
	cmd.Flags().StringVar(&repoPath, "repo-path", "", "path to source repository")
	cmd.MarkFlagRequired("alias")
	return cmd
}

func projectListCmd() *cobra.Command {
	var includeArchived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			projects, err := svc.List(cmd.Context(), includeArchived)
			if err != nil {
				return err
			}
			app.Out.ProjectsWithDefault(projects, app.DefaultProject)
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "include archived projects")
	return cmd
}

func projectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <alias>",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			p, err := svc.GetByAlias(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			app.Out.Project(p)
			return nil
		},
	}
}

func projectUpdateCmd() *cobra.Command {
	var name, description, repoPath string
	cmd := &cobra.Command{
		Use:   "update <alias>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			in := models.UpdateProjectInput{}
			if cmd.Flags().Changed("name") {
				in.Name = &name
			}
			if cmd.Flags().Changed("description") {
				in.Description = &description
			}
			if cmd.Flags().Changed("repo-path") {
				in.RepoPath = &repoPath
			}
			p, err := svc.Update(cmd.Context(), args[0], in, app.Actor)
			if err != nil {
				return err
			}
			app.Out.Project(p)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&repoPath, "repo-path", "", "new repo path")
	return cmd
}

func projectArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <alias>",
		Short: "Archive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			p, err := svc.Archive(cmd.Context(), args[0], app.Actor)
			if err != nil {
				return err
			}
			app.Out.CommandResult("archive", "project", p.ID, p.Alias, "project archived: "+p.Alias)
			return nil
		},
	}
}

func projectDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <alias>",
		Short: "Delete a project (hard delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewProjectService(app.DB)
			p, err := svc.GetByAlias(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := svc.Delete(cmd.Context(), args[0], app.Actor); err != nil {
				return err
			}
			app.Out.CommandResult("delete", "project", p.ID, p.Alias, "project deleted: "+p.Alias)
			return nil
		},
	}
}

func projectClearDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default",
		Short: "Remove the active default project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ClearDefaultProject(); err != nil {
				return err
			}
			app.DefaultProject = ""
			fmt.Println("default project cleared — no active project")
			return nil
		},
	}
}

func projectSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <alias>",
		Short: "Set the default project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setDefaultProject(cmd, args[0], false)
		},
	}
}

func projectUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <alias>",
		Short: "Switch the active project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setDefaultProject(cmd, args[0], true)
		},
	}
}

func projectCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.DefaultProject == "" {
				if flagJSON {
					app.Out.PrintJSON(map[string]string{"default_project": ""})
					return nil
				}
				fmt.Fprintln(os.Stdout, "no active project — run `backlog project set-default <alias>`")
				return nil
			}
			svc := service.NewProjectService(app.DB)
			p, err := svc.GetByAlias(cmd.Context(), app.DefaultProject)
			if err != nil {
				if flagJSON {
					app.Out.PrintJSON(map[string]string{"default_project": app.DefaultProject, "status": "missing"})
					return nil
				}
				fmt.Printf("active project: %q (project not found)\n", app.DefaultProject)
				return nil
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"default_project": app.DefaultProject, "project": p})
				return nil
			}
			fmt.Printf("* %-20s %s\n", p.Alias, p.Name)
			return nil
		},
	}
}

func setDefaultProject(cmd *cobra.Command, alias string, active bool) error {
	svc := service.NewProjectService(app.DB)
	p, err := svc.GetByAlias(cmd.Context(), alias)
	if err != nil {
		return err
	}
	if err := config.SetDefaultProject(p.Alias); err != nil {
		return err
	}
	app.DefaultProject = p.Alias
	if flagJSON {
		action := "set-default"
		message := fmt.Sprintf("default project set to %q", p.Alias)
		if active {
			action = "use"
			message = fmt.Sprintf("active project: %q", p.Alias)
		}
		app.Out.CommandResult(action, "project", p.ID, p.Alias, message)
		return nil
	}
	if active {
		fmt.Printf("active project: %q → %s\n", p.Alias, p.Name)
		return nil
	}
	fmt.Printf("default project set to %q\n", p.Alias)
	return nil
}
