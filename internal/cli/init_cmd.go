package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/config"
	"github.com/mazen160/backlog/internal/migrate"
	"github.com/mazen160/backlog/internal/output"
	"github.com/mazen160/backlog/internal/profile"
	"github.com/mazen160/backlog/internal/repo"
)

func newInitCmd() *cobra.Command {
	var (
		profileName string
		customPath  string
		setDefault  bool
		actor       string
		priority    int
		status      string
		taskType    string
		reset       bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a backlog workspace",
		Long: `Creates a workspace directory and registers it as a named profile.

By default, the workspace is created at ~/.backlog/<profile-name>/.
Use --path to store it elsewhere (e.g. a project directory or a separate git repo).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if profileName == "" {
				profileName = "default"
			}

			// Resolve workspace directory
			workspaceDir := customPath
			if workspaceDir == "" {
				workspaceDir = config.DefaultWorkspaceDir(profileName)
			} else {
				var err error
				workspaceDir, err = filepath.Abs(workspaceDir)
				if err != nil {
					return err
				}
			}

			dirExisted := false
			if info, err := os.Stat(workspaceDir); err == nil {
				if !info.IsDir() {
					return fmt.Errorf("workspace path exists and is not a directory: %s", workspaceDir)
				}
				dirExisted = true
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("check workspace directory: %w", err)
			}

			// Check if already initialized
			dbPath := filepath.Join(workspaceDir, "backlog.db")
			if _, err := os.Stat(dbPath); err == nil && !reset {
				out := output.New(flagJSON, flagQuiet)
				out.Success(fmt.Sprintf("already initialized; workspace directory exists (profile: %q, path: %s)", profileName, workspaceDir))
				return nil
			}

			// Reset: remove only Backlog-managed files and preserve the directory.
			if reset {
				if err := resetWorkspaceFiles(workspaceDir); err != nil {
					return fmt.Errorf("reset: %w", err)
				}
			}

			if err := os.MkdirAll(workspaceDir, 0755); err != nil {
				return fmt.Errorf("create workspace: %w", err)
			}

			// Initialize DB
			db, err := repo.Open(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := migrate.Run(db); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			// Write workspace config
			cfg := config.DefaultWorkspaceConfig()
			if actor != "" {
				cfg.Defaults.Actor = actor
			} else if u := os.Getenv("USER"); u != "" {
				cfg.Defaults.Actor = "human:" + u
			}
			if priority != 0 {
				cfg.Defaults.Priority = priority
			}
			if status != "" {
				cfg.Defaults.Status = status
			}
			if taskType != "" {
				cfg.Defaults.Type = taskType
			}
			if err := config.WriteWorkspaceConfig(workspaceDir, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			// Register profile (overwrite if exists)
			if err := profile.Upsert(profileName, workspaceDir); err != nil {
				return fmt.Errorf("register profile: %w", err)
			}

			// Set as default if requested or if no default exists yet
			isDefault := false
			if setDefault {
				if err := profile.SetDefault(profileName); err != nil {
					return fmt.Errorf("set default profile: %w", err)
				}
				isDefault = true
			} else {
				hasDefault, err := profile.HasDefault()
				if err != nil {
					return fmt.Errorf("check default profile: %w", err)
				}
				if !hasDefault {
					if err := profile.SetDefault(profileName); err != nil {
						return fmt.Errorf("set default profile: %w", err)
					}
					isDefault = true
				}
			}

			out := output.New(flagJSON, flagQuiet)
			action := "initialized"
			if reset {
				action = "reset"
			}
			msg := fmt.Sprintf("workspace %s: %s (profile: %q)", action, workspaceDir, profileName)
			if dirExisted {
				msg += " [directory already existed; preserved]"
			}
			if isDefault {
				msg += " [default]"
			}
			out.Success(msg)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileName, "profile", "", "profile name (default: \"default\")")
	cmd.Flags().StringVar(&customPath, "path", "", "workspace directory (default: ~/.backlog/<profile>)")
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "make this the active profile")
	cmd.Flags().StringVar(&actor, "actor", "", "default actor (e.g. human:alice)")
	cmd.Flags().IntVar(&priority, "priority", 0, "default priority 1-5")
	cmd.Flags().StringVar(&status, "status", "", "default task status")
	cmd.Flags().StringVar(&taskType, "type", "", "default task type")
	cmd.Flags().BoolVar(&reset, "reset", false, "wipe and reinitialize existing workspace")
	return cmd
}

func resetWorkspaceFiles(workspaceDir string) error {
	managedFiles := []string{
		"backlog.db",
		"backlog.db-shm",
		"backlog.db-wal",
		"backlog.db-journal",
		"backlog.json",
		"backlog.config",
		"config.toml",
	}
	for _, name := range managedFiles {
		path := filepath.Join(workspaceDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
