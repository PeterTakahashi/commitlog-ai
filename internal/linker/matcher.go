package linker

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/aitrace/internal/model"
)

const (
	maxMatchWindow = 5 * time.Minute
)

// Match links sessions and commits into a timeline.
func Match(sessions []model.Session, commits []model.GitCommit) model.LinkedTimeline {
	// Track which sessions/commits have been matched
	sessionMatched := make(map[int]bool)
	commitMatched := make(map[int]bool)

	var entries []model.TimelineEntry

	// For each commit, find the best matching session
	for ci, commit := range commits {
		bestScore := 0.0
		bestSession := -1

		for si, session := range sessions {
			score := computeConfidence(commit, session)
			if score > bestScore {
				bestScore = score
				bestSession = si
			}
		}

		if bestSession >= 0 && bestScore > 0.3 {
			s := sessions[bestSession]
			c := commits[ci]
			entries = append(entries, model.TimelineEntry{
				Commit:         &c,
				Session:        &s,
				LinkConfidence: bestScore,
			})
			sessionMatched[bestSession] = true
			commitMatched[ci] = true
		}
	}

	// Add unmatched commits
	for ci, commit := range commits {
		if !commitMatched[ci] {
			c := commit
			entries = append(entries, model.TimelineEntry{
				Commit: &c,
			})
		}
	}

	// Add unmatched sessions
	for si, session := range sessions {
		if !sessionMatched[si] {
			s := session
			entries = append(entries, model.TimelineEntry{
				Session: &s,
			})
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		ti := entryTime(entries[i])
		tj := entryTime(entries[j])
		return ti.After(tj)
	})

	return model.LinkedTimeline{Entries: entries}
}

func computeConfidence(commit model.GitCommit, session model.Session) float64 {
	score := 0.0

	// Time-based matching
	if !commit.Timestamp.Before(session.StartedAt) && !commit.Timestamp.After(session.EndedAt) {
		// Commit within session time range → high confidence
		score = 0.9
	} else if commit.Timestamp.After(session.EndedAt) && commit.Timestamp.Sub(session.EndedAt) <= maxMatchWindow {
		// Commit shortly after session ended → user committed after agent finished
		score = 0.7
	} else if commit.Timestamp.Before(session.StartedAt) && session.StartedAt.Sub(commit.Timestamp) <= maxMatchWindow {
		// Session started shortly after commit → lower confidence
		score = 0.5
	} else {
		return 0 // No time proximity
	}

	// File path overlap bonus
	sessionFiles := extractFilePaths(session)
	if len(commit.ChangedFiles) > 0 && len(sessionFiles) > 0 {
		overlap := fileOverlap(commit.ChangedFiles, sessionFiles)
		if overlap > 0 {
			score += 0.1
		}
	}

	// Git branch match bonus
	if session.GitBranch != "" && session.GitBranch != "HEAD" {
		// We don't have branch info on commits from git log, but sessions carry it
		score += 0.05
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func extractFilePaths(session model.Session) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, msg := range session.Messages {
		for _, tc := range msg.ToolCalls {
			// Try to extract file paths from tool inputs
			// Common patterns: file_edit, write_file, read_file, etc.
			input := tc.Input
			for _, candidate := range extractPathsFromString(input) {
				if !seen[candidate] {
					seen[candidate] = true
					paths = append(paths, candidate)
				}
			}
		}
	}
	return paths
}

func extractPathsFromString(s string) []string {
	var paths []string
	// Simple heuristic: look for strings that look like file paths
	for _, word := range strings.Fields(s) {
		// Remove JSON quotes
		word = strings.Trim(word, `",'{}[]`)
		if looksLikeFilePath(word) {
			paths = append(paths, word)
		}
	}
	return paths
}

func looksLikeFilePath(s string) bool {
	if len(s) < 3 {
		return false
	}
	// Must contain a dot (extension) or slash (directory)
	hasDot := strings.Contains(filepath.Base(s), ".")
	hasSlash := strings.Contains(s, "/")
	// Common source file extensions
	extensions := []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".rb", ".css", ".html", ".json", ".yaml", ".yml", ".toml", ".md"}
	for _, ext := range extensions {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return hasDot && hasSlash
}

func fileOverlap(commitFiles, sessionFiles []string) int {
	commitSet := make(map[string]bool)
	for _, f := range commitFiles {
		commitSet[filepath.Base(f)] = true
		commitSet[f] = true
	}
	overlap := 0
	for _, f := range sessionFiles {
		if commitSet[filepath.Base(f)] || commitSet[f] {
			overlap++
		}
	}
	return overlap
}

func entryTime(e model.TimelineEntry) time.Time {
	if e.Commit != nil {
		return e.Commit.Timestamp
	}
	if e.Session != nil {
		return e.Session.StartedAt
	}
	return time.Time{}
}
