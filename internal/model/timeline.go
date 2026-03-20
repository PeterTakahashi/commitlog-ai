package model

import "time"

// LinkedTimeline is the result of matching sessions to git commits.
type LinkedTimeline struct {
	Entries []TimelineEntry `json:"entries"`
	GitRepo string          `json:"git_repo"`
}

// TimelineEntry pairs a git commit with an agent session.
type TimelineEntry struct {
	Commit         *GitCommit `json:"commit,omitempty"`
	Session        *Session   `json:"session,omitempty"`
	LinkConfidence float64    `json:"link_confidence"`
}

// GitCommit holds metadata about a single git commit.
type GitCommit struct {
	Hash         string    `json:"hash"`
	Author       string    `json:"author"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	FilesChanged int       `json:"files_changed"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	ChangedFiles []string  `json:"changed_files,omitempty"`
}
