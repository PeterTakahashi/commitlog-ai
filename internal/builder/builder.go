package builder

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

// ProgressFunc is called with (step description, current, total).
// current=0,total=0 means indeterminate.
type ProgressFunc func(step string, current, total int)

// Build runs parse + link for the given project directory.
// Uses cache to skip work when nothing changed.
func Build(projectDir string) (*Result, error) {
	return BuildWithProgress(projectDir, nil)
}

// BuildWithProgress runs parse + link with optional progress reporting.
func BuildWithProgress(projectDir string, progress ProgressFunc) (*Result, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}
	result := &Result{}

	// --- Parse phase ---
	progress("Detecting log sources...", 0, 0)
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

	// Get user name for per-user directory
	git := linker.NewGitClient(projectDir)
	userName, err := git.GetUserName()
	if err != nil {
		return nil, fmt.Errorf("git config user.name not set: %w", err)
	}

	// Migrate legacy sessions.json if present
	userpath.MigrateLegacy(projectDir, userName)

	outDir := filepath.Join(projectDir, ".commitlog-ai")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	sessionsPath := userpath.UserSessionsPath(projectDir, userName)

	// Check parse cache
	c := cache.Load(projectDir)
	if c.IsParseValid(parser.ParserVersion, allSourceFiles) {
		progress("Parse: using cache", 1, 1)
		result.ParseCached = true
	} else {
		var allSessions []model.Session
		totalFiles := 0
		for _, pf := range detected {
			totalFiles += len(pf.files)
		}
		parsed := 0
		for _, pf := range detected {
			for _, f := range pf.files {
				parsed++
				progress("Parsing sessions...", parsed, totalFiles)
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

		// Write to per-user directory
		userDir := userpath.UserSessionsDir(projectDir, userName)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return nil, fmt.Errorf("creating user sessions directory: %w", err)
		}

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
	progress("Reading git history...", 0, 0)
	gitHead, err := git.GetHead()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	timelinePath := filepath.Join(outDir, "timeline.json")

	// Read all per-user session files for linking
	allSessions, sessionFiles, err := userpath.ReadAllSessions(projectDir)
	if err != nil {
		return nil, fmt.Errorf("reading sessions: %w", err)
	}

	// Reload cache (UpdateParse may have changed it)
	c = cache.Load(projectDir)
	if c.IsLinkValid(parser.ParserVersion, sessionFiles, gitHead) {
		progress("Link: using cache", 1, 1)
		result.LinkCached = true
		return result, nil
	}

	if len(allSessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	progress("Loading commits...", 0, 0)
	commits, err := git.GetCommits()
	if err != nil {
		return nil, fmt.Errorf("getting git commits: %w", err)
	}

	// Enrich with diff stats
	for i := range commits {
		progress("Analyzing diffs...", i+1, len(commits))
		fc, add, del, files, err := git.GetDiffStats(commits[i].Hash)
		if err != nil {
			continue
		}
		commits[i].FilesChanged = fc
		commits[i].Additions = add
		commits[i].Deletions = del
		commits[i].ChangedFiles = files
	}

	progress("Matching sessions to commits...", 0, 0)
	timeline := linker.Match(allSessions, commits)
	timeline.GitRepo = repoRoot
	timeline = sanitizer.SanitizeTimeline(timeline)

	result.SessionCount = len(allSessions)
	result.CommitCount = len(commits)
	for _, e := range timeline.Entries {
		if e.Commit != nil && e.Session != nil {
			result.LinkedCount++
		}
	}
	result.EntryCount = len(timeline.Entries)

	progress("Writing timeline...", 0, 0)
	outData, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling timeline: %w", err)
	}
	if err := os.WriteFile(timelinePath, outData, 0644); err != nil {
		return nil, fmt.Errorf("writing timeline: %w", err)
	}

	c = cache.Load(projectDir)
	c.UpdateLink(parser.ParserVersion, sessionFiles, gitHead, timelinePath)
	c.Save()

	progress("Done", 1, 1)
	return result, nil
}
