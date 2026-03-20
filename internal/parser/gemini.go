package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aitrace/internal/model"
)

// GeminiParser reads Gemini CLI session JSON files.
type GeminiParser struct{}

func (p *GeminiParser) Name() string { return "gemini_cli" }

func (p *GeminiParser) Detect(projectDir string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Gemini CLI stores logs in ~/.gemini/tmp/<projectHash>/chats/
	tmpDir := filepath.Join(home, ".gemini", "tmp")
	projectHashes, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, nil
	}

	var files []string
	for _, ph := range projectHashes {
		if !ph.IsDir() {
			continue
		}
		chatsDir := filepath.Join(tmpDir, ph.Name(), "chats")
		sessions, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}
		for _, s := range sessions {
			if strings.HasPrefix(s.Name(), "session-") && strings.HasSuffix(s.Name(), ".json") {
				files = append(files, filepath.Join(chatsDir, s.Name()))
			}
		}
	}
	return files, nil
}

// Raw JSON structures for Gemini CLI logs
type geminiSessionFile struct {
	SessionID   string          `json:"sessionId"`
	ProjectHash string          `json:"projectHash"`
	StartTime   string          `json:"startTime"`
	LastUpdated string          `json:"lastUpdated"`
	Messages    []geminiMessage `json:"messages"`
}

type geminiMessage struct {
	ID        string           `json:"id"`
	Timestamp string           `json:"timestamp"`
	Type      string           `json:"type"` // "user" | "gemini"
	Content   string           `json:"content"`
	Model     string           `json:"model"`
	ToolCalls []geminiToolCall  `json:"toolCalls"`
	Tokens    *geminiTokenInfo  `json:"tokens"`
}

type geminiToolCall struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Args          json.RawMessage `json:"args"`
	Result        []geminiResult  `json:"result"`
	Status        string          `json:"status"`
	Timestamp     string          `json:"timestamp"`
	DisplayName   string          `json:"displayName"`
	Description   string          `json:"description"`
}

type geminiResult struct {
	FunctionResponse *geminiFunctionResponse `json:"functionResponse"`
}

type geminiFunctionResponse struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type geminiTokenInfo struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

func (p *GeminiParser) Parse(path string) ([]model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sf geminiSessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}

	if len(sf.Messages) == 0 {
		return nil, nil
	}

	var messages []model.Message
	var agentModel string

	for _, gm := range sf.Messages {
		ts, _ := time.Parse(time.RFC3339Nano, gm.Timestamp)

		role := "human"
		if gm.Type == "gemini" {
			role = "assistant"
		}

		if gm.Model != "" {
			agentModel = gm.Model
		}

		var toolCalls []model.ToolCall
		if role == "assistant" {
			for _, tc := range gm.ToolCalls {
				output := ""
				if len(tc.Result) > 0 && tc.Result[0].FunctionResponse != nil {
					if resp := tc.Result[0].FunctionResponse.Response; resp != nil {
						if out, ok := resp["output"]; ok {
							if s, ok := out.(string); ok {
								output = s
							}
						}
					}
				}
				toolCalls = append(toolCalls, model.ToolCall{
					Tool:   tc.Name,
					Input:  string(tc.Args),
					Output: output,
				})
			}
		}

		messages = append(messages, model.Message{
			Role:      role,
			Content:   gm.Content,
			Timestamp: ts,
			ToolCalls: toolCalls,
		})
	}

	startTime, _ := time.Parse(time.RFC3339Nano, sf.StartTime)
	endTime, _ := time.Parse(time.RFC3339Nano, sf.LastUpdated)

	// Try to infer project directory from tool call arguments
	project := inferGeminiProject(sf.Messages)

	session := model.Session{
		ID:        sf.SessionID,
		Agent:     model.Agent{Tool: "gemini_cli", Model: agentModel},
		Project:   project,
		StartedAt: startTime,
		EndedAt:   endTime,
		Messages:  messages,
	}

	return []model.Session{session}, nil
}

func inferGeminiProject(messages []geminiMessage) string {
	// Try to find CWD from shell command tool calls
	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			if tc.Name == "run_shell_command" && len(tc.Result) > 0 {
				if fr := tc.Result[0].FunctionResponse; fr != nil {
					if resp := fr.Response; resp != nil {
						if dir, ok := resp["output"].(string); ok {
							// Look for "Directory: <path>" in output
							for _, line := range strings.Split(dir, "\n") {
								if strings.HasPrefix(line, "Directory: ") {
									d := strings.TrimPrefix(line, "Directory: ")
									if d != "(root)" && d != "" {
										return filepath.Base(d)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return "unknown"
}
