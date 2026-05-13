package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mazen160/backlog/internal/service"
)

func newAttachmentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "attachment", Short: "Manage file attachments", Aliases: []string{"attach"}}
	cmd.AddCommand(
		attachmentAddCmd(),
		attachmentListCmd(),
		attachmentFetchCmd(),
		attachmentDeleteCmd(),
	)
	return cmd
}

func attachmentAddCmd() *cobra.Command {
	var taskRef, docID string
	cmd := &cobra.Command{
		Use:   "add <file>",
		Short: "Attach a file to a task or doc",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			var linkedType, linkedID string
			ctx := cmd.Context()
			if taskRef != "" {
				linkedType = "task"
				linkedID, err = resolveTaskRef(ctx, taskRef)
				if err != nil {
					return err
				}
			} else if docID != "" {
				linkedType = "doc"
				linkedID = docID
			}
			svc := service.NewAttachmentService(app.DB)
			a, err := svc.Add(ctx, filepath.Base(filePath), data, linkedType, linkedID, app.Actor)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(a)
			} else {
				fmt.Fprintf(os.Stdout, "attachment added: %s (%s, %d bytes)\n",
					a.ID[:8], a.Name, a.Size)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskRef, "task", "", "attach to task (TASK-N, int, or ULID)")
	cmd.Flags().StringVar(&docID, "doc", "", "attach to doc (ID)")
	return cmd
}

func attachmentListCmd() *cobra.Command {
	var taskRef, docID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List attachments for a task or doc",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var linkedType, linkedID string
			var err error
			if taskRef != "" {
				linkedType = "task"
				linkedID, err = resolveTaskRef(ctx, taskRef)
				if err != nil {
					return err
				}
			} else if docID != "" {
				linkedType = "doc"
				linkedID = docID
			} else {
				return fmt.Errorf("provide --task or --doc")
			}
			svc := service.NewAttachmentService(app.DB)
			attachments, err := svc.List(ctx, linkedType, linkedID)
			if err != nil {
				return err
			}
			if flagJSON {
				app.Out.PrintJSON(map[string]interface{}{"attachments": attachments})
				return nil
			}
			if len(attachments) == 0 {
				fmt.Fprintln(os.Stdout, "no attachments")
				return nil
			}
			fmt.Fprintf(os.Stdout, "%-10s  %-30s  %-28s  %10s  %s\n",
				"ID", "NAME", "MIME", "SIZE", "CREATED")
			fmt.Fprintf(os.Stdout, "%s  %s  %s  %s  %s\n",
				repeat("─", 10), repeat("─", 30), repeat("─", 28), repeat("─", 10), repeat("─", 19))
			for _, a := range attachments {
				fmt.Fprintf(os.Stdout, "%-10s  %-30s  %-28s  %10d  %s\n",
					a.ID[:8],
					truncateStr(a.Name, 30),
					truncateStr(a.MimeType, 28),
					a.Size,
					formatNano(a.CreatedAt))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskRef, "task", "", "task ref")
	cmd.Flags().StringVar(&docID, "doc", "", "doc ID")
	return cmd
}

func attachmentFetchCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "fetch <attachment-id>",
		Short: "Fetch attachment data (write to file or stdout)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewAttachmentService(app.DB)
			a, err := svc.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if outPath == "" {
				outPath = a.Name
			}
			if outPath == "-" {
				os.Stdout.Write(a.Data)
				return nil
			}
			if err := os.WriteFile(outPath, a.Data, 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
			fmt.Fprintf(os.Stderr, "written %d bytes to %s\n", len(a.Data), outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "output path (use - for stdout, default: original filename)")
	return cmd
}

func attachmentDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <attachment-id>",
		Short: "Delete an attachment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := service.NewAttachmentService(app.DB)
			if err := svc.Delete(cmd.Context(), args[0], app.Actor); err != nil {
				return err
			}
			ref := args[0]
			if len(ref) > 8 {
				ref = ref[:8]
			}
			app.Out.CommandResult("delete", "attachment", args[0], ref, "attachment deleted: "+ref)
			return nil
		},
	}
}
