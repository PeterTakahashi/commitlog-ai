package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

// ExportJSON writes the timeline to a JSON file in the output directory.
func ExportJSON(timeline model.LinkedTimeline, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	outPath := filepath.Join(outputDir, "timeline.json")
	data, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling timeline: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", outPath, err)
	}

	return outPath, nil
}
