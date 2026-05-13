package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "doctor", Short: "Check and repair the workspace"}
	cmd.AddCommand(doctorCheckCmd(), doctorBackupCmd())
	return cmd
}

func doctorCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check DB integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result string
			if err := app.DB.QueryRowContext(cmd.Context(), `PRAGMA integrity_check`).Scan(&result); err != nil {
				return err
			}
			if result == "ok" {
				app.Out.Success("database integrity: ok")
			} else {
				app.Out.Error("database integrity: " + result)
			}
			return nil
		},
	}
}

func doctorBackupCmd() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create an atomic backup using VACUUM INTO",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dest == "" {
				dest = filepath.Join(app.WorkDir, "backlog.backup.db")
			}
			if _, err := app.DB.ExecContext(cmd.Context(), `VACUUM INTO ?`, dest); err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}
			app.Out.Success("backup written to: " + dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "to", "", "backup destination path")
	return cmd
}
