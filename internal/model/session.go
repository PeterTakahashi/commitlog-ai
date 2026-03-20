package model

import "time"

// Session represents one conversation session with an AI agent.
type Session struct {
	ID        string    `json:"id"`
	Agent     Agent     `json:"agent"`
	Project   string    `json:"project"`
	CWD       string    `json:"cwd"`
	GitBranch string    `json:"git_branch,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Messages  []Message `json:"messages"`
}

// Agent identifies the AI tool and model used.
type Agent struct {
	Tool  string `json:"tool"`  // "claude_code" | "gemini_cli" | "codex_cli"
	Model string `json:"model"` // e.g. "claude-opus-4-6", "gemini-2.5-pro"
}

// Message is one turn in a conversation.
type Message struct {
	Role      string     `json:"role"`      // "human" | "assistant"
	Content   string     `json:"content"`
	Timestamp time.Time  `json:"timestamp"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall records a tool invocation by the assistant.
type ToolCall struct {
	Tool   string `json:"tool"`
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
}
