package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/mazen160/backlog/internal/web"
	"github.com/spf13/cobra"
)

func newWebCmd() *cobra.Command {
	var port int
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := web.New(app.DB, app.Actor)
			addr := fmt.Sprintf(":%d", port)
			url := fmt.Sprintf("http://localhost%s", addr)
			fmt.Printf("backlog web UI → %s\n", url)

			if !noBrowser {
				go func() {
					time.Sleep(150 * time.Millisecond)
					openBrowser(url)
				}()
			}

			return http.ListenAndServe(addr, srv)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "port to listen on")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open browser automatically")
	return cmd
}

func openBrowser(url string) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name = "cmd"
		args = []string{"/c", "start"}
	default:
		name = "xdg-open"
	}
	_ = exec.Command(name, append(args, url)...).Start()
}
