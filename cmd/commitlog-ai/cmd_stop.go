package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background server",
	Long:  `Stops a commitlog-ai server running in the background (started with -d flag).`,
	RunE:  runStop,
}

var stopProjectDir string

func init() {
	stopCmd.Flags().StringVarP(&stopProjectDir, "project", "p", ".", "Project directory")
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(stopProjectDir)
	if err != nil {
		return err
	}

	pidPath := pidFilePath(absDir)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("no running server found (no PID file at %s)", pidPath)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("invalid PID file")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("process %d not found", pid)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("failed to stop process %d: %w", pid, err)
	}

	os.Remove(pidPath)
	fmt.Printf("Server stopped (PID %d)\n", pid)
	return nil
}
