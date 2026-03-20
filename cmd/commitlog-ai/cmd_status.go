package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeterTakahashi/commitlog-ai/internal/parser"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detected log sources and counts",
	RunE:  runStatus,
}

var statusProjectDir string

func init() {
	statusCmd.Flags().StringVarP(&statusProjectDir, "project", "p", ".", "Project directory")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(statusProjectDir)
	if err != nil {
		return err
	}

	fmt.Printf("Project: %s\n\n", absDir)

	parsers := parser.AllParsers()
	total := 0

	for _, p := range parsers {
		files, err := p.Detect(absDir)
		if err != nil {
			fmt.Printf("  %-12s  error: %v\n", p.Name(), err)
			continue
		}
		total += len(files)
		status := "no logs found"
		if len(files) > 0 {
			status = fmt.Sprintf("%d log file(s)", len(files))
		}
		fmt.Printf("  %-12s  %s\n", p.Name(), status)
	}

	// Check for existing parse output
	sessionsPath := filepath.Join(absDir, ".commitlog-ai", "sessions.json")
	if _, err := os.Stat(sessionsPath); err == nil {
		fmt.Printf("\nParsed data: %s\n", sessionsPath)
	}

	timelinePath := filepath.Join(absDir, ".commitlog-ai", "timeline.json")
	if _, err := os.Stat(timelinePath); err == nil {
		fmt.Printf("Linked data: %s\n", timelinePath)
	}

	if total == 0 {
		fmt.Println("\nNo agent logs detected for this project.")
	}

	return nil
}
