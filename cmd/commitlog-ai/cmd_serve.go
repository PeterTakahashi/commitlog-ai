package main

import (
	"encoding/json"
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

// pidInfo is stored in server.pid as JSON.
type pidInfo struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

func pidFilePath(projectDir string) string {
	return filepath.Join(projectDir, ".commitlog-ai", "server.pid")
}

func writePIDInfo(projectDir string, pid, port int) error {
	pidPath := pidFilePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}
	data, _ := json.Marshal(pidInfo{PID: pid, Port: port})
	return os.WriteFile(pidPath, data, 0644)
}

func readPIDInfo(projectDir string) (*pidInfo, error) {
	data, err := os.ReadFile(pidFilePath(projectDir))
	if err != nil {
		return nil, err
	}
	var info pidInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// Legacy format: plain PID number
		pid, err2 := strconv.Atoi(strings.TrimSpace(string(data)))
		if err2 != nil {
			return nil, fmt.Errorf("invalid PID file")
		}
		return &pidInfo{PID: pid}, nil
	}
	return &info, nil
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

	srv := server.New(absDir, servePort, server.EmbeddedStaticFS())

	srv.OnReady = func(port int) {
		// Write PID file with actual port (after port fallback)
		writePIDInfo(absDir, os.Getpid(), port)
		if !serveNoBrowser {
			openBrowser(fmt.Sprintf("http://localhost:%d", port))
		}
	}

	// Start git commit watcher if --build is set
	if serveBuild {
		go watchCommits(absDir, srv)
	}

	defer removePID(absDir)
	return srv.Start()
}

func startDaemon(absDir string) error {
	// Build args for the child process, removing -d/--daemon
	var childArgs []string
	childArgs = append(childArgs, "serve")
	if serveProjectDir != "." {
		childArgs = append(childArgs, "-p", serveProjectDir)
	}
	// Always pass --port so child uses the same requested port
	childArgs = append(childArgs, "--port", strconv.Itoa(servePort))
	if serveBuild {
		childArgs = append(childArgs, "--build")
	}
	// Child doesn't open browser; parent will open it after child is ready
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

	childPID := child.Process.Pid

	// Detach child process
	child.Process.Release()

	// Wait for server to be ready by polling the PID file
	actualPort := servePort
	ready := false
	for i := 0; i < 60; i++ { // up to 30 seconds
		time.Sleep(500 * time.Millisecond)
		info, err := readPIDInfo(absDir)
		if err == nil && info.Port > 0 {
			actualPort = info.Port
			ready = true
			break
		}
	}

	if !ready {
		fmt.Printf("commitlog-ai server started in background (PID %d) but may still be building...\n", childPID)
	} else {
		fmt.Printf("commitlog-ai server started in background (PID %d, port %d)\n", childPID, actualPort)
	}
	fmt.Println("Run 'commitlog-ai stop' to stop the server.")

	// Open browser with the actual port
	if !serveNoBrowser {
		openBrowser(fmt.Sprintf("http://localhost:%d", actualPort))
	}

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
