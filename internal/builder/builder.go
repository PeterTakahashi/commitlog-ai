package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aitrace/internal/cache"
	"github.com/anthropics/aitrace/internal/linker"
	"github.com/anthropics/aitrace/internal/model"
	"github.com/anthropics/aitrace/internal/parser"
	"github.com/anthropics/aitrace/internal/sanitizer"
)

// Result holds the outcome of a build.
type Result struct {
	SessionCount int
	CommitCount  int
	LinkedCount  int
	EntryCount   int
	ParseCached  bool
	LinkCached   bool
}

// Build runs parse + link for the given project directory.
// Uses cache to skip work when nothing changed.
func Build(projectDir string) (*Result, error) {
	result := &Result{}

	// --- Parse phase ---
	parsers := parser.AllParsers()
	type parserFiles struct {
		parser parser.Parser
		files  []string
	}
	var detected []parserFiles
	var allSourceFiles []string

	for _, p := range parsers {
		files, err := p.Detect(projectDir)
		if err != nil {
			continue
		}
		if len(files) > 0 {
			detected = append(detected, parserFiles{parser: p, files: files})
			allSourceFiles = append(allSourceFiles, files...)
		}
	}

	outDir := filepath.Join(projectDir, ".aitrace")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	sessionsPath := filepath.Join(outDir, "sessions.json")

	// Check parse cache
	c := cache.Load(projectDir)
	if c.IsParseValid(parser.ParserVersion, allSourceFiles) {
		result.ParseCached = true
	} else {
		var allSessions []model.Session
		for _, pf := range detected {
			for _, f := range pf.files {
				sessions, err := pf.parser.Parse(f)
				if err != nil {
					continue
				}
				allSessions = append(allSessions, sessions...)
			}
		}

		if len(allSessions) == 0 {
			return nil, fmt.Errorf("no sessions found")
		}

		allSessions = sanitizer.SanitizeSessions(allSessions)
		result.SessionCount = len(allSessions)

		data, err := json.MarshalIndent(allSessions, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling sessions: %w", err)
		}
		if err := os.WriteFile(sessionsPath, data, 0644); err != nil {
			return nil, fmt.Errorf("writing sessions: %w", err)
		}

		c.UpdateParse(parser.ParserVersion, allSourceFiles, sessionsPath)
		c.Save()
	}

	// --- Link phase ---
	git := linker.NewGitClient(projectDir)
	gitHead, err := git.GetHead()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	timelinePath := filepath.Join(outDir, "timeline.json")

	// Reload cache (UpdateParse may have changed it)
	c = cache.Load(projectDir)
	if c.IsLinkValid(parser.ParserVersion, sessionsPath, gitHead) {
		result.LinkCached = true
		return result, nil
	}

	// Read sessions
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		return nil, fmt.Errorf("reading sessions: %w", err)
	}
	var sessions []model.Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions: %w", err)
	}

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	commits, err := git.GetCommits()
	if err != nil {
		return nil, fmt.Errorf("getting git commits: %w", err)
	}

	// Enrich with diff stats
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

	timeline := linker.Match(sessions, commits)
	timeline.GitRepo = repoRoot
	timeline = sanitizer.SanitizeTimeline(timeline)

	result.SessionCount = len(sessions)
	result.CommitCount = len(commits)
	for _, e := range timeline.Entries {
		if e.Commit != nil && e.Session != nil {
			result.LinkedCount++
		}
	}
	result.EntryCount = len(timeline.Entries)

	outData, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling timeline: %w", err)
	}
	if err := os.WriteFile(timelinePath, outData, 0644); err != nil {
		return nil, fmt.Errorf("writing timeline: %w", err)
	}

	c = cache.Load(projectDir)
	c.UpdateLink(parser.ParserVersion, sessionsPath, gitHead, timelinePath)
	c.Save()

	return result, nil
}
