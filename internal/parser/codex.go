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

// CodexParser reads Codex CLI session logs.
// Supports both the new JSONL format (v0.1+) and legacy JSON format.
type CodexParser struct{}

func (p *CodexParser) Name() string { return "codex_cli" }

func (p *CodexParser) Detect(projectDir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sessionsDir := filepath.Join(home, ".codex", "sessions")
	if _, err := os.Stat(sessionsDir); err != nil {
		return nil, nil // not installed or no logs
	}

	absProject, _ := filepath.Abs(projectDir)

	// Codex stores logs in ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl (new)
	// and ~/.codex/sessions/rollout-*.json (legacy)
	// We need to check each session's CWD to match the project.
	var files []string

	err = filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		isJSONL := strings.HasSuffix(path, ".jsonl") && strings.Contains(info.Name(), "rollout-")
		isJSON := strings.HasSuffix(path, ".json") && strings.Contains(info.Name(), "rollout-")
		if !isJSONL && !isJSON {
			return nil
		}

		// Quick check: read first few KB to find CWD
		cwd := p.extractCWD(path, isJSONL)
		if cwd == "" {
			return nil
		}

		resolvedCWD, _ := filepath.EvalSymlinks(cwd)
		resolvedProject, _ := filepath.EvalSymlinks(absProject)
		if resolvedCWD == "" {
			resolvedCWD = cwd
		}
		if resolvedProject == "" {
			resolvedProject = absProject
		}

		if resolvedCWD == resolvedProject || cwd == absProject {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil
	}

	return files, nil
}

// extractCWD reads just enough of the file to get the session CWD.
func (p *CodexParser) extractCWD(path string, isJSONL bool) string {
	if isJSONL {
		return p.extractCWDFromJSONL(path)
	}
	return p.extractCWDFromJSON(path)
}

func (p *CodexParser) extractCWDFromJSONL(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0), 1*1024*1024)

	// Read up to 10 lines to find session_meta
	for i := 0; i < 10 && scanner.Scan(); i++ {
		var entry codexJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type == "session_meta" {
			var payload codexSessionMeta
			if err := json.Unmarshal(entry.Payload, &payload); err == nil {
				return payload.CWD
			}
		}
	}
	return ""
}

func (p *CodexParser) extractCWDFromJSON(path string) string {
	// Legacy format: look for CWD in items or session metadata
	// Read first 8KB
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	content := string(buf[:n])

	// Try to find cwd in the partial JSON
	if idx := strings.Index(content, `"cwd"`); idx >= 0 {
		// Simple extraction
		rest := content[idx:]
		if start := strings.Index(rest, `"`+":"+`"`); start >= 0 {
			rest = rest[start+3:]
			if end := strings.Index(rest, `"`); end >= 0 {
				return rest[:end]
			}
		}
	}
	return ""
}

func (p *CodexParser) Parse(path string) ([]model.Session, error) {
	if strings.HasSuffix(path, ".jsonl") {
		return p.parseJSONL(path)
	}
	return p.parseJSON(path)
}

// --- JSONL format (new, v0.1+) ---

type codexJSONLEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"` // "session_meta", "event_msg", "response_item", "turn_context"
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	CWD        string `json:"cwd"`
	CLIVersion string `json:"cli_version"`
	Source     string `json:"source"`
	Git        *struct {
		CommitHash    string `json:"commit_hash"`
		Branch        string `json:"branch"`
		RepositoryURL string `json:"repository_url"`
	} `json:"git,omitempty"`
}

type codexTurnContext struct {
	TurnID string `json:"turn_id"`
	CWD    string `json:"cwd"`
	Model  string `json:"model"`
}

