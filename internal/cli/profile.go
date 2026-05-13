package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/profile"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named workspace profiles",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Profile commands don't need a DB
			return nil
		},
	}
	cmd.AddCommand(
		profileAddCmd(),
		profileListCmd(),
		profileShowCmd(),
		profileSetDefaultCmd(),
		profileClearDefaultCmd(),
		profileUseCmd(),
		profileCurrentCmd(),
		profileRemoveCmd(),
	)
	return cmd
}

func profileAddCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profile.Add(args[0], path); err != nil {
				return err
			}
			fmt.Printf("profile %q registered → %s\n", args[0], path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "workspace directory path")
	cmd.MarkFlagRequired("path")
	return cmd
}

func profileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, def, err := profile.List()
			if err != nil {
				return err
			}
			if len(profiles) == 0 {
				fmt.Println("no profiles registered")
				return nil
			}
			for name, p := range profiles {
				marker := " "
				if name == def {
					marker = "*"
				}
				fmt.Printf("%s %-20s %s\n", marker, name, p.Path)
			}
			return nil
		},
	}
}

func profileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := profile.Resolve(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Profile: %s\nPath:    %s\n", args[0], dir)
			return nil
		},
	}
}

func profileClearDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default",
		Short: "Remove the active default profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profile.ClearDefault(); err != nil {
				return err
			}
			fmt.Println("default profile cleared — no active profile")
			return nil
		},
	}
}

func profileSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profile.SetDefault(args[0]); err != nil {
				return err
			}
			fmt.Printf("default profile set to %q\n", args[0])
			return nil
		},
	}
}

func profileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := profile.Resolve(args[0])
			if err != nil {
				return err
			}
			if err := profile.SetDefault(args[0]); err != nil {
				return err
			}
			fmt.Printf("active profile: %q → %s\n", args[0], dir)
			return nil
		},
	}
}

func profileCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, def, err := profile.List()
			if err != nil {
				return err
			}
			if def == "" {
				fmt.Println("no active profile — run `backlog init` to create one")
				return nil
			}
			p, ok := profiles[def]
			if !ok {
				fmt.Printf("active profile: %q (path not found in registry)\n", def)
				return nil
			}
			fmt.Printf("* %-20s %s\n", def, p.Path)
			return nil
		},
	}
}

func profileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile (registry only, does not delete files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return profile.Remove(args[0])
		},
	}
}
