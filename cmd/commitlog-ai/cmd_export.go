package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeterTakahashi/commitlog-ai/internal/exporter"
	"github.com/PeterTakahashi/commitlog-ai/internal/model"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export linked timeline",
	Long:  `Exports the linked timeline data as JSON or Markdown.`,
	RunE:  runExport,
}

var (
	exportProjectDir string
	exportFormat     string
)

func init() {
	exportCmd.Flags().StringVarP(&exportProjectDir, "project", "p", ".", "Project directory")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, markdown)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(exportProjectDir)
	if err != nil {
		return err
	}

	// Read timeline
	timelinePath := filepath.Join(absDir, ".commitlog-ai", "timeline.json")
	data, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("no timeline found. Run 'commitlog-ai link' first: %w", err)
	}

	var timeline model.LinkedTimeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return fmt.Errorf("parsing timeline.json: %w", err)
	}

	outputDir := filepath.Join(absDir, ".commitlog-ai", "output")

	switch exportFormat {
	case "json":
		outPath, err := exporter.ExportJSON(timeline, outputDir)
		if err != nil {
			return err
		}
		fmt.Printf("Exported → %s\n", outPath)
	case "markdown", "md":
		outPath, err := exporter.ExportMarkdown(timeline, outputDir)
		if err != nil {
			return err
		}
		fmt.Printf("Exported → %s\n", outPath)
	default:
		return fmt.Errorf("unsupported format: %s (use json or markdown)", exportFormat)
	}

	return nil
}
