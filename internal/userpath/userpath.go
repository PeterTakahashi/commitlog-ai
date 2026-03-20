package userpath

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

// SanitizeEmail converts an email address into a safe directory name.
// alice@example.com → alice_at_example_com
func SanitizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	email = strings.ReplaceAll(email, "@", "_at_")
	email = strings.ReplaceAll(email, ".", "_")
	return email
}

// UserSessionsDir returns the per-user sessions directory path.
func UserSessionsDir(projectDir, email string) string {
	return filepath.Join(projectDir, ".commitlog-ai", "sessions", SanitizeEmail(email))
}

// UserSessionsPath returns the per-user sessions.json file path.
func UserSessionsPath(projectDir, email string) string {
	return filepath.Join(UserSessionsDir(projectDir, email), "sessions.json")
}

// ReadAllSessions reads all sessions from all per-user session files.
// Returns the merged sessions, the list of file paths read, and any error.
func ReadAllSessions(projectDir string) ([]model.Session, []string, error) {
	sessionsDir := filepath.Join(projectDir, ".commitlog-ai", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var allSessions []model.Session
	var filePaths []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessPath := filepath.Join(sessionsDir, entry.Name(), "sessions.json")
		data, err := os.ReadFile(sessPath)
		if err != nil {
			continue // skip users with no sessions file
		}

		var sessions []model.Session
		if err := json.Unmarshal(data, &sessions); err != nil {
			continue // skip corrupt files
		}

		allSessions = append(allSessions, sessions...)
		filePaths = append(filePaths, sessPath)
	}

	return allSessions, filePaths, nil
}

// AllSessionFiles returns paths of all per-user sessions.json files.
func AllSessionFiles(projectDir string) []string {
	sessionsDir := filepath.Join(projectDir, ".commitlog-ai", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessPath := filepath.Join(sessionsDir, entry.Name(), "sessions.json")
		if _, err := os.Stat(sessPath); err == nil {
			paths = append(paths, sessPath)
		}
	}
	return paths
}

// MigrateLegacy moves the old .commitlog-ai/sessions.json into the per-user directory.
// No-op if the legacy file doesn't exist.
func MigrateLegacy(projectDir, email string) error {
	legacyPath := filepath.Join(projectDir, ".commitlog-ai", "sessions.json")
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil // nothing to migrate
	}

	userDir := UserSessionsDir(projectDir, email)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("creating user sessions directory: %w", err)
	}

	destPath := UserSessionsPath(projectDir, email)

	// If the user already has a file, merge the legacy data into it
	if _, err := os.Stat(destPath); err == nil {
		// Read both files and merge
		legacyData, err := os.ReadFile(legacyPath)
		if err != nil {
			return err
		}
		existingData, err := os.ReadFile(destPath)
		if err != nil {
			return err
		}

		var legacySessions, existingSessions []model.Session
		json.Unmarshal(legacyData, &legacySessions)
		json.Unmarshal(existingData, &existingSessions)

		// Deduplicate by session ID
		seen := make(map[string]bool)
		for _, s := range existingSessions {
			seen[s.ID] = true
		}
		for _, s := range legacySessions {
			if !seen[s.ID] {
				existingSessions = append(existingSessions, s)
			}
		}

		merged, err := json.MarshalIndent(existingSessions, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, merged, 0644); err != nil {
			return err
		}
	} else {
		// Just move the file
		data, err := os.ReadFile(legacyPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}

	// Remove legacy file
	return os.Remove(legacyPath)
}
