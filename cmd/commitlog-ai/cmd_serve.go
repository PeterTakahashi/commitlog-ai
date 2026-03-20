package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/PeterTakahashi/commitlog-ai/internal/builder"
	"github.com/PeterTakahashi/commitlog-ai/internal/linker"
	"github.com/PeterTakahashi/commitlog-ai/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web UI server",
	Long:  `Starts a local web server to browse the linked timeline.`,
	RunE:  runServe,
}

var (
	serveProjectDir string
	servePort       int
	serveNoBrowser  bool
	serveBuild      bool
)

func init() {
	serveCmd.Flags().StringVarP(&serveProjectDir, "project", "p", ".", "Project directory")
	serveCmd.Flags().IntVar(&servePort, "port", 3100, "Server port")
	serveCmd.Flags().BoolVar(&serveNoBrowser, "no-browser", false, "Don't open browser automatically")
	serveCmd.Flags().BoolVar(&serveBuild, "build", false, "Run parse+link before serving and auto-rebuild on new commits")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(serveProjectDir)
	if err != nil {
		return err
	}

	// Initial build if --build flag is set
	if serveBuild {
		fmt.Println("Building...")
		result, err := builder.Build(absDir)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
		if result.ParseCached && result.LinkCached {
			fmt.Println("Using cached data (no changes)")
		} else {
			fmt.Printf("Parsed %d session(s), linked %d pair(s), %d total entries\n",
				result.SessionCount, result.LinkedCount, result.EntryCount)
		}
	}

	srv := server.New(absDir, servePort, server.EmbeddedStaticFS())

	if !serveNoBrowser {
		srv.OnReady = func(port int) {
			openBrowser(fmt.Sprintf("http://localhost:%d", port))
		}
	}

	// Start git commit watcher if --build is set
	if serveBuild {
		go watchCommits(absDir, srv)
	}

	return srv.Start()
}

func watchCommits(projectDir string, srv *server.Server) {
	git := linker.NewGitClient(projectDir)
	lastHead, err := git.GetHead()
	if err != nil {
		fmt.Printf("Warning: cannot watch for commits: %v\n", err)
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		currentHead, err := git.GetHead()
		if err != nil {
			continue
		}
		if currentHead == lastHead {
			continue
		}
		lastHead = currentHead

		fmt.Printf("New commit detected (%s), rebuilding...\n", currentHead[:7])
		result, err := builder.Build(projectDir)
		if err != nil {
			fmt.Printf("Rebuild failed: %v\n", err)
			continue
		}

		if err := srv.ReloadData(); err != nil {
			fmt.Printf("Reload failed: %v\n", err)
			continue
		}

		fmt.Printf("Rebuilt: %d session(s), %d pair(s), %d entries\n",
			result.SessionCount, result.LinkedCount, result.EntryCount)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Run()
}
