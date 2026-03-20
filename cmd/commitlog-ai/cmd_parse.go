package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeterTakahashi/commitlog-ai/internal/cache"
	"github.com/PeterTakahashi/commitlog-ai/internal/linker"
	"github.com/PeterTakahashi/commitlog-ai/internal/model"
	"github.com/PeterTakahashi/commitlog-ai/internal/parser"
	"github.com/PeterTakahashi/commitlog-ai/internal/sanitizer"
	"github.com/PeterTakahashi/commitlog-ai/internal/userpath"
	"github.com/spf13/cobra"
)

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse agent logs into unified format",
	Long:  `Reads Claude Code, Gemini CLI, and Codex CLI logs and converts them to a unified JSON format.`,
	RunE:  runParse,
}

var (
	parseProjectDir string
	parseForce      bool
)

func init() {
	parseCmd.Flags().StringVarP(&parseProjectDir, "project", "p", ".", "Project directory to scan for")
	parseCmd.Flags().BoolVar(&parseForce, "force", false, "Force re-parse ignoring cache")
	rootCmd.AddCommand(parseCmd)
}

func runParse(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(parseProjectDir)
	if err != nil {
		return err
	}

	// Detect all source log files first
	parsers := parser.AllParsers()
	type parserFiles struct {
		parser parser.Parser
		files  []string
	}
	var detected []parserFiles
	var allSourceFiles []string

	for _, p := range parsers {
		files, err := p.Detect(absDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s detection failed: %v\n", p.Name(), err)
			continue
		}
		if len(files) > 0 {
			detected = append(detected, parserFiles{parser: p, files: files})
			allSourceFiles = append(allSourceFiles, files...)
		}
	}

	// Check cache
	if !parseForce {
		c := cache.Load(absDir)
		if c.IsParseValid(parser.ParserVersion, allSourceFiles) {
			fmt.Println("Using cached parse results (no changes detected)")
			fmt.Println("Use --force to regenerate")
			return nil
		}
	}

	// Parse all detected files
	var allSessions []model.Session
	for _, pf := range detected {
		fmt.Printf("[%s] Found %d log file(s)\n", pf.parser.Name(), len(pf.files))
		for _, f := range pf.files {
			sessions, err := pf.parser.Parse(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to parse %s: %v\n", f, err)
				continue
			}
			for _, s := range sessions {
				fmt.Printf("  Session %s: %d messages (%s to %s)\n",
					s.ID[:8], len(s.Messages),
					s.StartedAt.Format("15:04:05"),
					s.EndedAt.Format("15:04:05"))
			}
			allSessions = append(allSessions, sessions...)
		}
	}

	if len(allSessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	// Get user name for per-user directory
	git := linker.NewGitClient(absDir)
	userName, err := git.GetUserName()
	if err != nil {
		return fmt.Errorf("git config user.name not set: %w", err)
	}

	// Migrate legacy sessions.json if present
	if err := userpath.MigrateLegacy(absDir, userName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to migrate legacy sessions: %v\n", err)
	}

	// Sanitize secrets before writing
	allSessions = sanitizer.SanitizeSessions(allSessions)

	// Write output to per-user sessions directory
	userDir := userpath.UserSessionsDir(absDir, userName)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("creating user sessions directory: %w", err)
	}

	outPath := userpath.UserSessionsPath(absDir, userName)
	data, err := json.MarshalIndent(allSessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sessions: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Printf("\nParsed %d session(s) → %s\n", len(allSessions), outPath)

	// Update cache
	c := cache.Load(absDir)
	c.UpdateParse(parser.ParserVersion, allSourceFiles, outPath)
	c.Save()

	return nil
}
