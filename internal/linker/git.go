package linker

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/PeterTakahashi/commitlog-ai/internal/model"
)

// GitClient wraps git CLI operations.
type GitClient struct {
	RepoDir string
}

// NewGitClient creates a client for the given repository directory.
func NewGitClient(repoDir string) *GitClient {
	return &GitClient{RepoDir: repoDir}
}

// GetCommits returns all commits in reverse chronological order.
func (g *GitClient) GetCommits() ([]model.GitCommit, error) {
	// Use %x00 as field separator and %x01 as record separator.
	// %B gives the full commit message (multi-line), so we can't use newline as record delimiter.
	cmd := exec.Command("git", "log", "--format=%H%x00%aI%x00%an%x00%ae%x00%B%x01")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	records := strings.Split(string(out), "\x01")
	var commits []model.GitCommit

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x00", 5)
		if len(parts) < 5 {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, parts[1])

		commits = append(commits, model.GitCommit{
			Hash:        parts[0],
			Timestamp:   ts,
			Author:      parts[2],
			AuthorEmail: parts[3],
			Message:     strings.TrimSpace(parts[4]),
		})
	}

	return commits, nil
}

// GetDiffStats fills in file change statistics for a commit.
func (g *GitClient) GetDiffStats(hash string) (filesChanged int, additions int, deletions int, changedFiles []string, err error) {
	cmd := exec.Command("git", "diff", "--numstat", hash+"^.."+hash)
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		// Might be the initial commit
		cmd = exec.Command("git", "diff", "--numstat", "--root", hash)
		cmd.Dir = g.RepoDir
		out, err = cmd.Output()
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf("git diff: %w", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		filesChanged++
		if a, err := strconv.Atoi(parts[0]); err == nil {
			additions += a
		}
		if d, err := strconv.Atoi(parts[1]); err == nil {
			deletions += d
		}
		changedFiles = append(changedFiles, parts[2])
	}

	return filesChanged, additions, deletions, changedFiles, nil
}

// GetDiff returns the full diff for a commit.
func (g *GitClient) GetDiff(hash string) (string, error) {
	cmd := exec.Command("git", "diff", hash+"^.."+hash)
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "diff", "--root", hash)
		cmd.Dir = g.RepoDir
		out, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git diff: %w", err)
		}
	}
	return string(out), nil
}

// GetBranches returns all local and remote branch names.
func (g *GitClient) GetBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "-a", "--format=%(refname:short)")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "origin/HEAD" {
			continue
		}
		branches = append(branches, line)
	}
	return branches, nil
}

// GetBranchCommitHashes returns the set of commit hashes reachable from a branch.
func (g *GitClient) GetBranchCommitHashes(branch string) (map[string]bool, error) {
	cmd := exec.Command("git", "log", branch, "--format=%H")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", branch, err)
	}

	hashes := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			hashes[line] = true
		}
	}
	return hashes, nil
}

// GetUserEmail returns the git user.email for the repository.
func (g *GitClient) GetUserEmail() (string, error) {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git config user.email: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetHead returns the current HEAD commit hash.
func (g *GitClient) GetHead() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetRepoRoot returns the root directory of the git repository.
func (g *GitClient) GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = g.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
