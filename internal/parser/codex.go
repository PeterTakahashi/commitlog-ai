package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aitrace/internal/model"
)

// CodexParser reads Codex CLI JSONL session logs.
type CodexParser struct{}

func (p *CodexParser) Name() string { return "codex_cli" }

func (p *CodexParser) Detect(projectDir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sessionsDir := filepath.Join(home, ".codex", "sessions")
	var files []string
	err = filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil
	}
	return files, nil
}

// Raw JSONL entry types for Codex CLI
type codexEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"` // "session_meta", "response_item", "event_msg", "turn_context"
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID            string   `json:"id"`
	Timestamp     string   `json:"timestamp"`
	CWD           string   `json:"cwd"`
	CLIVersion    string   `json:"cli_version"`
	Source        string   `json:"source"`
	ModelProvider string   `json:"model_provider"`
	Git           *codexGit `json:"git"`
}

type codexGit struct {
	CommitHash    string `json:"commit_hash"`
	Branch        string `json:"branch"`
	RepositoryURL string `json:"repository_url"`
}

type codexResponseItem struct {
	Type      string              `json:"type"` // "message", "function_call", "function_call_output"
	Role      string              `json:"role"`
	Content   []codexContentBlock `json:"content"`
	Name      string              `json:"name"`      // function name
	Arguments string              `json:"arguments"`  // function args (JSON string)
	CallID    string              `json:"call_id"`
	Output    string              `json:"output"`     // function_call_output
}

type codexContentBlock struct {
	Type string `json:"type"` // "input_text", "output_text"
	Text string `json:"text"`
}

type codexEventMsg struct {
	Type    string `json:"type"` // "user_message", "agent_reasoning", "token_count"
	Message string `json:"message"`
	Text    string `json:"text"`
}

type codexTurnContext struct {
	CWD    string `json:"cwd"`
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

func (p *CodexParser) Parse(path string) ([]model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0), 10*1024*1024)

	var (
		sessionID string
		cwd       string
		gitBranch string
		agentModel string
		messages  []model.Message
		pendingFuncCalls = make(map[string]*model.ToolCall) // call_id -> ToolCall
	)

	for scanner.Scan() {
		var entry codexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

		switch entry.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(entry.Payload, &meta); err != nil {
				continue
			}
			sessionID = meta.ID
			cwd = meta.CWD
			if meta.Git != nil {
				gitBranch = meta.Git.Branch
			}

		case "turn_context":
			var tc codexTurnContext
			if err := json.Unmarshal(entry.Payload, &tc); err != nil {
				continue
			}
			if tc.Model != "" {
				agentModel = tc.Model
			}
			if tc.CWD != "" {
				cwd = tc.CWD
			}

		case "event_msg":
			var em codexEventMsg
			if err := json.Unmarshal(entry.Payload, &em); err != nil {
				continue
			}
			// Real user messages come from event_msg with type "user_message"
			if em.Type == "user_message" && em.Message != "" {
				messages = append(messages, model.Message{
					Role:      "human",
					Content:   em.Message,
					Timestamp: ts,
				})
			}

		case "response_item":
			var ri codexResponseItem
			if err := json.Unmarshal(entry.Payload, &ri); err != nil {
				continue
			}

			switch ri.Type {
			case "message":
				if ri.Role == "assistant" {
					var text string
					for _, block := range ri.Content {
						if block.Text != "" {
							text += block.Text
						}
					}
					if text != "" {
						messages = append(messages, model.Message{
							Role:      "assistant",
							Content:   text,
							Timestamp: ts,
						})
					}
				}
				// Skip role="user" messages (system prompts)

			case "function_call":
				tc := model.ToolCall{
					Tool:  ri.Name,
					Input: ri.Arguments,
				}
				// Attach to last assistant message or create one
				if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
					messages[len(messages)-1].ToolCalls = append(
						messages[len(messages)-1].ToolCalls, tc,
					)
					idx := len(messages[len(messages)-1].ToolCalls) - 1
					pendingFuncCalls[ri.CallID] = &messages[len(messages)-1].ToolCalls[idx]
				} else {
					// Create an assistant message for orphaned tool calls
					messages = append(messages, model.Message{
						Role:      "assistant",
						Timestamp: ts,
						ToolCalls: []model.ToolCall{tc},
					})
					pendingFuncCalls[ri.CallID] = &messages[len(messages)-1].ToolCalls[0]
				}

			case "function_call_output":
				if tc, ok := pendingFuncCalls[ri.CallID]; ok {
					tc.Output = ri.Output
					delete(pendingFuncCalls, ri.CallID)
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

	project := filepath.Base(cwd)
	if project == "" || project == "." {
		project = filepath.Base(path)
	}

	session := model.Session{
		ID:        sessionID,
		Agent:     model.Agent{Tool: "codex_cli", Model: agentModel},
		Project:   project,
		CWD:       cwd,
		GitBranch: gitBranch,
		StartedAt: messages[0].Timestamp,
		EndedAt:   messages[len(messages)-1].Timestamp,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}
