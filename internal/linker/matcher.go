package linker

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

const (
	maxMatchWindow = 5 * time.Minute
)

// Match links sessions and commits into a timeline.
// When a session spans multiple commits, it splits the session messages
// into segments so each commit is paired with the relevant conversation portion.
func Match(sessions []model.Session, commits []model.GitCommit) model.LinkedTimeline {
	commitUsed := make(map[int]bool)
	sessionUsed := make(map[int]bool)

	var entries []model.TimelineEntry

	// For each session, find all commits that fall within its timeframe
	for si := range sessions {
		session := &sessions[si]
		var matchedCommits []indexedCommit

		for ci := range commits {
			score := computeConfidence(commits[ci], *session)
			if score > 0.3 {
				matchedCommits = append(matchedCommits, indexedCommit{
					index:      ci,
					commit:     &commits[ci],
					confidence: score,
				})
			}
		}

		if len(matchedCommits) == 0 {
			continue
		}

		sessionUsed[si] = true
		for _, mc := range matchedCommits {
			commitUsed[mc.index] = true
		}

		// Sort matched commits by timestamp (oldest first) for segmentation
		sort.Slice(matchedCommits, func(i, j int) bool {
			return matchedCommits[i].commit.Timestamp.Before(matchedCommits[j].commit.Timestamp)
		})

		if len(matchedCommits) == 1 {
			// Single commit: link entire session
			c := *matchedCommits[0].commit
			s := *session
			entries = append(entries, model.TimelineEntry{
				Commit:          &c,
				Session:         &s,
				LinkConfidence:  matchedCommits[0].confidence,
				MessageStartIdx: 0,
				MessageEndIdx:   len(s.Messages),
			})
		} else {
			// Multiple commits: split session messages into segments
			segmentEntries := splitSessionByCommits(session, matchedCommits)
			entries = append(entries, segmentEntries...)
		}
	}

	// Add unmatched commits
	for ci := range commits {
		if !commitUsed[ci] {
			c := commits[ci]
			entries = append(entries, model.TimelineEntry{
				Commit: &c,
			})
		}
	}

	// Add unmatched sessions
	for si := range sessions {
		if !sessionUsed[si] {
			s := sessions[si]
			entries = append(entries, model.TimelineEntry{
				Session:         &s,
				MessageStartIdx: 0,
				MessageEndIdx:   len(s.Messages),
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

type indexedCommit struct {
	index      int
	commit     *model.GitCommit
	confidence float64
}

// splitSessionByCommits creates one TimelineEntry per commit, with each entry
// containing the message range between the previous commit and this commit.
// When commits are very close together, segments may overlap to ensure each
// commit has at least some conversation context.
func splitSessionByCommits(session *model.Session, commits []indexedCommit) []model.TimelineEntry {
	messages := session.Messages
	var entries []model.TimelineEntry

	// First pass: compute raw boundaries
	type segment struct {
		startIdx int
		endIdx   int
	}
	segments := make([]segment, len(commits))

	for i := range commits {
		var startIdx, endIdx int

		if i == 0 {
			startIdx = 0
		} else {
			prevCommitTime := commits[i-1].commit.Timestamp
			startIdx = findMessageIndexAfter(messages, prevCommitTime)
		}

		if i == len(commits)-1 {
			endIdx = len(messages)
		} else {
			endIdx = findMessageIndexAfter(messages, commits[i].commit.Timestamp)
		}

		// Clamp
		if startIdx > len(messages) {
			startIdx = len(messages)
		}
		if endIdx > len(messages) {
			endIdx = len(messages)
		}
		if endIdx < startIdx {
			endIdx = startIdx
		}

		segments[i] = segment{startIdx, endIdx}
	}

	// Second pass: fix empty segments by extending backwards with overlap
	const minMessages = 4 // minimum messages per segment
	for i := range segments {
		if segments[i].endIdx-segments[i].startIdx < minMessages && i > 0 {
			// Extend start backwards into previous segment's range
			newStart := segments[i].endIdx - minMessages
			if newStart < segments[i-1].startIdx {
				newStart = segments[i-1].startIdx
			}
			if newStart < 0 {
				newStart = 0
			}
			segments[i].startIdx = newStart
		}
	}

	for i, mc := range commits {
		c := *mc.commit
		s := *session
		entries = append(entries, model.TimelineEntry{
			Commit:          &c,
			Session:         &s,
			LinkConfidence:  mc.confidence,
			MessageStartIdx: segments[i].startIdx,
			MessageEndIdx:   segments[i].endIdx,
		})
	}

	return entries
}

// findMessageIndexAfter returns the index of the first message whose timestamp
// is after the given time. Uses the message timestamps to split segments.
func findMessageIndexAfter(messages []model.Message, t time.Time) int {
	for i, msg := range messages {
		if msg.Timestamp.After(t) {
			return i
		}
	}
	return len(messages)
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
	for _, word := range strings.Fields(s) {
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
	hasDot := strings.Contains(filepath.Base(s), ".")
	hasSlash := strings.Contains(s, "/")
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
