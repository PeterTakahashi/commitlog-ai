package sanitizer

import (
	"regexp"
	"strings"

	"github.com/anthropics/commitlog-ai/internal/model"
)

// Common API key / secret patterns
var secretPatterns = []*regexp.Regexp{
	// Generic long hex/base64 tokens (API keys, secret keys)
	// Matches: sk-..., pk-..., key-..., token-... prefixed strings
	regexp.MustCompile(`(?i)(sk|pk|api[_-]?key|secret[_-]?key|access[_-]?key|token|bearer)[-_]?[A-Za-z0-9/+_\-]{20,}`),

	// AWS keys
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),

	// Azure keys (hex strings of 32+ chars often used as keys)
	regexp.MustCompile(`(?i)(azure|subscription|endpoint)[_-]?(key|secret|token)\s*[:=]\s*["']?[A-Za-z0-9/+_\-]{16,}["']?`),

	// OpenAI keys
	regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`),

	// Anthropic keys
	regexp.MustCompile(`sk-ant-[A-Za-z0-9\-]{32,}`),

	// Google API keys
	regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),

	// GitHub tokens
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`),

	// Generic: long strings after common key-value patterns
	regexp.MustCompile(`(?i)(?:api_key|apikey|api-key|secret|password|passwd|token|auth|credential|private_key)\s*[:=]\s*["']?([A-Za-z0-9/+_\-]{20,})["']?`),

	// Bearer tokens in headers
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9/+_\-\.]{20,}`),

	// Connection strings
	regexp.MustCompile(`(?i)(?:mongodb|postgres|mysql|redis|amqp)://[^\s"']+@[^\s"']+`),
}

// MaskString replaces detected secrets in a string with "[MASKED]".
func MaskString(s string) string {
	for _, pat := range secretPatterns {
		s = pat.ReplaceAllStringFunc(s, func(match string) string {
			// Keep a short prefix for identification, mask the rest
			if len(match) <= 8 {
				return "[MASKED]"
			}
			// Find the sensitive part (after = or : or space)
			for _, sep := range []string{"= ", "=", ": ", ":"} {
				if idx := strings.Index(match, sep); idx >= 0 {
					prefix := match[:idx+len(sep)]
					rest := match[idx+len(sep):]
					rest = strings.TrimLeft(rest, `"'`)
					if len(rest) > 4 {
						return prefix + rest[:4] + "****[MASKED]"
					}
					return prefix + "[MASKED]"
				}
			}
			// For standalone tokens like sk-xxx, keep prefix
			if len(match) > 8 {
				return match[:8] + "****[MASKED]"
			}
			return "[MASKED]"
		})
	}
	return s
}

// SanitizeSessions masks secrets in all session data.
func SanitizeSessions(sessions []model.Session) []model.Session {
	result := make([]model.Session, len(sessions))
	for i, s := range sessions {
		result[i] = sanitizeSession(s)
	}
	return result
}

func sanitizeSession(s model.Session) model.Session {
	orig := s.Messages
	s.Messages = make([]model.Message, len(orig))
	for j, msg := range orig {
		s.Messages[j] = sanitizeMessage(msg)
	}
	return s
}

func sanitizeMessage(m model.Message) model.Message {
	m.Content = MaskString(m.Content)
	if len(m.ToolCalls) > 0 {
		origTC := m.ToolCalls
		m.ToolCalls = make([]model.ToolCall, len(origTC))
		for k, tc := range origTC {
			m.ToolCalls[k] = model.ToolCall{
				Tool:   tc.Tool,
				Input:  MaskString(tc.Input),
				Output: MaskString(tc.Output),
			}
		}
	}
	return m
}

// SanitizeTimeline masks secrets in all timeline data.
func SanitizeTimeline(timeline model.LinkedTimeline) model.LinkedTimeline {
	result := timeline
	result.Entries = make([]model.TimelineEntry, len(timeline.Entries))
	for i, e := range timeline.Entries {
		entry := e
		if entry.Session != nil {
			sanitized := sanitizeSession(*entry.Session)
			entry.Session = &sanitized
		}
		result.Entries[i] = entry
	}
	return result
}
