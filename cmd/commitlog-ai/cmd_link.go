package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PeterTakahashi/commitlog-ai/internal/cache"
	"github.com/PeterTakahashi/commitlog-ai/internal/linker"
	"github.com/PeterTakahashi/commitlog-ai/internal/parser"
	"github.com/PeterTakahashi/commitlog-ai/internal/sanitizer"
	"github.com/PeterTakahashi/commitlog-ai/internal/userpath"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Match parsed sessions to git commits",
	Long:  `Links parsed agent sessions to git commit history using timestamp and file path matching.`,
	RunE:  runLink,
}

var (
	linkProjectDir string
	linkForce      bool
)

func init() {
	linkCmd.Flags().StringVarP(&linkProjectDir, "project", "p", ".", "Project directory")
	linkCmd.Flags().BoolVar(&linkForce, "force", false, "Force re-link ignoring cache")
	rootCmd.AddCommand(linkCmd)
}

func runLink(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(linkProjectDir)
	if err != nil {
		return err
	}

	// Read all per-user session files
	sessions, sessionFiles, err := userpath.ReadAllSessions(absDir)
	if err != nil {
		return fmt.Errorf("reading sessions: %w", err)
	}
	if len(sessions) == 0 {
		return fmt.Errorf("no parsed sessions found. Run 'commitlog-ai parse' first")
	}

	// Check cache
	git := linker.NewGitClient(absDir)
	if !linkForce {
		gitHead, err := git.GetHead()
		if err == nil {
			c := cache.Load(absDir)
			if c.IsLinkValid(parser.ParserVersion, sessionFiles, gitHead) {
				fmt.Println("Using cached link results (no changes detected)")
				fmt.Println("Use --force to regenerate")
				return nil
			}
		}
	}

	// Get git commits
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	commits, err := git.GetCommits()
	if err != nil {
		return fmt.Errorf("getting git commits: %w", err)
	}

	fmt.Printf("Found %d session(s) and %d commit(s)\n", len(sessions), len(commits))

	// Enrich commits with diff stats
	for i := range commits {
		fc, add, del, files, err := git.GetDiffStats(commits[i].Hash)
		if err != nil {
			continue
		}
		commits[i].FilesChanged = fc
		commits[i].Additions = add
		commits[i].Deletions = del
		commits[i].ChangedFiles = files
	}

	// Match sessions to commits
	timeline := linker.Match(sessions, commits)
	timeline.GitRepo = repoRoot

	// Sanitize secrets in timeline data
	timeline = sanitizer.SanitizeTimeline(timeline)

	// Count matches
	linked := 0
	for _, e := range timeline.Entries {
		if e.Commit != nil && e.Session != nil {
			linked++
		}
	}

	// Write output
	outPath := filepath.Join(absDir, ".commitlog-ai", "timeline.json")
	outData, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling timeline: %w", err)
	}

	if err := os.WriteFile(outPath, outData, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Printf("Linked %d pair(s), %d total entries → %s\n", linked, len(timeline.Entries), outPath)

	// Update cache
	if gitHead, err := git.GetHead(); err == nil {
		c := cache.Load(absDir)
		c.UpdateLink(parser.ParserVersion, sessionFiles, gitHead, outPath)
		c.Save()
	}

	return nil
}
