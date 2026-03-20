package parser

import "github.com/anthropics/aitrace/internal/model"

// Parser can detect and parse agent log files into unified sessions.
type Parser interface {
	// Name returns the agent name (e.g. "claude_code").
	Name() string
	// Detect finds log file paths for the current project directory.
	Detect(projectDir string) ([]string, error)
	// Parse reads a log file and returns sessions.
	Parse(path string) ([]model.Session, error)
}

// AllParsers returns instances of every supported parser.
func AllParsers() []Parser {
	return []Parser{
		&ClaudeParser{},
		&GeminiParser{},
		&CodexParser{},
	}
}
