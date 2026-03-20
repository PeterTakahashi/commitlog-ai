package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

// GeminiParser reads Gemini CLI session logs.
type GeminiParser struct{}

func (p *GeminiParser) Name() string { return "gemini_cli" }

func (p *GeminiParser) Detect(projectDir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	absProject, _ := filepath.Abs(projectDir)

	// Gemini CLI stores logs in ~/.gemini/tmp/<dir>/chats/session-*.json
	// where <dir> is either the project base name or a hash.
	// Each dir has a .project_root file containing the absolute project path.
	tmpDir := filepath.Join(home, ".gemini", "tmp")
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, nil // not installed or no logs
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Check .project_root to match project directory
		rootFile := filepath.Join(tmpDir, entry.Name(), ".project_root")
		rootBytes, err := os.ReadFile(rootFile)
		if err != nil {
			continue
		}
		projectRoot := strings.TrimSpace(string(rootBytes))

		// Resolve symlinks / /private/tmp → /tmp differences
		resolvedRoot, _ := filepath.EvalSymlinks(projectRoot)
		resolvedProject, _ := filepath.EvalSymlinks(absProject)
		if resolvedRoot == "" {
			resolvedRoot = projectRoot
		}
		if resolvedProject == "" {
			resolvedProject = absProject
		}

		if resolvedRoot != resolvedProject && projectRoot != absProject {
			continue
		}

		chatsDir := filepath.Join(tmpDir, entry.Name(), "chats")
		chatFiles, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}
		for _, f := range chatFiles {
			if strings.HasPrefix(f.Name(), "session-") && strings.HasSuffix(f.Name(), ".json") && !f.IsDir() {
				files = append(files, filepath.Join(chatsDir, f.Name()))
			}
		}
	}
	return files, nil
}

// Gemini session JSON structure
type geminiSession struct {
	SessionID   string          `json:"sessionId"`
	StartTime   string          `json:"startTime"`
	LastUpdated string          `json:"lastUpdated"`
	Messages    []geminiMessage `json:"messages"`
	Kind        string          `json:"kind"`
}

type geminiMessage struct {
	ID        string             `json:"id"`
	Timestamp string             `json:"timestamp"`
	Type      string             `json:"type"` // "user" | "gemini"
	Content   json.RawMessage    `json:"content"`
	Thoughts  []geminiThought    `json:"thoughts,omitempty"`
	Tokens    *geminiTokens      `json:"tokens,omitempty"`
	Model     string             `json:"model,omitempty"`
	ToolCalls []geminiToolCall   `json:"toolCalls,omitempty"`
}

type geminiThought struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

type geminiTokens struct {
	Input    int `json:"input"`
	Output   int `json:"output"`
	Cached   int `json:"cached"`
	Thoughts int `json:"thoughts"`
	Tool     int `json:"tool"`
	Total    int `json:"total"`
}

type geminiToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
	Result []geminiToolResult `json:"result,omitempty"`
	Status string          `json:"status"`
}

type geminiToolResult struct {
	FunctionResponse struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Response struct {
			Output string `json:"output"`
		} `json:"response"`
	} `json:"functionResponse"`
}

func (p *GeminiParser) Parse(path string) ([]model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gs geminiSession
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Skip non-main sessions (sidechains)
	if gs.Kind != "" && gs.Kind != "main" {
		return nil, nil
	}

	if len(gs.Messages) == 0 {
		return nil, nil
	}

	var messages []model.Message
	var agentModel string

	for _, gm := range gs.Messages {
		ts, _ := time.Parse(time.RFC3339Nano, gm.Timestamp)

		role := "human"
		if gm.Type == "gemini" {
			role = "assistant"
		}

		// Parse content - can be a string or an array of objects
		var content string
		var rawStr string
		if err := json.Unmarshal(gm.Content, &rawStr); err == nil {
			content = rawStr
		} else {
			// Content is an array of text blocks (user messages)
			var blocks []struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(gm.Content, &blocks); err == nil {
				var parts []string
				for _, b := range blocks {
					parts = append(parts, b.Text)
				}
				content = strings.Join(parts, "\n")
			}
		}

		if gm.Model != "" {
			agentModel = gm.Model
		}

		// Convert tool calls
		var toolCalls []model.ToolCall
		for _, tc := range gm.ToolCalls {
			argsStr := string(tc.Args)
			var output string
			if len(tc.Result) > 0 {
				output = tc.Result[0].FunctionResponse.Response.Output
			}
			toolCalls = append(toolCalls, model.ToolCall{
				Tool:   tc.Name,
				Input:  argsStr,
				Output: output,
			})
		}

		m := model.Message{
			Role:      role,
			Content:   content,
			Timestamp: ts,
		}
		if role == "assistant" {
			m.ToolCalls = toolCalls
			m.Model = gm.Model
			if gm.Tokens != nil {
				m.Usage = &model.TokenUsage{
					InputTokens:  gm.Tokens.Input,
					OutputTokens: gm.Tokens.Output,
				}
			}
		}
		messages = append(messages, m)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	startedAt, _ := time.Parse(time.RFC3339Nano, gs.StartTime)
	endedAt, _ := time.Parse(time.RFC3339Nano, gs.LastUpdated)
	if startedAt.IsZero() {
		startedAt = messages[0].Timestamp
	}
	if endedAt.IsZero() {
		endedAt = messages[len(messages)-1].Timestamp
	}

	session := model.Session{
		ID:        gs.SessionID,
		Agent:     model.Agent{Tool: "gemini_cli", Model: agentModel},
		Project:   filepath.Base(filepath.Dir(filepath.Dir(path))), // chats -> project dir
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}