type codexResponseItem struct {
	Type    string          `json:"type"` // "message", "function_call", "function_call_output", "reasoning"
	Role    string          `json:"role,omitempty"`
	Name    string          `json:"name,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	// function_call fields
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	// function_call_output
	Output string `json:"output,omitempty"`
	// message phase
	Phase string `json:"phase,omitempty"`
}

type codexEventMsg struct {
	Type    string `json:"type"` // "user_message", "agent_message", "task_started", "task_complete", "token_count"
	Message string `json:"message,omitempty"`
}

func (p *CodexParser) parseJSONL(path string) ([]model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0), 10*1024*1024)

	var meta codexSessionMeta
	var agentModel string
	var messages []model.Message
	pendingToolCalls := make(map[string]*model.ToolCall) // call_id -> ToolCall

	for scanner.Scan() {
		var entry codexJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "session_meta":
			json.Unmarshal(entry.Payload, &meta)

		case "turn_context":
			var tc codexTurnContext
			if json.Unmarshal(entry.Payload, &tc) == nil && tc.Model != "" {
				agentModel = tc.Model
			}

		case "event_msg":
			var em codexEventMsg
			if json.Unmarshal(entry.Payload, &em) != nil {
				continue
			}
			ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

			switch em.Type {
			case "user_message":
				messages = append(messages, model.Message{
					Role:      "human",
					Content:   em.Message,
					Timestamp: ts,
				})
			case "agent_message":
				messages = append(messages, model.Message{
					Role:      "assistant",
					Content:   em.Message,
					Timestamp: ts,
					Model:     agentModel,
				})
			}

		case "response_item":
			var ri codexResponseItem
			if json.Unmarshal(entry.Payload, &ri) != nil {
				continue
			}
			ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

			switch ri.Type {
			case "function_call":
				tc := model.ToolCall{
					Tool:  ri.Name,
					Input: ri.Arguments,
				}
				// Attach to the last assistant message, or create one
				if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
					messages[len(messages)-1].ToolCalls = append(messages[len(messages)-1].ToolCalls, tc)
					pendingToolCalls[ri.CallID] = &messages[len(messages)-1].ToolCalls[len(messages[len(messages)-1].ToolCalls)-1]
				} else {
					// Create a placeholder assistant message
					messages = append(messages, model.Message{
						Role:      "assistant",
						Timestamp: ts,
						Model:     agentModel,
						ToolCalls: []model.ToolCall{tc},
					})
					pendingToolCalls[ri.CallID] = &messages[len(messages)-1].ToolCalls[0]
				}

			case "function_call_output":
				if tc, ok := pendingToolCalls[ri.CallID]; ok {
					tc.Output = ri.Output
					delete(pendingToolCalls, ri.CallID)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s: %w", path, err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	sessionID := meta.ID
	if sessionID == "" {
		// Derive from filename
		base := filepath.Base(path)
		sessionID = strings.TrimSuffix(strings.TrimPrefix(base, "rollout-"), ".jsonl")
	}

	cwd := meta.CWD
	gitBranch := ""
	if meta.Git != nil {
		gitBranch = meta.Git.Branch
	}

	session := model.Session{
		ID:        sessionID,
		Agent:     model.Agent{Tool: "codex_cli", Model: agentModel},
		Project:   filepath.Base(cwd),
		CWD:       cwd,
		GitBranch: gitBranch,
		StartedAt: messages[0].Timestamp,
		EndedAt:   messages[len(messages)-1].Timestamp,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}

// --- Legacy JSON format ---

type codexLegacySession struct {
	Session struct {
		Timestamp string `json:"timestamp"`
		ID        string `json:"id"`
	} `json:"session"`
	Items []codexLegacyItem `json:"items"`
}

type codexLegacyItem struct {
	Type    string          `json:"type"` // "message", "reasoning", "local_shell_call", "local_shell_call_output"
	Role    string          `json:"role,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	// local_shell_call
	Action *struct {
		Type    string   `json:"type"`
		Command []string `json:"command"`
	} `json:"action,omitempty"`
	CallID string `json:"call_id,omitempty"`
	// local_shell_call_output
	Output string `json:"output,omitempty"`
	Status string `json:"status,omitempty"`
}

func (p *CodexParser) parseJSON(path string) ([]model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ls codexLegacySession
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(ls.Items) == 0 {
		return nil, nil
	}

	var messages []model.Message
	pendingToolCalls := make(map[string]*model.ToolCall)

	sessionTime, _ := time.Parse(time.RFC3339Nano, ls.Session.Timestamp)

	for _, item := range ls.Items {
		switch item.Type {
		case "message":
			role := "human"
			if item.Role == "assistant" {
				role = "assistant"
			}
			// Parse content
			var contentBlocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			var contentStr string
			if json.Unmarshal(item.Content, &contentBlocks) == nil {
				var parts []string
				for _, b := range contentBlocks {
					if b.Text != "" {
						parts = append(parts, b.Text)
					}
				}
				contentStr = strings.Join(parts, "\n")
			}

			messages = append(messages, model.Message{
				Role:      role,
				Content:   contentStr,
				Timestamp: sessionTime, // legacy format doesn't have per-message timestamps
			})

		case "local_shell_call":
			cmdStr := ""
			if item.Action != nil {
				cmdStr = strings.Join(item.Action.Command, " ")
			}
			tc := model.ToolCall{
				Tool:  "shell",
				Input: cmdStr,
			}
			// Attach to last assistant message or create one
			if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
				messages[len(messages)-1].ToolCalls = append(messages[len(messages)-1].ToolCalls, tc)
				pendingToolCalls[item.CallID] = &messages[len(messages)-1].ToolCalls[len(messages[len(messages)-1].ToolCalls)-1]
			} else {
				messages = append(messages, model.Message{
					Role:      "assistant",
					Timestamp: sessionTime,
					ToolCalls: []model.ToolCall{tc},
				})
				pendingToolCalls[item.CallID] = &messages[len(messages)-1].ToolCalls[0]
			}

		case "local_shell_call_output":
			if tc, ok := pendingToolCalls[item.CallID]; ok {
				tc.Output = item.Output
				delete(pendingToolCalls, item.CallID)
			}
		}
	}

	if len(messages) == 0 {
		return nil, nil
	}

	session := model.Session{
		ID:        ls.Session.ID,
		Agent:     model.Agent{Tool: "codex_cli"},
		Project:   filepath.Base(path),
		StartedAt: sessionTime,
		EndedAt:   sessionTime,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}
