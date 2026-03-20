package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

// ClaudeParser reads Claude Code JSONL session logs.
type ClaudeParser struct{}

func (p *ClaudeParser) Name() string { return "claude_code" }

func (p *ClaudeParser) Detect(projectDir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Claude Code stores logs in ~/.claude/projects/<project-hash>/
	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, nil // not installed or no logs
	}

	absProject, _ := filepath.Abs(projectDir)
	// Project hash format: path with / replaced by -
	// e.g. /Users/apple/dev/myproject -> -Users-apple-dev-myproject
	projectHash := strings.ReplaceAll(absProject, "/", "-")

	var files []string
	for _, entry := range entries {
		if entry.Name() == projectHash && entry.IsDir() {
			dir := filepath.Join(projectsDir, entry.Name())
			sessions, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, s := range sessions {
				if strings.HasSuffix(s.Name(), ".jsonl") && !s.IsDir() {
					files = append(files, filepath.Join(dir, s.Name()))
				}
			}
		}
	}
	return files, nil
}

// Raw JSONL entry from Claude Code logs
type claudeEntry struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	CWD         string          `json:"cwd"`
	GitBranch   string          `json:"gitBranch"`
	Message     json.RawMessage `json:"message"`
	UUID        string          `json:"uuid"`
}

type claudeMessage struct {
	Role       string               `json:"role"`
	Model      string               `json:"model"`
	RawContent json.RawMessage      `json:"content"`
	Content    []claudeContentBlock `json:"-"` // populated after custom unmarshal
	Usage      *claudeUsage         `json:"usage,omitempty"`
	// StringContent holds the text when content is a plain string (Claude Code terminal user prompts)
	StringContent string `json:"-"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type claudeContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id"`        // tool_use_id
	Name      string `json:"name"`      // tool name
	Input     any    `json:"input"`     // tool input
	ToolUseID string `json:"tool_use_id"` // for tool_result
	Content   any    `json:"content"`   // tool_result content
}

func (p *ClaudeParser) Parse(path string) ([]model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0), 10*1024*1024) // 10MB max line

	var messages []model.Message
	var sessionID, cwd, gitBranch, agentModel string
	pendingToolCalls := make(map[string]*model.ToolCall) // tool_use_id -> ToolCall

	for scanner.Scan() {
		var entry claudeEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		// Skip non-message entries and subagent messages
		if entry.IsSidechain {
			continue
		}
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		if sessionID == "" {
			sessionID = entry.SessionID
		}
		if entry.CWD != "" {
			cwd = entry.CWD
		}
		if entry.GitBranch != "" {
			gitBranch = entry.GitBranch
		}

		var msg claudeMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// content can be a string (user prompt in terminal Claude Code) or an array of content blocks
		if len(msg.RawContent) > 0 {
			if msg.RawContent[0] == '"' {
				// String content – user typed prompt
				var s string
				if err := json.Unmarshal(msg.RawContent, &s); err == nil {
					msg.StringContent = s
				}
			} else if msg.RawContent[0] == '[' {
				// Array content – normal content blocks
				var blocks []claudeContentBlock
				if err := json.Unmarshal(msg.RawContent, &blocks); err == nil {
					msg.Content = blocks
				}
			}
		}

		if msg.Model != "" {
			agentModel = msg.Model
		}

		ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

		var textParts []string
		var toolCalls []model.ToolCall

		// If content was a plain string, use it directly
		if msg.StringContent != "" {
			textParts = append(textParts, msg.StringContent)
		}

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				textParts = append(textParts, block.Text)
			case "tool_use":
				inputJSON, _ := json.Marshal(block.Input)
				tc := model.ToolCall{
					Tool:  block.Name,
					Input: string(inputJSON),
				}
				toolCalls = append(toolCalls, tc)
				// Store pending for output matching
				pendingToolCalls[block.ID] = &toolCalls[len(toolCalls)-1]
			case "tool_result":
				// Match output to pending tool call
				if tc, ok := pendingToolCalls[block.ToolUseID]; ok {
					switch v := block.Content.(type) {
					case string:
						tc.Output = v
					case []any:
						// Content is array of content blocks
						var parts []string
						for _, item := range v {
							if m, ok := item.(map[string]any); ok {
								if text, ok := m["text"].(string); ok {
									parts = append(parts, text)
								}
							}
						}
						tc.Output = strings.Join(parts, "\n")
					default:
						out, _ := json.Marshal(block.Content)
						tc.Output = string(out)
					}
					delete(pendingToolCalls, block.ToolUseID)
				}
			}
		}

		role := "human"
		if entry.Type == "assistant" {
			role = "assistant"
		}

		m := model.Message{
			Role:      role,
			Content:   strings.Join(textParts, "\n"),
			Timestamp: ts,
		}
		if role == "assistant" {
			m.ToolCalls = toolCalls
			m.Model = msg.Model
			if msg.Usage != nil {
				m.Usage = &model.TokenUsage{
					InputTokens:              msg.Usage.InputTokens,
					OutputTokens:             msg.Usage.OutputTokens,
					CacheCreationInputTokens: msg.Usage.CacheCreationInputTokens,
					CacheReadInputTokens:     msg.Usage.CacheReadInputTokens,
				}
			}
		}
		messages = append(messages, m)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s: %w", path, err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	// Derive project name from CWD
	project := filepath.Base(cwd)
	if project == "" || project == "." {
		project = filepath.Base(path)
	}

	session := model.Session{
		ID:        sessionID,
		Agent:     model.Agent{Tool: "claude_code", Model: agentModel},
		Project:   project,
		CWD:       cwd,
		GitBranch: gitBranch,
		StartedAt: messages[0].Timestamp,
		EndedAt:   messages[len(messages)-1].Timestamp,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}
