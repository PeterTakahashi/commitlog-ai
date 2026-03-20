package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	info, err := readPIDInfo(absDir)
	if err != nil {
		return fmt.Errorf("no running server found (no PID file at %s)", pidPath)
	}

	process, err := os.FindProcess(info.PID)
	if err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("process %d not found", info.PID)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("failed to stop process %d: %w", info.PID, err)
	}

	os.Remove(pidPath)
	fmt.Printf("Server stopped (PID %d)\n", info.PID)
	return nil
}
