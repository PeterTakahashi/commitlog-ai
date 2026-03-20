package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	serveDaemon     bool
)

func init() {
	serveCmd.Flags().StringVarP(&serveProjectDir, "project", "p", ".", "Project directory")
	serveCmd.Flags().IntVar(&servePort, "port", 3100, "Server port")
	serveCmd.Flags().BoolVar(&serveNoBrowser, "no-browser", false, "Don't open browser automatically")
	serveCmd.Flags().BoolVar(&serveBuild, "build", false, "Run parse+link before serving and auto-rebuild on new commits")
	serveCmd.Flags().BoolVarP(&serveDaemon, "daemon", "d", false, "Run server in background")
	rootCmd.AddCommand(serveCmd)
}

func pidFilePath(projectDir string) string {
	return filepath.Join(projectDir, ".commitlog-ai", "server.pid")
}

func writePID(projectDir string, pid int) error {
	pidPath := pidFilePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

func removePID(projectDir string) {
	os.Remove(pidFilePath(projectDir))
}

func runServe(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(serveProjectDir)
	if err != nil {
		return err
	}

	// Daemon mode: re-exec ourselves in background
	if serveDaemon {
		return startDaemon(absDir)
	}

	// Initial build if --build flag is set
	if serveBuild {
		result, err := builder.BuildWithProgress(absDir, printProgress)
		fmt.Print("\r\033[K") // clear progress line
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

	// Write PID file
	writePID(absDir, os.Getpid())
	defer removePID(absDir)

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

func startDaemon(absDir string) error {
	// Build args for the child process, removing -d/--daemon
	var childArgs []string
	childArgs = append(childArgs, "serve")
	if serveProjectDir != "." {
		childArgs = append(childArgs, "-p", serveProjectDir)
	}
	if servePort != 3100 {
		childArgs = append(childArgs, "--port", strconv.Itoa(servePort))
	}
	if serveBuild {
		childArgs = append(childArgs, "--build")
	}
	childArgs = append(childArgs, "--no-browser")

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable: %w", err)
	}

	child := exec.Command(executable, childArgs...)
	child.Dir = absDir
	// Detach from terminal
	child.Stdout = nil
	child.Stderr = nil
	child.Stdin = nil

	if err := child.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("commitlog-ai server started in background (PID %d, port %d)\n", child.Process.Pid, servePort)
	fmt.Println("Run 'commitlog-ai stop' to stop the server.")

	// Detach child process
	child.Process.Release()
	return nil
}

// printProgress renders a progress bar to the terminal.
func printProgress(step string, current, total int) {
	const barWidth = 30
	if total <= 0 {
		// Indeterminate: just show the step
		fmt.Printf("\r\033[K  %s", step)
		return
	}
	ratio := float64(current) / float64(total)
	filled := int(ratio * barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Printf("\r\033[K  %s [%s] %d/%d", step, bar, current, total)
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

		fmt.Printf("\nNew commit detected (%s), rebuilding...\n", currentHead[:7])
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
