package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/config"
	"github.com/mazen160/backlog/internal/migrate"
	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/output"
	"github.com/mazen160/backlog/internal/profile"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

var appVersion = "dev"

// SetVersion is called by main with the value injected via ldflags.
func SetVersion(v string) { appVersion = v }

// App holds global CLI state threaded through all commands.
type App struct {
	DB             *sql.DB
	Out            *output.Printer
	Actor          models.Actor
	DBPath         string
	WorkDir        string
	DefaultProject string
}

// flags set by the root command's persistent flags
var (
	flagDB      string
	flagProfile string
	flagJSON    bool
	flagQuiet   bool
	flagAs      string
)

var app = &App{}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "backlog",
		Short: "A local-first, agent-friendly task backlog",
		Long:  "Backlog is a fast CLI for managing tasks, plans, and backlogs — designed for humans and AI agents.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Commands that don't need a DB
			switch cmd.Name() {
			case "version", "profile", "completion", "init", "install-skills":
				return nil
			}
			if cmd.Parent() != nil && cmd.Parent().Name() == "profile" {
				return nil
			}
			return openApp(cmd)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if app.DB != nil {
				return app.DB.Close()
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flagDB, "db", "", "path to backlog.db")
	root.PersistentFlags().StringVar(&flagProfile, "profile", "", "use named profile")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "output JSON")
	root.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress non-essential output")
	root.PersistentFlags().StringVar(&flagAs, "as", "", "actor for this operation (e.g. human:alice or ai:claude-code)")

	root.AddCommand(
		newInitCmd(),
		newSyncCmd(),
		newProjectCmd(),
		newTaskCmd(),
		newPlanCmd(),
		newCommentCmd(),
		newLabelCmd(),
		newProfileCmd(),
		newImportCmd(),
		newImportFindingsCmd(),
		newExportCmd(),
		newMCPCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newCompletionCmd(),
		newActivityCmd(),
		newMemoryCmd(),
		newDocCmd(),
		newAttachmentCmd(),
		newWebCmd(),
		newInstallSkillsCmd(),
	)

	return root
}

func openApp(cmd *cobra.Command) error {
	dbPath, workDir, err := resolveDB()
	if err != nil {
		return err
	}
	app.DBPath = dbPath
	app.WorkDir = workDir

	// Load user-level config, then workspace config, then merge with fallback.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	workspaceCfg, err := config.LoadWorkspaceConfig(workDir)
	if err != nil {
		return fmt.Errorf("load workspace config: %w", err)
	}
	cfg := config.EffectiveConfig(globalCfg, workspaceCfg)

	if !cmd.Flags().Changed("json") && cfg.Output.DefaultFormat == "json" {
		flagJSON = true
	}
	if !cmd.Flags().Changed("as") && cfg.Defaults.Actor != "" {
		flagAs = cfg.Defaults.Actor
	}

	app.Out = output.New(flagJSON, flagQuiet)
	app.Actor = resolveActor(flagAs)
	app.DefaultProject = cfg.DefaultProject

	db, err := repo.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if err := migrate.Run(db); err != nil {
		db.Close()
		return fmt.Errorf("migrate: %w", err)
	}
	app.DB = db
	return nil
}

func resolveDB() (dbPath, workDir string, err error) {
	// 1. Explicit --db flag
	if flagDB != "" {
		abs, err := filepath.Abs(flagDB)
		if err != nil {
			return "", "", err
		}
		return abs, filepath.Dir(abs), nil
	}

	// 2. $BACKLOG_DB env var
	if envDB := os.Getenv("BACKLOG_DB"); envDB != "" {
		abs, err := filepath.Abs(envDB)
		if err != nil {
			return "", "", err
		}
		return abs, filepath.Dir(abs), nil
	}

	// 3. --profile flag
	if flagProfile != "" {
		dir, err := profile.Resolve(flagProfile)
		if err != nil {
			return "", "", err
		}
		db := filepath.Join(dir, "backlog.db")
		return db, dir, nil
	}

	// 4. Default profile
	defDir, _ := profile.Resolve("")
	if defDir != "" {
		return filepath.Join(defDir, "backlog.db"), defDir, nil
	}

	return "", "", fmt.Errorf("no backlog workspace found — run `backlog init` to create one")
}

func resolveActor(as string) models.Actor {
	if as != "" {
		parts := strings.SplitN(as, ":", 2)
		if len(parts) == 2 {
			return models.Actor{Kind: models.ActorKind(parts[0]), Name: parts[1]}
		}
		return models.Actor{Kind: models.ActorKindHuman, Name: as}
	}
	name := os.Getenv("USER")
	if name == "" {
		name = "user"
	}
	return models.Actor{Kind: models.ActorKindHuman, Name: name}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := appVersion
			if v != "dev" && !strings.HasPrefix(v, "v") {
				v = "v" + v
			}
			fmt.Printf("backlog %s\n", v)
			return nil
		},
	}
}

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unknown shell %q", args[0])
			}
		},
	}
	return cmd
}

// resolveTaskRef resolves a TASK-N, bare integer, or ULID to a ULID.
func resolveTaskRef(ctx context.Context, ref string) (string, error) {
	taskSvc := service.NewTaskService(app.DB, nil, nil)
	return taskSvc.ResolveRef(ctx, ref)
}
